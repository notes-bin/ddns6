package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notes-bin/cron"
	"github.com/notes-bin/ddns6/internal/providers"
	"github.com/notes-bin/ddns6/internal/providers/alicloud"
	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
	"github.com/notes-bin/ddns6/internal/providers/godaddy"
	"github.com/notes-bin/ddns6/internal/providers/huaweicloud"
	"github.com/notes-bin/ddns6/internal/providers/tencent"
	"github.com/notes-bin/ddns6/pkg/ipaddr"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the DDNS update service",
}

// tencentCmd 使用 Tencent Cloud DNS
var tencentCmd = &cobra.Command{
	Use:   "tencent",
	Short: "Use Tencent Cloud DNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		secretId := cmd.Flag("secret-id").Value.String()
		secretKey := cmd.Flag("secret-key").Value.String()

		ddns := createDomainConfig(cmd)
		task := tencent.NewDNSPod(secretId, secretKey)
		return runDDNSService(ddns, task, interval)
	},
}

// cloudflareCmd 使用 Cloudflare DNS
var cloudflareCmd = &cobra.Command{
	Use:   "cloudflare",
	Short: "Use Cloudflare DNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		apiToken := cmd.Flag("api-token").Value.String()

		ddns := createDomainConfig(cmd)
		task := cloudflare.NewClient(cloudflare.WithAPIToken(apiToken))
		return runDDNSService(ddns, task, interval)
	},
}

// alicloudCmd 使用 Alibaba Cloud DNS
var alicloudCmd = &cobra.Command{
	Use:   "alicloud",
	Short: "Use Alibaba Cloud DNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		accessKeyId := cmd.Flag("access-key-id").Value.String()
		accessKeySecret := cmd.Flag("access-key-secret").Value.String()

		ddns := createDomainConfig(cmd)
		task := alicloud.NewClient(accessKeyId, accessKeySecret)
		return runDDNSService(ddns, task, interval)
	},
}

// godaddyCmd 使用 GoDaddy DNS
var godaddyCmd = &cobra.Command{
	Use:   "godaddy",
	Short: "Use GoDaddy DNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		apiKey := cmd.Flag("api-key").Value.String()
		apiSecret := cmd.Flag("api-secret").Value.String()

		ddns := createDomainConfig(cmd)
		task := godaddy.NewClient(apiKey, apiSecret)
		return runDDNSService(ddns, task, interval)
	},
}

// huaweicloudCmd 使用华为云 DNS
var huaweicloudCmd = &cobra.Command{
	Use:   "huaweicloud",
	Short: "Use Huawei Cloud DNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		username := cmd.Flag("username").Value.String()
		password := cmd.Flag("password").Value.String()
		domainName := cmd.Flag("domain-name").Value.String()

		ddns := createDomainConfig(cmd)
		task := huaweicloud.NewClient(username, password, domainName)
		return runDDNSService(ddns, task, interval)
	},
}

// createDomainConfig 创建域名配置
func createDomainConfig(cmd *cobra.Command) *providers.Domain {
	domainName, _ := cmd.Flags().GetString("domain")
	subdomain, _ := cmd.Flags().GetString("subdomain")

	return &providers.Domain{
		Type:      "AAAA",
		Domain:    domainName,
		SubDomain: subdomain,
	}
}

var ipv6Fetchers = []ipaddr.IPv6Fetcher{
	ipaddr.NewHttpIPv6Fetcher("6.ipw.cn"),
	ipaddr.NewHttpIPv6Fetcher("ifconfig.co"),
	ipaddr.NewHttpIPv6Fetcher("v6.ident.me"),
	ipaddr.NewDnsFetcher("2402:4e00::"),
	ipaddr.NewDnsFetcher("2400:3200:baba::1"),
	ipaddr.NewDnsFetcher("2001:4860:4860::8888"),
	ipaddr.NewDnsFetcher("2606:4700:4700::1111"),
}

// runDDNSService 运行 DDNS 服务
func runDDNSService(ddns *providers.Domain, p providers.DNSProvider, interval time.Duration) error {
	slog.Debug("启动 DDNS 更新服务",
		"domain", ddns.Domain, "subdomain", ddns.SubDomain,
		"interval", interval.String())

	sched := cron.New()
	defer sched.Stop()

	// 首次获取并更新
	slog.Info("首次获取 IPv6 地址")
	ipsvc, err := ipaddr.GetIPv6Addr(ipv6Fetchers...)
	if err != nil {
		return fmt.Errorf("获取 IPv6 地址失败: %w", err)
	}
	slog.Info("首次 IPv6 地址获取成功", "ipv6", ipsvc.String())
	if err := ddns.UpdateRecord(context.Background(), ipsvc, p); err != nil {
		return fmt.Errorf("首次更新记录失败: %w", err)
	}

	// 启动定时任务，每次重新获取 IPv6 地址
	sched.AddFunc(cron.Every(interval), func() {
		slog.Debug("定时任务触发",
			"domain", ddns.Domain, "subdomain", ddns.SubDomain)
		ipsvc, err := ipaddr.GetIPv6Addr(ipv6Fetchers...)
		if err != nil {
			slog.Error("获取 IPv6 地址失败", "err", err,
				"domain", ddns.Domain, "subdomain", ddns.SubDomain)
			return
		}
		if err := ddns.UpdateRecord(context.Background(), ipsvc, p); err != nil {
			slog.Error("更新dns记录失败", "err", err,
				"domain", ddns.Domain, "subdomain", ddns.SubDomain)
		} else {
			slog.Debug("DNS 更新周期完成",
				"domain", ddns.Domain, "subdomain", ddns.SubDomain)
		}
	})

	slog.Info("ddns6 启动成功",
		"pid", os.Getpid(),
		"domain", ddns.Domain,
		"subdomain", ddns.SubDomain,
		"interval", interval.String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh

	slog.Info("ddns6 退出成功")
	return nil
}
