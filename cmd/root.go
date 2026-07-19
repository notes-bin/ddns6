// Package cmd 提供 ddns6 CLI 的命令定义和注册。
//
// 命令结构：
//
//	ddns6
//	├── init      生成 ~/.ddns6/config.yaml 配置文件模板
//	├── version   显示版本信息
//	├── help      查看命令帮助（cobra 内置）
//	└── run       运行 DDNS 服务
//	    ├── tencent      腾讯云 DNSPod (API v3)
//	    ├── cloudflare   Cloudflare DNS
//	    ├── alicloud     阿里云 DNS
//	    ├── godaddy      GoDaddy DNS
//	    ├── huaweicloud  华为云 DNS
//	    ├── duckdns      DuckDNS (免费 DDNS)
//	    ├── noip         No-IP (经典 DDNS)
//	    ├── he           Hurricane Electric (免费 DNS 托管)
//	    ├── dynv6        Dynv6 (免费 IPv6 DDNS)
//	    ├── porkbun      Porkbun DNS API
//	    ├── digitalocean DigitalOcean DNS API
//	    ├── baiducloud   百度云 DNS
//	    └── dnspod       DNSPod (旧版 API)
//
// 使用方式：
//   - 临时运行: ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy
//   - 长期运行: ddns6 init → 编辑 ~/.ddns6/config.yaml → ddns6 run
//   - 查看帮助: ddns6 run tencent --help
package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
)

var (
	Version = "dev"
	Commit  = "none"
	buildAt = "unknown"
)

var log = slog.With("module", "cmd")

// usageTemplate 中文版 cobra 使用信息模板。
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

// rootCmd 根命令，设置全局参数和子命令结构。
var rootCmd = &cobra.Command{
	Use:   "ddns6",
	Short: "IPv6 动态域名解析（DDNS）工具",
	SilenceErrors: true,               // 未知命令等错误由 Execute 统一处理，不打印双重日志
	SilenceUsage: true,                // 不重复打印用法提示，由 Execute 自行决定
	Long: `DDNS6 — 动态域名解析工具，自动将本机 IPv6 地址更新到 DNS 记录。

自动检测本地 IPv6 地址变化，实时更新到 DNS 服务商的 AAAA 记录。

触发方式（自动选择）:
  Linux   通过 Netlink 监听内核地址变化事件，实时触发（10 秒防抖）
  其他    定时轮询（默认间隔 5 分钟，可通过 --interval 调整）

支持的 DNS 服务商（13 个）:
  tencent, cloudflare, alicloud, godaddy, huaweicloud,
  duckdns, noip, he, dynv6, porkbun, digitalocean, baiducloud, dnspod

快速开始:
  1. 临时测试:  ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy
  2. 长期运行:  ddns6 init → 编辑 ~/.ddns6/config.yaml → ddns6 run
  3. 查看详情:  ddns6 run --help`,
			PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// version 命令不需要初始化日志
		if cmd.Name() == "version" || cmd.Name() == "init" {
			return
		}

		logFile, err := os.OpenFile("ddns6.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Error("failed to create log file", "err", err)
			os.Exit(1)
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
		slog.SetDefault(slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stderr, logFile), opts)))
	},
}

// initCmd 生成 ~/.ddns6/config.yaml 配置文件模板。
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "生成 ~/.ddns6/config.yaml 配置文件模板",
	Long: `生成 DDNS6 配置文件模板。

在用户主目录下创建 ~/.ddns6/config.yaml 文件，包含所有配置字段的
详细说明和示例。编辑此文件后运行 ddns6 run 即可启动服务。

使用配置文件后，无需每次运行时重复输入参数。

支持通过 --domain、--subdomain 等参数预填配置值，生成后无需
再用编辑器修改对应字段。

示例:
  ddns6 init                          生成配置文件模板，手动编辑
  ddns6 init --domain example.com --subdomain www --subdomain @
                                      生成模板并预填域名和子域名
  vim ~/.ddns6/config.yaml           编辑配置（填入运营商和凭证）
  ddns6 run                           从配置文件读取并运行`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Generating DDNS6 configuration file...")
			domain, _ := cmd.Flags().GetString("domain")
			subdomains, _ := cmd.Flags().GetStringArray("subdomain")
			ttl, _ := cmd.Flags().GetInt("ttl")
			interval, _ := cmd.Flags().GetString("interval")
			iface, _ := cmd.Flags().GetString("interface")

			params := config.InitParams{
				Domain:     domain,
				Subdomains: subdomains,
				TTL:        ttl,
				Interval:   interval,
				Interface:  iface,
			}


		if err := config.Generate(params); err != nil {
			return fmt.Errorf("failed to generate config: %w", err)
		}
		return nil
	},
}

// runCmd 运行 DDNS 服务（父命令，子命令为各运营商）。
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
  2. ~/.ddns6/config.yaml 配置文件

