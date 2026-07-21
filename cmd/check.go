package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
)

// checkCmd 验证配置和 API 连通性。
var checkCmd = &cobra.Command{
	Use:   "check [provider]",
	Short: "验证配置和 API 连通性",
	Long: `验证 DDNS6 配置和 DNS 服务商 API 连通性。

不指定 provider 时，从 ~/.ddns6/config.yaml 读取配置。
指定 provider 则使用命令行参数直接验证。

检查项:
  1. 配置文件解析（如适用）
  2. Provider 名称是否有效
  3. 认证参数是否完整
  4. API 连通性测试（查询域名下的 AAAA 记录）

示例:
  # 验证配置文件
  ddns6 check

  # 验证命令行参数
  ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy

  # 调试模式（显示详细 API 响应）
  ddns6 check --debug tencent --domain example.com --secret-id xxx --secret-key yyy`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// check help — 显示帮助
		if len(args) > 0 && args[0] == "help" {
			cmd.Help()
			return nil
		}

		if len(args) > 0 {
			// CLI 模式：参数中指定了 provider 名称，用命令行参数验证
			provider := args[0]
			fmt.Printf("🔍 Checking provider: %s\n\n", provider)

			// 查找 provider 工厂
			var factory *providerFactory
			for i, p := range providerFactories {
				if p.name == provider {
					factory = &providerFactories[i]
					break
				}
			}
			if factory == nil {
				fmt.Printf("❌ Unknown provider: %s\n", provider)
				fmt.Println("Available providers: tencent, cloudflare, alicloud, godaddy, huaweicloud, duckdns, noip, he, dynv6, porkbun, digitalocean, baiducloud, dnspod")
				return nil
			}
			fmt.Printf("✅ Provider '%s' is valid\n", provider)

			// 检查认证参数
			fmt.Println("\n--- Auth Check ---")
			for _, f := range factory.flags {
				v := getString(cmd, f.name)
				if v == "" {
					fmt.Printf("❌ --%s is missing\n", f.name)
				} else {
					fmt.Printf("✅ --%s is set\n", f.name)
				}
			}

			// 检查域名
			domain := getString(cmd, "domain")
			if domain == "" {
				fmt.Println("❌ --domain is missing")
				return nil
			}
			fmt.Printf("✅ --domain is set to %s\n", domain)

			// API 连通性测试
			fmt.Println("\n--- API Connectivity Test ---")
			domains, providerClient, err := factory.run(cmd)
			if err != nil {
				fmt.Printf("❌ Failed to create provider: %v\n", err)
				return nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			records, err := ddns.CollectMatchingRecords(ctx, providerClient, domains, "AAAA", false)
			if err != nil {
				fmt.Printf("❌ API test failed: %v\n", err)
				return nil
			}

			fmt.Printf("✅ API connection successful (found %d AAAA records)\n", len(records))
			return nil
		}

		// 配置文件模式
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("❌ Config load failed: %v\n", err)
			return nil
		}
		fmt.Printf("✅ Config loaded successfully\n\n")

		return checkFromConfig(cfg)
	},
}

// checkFromConfig 从配置文件执行验证。
func checkFromConfig(cfg *config.Config) error {
	fmt.Println("--- Config Validation ---")
	if cfg.Provider != "" {
		fmt.Printf("✅ provider: %s\n", cfg.Provider)
	} else {
		fmt.Println("❌ provider: empty")
		return nil
	}
	if cfg.Domain != "" {
		fmt.Printf("✅ domain: %s\n", cfg.Domain)
	} else {
		fmt.Println("❌ domain: empty")
		return nil
	}
	if len(cfg.Subdomains) > 0 {
		fmt.Printf("✅ subdomains: %v\n", cfg.Subdomains)
	} else {
		fmt.Println("⚠️  subdomains: none (will default to @)")
	}
	if len(cfg.Auth) > 0 {
		fmt.Printf("✅ auth: %d field(s) configured\n", len(cfg.Auth))
		for k := range cfg.Auth {
			fmt.Printf("   - %s: ***\n", k)
		}
	} else {
		fmt.Println("❌ auth: empty")
		return nil
	}
	fmt.Printf("   interval: %s\n", cfg.GetInterval())
	fmt.Printf("   interface: %s\n", cfg.Interface)
	fmt.Printf("   ttl: %d\n", cfg.GetTTL())

	// 验证 Provider 是否有效
	var factory *providerFactory
	for i, p := range providerFactories {
		if p.name == cfg.Provider {
			factory = &providerFactories[i]
			break
		}
	}
	if factory == nil {
		fmt.Printf("❌ Unknown provider '%s' in config\n", cfg.Provider)
		fmt.Println("Available: tencent, cloudflare, alicloud, godaddy, huaweicloud, duckdns, noip, he, dynv6, porkbun, digitalocean, baiducloud, dnspod")
		return nil
	}
	fmt.Printf("✅ Provider '%s' is valid\n", cfg.Provider)

	// API 连通性测试
	fmt.Println("\n--- API Connectivity Test ---")
	providerClient, err := factory.fromConfig(cfg)
	if err != nil {
		fmt.Printf("❌ Failed to create provider: %v\n", err)
		return nil
	}

	domains := buildDomains(cfg.Domain, cfg.Subdomains, cfg.GetTTL())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	records, err := ddns.CollectMatchingRecords(ctx, providerClient, domains, "AAAA", false)
	if err != nil {
		fmt.Printf("❌ API test failed: %v\n", err)
		return nil
	}

	fmt.Printf("✅ API connection successful (found %d AAAA records)\n", len(records))
	return nil
}
