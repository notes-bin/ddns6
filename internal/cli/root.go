// Package cli 实现 ddns6 的 CLI 命令树与运营商注册。
//
// 对外入口为 Execute。子命令覆盖配置初始化、连通性检查、DDNS 运行、记录查询/删除，
// 以及 23 家 DNS 运营商的认证参数工厂（见 providers.go）。
//
// 命令结构：
//
//	ddns6
//	├── init      生成 ~/.ddns6/config.yaml 配置文件模板
//	├── version   显示版本信息
//	├── help      查看命令帮助（cobra 内置）
//	├── check     验证配置和 API 连通性
//	├── completion 生成 Shell 自动补全脚本
//	├── run       运行 DDNS 服务
//	│   ├── tencent      腾讯云 DNSPod (API v3)
//	│   ├── cloudflare   Cloudflare DNS
//	│   ├── alicloud     阿里云 DNS
//	│   ├── godaddy      GoDaddy DNS
//	│   ├── huaweicloud  华为云 DNS
//	│   ├── duckdns      DuckDNS (免费 DDNS)
//	│   ├── noip         No-IP (经典 DDNS)
//	│   ├── he           Hurricane Electric (免费 DNS 托管)
//	│   ├── dynv6        Dynv6 (免费 IPv6 DDNS)
//	│   ├── porkbun      Porkbun DNS API
//	│   ├── digitalocean DigitalOcean DNS API
//	│   ├── baiducloud   百度云 DNS
//	│   ├── dnspod       DNSPod (旧版 API)
//	│   ├── desec        deSEC.io DNS
//	│   ├── linode       Linode (Akamai) DNS
//	│   ├── namesilo     NameSilo DNS
//	│   ├── ionos        IONOS DNS
//	│   ├── hetzner      Hetzner Cloud DNS
//	│   ├── aws          AWS Route 53
//	│   ├── gcloud       Google Cloud DNS
//	│   ├── azure        Azure DNS
//	│   ├── namecheap    Namecheap DNS
//	│   └── dpi          DNSPod.com 国际版
//	├── list                 列出可用 DNS 运营商
//	├── records [provider]   列出 DNS 记录
//	└── clean   [provider]   删除 DNS 记录
//
// 使用方式：
//   - 临时运行: ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy
//   - 长期运行: ddns6 init tencent --domain example.com --secret-id xxx --secret-key yyy -> ddns6 run
//   - 查看帮助: ddns6 run tencent --help
package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
)

// Version 是构建时经 -ldflags 注入的版本号；未注入时为 "dev"。
var Version = "dev"

// Commit 是构建时经 -ldflags 注入的 Git 提交标识；未注入时为 "none"。
var Commit = "none"

// buildAt 保存构建时间字符串；未注入时为 "unknown"。
var buildAt = "unknown"

// usageTemplate 用中文替换 cobra 默认英文 Usage，统一帮助输出语言。
const usageTemplate = `使用方式:

  {{.UseLine}}

{{if .HasAvailableSubCommands}}可用命令:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

{{end}}{{if .HasAvailableLocalFlags}}选项:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

全局参数:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

使用 "{{.CommandPath}} [command] --help" 查看子命令详细帮助。{{end}}
`

