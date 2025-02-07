package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/configs"
	"github.com/notes-bin/ddns6/pkg/cloudflare"
	"github.com/notes-bin/ddns6/pkg/tencent"
	"github.com/notes-bin/ddns6/utils"
)

var (
	Version = "dev"
	Commit  = "none"
)

type IPv6Getter interface {
	GetIPV6Addr() (ipv6 []*net.IP, err error)
}

type Tasker interface {
	Task(domain, subdomain, ipv6addr string) error
}

type dns struct {
	Domain    string
	SubDomain string
	Type      string
	Addr      []*net.IP
}

func (d *dns) String() string {
	return fmt.Sprintf("domain: %s, subdomain: %s, type: %s, addr: %s", d.Domain, d.SubDomain, d.Type, d.Addr)
}

func (d *dns) updateRecord(ctx context.Context, ipv6Getter IPv6Getter, t Tasker, ticker *time.Ticker) {
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			addr, err := ipv6Getter.GetIPV6Addr()
			if err != nil {
				slog.Error("获取 IPv6 地址失败", "err", err)
				continue
			}

			// 确保获取到 addr
			if len(addr) == 0 {
				slog.Warn("获取到的 IPv6 地址为空")
				continue
			}

			// 检查 IPv6 地址是否改变, 如果发生改变, 则更新记录, 否则不更新
			if d.Addr == nil || !d.Addr[0].Equal(*addr[0]) {
				d.Addr = addr
				if err := t.Task(d.Domain, d.SubDomain, d.Addr[0].String()); err != nil {
					if errors.Is(err, tencent.ErrIPv6NotChanged) {
						slog.Info("IPv6 地址未改变, 无法配置ddns", "domain", d.Domain, "subdomain", d.SubDomain, "ipv6", d.Addr[0].String())
					} else {
						slog.Error("配置ddns解析失败", "err", err)
					}
				} else {
					slog.Info("IPv6 地址发生变化, ddns配置完成", "domain", d.Domain, "subdomain", d.SubDomain, "ipv6", d.Addr[0].String())
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func logger(w io.Writer, debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		AddSource: debug,
		Level:     level,
	}
	handler := slog.NewJSONHandler(w, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func showHelp() {
	fmt.Fprintf(os.Stderr, "简单的dnns6 命令行工具\n\n在全局命令或子命令选项使用 -h 或 --help 查看帮助\n\n")
	fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n示例:\n")
	fmt.Fprintf(os.Stderr, "  %s -domain 域名 -service tencent \n", os.Args[0])
}

func main() {
	var (
		task Tasker
		ip   IPv6Getter
	)

	// 选择 IPv6 地址获取方式
	ipv6s := []string{"dns", "site", "iface"}
	ipv6Choice := utils.ChoiceValue{
		Value:   ipv6s[0], // 默认值为第一个可选值
		Options: ipv6s,
	}
	flag.Var(&ipv6Choice, "ipv6", fmt.Sprintf("选择一个IPv6 获取方式(可选值: %v)", ipv6s))

	// 选择 ddns 服务商
	services := []string{"tencent", "cloudflare"}
	serviceChoice := utils.ChoiceValue{
		Value:   services[0], // 默认值为第一个可选值
		Options: services,
	}
	flag.Var(&serviceChoice, "service", fmt.Sprintf("选择一个 ddns 服务商(可选值: %v)", services))

	// 公共DNS 选项
	pdns := utils.StringSlice{"2400:3200:baba::1", "2001:4860:4860::8888"}
	flag.Var(&pdns, "public-dns", "添加自定义公共IPv6 DNS, 多个DNS用逗号分隔")

	//	自定义网站选项
	site := utils.StringSlice{"https://6.ipw.cn"}
	flag.Var(&site, "site", "添加一个可以查询IPv6地址的自定义网站, 多个网站用逗号分隔")

	// 定时任务选项
	interval := utils.Duration(5 * time.Minute)
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
	ddns := &dns{Type: "AAAA"}
	flag.StringVar(&ddns.Domain, "domain", "", "设置域名")
	// 子域名选项
	flag.StringVar(&ddns.SubDomain, "subdomain", "@", "设置子域名")

	flag.Usage = showHelp
	flag.Parse()

	logger(os.Stderr, *debug)

	if *version {
		fmt.Printf("Version: %s\nCommit: %s\n", Version, Commit)
		return
	}
	// 获取 ddns 服务商
	switch serviceChoice.Value {
	case "tencent":
		secret, err := utils.GetEnvSafe("TENCENTCLOUD_SECRET_ID", "TENCENTCLOUD_SECRET_KEY")
		if err != nil {
			slog.Error("获取腾讯云密钥失败", "err", err)
			return
		}
		task = tencent.New(secret["TENCENTCLOUD_SECRET_ID"], secret["TENCENTCLOUD_SECRET_KEY"])
	case "cloudflare":
		secret, err := utils.GetEnvSafe("CLOUDFLARE_AUTH_TOKEN")
		if err != nil {
			slog.Error("获取cloudflare密钥失败", "err", err)
			return
		}
		task = cloudflare.NewCloudflare(secret["CLOUDFLARE_AUTH_TOKEN"])
	default:
		slog.Error("不支持的ddns服务商", "service", serviceChoice.Value)
		return
	}

	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("获取可执行程序路径失败:", err)
		return
	}

	// 添加可执行文件路径到参数列表
	params := []string{filepath.Dir(exePath)}
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

	if *init {

		configs.GenerateService(params...)
		return
	}

	// 获取 IPv6 地址
	switch ipv6Choice.Value {
	case "dns":
		ip = utils.NewPublicDNS(pdns...)
	case "site":
		ip = utils.NewSite(site...)
	case "iface":
		ip = utils.NewIface(*iface)
	default:
		slog.Error("不支持的ipv6获取方式", "ipv6", ipv6Choice.Value)
		return
	}

	// 创建一个可以取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(time.Duration(interval))
	go ddns.updateRecord(ctx, ip, task, ticker)
	slog.Info("ddns6 启动成功...", "pid", os.Getpid())

	<-sigCh

	cancel()

	slog.Info("ddns6 退出成功...")
	os.Exit(0)
}
