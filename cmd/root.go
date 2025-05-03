package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/notes-bin/ddns6/utils/logging"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
	buildAt = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "ddns6",
	Short: "Dynamic DNS update tool for IPv6 addresses",
	Long:  `DDNS6 is a tool for automatically updating DNS records with your current IPv6 address`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			logFile, err := os.OpenFile("ddns6.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				slog.Error("创建日志文件失败", "err", err)
				os.Exit(1)
			}
			defer logFile.Close()
			logging.SetLogger(debug, io.MultiWriter(os.Stderr, logFile))
		} else {
			logging.SetLogger(debug, os.Stderr)
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s\nCommit: %s\nbuildAt: %s\n", Version, Commit, buildAt)
	},
}

func initRootCmd() {
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().Duration("interval", 5*time.Minute, "Update interval")
	rootCmd.PersistentFlags().String("domain", "", "Domain name to update")
	rootCmd.PersistentFlags().String("subdomain", "@", "Subdomain name")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(runCmd)

	// Tencent specific flags
	tencentCmd.Flags().String("secret-id", "", "Tencent Cloud Secret ID")
	tencentCmd.Flags().String("secret-key", "", "Tencent Cloud Secret Key")
	runCmd.AddCommand(tencentCmd)

	// Cloudflare specific flags
	cloudflareCmd.Flags().String("api-token", "", "Cloudflare API Token")
	runCmd.AddCommand(cloudflareCmd)

	// Alibaba Cloud specific flags
	alicloudCmd.Flags().String("access-key-id", "", "Alibaba Cloud Access Key ID")
	alicloudCmd.Flags().String("access-key-secret", "", "Alibaba Cloud Access Key Secret")
	runCmd.AddCommand(alicloudCmd)

	// GoDaddy specific flags
	godaddyCmd.Flags().String("api-key", "", "GoDaddy API Key")
	godaddyCmd.Flags().String("api-secret", "", "GoDaddy API Secret")
	runCmd.AddCommand(godaddyCmd)

	// Huawei Cloud specific flags
	huaweicloudCmd.Flags().String("username", "", "Huawei Cloud Username")
	huaweicloudCmd.Flags().String("password", "", "Huawei Cloud Password")
	huaweicloudCmd.Flags().String("domain-name", "", "Huawei Cloud Domain Name")
	runCmd.AddCommand(huaweicloudCmd)
}

func Execute() error {
	initRootCmd()
	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("Command failed: %w", err)
	}
	return nil
}
