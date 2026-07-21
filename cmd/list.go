package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
)

// listCmd 列出 DNS 记录。
var listCmd = &cobra.Command{
	Use:   "list [provider]",
	Short: "列出 DNS 记录",
	Long: `查询并列出 DNS 服务商下的域名解析记录。

不指定 provider 时，从 ~/.ddns6/config.yaml 读取配置。
指定 provider 则使用命令行参数直接查询。

默认只显示 AAAA 记录，可通过 --type 参数查看其他类型。

示例:
  # 列出 AAAA 记录
  ddns6 list tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

  # 列出所有类型的记录
  ddns6 list tencent --domain example.com --type ""

  # 从配置文件读取
  ddns6 list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// ddns6 list help — 显示帮助
		if len(args) > 0 && args[0] == "help" {
			cmd.Help()
			return nil
		}
		if err := runListWithConfig(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			os.Exit(1)
		}
		return nil
	},
}

// registerListCommands 注册 list 命令的 provider 子命令。
func registerListCommands() {
	// listCmd 自身的 --type 参数
	listCmd.Flags().String("type", "AAAA", "DNS 记录类型过滤（默认 AAAA，设为空字符串展示所有类型）")

	registerProviderSubCommands(listCmd, "list", func(cmd *cobra.Command) {
		cmd.Flags().String("type", "AAAA", "DNS 记录类型过滤（默认 AAAA，设为空字符串展示所有类型）")
	}, handleList)
}

// handleList 处理 list 命令的业务逻辑。
func handleList(cmd *cobra.Command, domains []*ddns.Domain, p ddns.DNSProvider) error {
	// 获取 --type 参数
	recordType, err := cmd.Flags().GetString("type")
	if err != nil {
		return fmt.Errorf("invalid --type flag: %w", err)
	}

	// 用户显式指定了 --subdomain 时才按子域名过滤
	// 未指定时展示该域名下所有匹配 --type 的记录
	filterBySubdomain := cmd.Flags().Changed("subdomain")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	allRecords, err := ddns.CollectMatchingRecords(ctx, p, domains, recordType, filterBySubdomain)
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

	// 输出
	filterInfo := buildFilterInfo(domains)
	heading := fmt.Sprintf("Listing %s for %s", recordTypeDesc(recordType), filterInfo)
	if filterBySubdomain {
		heading += " (filtered by subdomain)"
	}
	fmt.Println(heading + ":\n")

	if len(allRecords) == 0 {
		fmt.Println("No records found.")
		return nil
	}

	fmt.Println(ddns.FormatRecords(allRecords))
	fmt.Printf("\nFound %d %s.\n", len(allRecords), recordTypeDesc(recordType))
	return nil
}

// runListWithConfig 从 ~/.ddns6/config.yaml 加载配置并执行 list。
func runListWithConfig(cmd *cobra.Command) error {
	return runWithConfig(cmd, "list", func(cmd *cobra.Command, cfg *config.Config, domains []*ddns.Domain, p ddns.DNSProvider) error {
		return handleList(cmd, domains, p)
	})
}

// recordTypeDesc 返回记录类型的中文描述。
func recordTypeDesc(t string) string {
	if t == "" {
		return "DNS records"
	}
	return t + " records"
}

// buildFilterInfo 构建过滤条件描述文本。
func buildFilterInfo(domains []*ddns.Domain) string {
	// 收集所有唯一的 fullDomain
	seen := make(map[string]bool)
	var parts []string
	for _, d := range domains {
		fqdn := d.FullDomain()
		if !seen[fqdn] {
			seen[fqdn] = true
			parts = append(parts, fqdn)
		}
	}
	return strings.Join(parts, ", ")
}
