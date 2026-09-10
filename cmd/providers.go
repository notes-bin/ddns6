package cmd

// 本文件集中注册 23 家 DNS 运营商工厂，并挂载到 run / records / clean / init。
//
// 新增运营商步骤：
//  1. 在 internal/providers/<name>/ 实现 ddns.DNSProvider
//  2. 在 providerFactories 追加一条（flags、run、fromConfig）
//  3. 若 API 仅支持更新、不支持查询/删除，设置 noListClean 并加入 restrictedProviders
//  4. 补充 docker-compose.yml / .env.example / README 中的对应说明

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/internal/providers/alicloud"
	"github.com/notes-bin/ddns6/internal/providers/aws"
	"github.com/notes-bin/ddns6/internal/providers/azure"
	"github.com/notes-bin/ddns6/internal/providers/baiducloud"
	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
	"github.com/notes-bin/ddns6/internal/providers/desec"
	"github.com/notes-bin/ddns6/internal/providers/digitalocean"
	"github.com/notes-bin/ddns6/internal/providers/dnspod"
	"github.com/notes-bin/ddns6/internal/providers/dpi"
	"github.com/notes-bin/ddns6/internal/providers/duckdns"
	"github.com/notes-bin/ddns6/internal/providers/dynv6"
	"github.com/notes-bin/ddns6/internal/providers/gcloud"
	"github.com/notes-bin/ddns6/internal/providers/godaddy"
	"github.com/notes-bin/ddns6/internal/providers/he"
	"github.com/notes-bin/ddns6/internal/providers/hetzner"
	"github.com/notes-bin/ddns6/internal/providers/huaweicloud"
	"github.com/notes-bin/ddns6/internal/providers/ionos"
	"github.com/notes-bin/ddns6/internal/providers/linode"
	"github.com/notes-bin/ddns6/internal/providers/namecheap"
	"github.com/notes-bin/ddns6/internal/providers/namesilo"
	"github.com/notes-bin/ddns6/internal/providers/noip"
	"github.com/notes-bin/ddns6/internal/providers/porkbun"
	"github.com/notes-bin/ddns6/internal/providers/tencent"
)

// providerFlag 描述单个运营商认证/选项 flag 的名称与帮助文案。
type providerFlag struct {
	name  string
	usage string
}

// providerFactory 定义一家 DNS 运营商的 CLI 与配置文件两种创建路径。
//
// run 从命令行 flag 构造域名列表与 Provider；fromConfig 从 config.Config 构造。
// noListClean 为 true 时 records/clean 仅注册提示命令（API 无查询/删除能力）。
type providerFactory struct {
	name        string
	short       string
	flags       []providerFlag
	noListClean bool // 如 duckdns / he / noip：仅更新端点
	run         func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error)
	fromConfig  func(cfg *config.Config) (ddns.DNSProvider, error)
}

// restrictedProviders 标记 API 仅提供更新、不支持 records/clean 的运营商名。
var restrictedProviders = map[string]bool{
	"duckdns": true,
	"he":      true,
	"noip":    true,
}

// serviceRunner 启动 DDNS 服务；测试可替换以避免真实网络与长阻塞。
var serviceRunner = ddns.RunService

