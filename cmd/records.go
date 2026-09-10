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

// recordsCmd 查询并打印 DNS 记录；默认过滤 AAAA，可通过 --type 调整。
var recordsCmd = &cobra.Command{
	Use:   "records [provider]",
	Short: "列出 DNS 记录",
	Long: `查询并列出 DNS 服务商下的域名解析记录。

不指定 provider 时，从 ~/.ddns6/config.yaml 读取配置。
指定 provider 则使用命令行参数直接查询。

默认只显示 AAAA 记录，可通过 --type 参数查看其他类型。

示例:
  # 列出 AAAA 记录
  ddns6 records tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

  # 列出所有类型的记录
  ddns6 records tencent --domain example.com --type ""

  # 从配置文件读取
  ddns6 records`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && args[0] == "help" {
			return cmd.Help()
		}
		err := runRecordsWithConfig(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		}
		return err
	},
}

// registerRecordsCommands 为 records 注册 --type 及各 provider 子命令。
func registerRecordsCommands() {
	recordsCmd.Flags().String("type", "AAAA", "DNS 记录类型过滤（默认 AAAA，设为空字符串展示所有类型）")

	registerProviderSubCommands(recordsCmd, "records", func(cmd *cobra.Command) {
		cmd.Flags().String("type", "AAAA", "DNS 记录类型过滤（默认 AAAA，设为空字符串展示所有类型）")
	}, handleRecords)
}

// handleRecords 按 --type 收集记录并格式化输出；仅当用户显式传 --subdomain 时按子域名过滤。
func handleRecords(cmd *cobra.Command, domains []*ddns.Domain, p ddns.DNSProvider) error {
	recordType, err := cmd.Flags().GetString("type")
	if err != nil {
		return fmt.Errorf("invalid --type flag: %w", err)
	}

	// 未显式指定 --subdomain 时展示该域名下匹配类型的全部记录
	filterBySubdomain := cmd.Flags().Changed("subdomain")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	allRecords, err := ddns.CollectMatchingRecords(ctx, p, domains, recordType, filterBySubdomain)
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

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

// runRecordsWithConfig 走配置文件模式执行 records；受限运营商直接返回错误。
func runRecordsWithConfig(cmd *cobra.Command) error {
	return runWithConfig(cmd, "records", func(cmd *cobra.Command, cfg *config.Config, domains []*ddns.Domain, p ddns.DNSProvider) error {
		if restrictedProviders[cfg.Provider] {
			return fmt.Errorf("%s does not support 'records' via API - %s only provides update endpoints, use its web panel to manage records", cfg.Provider, cfg.Provider)
		}
		return handleRecords(cmd, domains, p)
	})
}

// recordTypeDesc 将记录类型转为列表标题用语；空类型表示全部记录。
func recordTypeDesc(t string) string {
	if t == "" {
		return "DNS records"
	}
	return t + " records"
}

// buildFilterInfo 将域名列表去重后的 FQDN 拼成过滤条件描述。
func buildFilterInfo(domains []*ddns.Domain) string {
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
