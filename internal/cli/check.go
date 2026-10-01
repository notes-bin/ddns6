package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
)

// checkCmd 校验配置/认证并探测 DNS API 连通性（通过查询 AAAA）。
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
		if len(args) > 0 && args[0] == "help" {
			return cmd.Help()
		}

		if len(args) > 0 {
			// CLI 模式：用命令行 provider 与 flag 做连通性探测
			provider := args[0]

			fmt.Printf("Checking provider: %s\n\n", provider)

			var factory *providerFactory
			for i, p := range providerFactories {
				if p.name == provider {
					factory = &providerFactories[i]
					break
				}
			}
			if factory == nil {
				fmt.Printf("Unknown provider: %s\n", provider)
				fmt.Println("Available providers: run 'ddns6 list' to see the full list")
				return fmt.Errorf("unknown provider: %s", provider)
			}
			fmt.Printf("Provider '%s' is valid\n", provider)

			fmt.Println("\n--- Auth Check ---")
			missingAuth := false
			for _, f := range factory.flags {
				v := getString(cmd, f.name)
				if v == "" {
					fmt.Printf("--%s is missing\n", f.name)
					if !f.optional {
						missingAuth = true
					}
				} else {
					fmt.Printf("--%s is set\n", f.name)
				}
			}

			domain := getString(cmd, "domain")
			if domain == "" {
				fmt.Println("--domain is missing")
				return fmt.Errorf("--domain is required")
			}
			fmt.Printf("--domain is set to %s\n", domain)

			if missingAuth {
				return fmt.Errorf("required auth flags are missing")
			}

			fmt.Println("\n--- API Connectivity Test ---")
			domains, providerClient, err := factory.run(cmd)
			if err != nil {
				fmt.Printf("Failed to create provider: %v\n", err)
				return fmt.Errorf("failed to create provider: %w", err)
			}

			ctx, cancel := context.WithTimeout(commandContext(cmd), 15*time.Second)
			defer cancel()

			records, err := ddns.CollectMatchingRecords(ctx, providerClient, domains, "AAAA", false)
			if err != nil {
				fmt.Printf("API test failed: %v\n", err)
				return fmt.Errorf("API test failed: %w", err)
			}

			fmt.Printf("API connection successful (found %d AAAA records)\n", len(records))
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("Config load failed: %v\n", err)
			return fmt.Errorf("config load failed: %w", err)
		}
		fmt.Printf("Config loaded successfully\n\n")

		return checkFromConfig(commandContext(cmd), cfg)
	},
}

// checkFromConfig 校验配置字段后创建 Provider，并以查询 AAAA 探测 API。
//
// ctx 为父 context（通常为 cmd.Context()），其上叠加 15s 超时用于 API 探测。
func checkFromConfig(ctx context.Context, cfg *config.Config) error {
	fmt.Println("--- Config Validation ---")
	if cfg.Provider != "" {
		fmt.Printf("provider: %s\n", cfg.Provider)
	} else {
		fmt.Println("provider: empty")
		return fmt.Errorf("provider is empty")
	}
	if cfg.Domain != "" {
		fmt.Printf("domain: %s\n", cfg.Domain)
	} else {
		fmt.Println("domain: empty")
		return fmt.Errorf("domain is empty")
	}
	if len(cfg.Subdomains) > 0 {
		fmt.Printf("subdomains: %v\n", cfg.Subdomains)
	} else {
		fmt.Println("subdomains: none (will default to @)")
	}
	if len(cfg.Auth) > 0 {
		fmt.Printf("auth: %d field(s) configured\n", len(cfg.Auth))
		for k := range cfg.Auth {
			fmt.Printf("- %s: ***\n", k)
		}
	} else {
		fmt.Println("auth: empty")
		return fmt.Errorf("auth is empty")
	}
	interval, err := cfg.ParseInterval()
	if err != nil {
		fmt.Printf("interval: %s (parse error: %v)\n", interval, err)
	} else {
		fmt.Printf("interval: %s\n", interval)
	}
	fmt.Printf("interface: %s\n", cfg.Interface)
	fmt.Printf("ttl: %d\n", cfg.EffectiveTTL())

	var factory *providerFactory
	for i, p := range providerFactories {
		if p.name == cfg.Provider {
			factory = &providerFactories[i]
			break
		}
	}
	if factory == nil {
		fmt.Printf("Unknown provider '%s' in config\n", cfg.Provider)
		fmt.Println("Available providers: run 'ddns6 list' to see the full list")
		return fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
	fmt.Printf("Provider '%s' is valid\n", cfg.Provider)

	fmt.Println("\n--- API Connectivity Test ---")
	providerClient, err := factory.fromConfig(cfg)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		return fmt.Errorf("failed to create provider: %w", err)
	}

	domains := buildDomains(cfg.Domain, cfg.Subdomains, cfg.EffectiveTTL())
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	records, err := ddns.CollectMatchingRecords(ctx, providerClient, domains, "AAAA", false)
	if err != nil {
		fmt.Printf("API test failed: %v\n", err)
		return fmt.Errorf("API test failed: %w", err)
	}

	fmt.Printf("API connection successful (found %d AAAA records)\n", len(records))
	return nil
}
