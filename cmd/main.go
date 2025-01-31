package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/pkg/tencent"
	"github.com/notes-bin/ddns6/utils"
)

var Version = "unknown"

// IPv6Geter 接口定义了一个方法 GetIPV6Addr，用于获取 IPv6 地址列表。
// 实现该接口的类型需要提供一个返回 IPv6 地址切片和错误信息的方法。
type IPv6Geter interface {
	GetIPV6Addr() (ipv6 []*net.IP, err error)
}

// Tasker 是一个接口，定义了执行 DNS 更新任务的方法。
// Task 方法接受三个参数：域名、子域名和 IPv6 地址，
// 并返回一个错误（如果有）。实现该接口的类型需要提供具体的任务执行逻辑。
type Tasker interface {
	Task(domain, subdomain, ipv6addr string) error
}

// DNS 结构体表示一个 DNS 记录，包含以下字段：
// - Domain: 域名
// - Name: 记录名称
// - Type: 记录类型，例如 A、AAAA、CNAME 等
// - Addr: 指向 IP 地址的指针
type dns struct {
	Domain    string
	SubDomain string
	Type      string
	Addr      []*net.IP
}

func (d *dns) updateRecord(ip IPv6Geter, t Tasker, ticker *time.Ticker) {
	defer ticker.Stop()
	for range ticker.C {
		addr, err := ip.GetIPV6Addr()
		if err != nil {
			slog.Error("获取 IPv6 地址失败", "err", err)
			continue
		}
		d.Addr = addr
		if err := t.Task(d.Domain, d.SubDomain, d.Addr[0].String()); err != nil {
			slog.Error("配置ddns解析失败", "err", err)
			continue
		}
		slog.Info("更新成功", "domain", d.Domain, "subdomain", d.SubDomain, "ipv6", d.Addr[0].String())
	}
}

func logger(w io.Writer, level slog.Level) {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	}
	handler := slog.NewJSONHandler(w, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func showHelp() {
	fmt.Fprintf(os.Stderr, "简单的dnns6 命令行工具\n\n在全局命令或子命令选项使用 -h 或 --help 查看帮助\n\n")
	fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "全局命令:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n子命令:\n")
	fmt.Fprintf(os.Stderr, "  tencent 为腾讯云域名添加 IPv6 记录\n")
	fmt.Fprintf(os.Stderr, "  help    显示帮助信息\n")
	fmt.Fprintf(os.Stderr, "\n示例:\n")
	fmt.Fprintf(os.Stderr, "  %s -domain 域名 tencent -secret-id xxx -secret-key yyy\n", os.Args[0])
}

func main() {
	var (
		task           Tasker
		ip             IPv6Geter
		debug, version bool
	)
	ipopts := []string{"dns", "site", "iface"}
	choice := utils.ChoiceValue{
		Value:   ipopts[0], // 默认值为第一个可选值
		Options: ipopts,
	}
	flag.Var(&choice, "ipv6", fmt.Sprintf("选择一个值(可选值: %v)", ipopts))

	pdns := utils.StringSlice{"2400:3200:baba::1", "2001:4860:4860::8888"}
	flag.Var(&pdns, "public-dns", "添加自定义公共IPv6 DNS, 多个DNS用逗号分隔")

	site := utils.StringSlice{"https://6.ipw.cn"}
	flag.Var(&site, "site", "添加一个可以查询IPv6地址的自定义网站, 多个网站用逗号分隔")

	var interval utils.Duration = utils.Duration(5 * time.Minute)
	flag.Var(&interval, "interval", "定时任务时间间隔（例如 1s、2m、3h、5m2s、1h15m)")

	iface := flag.String("iface", "eth0", "设备的物理网卡名称")

	ddns := &dns{Type: "AAAA"}
	flag.StringVar(&ddns.Domain, "domain", "", "设置域名")
	flag.StringVar(&ddns.SubDomain, "subdomain", "@", "设置子域名")

	flag.BoolVar(&debug, "debug", false, "开启调试模式")
	flag.BoolVar(&version, "version", false, "显示版本信息")

	flag.Usage = showHelp
	flag.Parse()

	if debug {
		logger(os.Stderr, slog.LevelDebug)
	} else {
		logger(os.Stderr, slog.LevelInfo)
	}

	if version {
		fmt.Printf("ddns6 version: %s\n", Version)
		os.Exit(0)
	}

	switch choice.Value {
	case "dns":
		ip = utils.NewPublicDNS(pdns...)
	case "site":
		ip = utils.NewSite(site...)
	case "iface":
		ip = utils.NewIface(*iface)
	default:
		panic("ipv6 must be dns or site or iface")
	}

	args := flag.Args()
	if len(args) == 0 {
		panic("请指定子命令")
	}

	switch args[0] {
	case "help":
		showHelp()
		os.Exit(0)
	case "tencent":
		cmd := utils.NewSubCmd("tencent", "腾讯云dns服务")
		secretId := cmd.String("secret-id", "", "腾讯云 API 密钥 ID")
		secretKey := cmd.String("secret-key", "", "腾讯云 API 密钥 Key")
		cmd.Parse(args[1:])
		task = tencent.New(*secretId, *secretKey)
	default:
		panic("子命令必须为 tencent")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(time.Duration(interval))
	go ddns.updateRecord(ip, task, ticker)
	slog.Info("ddns6 启动成功...", "pid", os.Getpid())
	<-sigCh
	slog.Info("ddns6 退出成功...")
}
