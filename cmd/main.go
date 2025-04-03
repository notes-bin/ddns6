package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/internal/domain"
	"github.com/notes-bin/ddns6/internal/iputil"
	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
	"github.com/notes-bin/ddns6/internal/providers/tencent"
	"github.com/notes-bin/ddns6/internal/scheduler"
	"github.com/notes-bin/ddns6/utils/logging"
)

// 全局版本信息变量
var (
	Version = "dev"     // 程序版本号，默认dev
	Commit  = "none"    // Git提交哈希，默认none
	buildAt = "unknown" // 构建时间，默认unknown
)

// command 结构体定义了子命令的基本信息
type command struct {
	flagSet     *flag.FlagSet             // 子命令的flag集合
	run         func(args []string) error // 子命令的执行函数
	description string                    // 子命令的描述信息
}

func main() {
	// 初始化所有子命令
	subcommands := map[string]command{
		"run":    runCmd(),    // 运行DDNS服务
		"config": configCmd(), // 生成配置文件
		"stop":   stopCmd(),   // 停止服务
	}

	// 处理顶级flags
	mainFlagSet := flag.NewFlagSet("ddns6", flag.ExitOnError)
	versionFlag := mainFlagSet.Bool("version", false, "Print version and exit")
	debugFlag := mainFlagSet.Bool("debug", false, "Enable debug logging")

	// 自定义usage信息
	mainFlagSet.Usage = func() {
		fmt.Println("Usage: ddns6 [options] <command> [command options]")
		fmt.Println("Dynamic DNS update tool for IPv6 addresses")

		fmt.Println("\nGlobal options:")
		mainFlagSet.PrintDefaults()

		fmt.Println("\nCommands:")
		for name, cmd := range subcommands {
			fmt.Printf("  %-10s%s\n", name, cmd.description)
		}
	}

	// 解析顶级标志
	mainFlagSet.Parse(os.Args[1:])

	if *versionFlag {
		fmt.Printf("Version: %s\nCommit: %s\nbuildAt: %s\n", Version, Commit, buildAt)
		return
	}

	// 配置日志
	// 在 main 函数中添加
	if *debugFlag {
		logFile, err := os.OpenFile("ddns6.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			slog.Error("创建日志文件失败", "err", err)
			os.Exit(1)
		}
		defer logFile.Close()
		logging.SetLogger(*debugFlag, io.MultiWriter(os.Stderr, logFile))
	} else {
		logging.SetLogger(*debugFlag, os.Stderr)
	}

	// 处理子命令
	args := mainFlagSet.Args()
	if len(args) == 0 {
		mainFlagSet.Usage()
		os.Exit(1)
	}

	cmd, ok := subcommands[args[0]]
	if !ok {
		fmt.Printf("Unknown command: %s\n", args[0])
		mainFlagSet.Usage()
		os.Exit(1)
	}

	if err := cmd.flagSet.Parse(args[1:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.run(cmd.flagSet.Args()); err != nil {
		slog.Error("Command failed", "err", err)
		os.Exit(1)
	}
}

// runCmd 定义并返回run子命令
func runCmd() command {
	flagSet := flag.NewFlagSet("run", flag.ExitOnError)
	interval := flagSet.Duration("interval", 5*time.Minute, "Update interval")
	service := flagSet.String("service", "tencent", "DNS service provider (tencent|cloudflare)")

	return command{
		flagSet:     flagSet,
		description: "Run the DDNS update service",
		run: func(args []string) error {
			// 初始化域名配置
			ddns := &domain.Domain{
				Type:      "AAAA", // IPv6记录类型
				Domain:    os.Getenv("DOMAIN"),
				SubDomain: getEnvWithDefault("SUBDOMAIN", "@"),
			}

			// 初始化DNS提供商
			task, err := createProvider(*service)
			if err != nil {
				return fmt.Errorf("创建DNS提供商失败: %w", err)
			}

			// 创建调度器
			sched := scheduler.New()
			defer sched.GracefulShutdown()

			// 定义任务函数
			taskFunc := func() error {
				return ddns.UpdateRecord(context.Background(), iputil.NewMultiProvider(), task)
			}

			// 首次更新记录
			if err := taskFunc(); err != nil {
				return fmt.Errorf("首次更新记录失败: %w", err)
			}

			// 定时任务
			// 定义一个包装函数，将 func() error 转换为 scheduler.TaskFunc 类型
			taskWrapper := func(ctx context.Context) error {
				return taskFunc()
			}

			if err := sched.AddJob("ddns_update", *interval, taskWrapper); err != nil {
				return fmt.Errorf("创建定时任务失败: %w", err)
			}

			slog.Info("ddns6 启动成功", "pid", os.Getpid())

			// 信号处理
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			<-sigCh
			slog.Info("ddns6 退出成功")
			return nil
		},
	}
}

// configCmd 定义并返回config子命令
func configCmd() command {
	flagSet := flag.NewFlagSet("config", flag.ExitOnError)
	systemd := flagSet.Bool("systemd", false, "Generate systemd service file")
	docker := flagSet.Bool("docker", false, "Generate docker .env file")

	// config子命令的usage信息
	flagSet.Usage = func() {
		fmt.Println("Usage: ddns6 config [options]")
		fmt.Println("Generate configuration files")
		fmt.Println()
		flagSet.PrintDefaults()
	}

	return command{
		flagSet:     flagSet,
		description: "Generate configuration files",
		run: func(args []string) error {
			// 根据参数生成不同的配置文件
			if *systemd {
				generateSystemdService() // 生成systemd服务文件
				return nil
			}
			if *docker {
				generateDockerEnv() // 生成docker环境变量文件
				return nil
			}
			flagSet.Usage()
			return errors.New("must specify either -systemd or -docker")
		},
	}
}

// stopCmd 定义并返回stop子命令
func stopCmd() command {
	return command{
		flagSet:     flag.NewFlagSet("stop", flag.ExitOnError),
		description: "Stop running service",
		run: func(args []string) error {
			// 通过PID文件停止服务
			pidFile := "/var/run/ddns6.pid"
			pidData, err := os.ReadFile(pidFile)
			if err != nil {
				return fmt.Errorf("读取PID文件失败: %w", err)
			}

			pid, _ := strconv.Atoi(strings.TrimSpace(string(pidData)))
			slog.Info("正在停止服务", "pid", pid)

			// 发送SIGTERM信号
			process, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("查找进程失败: %w", err)
			}

			if err := process.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("发送停止信号失败: %w", err)
			}

			slog.Info("服务停止信号已发送", "pid", pid)
			return nil
		},
	}
}

