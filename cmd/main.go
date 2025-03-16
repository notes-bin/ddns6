package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/internal/domain"
	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
	"github.com/notes-bin/ddns6/internal/providers/tencent"
	"github.com/notes-bin/ddns6/utils/cli"
	"github.com/notes-bin/ddns6/utils/env"
	"github.com/notes-bin/ddns6/utils/logging"
)

var (
	Version = "dev"
	Commit  = "none"
)

type config struct {
	// 选择 IPv6 地址获取方式
	IPv6Method string `env:"IPV6_METHOD"`
	// 选择 ddns 服务商
	Service string `env:"DNS_SERVICE"`
	// 定时任务选项
	Interval time.Duration `env:"INTERVAL"`
	// 调试选项
	Debug bool `env:"DEBUG"`
	// 域名选项
	Domain string `env:"DOMAIN"`
	// 子域名选项
	SubDomain string `env:"SUB_DOMAIN"`
}

func main() {
	var (
		task domain.Tasker
		ip   domain.IPv6Getter
		ddns = &domain.Domain{Type: "AAAA"}
	)

	if err := env.EnvToStruct(ddns, true); err != nil {
		slog.Error("获取域名环境变量失败", "err", err)
		return
	}

	// 选择 IPv6 地址获取方式
	ipv6s := []string{"dns", "site"}
	ipv6Choice := cli.ChoiceValue{
		Value:   ipv6s[0], // 默认值为第一个可选值
		Options: ipv6s,
	}
	flag.Var(&ipv6Choice, "ipv6", fmt.Sprintf("选择一个IPv6 获取方式(可选值: %v)", ipv6s))

	// 选择 ddns 服务商
	services := []string{"tencent", "cloudflare"}
	serviceChoice := cli.ChoiceValue{
		Value:   services[0], // 默认值为第一个可选值
		Options: services,
	}
	flag.Var(&serviceChoice, "service", fmt.Sprintf("选择一个 ddns 服务商(可选值: %v)", services))

	// 定时任务选项
	interval := time.Duration(5 * time.Minute)
	flag.DurationVar(&interval, "interval", interval, "定时任务时间间隔（例如 1s、2m、3h、5m2s、1h15m)")

	// 调试选项
	debug := flag.Bool("debug", false, "开启调试模式")

	// 版本选项
	version := flag.Bool("version", false, "显示版本信息")

	// 域名选项
	flag.StringVar(&ddns.Domain, "domain", "", "设置域名")
	// 子域名选项
	flag.StringVar(&ddns.SubDomain, "subdomain", "@", "设置子域名")

	flag.Usage = usages
	flag.Parse()

	var logFile *os.File
	defer logFile.Close()
	if *debug {
		logFile, err := os.OpenFile("ddns6.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			slog.Error("创建日志文件失败", "err", err)
			return
		}
		logging.Logger(*debug, io.MultiWriter(os.Stderr, logFile))
	} else {
		logging.Logger(*debug, os.Stderr)
	}

	// 显示版本信息
	if *version {
		fmt.Printf("Version: %s\nCommit: %s\n", Version, Commit)
		return
	}

	// 获取 ddns 服务商
	switch serviceChoice.Value {
	case "tencent":
		task = tencent.New()
	case "cloudflare":
		task = cloudflare.New()
	default:
		slog.Error("不支持的ddns服务商", "service", serviceChoice.Value)
		return
	}

	if err := env.EnvToStruct(task, true); err != nil {
		slog.Error("获取服务商环境变量失败", "err", err)
		return
	}

	params := make([]string, 0, len(flag.Args()))
	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("获取可执行程序路径失败:", err)
		return
	}

	// 添加可执行文件路径到参数列表
	params = append(params, exePath)
	flag.Visit(func(f *flag.Flag) {
		params = append(params, fmt.Sprintf("-%s=%v", f.Name, f.Value))
	})
	slog.Debug("参数列表", "params", params)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go ddns.UpdateRecord(ctx, ip, task, tencent.ErrIPv6NotChanged)
	if ddns.Err != nil {
		slog.Error("更新记录失败", "err", ddns.Err)
		return
	}

	go scheduler(ctx, ddns, task, ip, time.Duration(interval), tencent.ErrIPv6NotChanged)
	slog.Info("ddns6 启动成功...", "pid", os.Getpid())

	<-sigCh

	cancel()

	slog.Info("ddns6 退出成功...")
	os.Exit(0)
}

func scheduler(ctx context.Context, record domain.UpdateRecorder, task domain.Tasker, ipv6Getter domain.IPv6Getter, interval time.Duration, e error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go record.UpdateRecord(ctx, ipv6Getter, task, e)
		}
	}
}
func usages() {
	fmt.Fprintf(os.Stderr, "简单的dnns6 命令行工具\n\n在全局命令或子命令选项使用 -h 或 --help 查看帮助\n\n")
	fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n示例:\n")
	fmt.Fprintf(os.Stderr, "  %s -domain 域名 -service tencent \n", os.Args[0])
}
