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
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/internal/domain"
	"github.com/notes-bin/ddns6/internal/iputil"
	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
	"github.com/notes-bin/ddns6/internal/providers/tencent"
	"github.com/notes-bin/ddns6/internal/scheduler" // 新增调度器导入
	"github.com/notes-bin/ddns6/utils/env"
	"github.com/notes-bin/ddns6/utils/logging"
)

var (
	Version = "dev"
	Commit  = "none"
)

type Config struct {
	// 选择 ddns 服务商
	Service string `env:"DNS_SERVICE" default:"tencent" required:"true"`
	// 定时任务选项
	Interval time.Duration `env:"INTERVAL" default:"5m"`
	// 调试选项
	Debug bool
	// 查看版本
	showVersion bool
}

var config = Config{}

func init() {
	// 调试选项
	flag.BoolVar(&config.Debug, "debug", false, "开启调试模式")

	// 版本选项
	flag.BoolVar(&config.showVersion, "version", false, "显示版本信息")

	flag.Usage = usages
}

func main() {
	flag.Parse()

	// 显示版本信息
	if config.showVersion {
		fmt.Printf("Version: %s\nCommit: %s\n", Version, Commit)
		return
	}

	// 初始化日志
	var logWriter io.Writer
	if config.Debug {
		logFile, err := os.OpenFile("ddns6.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			slog.Error("创建日志文件失败", "err", err)
			return
		}
		defer logFile.Close()
		logWriter = io.MultiWriter(os.Stderr, logFile)
	} else {
		logWriter = os.Stderr
	}
	logging.Logger(config.Debug, logWriter)

	// 获取域名环境变量
	ddns := &domain.Domain{Type: "AAAA"}
	if err := env.EnvToStruct("", ddns); err != nil {
		slog.Error("获取域名环境变量失败", "err", err)
		return
	}

	// 获取 ddns 服务商
	var task domain.Tasker
	switch config.Service {
	case "tencent":
		task = tencent.New()
	case "cloudflare":
		task = cloudflare.New()
	default:
		slog.Error("不支持的ddns服务商", "service", config.Service)
		return
	}

	if err := env.EnvToStruct("", task); err != nil {
		slog.Error("获取服务商环境变量失败", "err", err)
		return
	}

	// 创建多 IPv6 地址提供者
	providers := []iputil.IPv6Provider{
		iputil.NewDNSProvider(),
		iputil.NewIfaceProvider(),
		iputil.NewSiteProvider(),
	}
	multiProvider := iputil.NewMultiProvider(providers...)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建调度器实例
	sched := scheduler.New()

	// 优化错误处理协程
	go func() {
		for err := range sched.Errors() {
			if errors.Is(err, context.Canceled) {
				slog.Info("任务被取消", "err", err)
				continue
			}
			slog.Error("调度器错误", "err", err)
		}
	}()

	// 首次更新记录
	if err := ddns.UpdateRecord(ctx, multiProvider, task); err != nil {
		slog.Error("更新记录失败", "err", err)
		return
	}

	// 启动定时任务
	// 添加定时任务到调度器
	taskFunc := func(ctx context.Context) error {
		return ddns.UpdateRecord(ctx, multiProvider, task)
	}
	if err := sched.AddJob("ddns_update", config.Interval, taskFunc); err != nil {
		slog.Error("创建定时任务失败", "err", err)
		return
	}

	// 首次立即执行
	if err := taskFunc(context.Background()); err != nil {
		slog.Error("首次更新记录失败", "err", err)
	}

	slog.Info("ddns6 启动成功...", "pid", os.Getpid())

	// 信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	// 优雅关闭
	sched.GracefulShutdown()
	slog.Info("ddns6 退出成功...")
}

func usages() {
	fmt.Fprintf(os.Stderr, "简单的 ddns6 命令行工具，用于动态更新 DNS 记录以支持 IPv6 地址。\n\n")
	fmt.Fprintf(os.Stderr, "在全局命令或子命令选项使用 -h 或 --help 查看帮助。\n\n")
	fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "可用选项:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n环境变量:\n")
	fmt.Fprintf(os.Stderr, "  DNS_SERVICE: 选择 DDNS 服务商，可选值为 'tencent' 或 'cloudflare'，默认值为 'tencent'。\n")
	fmt.Fprintf(os.Stderr, "  INTERVAL: 定时任务的执行间隔，格式为 Go 语言的时间间隔字符串，例如 '5m' 表示 5 分钟，默认值为 '5m'。\n")
	fmt.Fprintf(os.Stderr, "  其他与域名和服务商相关的环境变量，用于配置域名和服务商的认证信息。\n")
	fmt.Fprintf(os.Stderr, "\n示例:\n")
	fmt.Fprintf(os.Stderr, "  %s -debug -service cloudflare \n", os.Args[0])
	fmt.Fprintf(os.Stderr, "    以调试模式启动，使用 Cloudflare 作为 DDNS 服务商。\n")
	fmt.Fprintf(os.Stderr, "  %s -version\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "    显示程序的版本信息。\n")
}
