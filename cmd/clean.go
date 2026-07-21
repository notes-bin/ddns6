package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
)

// cleanCmd 删除 DNS 记录。
var cleanCmd = &cobra.Command{
	Use:   "clean [provider]",
	Short: "删除 DNS 记录",
	Long: `删除 DNS 服务商下的域名解析记录。

不指定 provider 时，从 ~/.ddns6/config.yaml 读取配置。
指定 provider 则使用命令行参数直接操作。

删除前会列出将删除的记录并要求确认（除非指定 --yes）。

安全选项:
  --dry-run    仅展示将删除的记录，不实际执行删除
  --yes        跳过确认提示（用于自动化脚本）

示例:
  # 预览将删除的记录
  ddns6 clean tencent --domain example.com --subdomain www --dry-run --secret-id xxx --secret-key yyy

  # 交互式删除（需确认）
  ddns6 clean tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

  # 自动删除（跳过确认）
  ddns6 clean tencent --domain example.com --subdomain www --yes --secret-id xxx --secret-key yyy

  # 从配置文件读取并自动删除
  ddns6 clean --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// ddns6 clean help — 显示帮助
		if len(args) > 0 && args[0] == "help" {
			cmd.Help()
			return nil
		}
		if err := runCleanWithConfig(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			os.Exit(1)
		}
		return nil
	},
}

// registerCleanCommands 注册 clean 命令的 provider 子命令。
func registerCleanCommands() {
	// cleanCmd 自身的参数
	cleanCmd.Flags().String("type", "AAAA", "DNS 记录类型过滤（默认 AAAA）")
	cleanCmd.Flags().Bool("dry-run", false, "仅展示将删除的记录，不实际执行删除")
	cleanCmd.Flags().Bool("yes", false, "跳过确认提示（用于自动化脚本）")

	registerProviderSubCommands(cleanCmd, "clean", func(cmd *cobra.Command) {
		cmd.Flags().String("type", "AAAA", "DNS 记录类型过滤（默认 AAAA）")
		cmd.Flags().Bool("dry-run", false, "仅展示将删除的记录，不实际执行删除")
		cmd.Flags().Bool("yes", false, "跳过确认提示（用于自动化脚本）")
	}, handleClean)
}

// handleClean 处理 clean 命令的业务逻辑。
func handleClean(cmd *cobra.Command, domains []*ddns.Domain, p ddns.DNSProvider) error {
	// 获取参数
	recordType, err := cmd.Flags().GetString("type")
	if err != nil {
		return fmt.Errorf("invalid --type flag: %w", err)
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("invalid --dry-run flag: %w", err)
	}
	yes, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("invalid --yes flag: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 收集要删除的记录
	toDelete, err := ddns.CollectMatchingRecords(ctx, p, domains, recordType, true)
	if err != nil {
		return fmt.Errorf("failed to query records: %w", err)
	}

	// 无记录可删除
	if len(toDelete) == 0 {
		fmt.Println("No matching records to delete.")
		return nil
	}

	// 展示将删除的记录
	fmt.Printf("Will delete %d records:\n\n", len(toDelete))
	fmt.Println(ddns.FormatRecords(toDelete))

	// --dry-run 模式：仅展示，不执行
	if dryRun {
		fmt.Printf("\nDry-run mode. Use --dry-run=false or omit --dry-run to actually delete.\n")
		return nil
	}

	// 确认提示
	if !yes {
		fmt.Printf("\nDelete %d records? [y/N] ", len(toDelete))
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// 执行删除（限流 5 并发）
	sem := make(chan struct{}, 5)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var failed int

	for _, r := range toDelete {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量，满 5 时阻塞

		go func(rec ddns.RecordInfo) {
			defer wg.Done()
			defer func() { <-sem }() // 释放信号量

			slog.Info("deleting DNS record",
				"module", "cmd", "record_id", rec.ID, "name", rec.Name,
				"type", rec.Type, "value", rec.Value)

			if err := p.DeleteRecord(ctx, rec); err != nil {
				slog.Error("failed to delete record",
					"module", "cmd", "record_id", rec.ID, "name", rec.Name, "err", err)
				fmt.Fprintf(os.Stderr, "Error deleting %s (ID: %s): %v\n", rec.Name, rec.ID, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			fmt.Printf("Deleted: %s %s -> %s\n", rec.Name, rec.Type, rec.Value)
		}(r)
	}

	wg.Wait()

	// 汇总
	success := len(toDelete) - failed
	fmt.Printf("\nDeleted %d records", success)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println(".")

	if failed > 0 {
		return fmt.Errorf("%d record(s) failed to delete", failed)
	}
	return nil
}

// runCleanWithConfig 从 ~/.ddns6/config.yaml 加载配置并执行 clean。
func runCleanWithConfig(cmd *cobra.Command) error {
	return runWithConfig(cmd, "clean", func(cmd *cobra.Command, cfg *config.Config, domains []*ddns.Domain, p ddns.DNSProvider) error {
		if restrictedProviders[cfg.Provider] {
			return fmt.Errorf("%s does not support 'clean' via API — %s only provides update endpoints, use its web panel to manage records", cfg.Provider, cfg.Provider)
		}
		return handleClean(cmd, domains, p)
	})
}
