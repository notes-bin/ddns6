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

	// Cloudflare specific flags
	cloudflareCmd.Flags().String("api-token", "", "Cloudflare API Token")

	// Add provider commands under run command
	runCmd.AddCommand(tencentCmd)
	runCmd.AddCommand(cloudflareCmd)
	runCmd.AddCommand(alicloudCmd)
	runCmd.AddCommand(godaddyCmd)
	runCmd.AddCommand(huaweicloudCmd)
}

func Execute() error {
	initRootCmd()
	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("Command failed: %w", err)
	}
	return nil
}
