package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/internal/domain"
	"github.com/notes-bin/ddns6/internal/iputil"
	"github.com/notes-bin/ddns6/internal/providers/alicloud"
	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
	"github.com/notes-bin/ddns6/internal/providers/godaddy"
	"github.com/notes-bin/ddns6/internal/providers/huaweicloud"
	"github.com/notes-bin/ddns6/internal/providers/tencent"
	"github.com/notes-bin/ddns6/internal/scheduler"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the DDNS update service",
}

var tencentCmd = &cobra.Command{
	Use:   "tencent",
	Short: "Use Tencent Cloud DNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		secretId := cmd.Flag("secret-id").Value.String()
		secretKey := cmd.Flag("secret-key").Value.String()

		ddns := createDomainConfig(cmd)
		task := tencent.NewClient(secretId, secretKey)
		return runDDNSService(ddns, task, interval)
	},
}

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

func createDomainConfig(cmd *cobra.Command) *domain.Domain {
	domainName, _ := cmd.Flags().GetString("domain")
	subdomain, _ := cmd.Flags().GetString("subdomain")

	return &domain.Domain{
		Type:      "AAAA",
		Domain:    domainName,
		SubDomain: subdomain,
	}
}

// runDDNSService is the common function to run the DDNS service
func runDDNSService(ddns *domain.Domain, task domain.Tasker, interval time.Duration) error {
	sched := scheduler.New()
	defer sched.GracefulShutdown()

	taskFunc := func() error {
		return ddns.UpdateRecord(context.Background(), iputil.NewMultiProvider(), task)
	}

	if err := taskFunc(); err != nil {
		return fmt.Errorf("首次更新记录失败: %w", err)
	}

	taskWrapper := func(ctx context.Context) error {
		return taskFunc()
	}

	if err := sched.AddJob("ddns_update", interval, taskWrapper); err != nil {
		return fmt.Errorf("创建定时任务失败: %w", err)
	}

	slog.Info("ddns6 启动成功", "pid", os.Getpid())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	slog.Info("ddns6 退出成功")
	return nil
}