// rootCmd 是 CLI 根命令，挂载全局 flag 与全部子命令。
var rootCmd = &cobra.Command{

	Use:           "ddns6",
	Short:         "IPv6 动态域名解析（DDNS）工具",
	SilenceErrors: true, // 未知命令等错误由 Execute 统一处理，避免双重日志
	SilenceUsage:  true, // 用法提示由 Execute 自行决定，避免重复输出
	Long: `DDNS6 - 动态域名解析工具，自动将本机 IPv6 地址更新到 DNS 记录。

自动检测本地 IPv6 地址变化，实时更新到 DNS 服务商的 AAAA 记录。

触发方式（自动选择）:
  Linux   通过 Netlink 监听内核地址变化事件，实时触发（10 秒防抖）
  其他    定时轮询（默认间隔 5 分钟，可通过 --interval 调整）

支持的 DNS 服务商: 运行 ddns6 list 查看完整列表与能力说明。

快速开始:
  1. 临时测试:  ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy
  2. 配置文件:  ddns6 init tencent --domain example.com --secret-id xxx --secret-key yyy -> ddns6 run
  3. 查看详情:  ddns6 run --help`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// -V/--version 须在日志初始化前拦截，否则会先打开日志文件再退出
		if showV, _ := cmd.Flags().GetBool("version"); showV {
			printVersion()
			os.Exit(0)
		}

		// 仅根命令自身（无子命令）时跳过日志，便于直接显示帮助
		if cmd.Parent() == nil {
			return
		}

		// version / init / list 只做本地输出，无需落盘日志
		if cmd.Name() == "version" || cmd.Name() == "init" || cmd.Name() == "list" {
			return
		}

		logFile := getString(cmd, "log-file")

		var writers []io.Writer
		writers = append(writers, os.Stderr)
		if logFile != "" {
			lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
			if err != nil {
				slog.Error("failed to create log file", "err", err, "module", "cmd")
				os.Exit(1)
			}
			writers = append(writers, lf)
		}

		opts := new(slog.HandlerOptions)

		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			opts.Level = slog.LevelDebug
			opts.AddSource = true
			opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.SourceKey {
					if source, ok := a.Value.Any().(*slog.Source); ok {
						source.File = filepath.Base(source.File)
					}
				}
				return a
			}
		}
		slog.SetDefault(slog.New(slog.NewJSONHandler(io.MultiWriter(writers...), opts)))
	},
	// 空 Run 让根命令可执行，否则 cobra 会跳过 PersistentPreRun，导致 -V 无响应；
	// -V 已在 PersistentPreRun 中退出，此处仅在无参时显示帮助。
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// initCmd 生成 ~/.ddns6/config.yaml 模板，并可用 provider 与 flag 预填字段。
var initCmd = &cobra.Command{

	Use:   "init [provider]",
	Short: "生成 ~/.ddns6/config.yaml 配置文件模板",
	Long: `生成 DDNS6 配置文件模板。

在用户主目录下创建 ~/.ddns6/config.yaml 文件，包含所有配置字段的
详细说明和示例。编辑此文件后运行 ddns6 run 即可启动服务。

使用配置文件后，无需每次运行时重复输入参数。

支持通过 --domain、--subdomain 等参数预填配置值。指定 provider
名称和相应认证参数可直接生成完整配置，无需手动编辑。

示例:
  ddns6 init                          生成配置文件模板，手动编辑
  ddns6 init --domain example.com --subdomain www --subdomain @
                                      生成模板并预填域名和子域名
  ddns6 init tencent --domain example.com --subdomain www \
          --secret-id xxx --secret-key yyy
                                      生成完整配置（含 provider 和 auth）
  vim ~/.ddns6/config.yaml           编辑配置（填入运营商和凭证）
  ddns6 run                           从配置文件读取并运行`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("generating DDNS6 configuration file...")

		domain, err := cmd.Flags().GetString("domain")
		if err != nil {
			return fmt.Errorf("invalid --domain flag: %w", err)
		}
		subdomains, err := cmd.Flags().GetStringArray("subdomain")
		if err != nil {
			return fmt.Errorf("invalid --subdomain flag: %w", err)
		}
		ttl, err := cmd.Flags().GetInt("ttl")
		if err != nil {
			return fmt.Errorf("invalid --ttl flag: %w", err)
		}
		interval, err := cmd.Flags().GetString("interval")
		if err != nil {
			return fmt.Errorf("invalid --interval flag: %w", err)
		}
		iface, err := cmd.Flags().GetString("interface")
		if err != nil {
			return fmt.Errorf("invalid --interface flag: %w", err)
		}

		params := config.InitParams{
			Domain:     domain,
			Subdomains: subdomains,
			TTL:        ttl,
			Interval:   interval,
			Interface:  iface,
		}

		// 指定 provider 时收集非空认证 flag 写入 Auth（YAML 键用下划线）
		if len(args) > 0 {

			provider := args[0]
			params.Provider = provider

			auth := make(map[string]string)
			for _, p := range providerFactories {
				if p.name == provider {
					for _, f := range p.flags {
						if v := getString(cmd, f.name); v != "" {
							auth[strings.ReplaceAll(f.name, "-", "_")] = v
						}
					}
					break
				}
			}
			if len(auth) > 0 {
				params.Auth = auth
			}
		}

		if err := config.Generate(params); err != nil {
			return fmt.Errorf("failed to generate config: %w", err)
		}
		return nil
	},
}

