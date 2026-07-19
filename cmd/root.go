package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
	buildAt = "unknown"
)

var log = slog.With("module", "cmd")

var rootCmd = &cobra.Command{
	Use:   "ddns6",
	Short: "Dynamic DNS update tool for IPv6 addresses",
	Long:  `DDNS6 is a tool for automatically updating DNS records with your current IPv6 address`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// version 命令不需要初始化日志
		if cmd.Name() == "version" {
			return
		}

		logFile, err := os.OpenFile("ddns6.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Error("failed to create log file", "err", err)
			os.Exit(1)
		}

		opts := new(slog.HandlerOptions)

		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			opts.Level = slog.LevelDebug
			opts.AddSource = true
			opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.SourceKey {
					if source, ok := a.Value.Any().(*slog.Source); ok {
						source.File = filepath.Base(source.File)
					}
				}
				return a
			}
		}
		slog.SetDefault(slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stderr, logFile), opts)))
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
	rootCmd.PersistentFlags().Int("ttl", 600, "DNS record TTL (seconds)")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(runCmd)

	// 数据驱动注册所有运营商命令
	registerProviders()
}

func Execute() error {
	initRootCmd()
	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("Command failed: %w", err)
	}
	return nil
}
