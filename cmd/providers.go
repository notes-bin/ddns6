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

// providerFlag 运营商命令行参数定义
type providerFlag struct {
	name  string
	usage string
}

// providerDef DNS 运营商命令模板
type providerDef struct {
	name  string
	short string
	flags []providerFlag
	// run 从命令行参数创建 Domain 和 DNSProvider
	run func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error)
}

// providerDefs 所有支持的 DNS 运营商
var providerDefs = []providerDef{
	{
		name: "tencent", short: "Use Tencent Cloud DNS",
		flags: []providerFlag{{"secret-id", "Tencent Cloud SecretID"}, {"secret-key", "Tencent Cloud SecretKey"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, tencent.NewDNSPod(getFlag(cmd, "secret-id"), getFlag(cmd, "secret-key")), nil
		},
	},
	{
		name: "cloudflare", short: "Use Cloudflare DNS",
		flags: []providerFlag{{"api-token", "Cloudflare API Token"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, cloudflare.NewClient(cloudflare.WithAPIToken(getFlag(cmd, "api-token"))), nil
		},
	},
	{
		name: "alicloud", short: "Use Alibaba Cloud DNS",
		flags: []providerFlag{{"access-key-id", "Alibaba Cloud Access Key ID"}, {"access-key-secret", "Alibaba Cloud Access Key Secret"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, alicloud.NewClient(getFlag(cmd, "access-key-id"), getFlag(cmd, "access-key-secret")), nil
		},
	},
	{
		name: "godaddy", short: "Use GoDaddy DNS",
		flags: []providerFlag{{"api-key", "GoDaddy API Key"}, {"api-secret", "GoDaddy API Secret"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, godaddy.NewClient(getFlag(cmd, "api-key"), getFlag(cmd, "api-secret")), nil
		},
	},
	{
		name: "huaweicloud", short: "Use Huawei Cloud DNS",
		flags: []providerFlag{
			{"username", "Huawei Cloud Username"},
			{"password", "Huawei Cloud Password"},
			{"domain-name", "Huawei Cloud Domain Name"},
		},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, huaweicloud.NewClient(getFlag(cmd, "username"), getFlag(cmd, "password"), getFlag(cmd, "domain-name")), nil
		},
	},
	{
		name: "duckdns", short: "Use DuckDNS (free DDNS service)",
		flags: []providerFlag{{"token", "DuckDNS API Token"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, duckdns.NewClient(getFlag(cmd, "token")), nil
		},
	},
	{
		name: "noip", short: "Use No-IP (classic DDNS service)",
		flags: []providerFlag{{"username", "No-IP Username"}, {"password", "No-IP Password"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, noip.NewClient(getFlag(cmd, "username"), getFlag(cmd, "password")), nil
		},
	},
	{
		name: "he", short: "Use Hurricane Electric DNS (free DNS hosting)",
		flags: []providerFlag{{"password", "HE DNS DDNS Key"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, he.NewClient(getFlag(cmd, "password")), nil
		},
	},
	{
		name: "dynv6", short: "Use Dynv6 (free IPv6 DDNS service)",
		flags: []providerFlag{{"token", "Dynv6 API Token"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, dynv6.NewClient(getFlag(cmd, "token")), nil
		},
	},
	{
		name: "porkbun", short: "Use Porkbun DNS API",
		flags: []providerFlag{{"api-key", "Porkbun API Key"}, {"api-secret", "Porkbun Secret API Key"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, porkbun.NewClient(getFlag(cmd, "api-key"), getFlag(cmd, "api-secret")), nil
		},
	},
	{
		name: "digitalocean", short: "Use DigitalOcean DNS API",
		flags: []providerFlag{{"token", "DigitalOcean API Token"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, digitalocean.NewClient(getFlag(cmd, "token")), nil
		},
	},
	{
		name: "baiducloud", short: "Use Baidu Cloud DNS",
		flags: []providerFlag{{"access-key", "Baidu Cloud Access Key"}, {"secret-key", "Baidu Cloud Secret Key"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, baiducloud.NewClient(getFlag(cmd, "access-key"), getFlag(cmd, "secret-key")), nil
		},
	},
	{
		name: "dnspod", short: "Use DNSPod (legacy API)",
		flags: []providerFlag{{"login-token", "DNSPod Login Token (format: ID,Token)"}},
		run: func(cmd *cobra.Command) (*providers.Domain, providers.DNSProvider, error) {
			ddns, err := createDomainConfig(cmd)
			if err != nil {
				return nil, nil, err
			}
			return ddns, dnspod.NewClient(getFlag(cmd, "login-token")), nil
		},
	},
}

// registerProviders 注册所有 DNS 运营商子命令
func registerProviders() {
	for i := range providerDefs {
		p := &providerDefs[i]
		cmd := &cobra.Command{
			Use:   p.name,
			Short: p.short,
			RunE: func(cmd *cobra.Command, args []string) error {
				ddns, task, err := p.run(cmd)
				if err != nil {
					return err
				}
				return runDDNSService(ddns, task, getDuration(cmd, "interval"), ipv6Fetchers)
			},
		}
		for _, f := range p.flags {
			cmd.Flags().String(f.name, "", f.usage)
		}
		runCmd.AddCommand(cmd)
	}
}

// getFlag 安全获取字符串类型 flag 值
func getFlag(cmd *cobra.Command, name string) string {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		log.Warn("flag not found, returning empty", "flag", name, "err", err)
		return ""
	}
	return v
}

// getDuration 安全获取 duration 类型 flag 值
func getDuration(cmd *cobra.Command, name string) time.Duration {
	v, err := cmd.Flags().GetDuration(name)
	if err != nil {
		log.Warn("duration flag not found, using default", "flag", name, "err", err)
		return 5 * time.Minute
	}
	return v
}

// createDomainConfig 从命令行参数创建域名配置
func createDomainConfig(cmd *cobra.Command) (*providers.Domain, error) {
	domainName, err := cmd.Flags().GetString("domain")
	if err != nil {
		return nil, fmt.Errorf("invalid --domain flag: %w", err)
	}
	subdomain, err := cmd.Flags().GetString("subdomain")
	if err != nil {
		return nil, fmt.Errorf("invalid --subdomain flag: %w", err)
	}
	ttl, err := cmd.Flags().GetInt("ttl")
	if err != nil {
		return nil, fmt.Errorf("invalid --ttl flag: %w", err)
	}
	return &providers.Domain{
		Type:      "AAAA",
		Domain:    domainName,
		SubDomain: subdomain,
		TTL:       ttl,
	}, nil
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
func runDDNSService(ddns *providers.Domain, p providers.DNSProvider, interval time.Duration, fetchers []ipaddr.IPv6Fetcher) error {
	log.Debug("starting DDNS update service",
		"domain", ddns.Domain, "subdomain", ddns.SubDomain,
		"interval", interval.String())

	sched := cron.New()
	sched.Start()
	defer sched.Stop()

	// 首次获取并更新
	log.Info("fetching IPv6 address for the first time")
	ipsvc, err := ipaddr.GetIPv6Addr(fetchers...)
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
		ipsvc, err := ipaddr.GetIPv6Addr(fetchers...)
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