// getEnvWithDefault 从环境变量获取值，如果不存在则返回默认值
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createProvider 根据服务商类型创建对应的DNS提供商实例
func createProvider(service string) (domain.Tasker, error) {
	switch service {
	case "tencent":
		// 腾讯云DNS服务商
		secretId := os.Getenv("TENCENT_SECRET_ID")
		secretKey := os.Getenv("TENCENT_SECRET_KEY")
		if secretId == "" || secretKey == "" {
			return nil, errors.New("必须设置TENCENT_SECRET_ID和TENCENT_SECRET_KEY环境变量")
		}
		return tencent.NewClient(secretId, secretKey), nil
	case "cloudflare":
		// Cloudflare DNS服务商
		apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
		if apiToken == "" {
			return nil, errors.New("必须设置CLOUDFLARE_API_TOKEN环境变量")
		}
		return cloudflare.NewClient(cloudflare.WithAPIToken(apiToken)), nil
	default:
		return nil, fmt.Errorf("不支持的DNS服务商: %s", service)
	}
}

// generateSystemdService 生成systemd服务文件
func generateSystemdService() {
	// 生成systemd服务文件内容
	content := `[Unit]
Description=DDNS6 Auto Update Service
After=network.target

[Service]
Type=simple
Environment=DOMAIN=%s
Environment=SUBDOMAIN=%s
Environment=SERVICE=%s
Environment=INTERVAL=%s
ExecStart=/usr/local/bin/ddns6 run
WorkingDirectory=/usr/local/bin
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
	fmt.Printf(content,
		os.Getenv("DOMAIN"),
		getEnvWithDefault("SUBDOMAIN", "@"),
		getEnvWithDefault("SERVICE", "tencent"),
		getEnvWithDefault("INTERVAL", "5m"))
}

// generateDockerEnv 生成docker环境变量文件
func generateDockerEnv() {
	// 生成docker环境变量文件
	content := `DOMAIN=%s
SUBDOMAIN=%s
SERVICE=%s
INTERVAL=%s
TENCENT_SECRET_ID=%s
TENCENT_SECRET_KEY=%s
CLOUDFLARE_API_TOKEN=%s
DEBUG=false
`
	fmt.Printf(content,
		os.Getenv("DOMAIN"),
		getEnvWithDefault("SUBDOMAIN", "@"),
		getEnvWithDefault("SERVICE", "tencent"),
		getEnvWithDefault("INTERVAL", "5m"),
		os.Getenv("TENCENT_SECRET_ID"),
		os.Getenv("TENCENT_SECRET_KEY"),
		os.Getenv("CLOUDFLARE_API_TOKEN"))
}

// printHelp 打印全局帮助信息
func printHelp() {
	fmt.Println(`DDNS6 动态DNS更新工具

用法:
  ddns6 <command> [arguments]

命令:
  run     启动DDNS服务
  config  生成配置文件
  stop    停止运行中的服务
  help    显示帮助信息

使用 "ddns6 help <command>" 查看具体命令帮助`)
}

// printCommandHelp 打印指定子命令的帮助信息
func printCommandHelp(command string) {
	switch command {
	case "run":
		fmt.Println(`启动DDNS服务

环境变量:
  DOMAIN      必须设置的域名
  SUBDOMAIN   子域名 (默认 "@")
  SERVICE     DNS服务商 (tencent|cloudflare, 默认 "tencent")
  INTERVAL    更新间隔 (默认 "5m")
  DEBUG       调试模式 (true|false)

提供商特定环境变量:
  TENCENT_SECRET_ID     腾讯云SecretId
  TENCENT_SECRET_KEY    腾讯云SecretKey
  CLOUDFLARE_API_TOKEN  Cloudflare API令牌

示例:
  DOMAIN=example.com SERVICE=tencent ddns6 run`)
	case "config":
		fmt.Println(`生成配置文件

选项:
  -systemd  生成systemd服务文件
  -docker   生成docker环境变量文件

示例:
  ddns6 config -systemd > /etc/systemd/system/ddns6.service`)
	case "stop":
		fmt.Println(`停止运行中的服务

示例:
  ddns6 stop`)
	default:
		printHelp()
	}
}