支持的运营商:
  tencent      腾讯云 DNSPod (API v3)
  cloudflare   Cloudflare DNS
  alicloud     阿里云 DNS
  godaddy      GoDaddy DNS
  huaweicloud  华为云 DNS
  duckdns      DuckDNS (免费 DDNS 服务)
  noip         No-IP (经典 DDNS 服务)
  he           Hurricane Electric (免费 DNS 托管)
  dynv6        Dynv6 (免费 IPv6 DDNS)
  porkbun      Porkbun DNS API
  digitalocean DigitalOcean DNS API
  baiducloud   百度云 DNS
  dnspod       DNSPod (旧版 API)

示例:
  # 临时运行（单子域名）
  ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

  # 临时运行（多子域名）
  ddns6 run cloudflare --domain example.com --subdomain www --subdomain @ --api-token xxx

  # 指定网络接口
  ddns6 run duckdns --domain example.com --interface ppp0 --token xxx

  # 调试模式
  ddns6 run --debug tencent --domain example.com --secret-id xxx --secret-key yyy

  # 使用配置文件
  ddns6 init
  vim ~/.ddns6/config.yaml
  ddns6 run`,
	// 不指定 provider 子命令时走配置文件模式
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithConfig(cmd)
	},
}

// runWithConfig 从 ~/.ddns6/config.yaml 加载配置并启动 DDNS 服务。
//
// 仅当 ddns6 run 未指定 provider 子命令时调用。
func runWithConfig(cmd *cobra.Command) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cannot load config: %w\n\nUse 'ddns6 init' to create a config file, or specify a provider: ddns6 run <provider> --help", err)
	}

	return startServiceFromConfig(cfg, cmd)
}

// versionCmd 显示版本信息。
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long:  `显示 ddns6 的版本、Git 提交和构建时间信息。`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s\nCommit: %s\nbuildAt: %s\n", Version, Commit, buildAt)
	},
}

// persistentFlag 持久化 flag 定义
var persistentFlags = []struct {
	name         string
	flagType     string
	defaultValue interface{}
	usage        string
}{
	{"debug", "bool", false, "启用调试日志（含源码位置）"},
	{"interval", "duration", 5 * time.Minute, "非 Linux 平台的轮询间隔（默认 5m，如 --interval 10m）"},
	{"domain", "string", "", "要更新的域名（如 example.com）"},
	{"subdomain", "stringArray", []string{"@"}, "子域名名称，可多次指定（默认 @，如 --subdomain www --subdomain @）"},
	{"ttl", "int", 600, "DNS 记录 TTL，单位秒（默认 600）"},
	{"interface", "string", "", "监听的网络接口（仅 Linux Netlink 模式，如 --interface ppp0）"},
}

// initRootCmd 初始化根命令，注册所有 flag 和子命令。
func initRootCmd() {
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(usageTemplate)

	// 覆盖 cobra 内置命令和参数的中文显示
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	// 自定义 help flag 中文描述
	f := rootCmd.PersistentFlags().Lookup("help")
	if f != nil {
		f.Usage = "显示帮助信息"
	}
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "查看命令帮助",
		Long:  "查看 ddns6 及其子命令的帮助信息。",
	})

	// 注册全局持久化参数
	for _, f := range persistentFlags {
		switch f.name {
		case "debug":
			rootCmd.PersistentFlags().Bool(f.name, f.defaultValue.(bool), f.usage)
		case "interval":
			rootCmd.PersistentFlags().Duration(f.name, f.defaultValue.(time.Duration), f.usage)
		case "domain":
			rootCmd.PersistentFlags().String(f.name, f.defaultValue.(string), f.usage)
		case "subdomain":
			rootCmd.PersistentFlags().StringArray(f.name, f.defaultValue.([]string), f.usage)
		case "ttl":
			rootCmd.PersistentFlags().Int(f.name, f.defaultValue.(int), f.usage)
		case "interface":
			rootCmd.PersistentFlags().String(f.name, f.defaultValue.(string), f.usage)
		}
	}

	// 注册子命令
	// init 子命令的本地参数（预填值到配置文件）
	initCmd.Flags().String("domain", "", "根域名, 预填入配置文件")
	initCmd.Flags().StringArray("subdomain", nil, "子域名列表, 可多次指定, 预填入配置文件")
	initCmd.Flags().Int("ttl", 0, "DNS 记录 TTL, 单位秒, 预填入配置文件")
	initCmd.Flags().String("interval", "", "轮询间隔, 如 10m, 预填入配置文件")
	initCmd.Flags().String("interface", "", "网络接口, 预填入配置文件")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(runCmd)

	// 数据驱动注册所有运营商命令
	registerProviders()
}

// Execute 是 CLI 入口，由 main.go 调用。
func Execute() error {
	initRootCmd()
	if err := rootCmd.Execute(); err != nil {
		// 未知子命令（如 ddns6 config）友好提示后 graceful exit，不视为错误
		if strings.Contains(err.Error(), "unknown command") {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return nil
		}
		return fmt.Errorf("Command failed: %w", err)
	}
	return nil
}