// providerFactories 为全部 23 家运营商的工厂表，注册与配置模式均依赖此表。
var providerFactories = []providerFactory{
	{
		name: "tencent", short: "Tencent Cloud DNS (DNSPod API v3) - 需 --secret-id 和 --secret-key",
		flags: []providerFlag{
			{"secret-id", "Tencent Cloud SecretID (必填，从 https://console.cloud.tencent.com/cam 获取)"},
			{"secret-key", "Tencent Cloud SecretKey (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, tencent.NewDNSPod(getString(cmd, "secret-id"), getString(cmd, "secret-key")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return tencent.NewDNSPod(cfg.Auth["secret_id"], cfg.Auth["secret_key"]), nil
		},
	},
	{
		name: "cloudflare", short: "Cloudflare DNS - 需 --api-token",
		flags: []providerFlag{
			{"api-token", "Cloudflare API Token (必填，需具有 DNS:Edit 权限)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, cloudflare.NewClient(cloudflare.WithAPIToken(getString(cmd, "api-token"))), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return cloudflare.NewClient(cloudflare.WithAPIToken(cfg.Auth["api_token"])), nil
		},
	},
	{
		name: "alicloud", short: "Alibaba Cloud DNS - 需 --access-key-id 和 --access-key-secret",
		flags: []providerFlag{
			{"access-key-id", "Alibaba Cloud Access Key ID (必填，从 RAM 用户获取)"},
			{"access-key-secret", "Alibaba Cloud Access Key Secret (必填)"},
			{"sign-version", "签名版本：v1（默认，HMAC-SHA1）或 v3（ACS3-HMAC-SHA256）"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			opts := []alicloud.Option{}
			if sv := getString(cmd, "sign-version"); sv != "" {
				opts = append(opts, alicloud.WithSignVersion(sv))
			}
			return domains, alicloud.NewClient(getString(cmd, "access-key-id"), getString(cmd, "access-key-secret"), opts...), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			opts := []alicloud.Option{}
			if sv, ok := cfg.Auth["sign_version"]; ok && sv != "" {
				opts = append(opts, alicloud.WithSignVersion(sv))
			}
			return alicloud.NewClient(cfg.Auth["access_key_id"], cfg.Auth["access_key_secret"], opts...), nil
		},
	},
	{
		name: "godaddy", short: "GoDaddy DNS - 需 --api-key 和 --api-secret",
		flags: []providerFlag{
			{"api-key", "GoDaddy API Key (必填，从 GoDaddy Developer Portal 获取)"},
			{"api-secret", "GoDaddy API Secret (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, godaddy.NewClient(getString(cmd, "api-key"), getString(cmd, "api-secret")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return godaddy.NewClient(cfg.Auth["api_key"], cfg.Auth["api_secret"]), nil
		},
	},
	{
		name: "huaweicloud", short: "Huawei Cloud DNS - 需 --access-key 和 --secret-key",
		flags: []providerFlag{
			{"access-key", "Huawei Cloud Access Key (必填，从 IAM 用户获取)"},
			{"secret-key", "Huawei Cloud Secret Key (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, huaweicloud.NewClient(getString(cmd, "access-key"), getString(cmd, "secret-key")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return huaweicloud.NewClient(cfg.Auth["access_key"], cfg.Auth["secret_key"]), nil
		},
	},
	{
		name: "duckdns", short: "DuckDNS (free DDNS) - 需 --token",
		flags: []providerFlag{
			{"token", "DuckDNS API Token (必填)"},
		},
		noListClean: true,
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, duckdns.NewClient(getString(cmd, "token")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return duckdns.NewClient(cfg.Auth["token"]), nil
		},
	},
	{
		name: "noip", short: "No-IP (classic DDNS) - 需 --username 和 --password",
		flags: []providerFlag{
			{"username", "No-IP Username (必填)"},
			{"password", "No-IP Password (必填)"},
		},
		noListClean: true,
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, noip.NewClient(getString(cmd, "username"), getString(cmd, "password")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return noip.NewClient(cfg.Auth["username"], cfg.Auth["password"]), nil
		},
	},
	{
		name: "he", short: "Hurricane Electric DNS (free DNS hosting) - 需 --password",
		flags: []providerFlag{
			{"password", "HE DNS DDNS Key (必填，从 dns.he.net 获取)"},
		},
		noListClean: true,
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, he.NewClient(getString(cmd, "password")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return he.NewClient(cfg.Auth["password"]), nil
		},
	},
	{
		name: "dynv6", short: "Dynv6 (free IPv6 DDNS) - 需 --token",
		flags: []providerFlag{
			{"token", "Dynv6 API Token (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, dynv6.NewClient(getString(cmd, "token")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return dynv6.NewClient(cfg.Auth["token"]), nil
		},
	},
	{
		name: "porkbun", short: "Porkbun DNS API - 需 --api-key 和 --api-secret",
		flags: []providerFlag{
			{"api-key", "Porkbun API Key (必填)"},
			{"api-secret", "Porkbun Secret API Key (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, porkbun.NewClient(getString(cmd, "api-key"), getString(cmd, "api-secret")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return porkbun.NewClient(cfg.Auth["api_key"], cfg.Auth["api_secret"]), nil
		},
	},
	{
		name: "digitalocean", short: "DigitalOcean DNS API - 需 --token",
		flags: []providerFlag{
			{"token", "DigitalOcean API Token (必填，需具有 write 权限)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, digitalocean.NewClient(getString(cmd, "token")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return digitalocean.NewClient(cfg.Auth["token"]), nil
		},
	},
	{
		name: "baiducloud", short: "Baidu Cloud DNS - 需 --access-key 和 --secret-key",
		flags: []providerFlag{
			{"access-key", "Baidu Cloud Access Key (必填)"},
			{"secret-key", "Baidu Cloud Secret Key (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, baiducloud.NewClient(getString(cmd, "access-key"), getString(cmd, "secret-key")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return baiducloud.NewClient(cfg.Auth["access_key"], cfg.Auth["secret_key"]), nil
		},
	},
	{
		name: "dnspod", short: "DNSPod (legacy API) - 需 --login-token",
		flags: []providerFlag{
			{"login-token", "DNSPod Login Token (必填，格式: ID,Token)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, dnspod.NewClient(getString(cmd, "login-token")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return dnspod.NewClient(cfg.Auth["login_token"]), nil
		},
	},
	{
		name: "desec", short: "deSEC.io DNS - 需 --token",
		flags: []providerFlag{
			{"token", "deSEC API Token (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, desec.NewClient(getString(cmd, "token")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return desec.NewClient(cfg.Auth["token"]), nil
		},
	},
	{
		name: "linode", short: "Linode (Akamai) DNS API v4 - 需 --api-key",
		flags: []providerFlag{
			{"api-key", "Linode Personal Access Token (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, linode.NewClient(getString(cmd, "api-key")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return linode.NewClient(cfg.Auth["api_key"]), nil
		},
	},
	{
		name: "namesilo", short: "NameSilo DNS - 需 --api-key",
		flags: []providerFlag{
			{"api-key", "NameSilo API Key (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, namesilo.NewClient(getString(cmd, "api-key")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return namesilo.NewClient(cfg.Auth["api_key"]), nil
		},
	},
	{
		name: "ionos", short: "IONOS DNS - 需 --prefix 和 --secret",
		flags: []providerFlag{
			{"prefix", "IONOS API Key Prefix (必填)"},
			{"secret", "IONOS API Key Secret (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, ionos.NewClient(getString(cmd, "prefix"), getString(cmd, "secret")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return ionos.NewClient(cfg.Auth["prefix"], cfg.Auth["secret"]), nil
		},
	},
	{
		name: "hetzner", short: "Hetzner Cloud DNS - 需 --token",
		flags: []providerFlag{
			{"token", "Hetzner Cloud API Token (必填，需 DNS 权限)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, hetzner.NewClient(getString(cmd, "token")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return hetzner.NewClient(cfg.Auth["token"]), nil
		},
	},
	{
		name: "aws", short: "AWS Route 53 - 需 --access-key-id 和 --secret-access-key",
		flags: []providerFlag{
			{"access-key-id", "AWS Access Key ID (必填)"},
			{"secret-access-key", "AWS Secret Access Key (必填)"},
			{"session-token", "AWS Session Token（可选，IAM Role/STS）"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			opts := []aws.Option{}
			if tok := getString(cmd, "session-token"); tok != "" {
				opts = append(opts, aws.WithSessionToken(tok))
			}
			return domains, aws.NewClient(getString(cmd, "access-key-id"), getString(cmd, "secret-access-key"), opts...), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			opts := []aws.Option{}
			if tok := cfg.Auth["session_token"]; tok != "" {
				opts = append(opts, aws.WithSessionToken(tok))
			}
			return aws.NewClient(cfg.Auth["access_key_id"], cfg.Auth["secret_access_key"], opts...), nil
		},
	},
	{
		name: "gcloud", short: "Google Cloud DNS - 需 --project 和 --access-token",
		flags: []providerFlag{
			{"project", "GCP 项目 ID (必填)"},
			{"access-token", "OAuth2 Access Token (必填，可用 gcloud auth print-access-token 获取)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, gcloud.NewClient(getString(cmd, "project"), getString(cmd, "access-token")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return gcloud.NewClient(cfg.Auth["project"], cfg.Auth["access_token"]), nil
		},
	},
	{
		name: "azure", short: "Azure DNS - 需 --subscription-id、--tenant-id、--client-id、--client-secret",
		flags: []providerFlag{
			{"subscription-id", "Azure Subscription ID (必填)"},
			{"tenant-id", "Azure Tenant ID (必填)"},
			{"client-id", "Azure App/Client ID (必填)"},
			{"client-secret", "Azure Client Secret (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, azure.NewClient(
				getString(cmd, "subscription-id"),
				getString(cmd, "tenant-id"),
				getString(cmd, "client-id"),
				getString(cmd, "client-secret"),
			), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return azure.NewClient(
				cfg.Auth["subscription_id"],
				cfg.Auth["tenant_id"],
				cfg.Auth["client_id"],
				cfg.Auth["client_secret"],
			), nil
		},
	},
	{
		name: "namecheap", short: "Namecheap DNS - 需 --api-key、--username、--client-ip",
		flags: []providerFlag{
			{"api-key", "Namecheap API Key (必填)"},
			{"username", "Namecheap Username (必填)"},
			{"client-ip", "Namecheap API 白名单 IP (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, namecheap.NewClient(
				getString(cmd, "api-key"),
				getString(cmd, "username"),
				getString(cmd, "client-ip"),
			), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return namecheap.NewClient(cfg.Auth["api_key"], cfg.Auth["username"], cfg.Auth["client_ip"]), nil
		},
	},
	{
		name: "dpi", short: "DNSPod.com 国际版 - 需 --login-token (ID,Key)",
		flags: []providerFlag{
			{"login-token", "DNSPod 国际版 Login Token，格式 ID,Key (必填)"},
		},
		run: func(cmd *cobra.Command) ([]*ddns.Domain, ddns.DNSProvider, error) {
			domains, err := createDomainConfigs(cmd)
			if err != nil {
				return nil, nil, err
			}
			return domains, dpi.NewClient(getString(cmd, "login-token")), nil
		},
		fromConfig: func(cfg *config.Config) (ddns.DNSProvider, error) {
			return dpi.NewClient(cfg.Auth["login_token"]), nil
		},
	},
}

// registerProviders 将 providerFactories 中每家运营商注册为 run 的子命令。
func registerProviders() {
	for i := range providerFactories {
		p := &providerFactories[i]
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
				if err := requireFlags(cmd, p.flags); err != nil {
					return err
				}
				domains, task, err := p.run(cmd)
				if err != nil {
					return err
				}
				iface := getString(cmd, "interface")
				return serviceRunner(domains, task, getDuration(cmd, "interval"), ddns.DefaultIPv6Fetchers(), iface)
			},
		}
		for _, f := range p.flags {
			cmd.Flags().String(f.name, "", f.usage)
		}
		runCmd.AddCommand(cmd)
	}
}

// providerCmdHandler 为 records/clean 等 provider 子命令的业务回调类型。
type providerCmdHandler func(cmd *cobra.Command, domains []*ddns.Domain, p ddns.DNSProvider) error

// registerProviderSubCommands 复用 providerFactories 的认证 flag 与 run，为 parent 挂载子命令。
//
// commandName 用于帮助文案与受限 API 错误信息；extraFlags 可为 nil；handler 执行实际业务。
func registerProviderSubCommands(parent *cobra.Command, commandName string, extraFlags func(cmd *cobra.Command), handler providerCmdHandler) {
	for i := range providerFactories {
		pd := &providerFactories[i]

		if pd.noListClean {
			registerRestrictedCommand(parent, commandName, pd)
			continue
		}

		cmd := &cobra.Command{
			Use:   pd.name,
			Short: fmt.Sprintf("%s DNS records for %s", commandName, pd.name),
			Long: fmt.Sprintf(`%s DNS records using %s provider.

使用方式:
  ddns6 %s %s [flags]

必填参数:
%s
全局参数:
  --domain string       根域名（必填，如 example.com）
  --subdomain string    子域名，可多次指定（默认 "@"）
  --type string         DNS 记录类型（默认 "AAAA"）
  --debug               开启调试日志

示例:
  ddns6 %s %s --domain example.com --subdomain www %s`,
				commandName, pd.name,
				commandName, pd.name,
				formatProviderFlags(pd.flags),
				commandName, pd.name, formatSampleFlags(pd.flags),
			),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := requireFlags(cmd, pd.flags); err != nil {
					return err
				}
				domains, provider, err := pd.run(cmd)
				if err != nil {
					return err
				}
				return handler(cmd, domains, provider)
			},
		}
		for _, f := range pd.flags {
			cmd.Flags().String(f.name, "", f.usage)
		}
		if extraFlags != nil {
			extraFlags(cmd)
		}
		parent.AddCommand(cmd)
	}
}

// registerRestrictedCommand 为仅支持更新的运营商注册 records/clean 占位命令，运行时返回明确错误。
func registerRestrictedCommand(parent *cobra.Command, commandName string, pd *providerFactory) {
	cmd := &cobra.Command{
		Use:   pd.name,
		Short: fmt.Sprintf("%s - %s API 不记录/管理", pd.name, pd.short),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s does not support '%s' via API - %s only provides update endpoints, use its web panel to manage records", pd.name, commandName, pd.name)
		},
	}
	parent.AddCommand(cmd)
}

// formatProviderFlags 将认证 flag 列表格式化为帮助文本中的「必填参数」段落。
func formatProviderFlags(flags []providerFlag) string {
	var b strings.Builder
	for _, f := range flags {
		fmt.Fprintf(&b, "  --%-20s %s\n", f.name, f.usage)
	}
	return b.String()
}

// formatSampleFlags 生成帮助示例中的占位 flag 片段（YOUR_<name>）。
func formatSampleFlags(flags []providerFlag) string {
	var b strings.Builder
	for _, f := range flags {
		fmt.Fprintf(&b, " --%s YOUR_%s", f.name, f.name)
	}
	return b.String()
}

// requireFlags 校验给定字符串 flag 均已提供非空值；供 RunE 在业务逻辑前调用。
func requireFlags(cmd *cobra.Command, flags []providerFlag) error {
	for _, f := range flags {
		v, err := cmd.Flags().GetString(f.name)
		if err != nil {
			return fmt.Errorf("invalid --%s flag: %w", f.name, err)
		}
		if v == "" {
			return fmt.Errorf("--%s is required (use --help to see details)", f.name)
		}
	}
	return nil
}

// --- 配置文件模式 ---

// runWithConfig 加载 ~/.ddns6/config.yaml，构造域名与 Provider，再交给 handler。
// commandName 写入加载失败时的提示（如 "run" / "records" / "clean"）。
func runWithConfig(cmd *cobra.Command, commandName string, handler func(cmd *cobra.Command, cfg *config.Config, domains []*ddns.Domain, p ddns.DNSProvider) error) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cannot load config: %w\n\nUse 'ddns6 init' to create a config file, or specify a provider: ddns6 %s <provider> --help", err, commandName)
	}

	domains := buildDomains(cfg.Domain, cfg.Subdomains, cfg.GetTTL())
	p, err := createProviderFromConfig(cfg)
	if err != nil {
		return err
	}

	return handler(cmd, cfg, domains, p)
}

// runServiceFromConfigHandler 合并配置与命令行（命令行优先）后启动 DDNS 服务。
func runServiceFromConfigHandler(cmd *cobra.Command, cfg *config.Config, domains []*ddns.Domain, p ddns.DNSProvider) error {
	interval, err := cfg.GetInterval()
	if err != nil {
		return err
	}
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

	return serviceRunner(domains, p, interval, ddns.DefaultIPv6Fetchers(), iface)
}

// createProviderFromConfig 按 cfg.Provider 在工厂表中查找并调用 fromConfig。
func createProviderFromConfig(cfg *config.Config) (ddns.DNSProvider, error) {
	for _, p := range providerFactories {
		if p.name == cfg.Provider {
			return p.fromConfig(cfg)
		}
	}
	return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
}

// --- flag / 域名辅助 ---

// getString 读取可选字符串 flag；未注册或出错时返回空串（必填项应先走 requireFlags）。
func getString(cmd *cobra.Command, name string) string {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		return ""
	}
	return v
}

// getDuration 读取可选 duration flag；未注册时回退为 5 分钟。
func getDuration(cmd *cobra.Command, name string) time.Duration {
	v, err := cmd.Flags().GetDuration(name)
	if err != nil {
		return 5 * time.Minute
	}
	return v
}

// createDomainConfigs 从 --domain / --subdomain / --ttl 构造 Domain 列表；缺省子域名为 "@"。
func createDomainConfigs(cmd *cobra.Command) ([]*ddns.Domain, error) {
	domainName, err := cmd.Flags().GetString("domain")
	if err != nil {
		return nil, fmt.Errorf("invalid --domain flag: %w", err)
	}
	if domainName == "" {
		return nil, fmt.Errorf("--domain is required (e.g. --domain example.com)")
	}

	subdomains, err := cmd.Flags().GetStringArray("subdomain")
	if err != nil {
		return nil, fmt.Errorf("invalid --subdomain flag: %w", err)
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

// buildDomains 为每个子域名生成 Type=AAAA 的 Domain；TTL 原样写入。
func buildDomains(domain string, subdomains []string, ttl int) []*ddns.Domain {
	domains := make([]*ddns.Domain, len(subdomains))
	for i, sd := range subdomains {
		domains[i] = &ddns.Domain{
			Type:      "AAAA",
			Domain:    domain,
			SubDomain: sd,
			TTL:       ttl,
		}
	}
	return domains
}
