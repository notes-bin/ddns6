package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
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
)

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
	// run 从命令行参数创建域名列表和 DNSProvider
	run func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error)
}

// providerDefs 所有支持的 DNS 运营商
var providerDefs = []providerDef{
	{
		name: "tencent", short: "Tencent Cloud DNS (DNSPod API v3) — 需 --secret-id 和 --secret-key",
		flags: []providerFlag{
			{"secret-id", "Tencent Cloud SecretID (必填，从 https://console.cloud.tencent.com/cam 获取)"},
			{"secret-key", "Tencent Cloud SecretKey (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, tencent.NewDNSPod(getString(cmd, "secret-id"), getString(cmd, "secret-key")), nil
		},
	},
	{
		name: "cloudflare", short: "Cloudflare DNS — 需 --api-token",
		flags: []providerFlag{
			{"api-token", "Cloudflare API Token (必填，需具有 DNS:Edit 权限)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, cloudflare.NewClient(cloudflare.WithAPIToken(getString(cmd, "api-token"))), nil
		},
	},
	{
		name: "alicloud", short: "Alibaba Cloud DNS — 需 --access-key-id 和 --access-key-secret",
		flags: []providerFlag{
			{"access-key-id", "Alibaba Cloud Access Key ID (必填，从 RAM 用户获取)"},
			{"access-key-secret", "Alibaba Cloud Access Key Secret (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, alicloud.NewClient(getString(cmd, "access-key-id"), getString(cmd, "access-key-secret")), nil
		},
	},
	{
		name: "godaddy", short: "GoDaddy DNS — 需 --api-key 和 --api-secret",
		flags: []providerFlag{
			{"api-key", "GoDaddy API Key (必填，从 GoDaddy Developer Portal 获取)"},
			{"api-secret", "GoDaddy API Secret (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, godaddy.NewClient(getString(cmd, "api-key"), getString(cmd, "api-secret")), nil
		},
	},
	{
		name: "huaweicloud", short: "Huawei Cloud DNS — 需 --username、--password 和 --domain-name",
		flags: []providerFlag{
			{"username", "Huawei Cloud Username (必填，IAM 用户名)"},
			{"password", "Huawei Cloud Password (必填)"},
			{"domain-name", "Huawei Cloud Domain Name (必填，IAM 用户所属账号)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, huaweicloud.NewClient(getString(cmd, "username"), getString(cmd, "password"), getString(cmd, "domain-name")), nil
		},
	},
	{
		name: "duckdns", short: "DuckDNS (free DDNS) — 需 --token",
		flags: []providerFlag{
			{"token", "DuckDNS API Token (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, duckdns.NewClient(getString(cmd, "token")), nil
		},
	},
	{
		name: "noip", short: "No-IP (classic DDNS) — 需 --username 和 --password",
		flags: []providerFlag{
			{"username", "No-IP Username (必填)"},
			{"password", "No-IP Password (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, noip.NewClient(getString(cmd, "username"), getString(cmd, "password")), nil
		},
	},
	{
		name: "he", short: "Hurricane Electric DNS (free DNS hosting) — 需 --password",
		flags: []providerFlag{
			{"password", "HE DNS DDNS Key (必填，从 dns.he.net 获取)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, he.NewClient(getString(cmd, "password")), nil
		},
	},
	{
		name: "dynv6", short: "Dynv6 (free IPv6 DDNS) — 需 --token",
		flags: []providerFlag{
			{"token", "Dynv6 API Token (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, dynv6.NewClient(getString(cmd, "token")), nil
		},
	},
	{
		name: "porkbun", short: "Porkbun DNS API — 需 --api-key 和 --api-secret",
		flags: []providerFlag{
			{"api-key", "Porkbun API Key (必填)"},
			{"api-secret", "Porkbun Secret API Key (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, porkbun.NewClient(getString(cmd, "api-key"), getString(cmd, "api-secret")), nil
		},
	},
	{
		name: "digitalocean", short: "DigitalOcean DNS API — 需 --token",
		flags: []providerFlag{
			{"token", "DigitalOcean API Token (必填，需具有 write 权限)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, digitalocean.NewClient(getString(cmd, "token")), nil
		},
	},
	{
		name: "baiducloud", short: "Baidu Cloud DNS — 需 --access-key 和 --secret-key",
		flags: []providerFlag{
			{"access-key", "Baidu Cloud Access Key (必填)"},
			{"secret-key", "Baidu Cloud Secret Key (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, baiducloud.NewClient(getString(cmd, "access-key"), getString(cmd, "secret-key")), nil
		},
	},
	{
		name: "dnspod", short: "DNSPod (legacy API) — 需 --login-token",
		flags: []providerFlag{
			{"login-token", "DNSPod Login Token (必填，格式: ID,Token)"},
		},
		run: func(cmd *cobra.Command) ([]*providers.Domain, providers.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, dnspod.NewClient(getString(cmd, "login-token")), nil
		},
	},
}

// registerProviders 注册所有 DNS 运营商子命令到 runCmd。
func registerProviders() {
	for i := range providerDefs {
		p := &providerDefs[i]
		cmd := &cobra.Command{
			Use:   p.name,
			Short: p.short,
			Long: fmt.Sprintf(`%s provider for DDNS6

使用方式:
  ddns6 run %s [flags]

必填参数:
%s
全局参数:
  --domain string       根域名（必填，如 example.com）
  --subdomain string    子域名，可多次指定（默认 "@"）
  --ttl int             DNS 记录 TTL，单位秒（默认 600）
  --interval duration   非 Linux 平台轮询间隔（默认 5m）
  --interface string    监听的网络接口（仅 Linux Netlink 模式）
  --debug               开启调试日志

示例:
  ddns6 run %s --domain example.com --subdomain www %s
  ddns6 run %s --domain example.com --subdomain www --subdomain @ %s`,
				p.name, p.name,
				formatProviderFlags(p.flags),
				p.name, formatSampleFlags(p.flags),
				p.name, formatSampleFlags(p.flags)),
			RunE: func(cmd *cobra.Command, args []string) error {
				domains, task, err := p.run(cmd)
				if err != nil {
					return err
				}
				iface := getString(cmd, "interface")
				return ddns.RunService(domains, task, getDuration(cmd, "interval"), ddns.DefaultIPv6Fetchers, iface)
			},
		}
		for _, f := range p.flags {
			cmd.Flags().String(f.name, "", f.usage)
		}
		runCmd.AddCommand(cmd)
	}
}

// formatProviderFlags 返回运营商的必填参数格式文本
func formatProviderFlags(flags []providerFlag) string {
	var b strings.Builder
	for _, f := range flags {
		fmt.Fprintf(&b, "  --%-20s %s\n", f.name, f.usage)
	}
	return b.String()
}

// formatSampleFlags 返回示例参数文本
func formatSampleFlags(flags []providerFlag) string {
	var b strings.Builder
	for _, f := range flags {
		fmt.Fprintf(&b, " --%s YOUR_%s", f.name, f.name)
	}
	return b.String()
}

// ============================================================
// 配置文件模式：从 ~/.ddns6/config.yaml 创建 provider
// ============================================================

// startServiceFromConfig 根据配置文件启动 DDNS 服务。
func startServiceFromConfig(cfg *config.Config, cmd *cobra.Command) error {
	// 从配置创建域名列表
	domains := buildDomains(cfg.Domain, cfg.Subdomains, cfg.GetTTL())

	// 从配置创建 DNS 服务商
	p, err := createProviderFromConfig(cfg)
	if err != nil {
		return err
	}

	// 合并配置与命令行参数（命令行参数优先）
	interval := cfg.GetInterval()
	if cmd != nil && cmd.Flags().Changed("interval") {
		if v, err := cmd.Flags().GetDuration("interval"); err == nil {
			interval = v
		}
	}

	iface := cfg.Interface
	if cmd != nil && cmd.Flags().Changed("interface") {
		if v, err := cmd.Flags().GetString("interface"); err == nil {
			iface = v
		}
	}

	return ddns.RunService(domains, p, interval, ddns.DefaultIPv6Fetchers, iface)
}

// createProviderFromConfig 根据配置的 provider 类型和 auth 字段创建对应的 DNS 服务商。
func createProviderFromConfig(cfg *config.Config) (providers.DNSProvider, error) {
	switch cfg.Provider {
	case "tencent":
		return tencent.NewDNSPod(cfg.Auth["secret_id"], cfg.Auth["secret_key"]), nil
	case "cloudflare":
		return cloudflare.NewClient(cloudflare.WithAPIToken(cfg.Auth["api_token"])), nil
	case "alicloud":
		return alicloud.NewClient(cfg.Auth["access_key_id"], cfg.Auth["access_key_secret"]), nil
	case "godaddy":
		return godaddy.NewClient(cfg.Auth["api_key"], cfg.Auth["api_secret"]), nil
	case "huaweicloud":
		return huaweicloud.NewClient(cfg.Auth["username"], cfg.Auth["password"], cfg.Auth["domain_name"]), nil
	case "duckdns":
		return duckdns.NewClient(cfg.Auth["token"]), nil
	case "noip":
		return noip.NewClient(cfg.Auth["username"], cfg.Auth["password"]), nil
	case "he":
		return he.NewClient(cfg.Auth["password"]), nil
	case "dynv6":
		return dynv6.NewClient(cfg.Auth["token"]), nil
	case "porkbun":
		return porkbun.NewClient(cfg.Auth["api_key"], cfg.Auth["api_secret"]), nil
	case "digitalocean":
		return digitalocean.NewClient(cfg.Auth["token"]), nil
	case "baiducloud":
		return baiducloud.NewClient(cfg.Auth["access_key"], cfg.Auth["secret_key"]), nil
	case "dnspod":
		return dnspod.NewClient(cfg.Auth["login_token"]), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported providers: tencent, cloudflare, alicloud, godaddy, huaweicloud, duckdns, noip, he, dynv6, porkbun, digitalocean, baiducloud, dnspod)", cfg.Provider)
	}
}

// ============================================================
// 辅助函数
// ============================================================

// getString 安全获取字符串类型 flag 值
func getString(cmd *cobra.Command, name string) string {
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

// createDomainConfigs 从命令行参数创建域名配置列表。
// 每个 --subdomain 值会生成一个对应的 Domain 实例。
func createDomainConfigs(cmd *cobra.Command) ([]*providers.Domain, error) {
	domainName, err := cmd.Flags().GetString("domain")
	if err != nil {
		return nil, fmt.Errorf("invalid --domain flag: %w", err)
	}
	if domainName == "" {
		return nil, fmt.Errorf("--domain is required (e.g. --domain example.com)")
	}

	subdomains, err := cmd.Flags().GetStringArray("subdomain")
	if err != nil {
		// 兼容 --subdomain 为单个字符串的情况
		sd, err2 := cmd.Flags().GetString("subdomain")
		if err2 != nil {
			return nil, fmt.Errorf("invalid --subdomain flag: %w", err)
		}
		subdomains = []string{sd}
	}
	if len(subdomains) == 0 {
		subdomains = []string{"@"}
	}

	ttl, err := cmd.Flags().GetInt("ttl")
	if err != nil {
		return nil, fmt.Errorf("invalid --ttl flag: %w", err)
	}

	return buildDomains(domainName, subdomains, ttl), nil
}

// buildDomains 根据根域名、子域名列表和 TTL 创建 Domain 列表。
func buildDomains(domain string, subdomains []string, ttl int) []*providers.Domain {
	domains := make([]*providers.Domain, len(subdomains))
	for i, sd := range subdomains {
		domains[i] = &providers.Domain{
			Type:      "AAAA",
			Domain:    domain,
			SubDomain: sd,
			TTL:       ttl,
		}
	}
	return domains
}
