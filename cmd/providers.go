package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notes-bin/cron"
	"github.com/notes-bin/ddns6/internal/providers"
	"github.com/notes-bin/ddns6/internal/providers/alicloud"
	"github.com/notes-bin/ddns6/internal/providers/baiducloud"
	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
	"github.com/notes-bin/ddns6/internal/providers/digitalocean"
	"github.com/notes-bin/ddns6/internal/providers/dnspod"
	"github.com/notes-bin/ddns6/internal/providers/duckdns"
	"github.com/notes-bin/ddns6/internal/providers/dynv6"
	"github.com/notes-bin/ddns6/internal/providers/godaddy"
	"github.com/notes-bin/ddns6/internal/providers/he"
	"github.com/notes-bin/ddns6/internal/providers/huaweicloud"
	"github.com/notes-bin/ddns6/internal/providers/noip"
	"github.com/notes-bin/ddns6/internal/providers/porkbun"
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

// duckdnsCmd 使用 DuckDNS
var duckdnsCmd = &cobra.Command{
	Use:   "duckdns",
	Short: "Use DuckDNS (free DDNS service)",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		token := cmd.Flag("token").Value.String()

		ddns := createDomainConfig(cmd)
		task := duckdns.NewClient(token)
		return runDDNSService(ddns, task, interval)
	},
}

// noipCmd 使用 No-IP
var noipCmd = &cobra.Command{
	Use:   "noip",
	Short: "Use No-IP (classic DDNS service)",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		username := cmd.Flag("username").Value.String()
		password := cmd.Flag("password").Value.String()

		ddns := createDomainConfig(cmd)
		task := noip.NewClient(username, password)
		return runDDNSService(ddns, task, interval)
	},
}

// heCmd 使用 Hurricane Electric DNS
var heCmd = &cobra.Command{
	Use:   "he",
	Short: "Use Hurricane Electric DNS (free DNS hosting)",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		password := cmd.Flag("password").Value.String()

		ddns := createDomainConfig(cmd)
		task := he.NewClient(password)
		return runDDNSService(ddns, task, interval)
	},
}

// dynv6Cmd 使用 Dynv6
var dynv6Cmd = &cobra.Command{
	Use:   "dynv6",
	Short: "Use Dynv6 (free IPv6 DDNS service)",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		token := cmd.Flag("token").Value.String()

		ddns := createDomainConfig(cmd)
		task := dynv6.NewClient(token)
		return runDDNSService(ddns, task, interval)
	},
}

// porkbunCmd 使用 Porkbun
var porkbunCmd = &cobra.Command{
	Use:   "porkbun",
	Short: "Use Porkbun DNS API",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		apiKey := cmd.Flag("api-key").Value.String()
		apiSecret := cmd.Flag("api-secret").Value.String()

		ddns := createDomainConfig(cmd)
		task := porkbun.NewClient(apiKey, apiSecret)
		return runDDNSService(ddns, task, interval)
	},
}

// digitaloceanCmd 使用 DigitalOcean
var digitaloceanCmd = &cobra.Command{
	Use:   "digitalocean",
	Short: "Use DigitalOcean DNS API",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		token := cmd.Flag("token").Value.String()

		ddns := createDomainConfig(cmd)
		task := digitalocean.NewClient(token)
		return runDDNSService(ddns, task, interval)
	},
}

// baiducloudCmd 使用百度云 DNS
var baiducloudCmd = &cobra.Command{
	Use:   "baiducloud",
	Short: "Use Baidu Cloud DNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		accessKey := cmd.Flag("access-key").Value.String()
		secretKey := cmd.Flag("secret-key").Value.String()

		ddns := createDomainConfig(cmd)
		task := baiducloud.NewClient(accessKey, secretKey)
		return runDDNSService(ddns, task, interval)
	},
}

// dnspodCmd 使用 DNSPod 旧版 API
var dnspodCmd = &cobra.Command{
	Use:   "dnspod",
	Short: "Use DNSPod (legacy API)",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		loginToken := cmd.Flag("login-token").Value.String()

		ddns := createDomainConfig(cmd)
		task := dnspod.NewClient(loginToken)
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
	log.Debug("starting DDNS update service",
		"domain", ddns.Domain, "subdomain", ddns.SubDomain,
		"interval", interval.String())

	sched := cron.New()
	sched.Start()
	defer sched.Stop()

	// 首次获取并更新
	log.Info("fetching IPv6 address for the first time")
	ipsvc, err := ipaddr.GetIPv6Addr(ipv6Fetchers...)
	if err != nil {
		return fmt.Errorf("failed to get IPv6 address: %w", err)
	}
	log.Info("initial IPv6 address obtained", "ipv6", ipsvc.String())
	if err := ddns.UpdateRecord(context.Background(), ipsvc, p); err != nil {
		return fmt.Errorf("failed to update DNS record on first run: %w", err)
	}

	// 启动定时任务，每次重新获取 IPv6 地址
	sched.AddFunc(cron.Every(interval), func() {
		log.Debug("scheduled task triggered",
			"domain", ddns.Domain, "subdomain", ddns.SubDomain)
		ipsvc, err := ipaddr.GetIPv6Addr(ipv6Fetchers...)
		if err != nil {
			log.Error("failed to get IPv6 address", "err", err,
				"domain", ddns.Domain, "subdomain", ddns.SubDomain)
			return
		}
		if err := ddns.UpdateRecord(context.Background(), ipsvc, p); err != nil {
			log.Error("failed to update DNS record", "err", err,
				"domain", ddns.Domain, "subdomain", ddns.SubDomain)
		} else {
			log.Debug("DNS update cycle completed",
				"domain", ddns.Domain, "subdomain", ddns.SubDomain)
		}
	})

	log.Info("ddns6 started successfully",
		"pid", os.Getpid(),
		"domain", ddns.Domain,
		"subdomain", ddns.SubDomain,
		"interval", interval.String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh

	log.Info("ddns6 shutting down")
	return nil
}
