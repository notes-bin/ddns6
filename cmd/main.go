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

	"github.com/notes-bin/ddns6/configs"
	"github.com/notes-bin/ddns6/internal/domain"
	"github.com/notes-bin/ddns6/pkg/cloudflare"
	"github.com/notes-bin/ddns6/pkg/tencent"
	"github.com/notes-bin/ddns6/utils/cli"
	"github.com/notes-bin/ddns6/utils/common"
	"github.com/notes-bin/ddns6/utils/network"
)

var (
	Version = "dev"
	Commit  = "none"
)

func main() {
	var (
		task domain.Tasker
		ip   domain.IPv6Getter
		ddns = &domain.Domain{Type: "AAAA"}
	)

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

	// 公共DNS 选项
	pdns := cli.StringSlice{"2400:3200:baba::1", "2001:4860:4860::8888"}
	flag.Var(&pdns, "public-dns", "添加自定义公共IPv6 DNS, 多个DNS用逗号分隔")

	//	自定义网站选项
	site := cli.StringSlice{"https://6.ipw.cn"}
	flag.Var(&site, "site", "添加一个可以查询IPv6地址的自定义网站, 多个网站用逗号分隔")

	// 定时任务选项
	interval := cli.Duration(5 * time.Minute)
	flag.Var(&interval, "interval", "定时任务时间间隔（例如 1s、2m、3h、5m2s、1h15m)")

	// 物理网卡选项
	iface := flag.String("iface", "eth0", "设备的物理网卡名称")

	// 生成服务选项
	init := flag.Bool("init", false, "生成 systemd 服务")

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
		common.Logger(io.MultiWriter(os.Stderr, logFile), *debug)
	} else {
		common.Logger(os.Stderr, *debug)
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
		secret, err := common.GetEnvSafe("CF_Token")
		if err != nil {
			slog.Error("获取cloudflare密钥失败", "err", err)
			return
		}
		task = cloudflare.NewCloudflare(secret["CF_Token"])
	default:
		slog.Error("不支持的ddns服务商", "service", serviceChoice.Value)
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

	for i, v := range params {
		if v == "-init=true" {
			// 删除索引i处的元素
			params = append(params[:i], params[i+1:]...)
		}
	}
	slog.Debug("参数列表", "params", params)

	// 生成 systemd 服务
	if *init {
		configs.GenerateService(params...)
		return
	}

	// 获取 IPv6 地址
	switch ipv6Choice.Value {
	case "dns":
		ip = network.NewPublicDNS(pdns...)
	case "site":
		ip = network.NewSite(site...)
	case "iface":
		ip = network.NewIface(*iface)
	default:
		slog.Error("不支持的ipv6获取方式", "ipv6", ipv6Choice.Value)
		return
	}

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