// runCmd 是 DDNS 服务父命令；未指定 provider 子命令时改走配置文件模式。
var runCmd = &cobra.Command{

	Use:   "run [provider]",
	Short: "运行 DDNS 更新服务",
	Long: `启动 DDNS 服务，持续监听 IPv6 地址变化并更新 DNS 记录。

不指定 provider 子命令时，尝试从 ~/.ddns6/config.yaml 读取配置。
指定 provider 则使用命令行参数直接运行。

触发机制:
  Linux   内核 Netlink 事件驱动。检测到新的全局单播 IPv6 地址后，
          等待 10 秒防抖（应对 PPPoE 重拨等不稳定场景），再执行更新。
  macOS/Windows  定时轮询（默认 5 分钟，通过 --interval 调整）。

多子域名支持:
  可通过多次 --subdomain 指定多个子域名，一次命令更新所有子域名。
  例如: --subdomain www --subdomain @ --subdomain api

配置优先级（从高到低）:
  1. 命令行参数（最高）
  2. 环境变量 DDNS6_*（如 DDNS6_DOMAIN、DDNS6_SUBDOMAIN）
  3. ~/.ddns6/config.yaml 配置文件

支持的运营商: 运行 ddns6 list 查看完整列表与能力说明。

示例:
  # 临时运行（单子域名）
  ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

  # 临时运行（多子域名）
  ddns6 run cloudflare --domain example.com --subdomain www --subdomain @ --api-token xxx

  # 指定网络接口
  ddns6 run duckdns --domain example.com --interface ppp0 --token xxx

  # 调试模式
  ddns6 run --debug tencent --domain example.com --secret-id xxx --secret-key yyy

  # 使用环境变量
  DDNS6_DOMAIN=example.com DDNS6_SUBDOMAIN=www ddns6 run tencent --secret-id xxx --secret-key yyy

  # 使用配置文件
  ddns6 init
  vim ~/.ddns6/config.yaml
  ddns6 run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// "run help" 视为求助，避免误入配置文件加载路径
		if len(args) > 0 && args[0] == "help" {
			return cmd.Help()
		}
		return runWithConfig(cmd, "run", runServiceFromConfigHandler)
	},
}

// versionCmd 输出构建版本信息（Version、Commit、BuildAt）。
var versionCmd = &cobra.Command{

	Use:   "version",
	Short: "显示版本信息",
	Long:  `显示 ddns6 的版本、Git 提交和构建时间信息。`,
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}

// printVersion 将 Version、Commit、BuildAt 格式化为三行写到标准输出。
func printVersion() {
	fmt.Printf("Version: %s\nCommit:  %s\nBuildAt: %s\n", Version, Commit, buildAt)
}

// envFlagBindings 定义持久化 flag 与 DDNS6_* 环境变量的对应关系，供 applyEnvOverrides 使用。
var envFlagBindings = []struct {
	name     string
	flagType string
	envName  string
}{
	{"debug", "bool", "DDNS6_DEBUG"},
	{"interval", "duration", "DDNS6_INTERVAL"},
	{"domain", "string", "DDNS6_DOMAIN"},
	{"subdomain", "stringArray", "DDNS6_SUBDOMAIN"},
	{"ttl", "int", "DDNS6_TTL"},
	{"interface", "string", "DDNS6_INTERFACE"},
	{"log-file", "string", "DDNS6_LOG_FILE"},
	{"metrics-addr", "string", "DDNS6_METRICS_ADDR"},
}

// initRootCmd 保证根命令只初始化一次，避免重复注册子命令。
var initRootCmd = sync.OnceFunc(doInitRootCmd)

// doInitRootCmd 注册全局 flag、子命令与运营商；由 initRootCmd 保证只执行一次。
func doInitRootCmd() {

	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(usageTemplate)

	rootCmd.CompletionOptions.HiddenDefaultCmd = false
	f := rootCmd.PersistentFlags().Lookup("help")
	if f != nil {
		f.Usage = "显示帮助信息"
	}
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "查看命令帮助",
		Long:  "查看 ddns6 及其子命令的帮助信息。",
	})

	rootCmd.PersistentFlags().BoolP("version", "V", false, "显示版本信息（版本号、Git 提交、构建时间）")

	pf := rootCmd.PersistentFlags()
	pf.Bool("debug", false, "启用调试日志（含源码位置）")
	pf.Duration("interval", 5*time.Minute, "非 Linux 平台的轮询间隔（默认 5m，如 --interval 10m）")
	pf.String("domain", "", "要更新的域名（如 example.com）")
	pf.StringArray("subdomain", []string{"@"}, "子域名名称，可多次指定（默认 @，如 --subdomain www --subdomain @）")
	pf.Int("ttl", 600, "DNS 记录 TTL，单位秒（默认 600）")
	pf.String("interface", "", "监听的网络接口（仅 Linux Netlink 模式，如 --interface ppp0）")
	pf.String("log-file", "ddns6.log", "日志文件路径，设为空字符串仅输出到 stderr")
	pf.String("metrics-addr", "", "可选 Prometheus /metrics 监听地址（仅 loopback，如 127.0.0.1:9090；空则禁用）")

	// 环境变量仅覆盖尚未被命令行显式设置的默认值
	applyEnvOverrides()

	rootCmd.AddCommand(&cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "生成 Shell 自动补全脚本",
		Long: `生成 Shell 自动补全脚本。

将输出添加到对应的 Shell 配置文件中即可启用自动补全：

  # Bash
  ddns6 completion bash > /etc/bash_completion.d/ddns6

  # Zsh
  ddns6 completion zsh > "${fpath[1]}/_ddns6"

  # Fish
  ddns6 completion fish > ~/.config/fish/completions/ddns6.fish

  # PowerShell
  ddns6 completion powershell > ddns6.ps1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", args[0])
			}
		},
	})

	// init 使用本地同名 flag，与全局 persistent 隔离，避免污染运行时默认值
	initCmd.Flags().String("domain", "", "根域名, 预填入配置文件")
	initCmd.Flags().StringArray("subdomain", nil, "子域名列表, 可多次指定, 预填入配置文件")
	initCmd.Flags().Int("ttl", 0, "DNS 记录 TTL, 单位秒, 预填入配置文件")
	initCmd.Flags().String("interval", "", "轮询间隔, 如 10m, 预填入配置文件")
	initCmd.Flags().String("interface", "", "网络接口, 预填入配置文件")

	// 合并各运营商认证 flag；同名只注册一次，避免 cobra 重复定义报错
	seenInitFlag := make(map[string]bool)

	for _, p := range providerFactories {
		for _, f := range p.flags {
			if !seenInitFlag[f.name] {
				seenInitFlag[f.name] = true
				initCmd.Flags().String(f.name, "", f.usage)
			}
		}
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(recordsCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(checkCmd)

	registerProviders()
	registerRecordsCommands()
	registerCleanCommands()
}

// applyEnvOverrides 在 flag 未被命令行显式设置时，用 DDNS6_* 环境变量覆盖其默认值。
func applyEnvOverrides() {

	for _, f := range envFlagBindings {
		val, ok := os.LookupEnv(f.envName)
		if !ok {
			continue
		}
		flag := rootCmd.PersistentFlags().Lookup(f.name)
		if flag == nil || flag.Changed {
			continue
		}
		switch f.flagType {
		case "string", "duration", "stringArray":
			rootCmd.PersistentFlags().Set(f.name, val)
		case "int":
			if _, err := strconv.Atoi(val); err == nil {
				rootCmd.PersistentFlags().Set(f.name, val)
			}
		case "bool":
			switch val {
			case "true", "1":
				rootCmd.PersistentFlags().Set(f.name, "true")
			case "false", "0":
				rootCmd.PersistentFlags().Set(f.name, "false")
			}
		}
	}
}

// Execute 启动 CLI：初始化根命令并以可取消的 context 执行。
//
// 对未知命令或无效 flag，打印错误与帮助后仍返回 error（由 main 以非 0 退出）；
// 其它错误向 stderr 打印一次后返回。错误文案已在此处输出，main 只负责 exit code。
// 收到 SIGINT/SIGTERM 时取消 context，以中止进行中的 API 请求。
func Execute() error {
	initRootCmd()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		errStr := err.Error()
		// 仅匹配 cobra 的未知命令/非法 flag，避免误伤业务错误（如 unknown provider）
		if strings.Contains(errStr, "unknown command") ||
			strings.Contains(errStr, "unknown flag") ||
			strings.Contains(errStr, "unknown shorthand flag") ||
			strings.Contains(errStr, "invalid argument") && strings.Contains(errStr, "flag") {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			_ = rootCmd.Help()
			return fmt.Errorf("command failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}
