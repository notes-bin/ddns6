package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// listCmd 列出项目内全部可用 DNS 运营商（不读配置、不访问网络）。
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出可用的 DNS 运营商",
	Long: `列出 ddns6 当前支持的全部 DNS 运营商。

输出包含 CLI 名称、是否支持 records/clean，以及简要说明。
数据来自内置注册表，无需配置文件或网络。

示例:
  ddns6 list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(formatProviderList(providerFactories))
		return nil
	},
}

// formatProviderList 按注册顺序格式化运营商表格。
func formatProviderList(factories []providerFactory) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tRECORDS/CLEAN\tDESCRIPTION")
	for _, p := range factories {
		rc := "yes"
		if p.noListClean || restrictedProviders[p.name] {
			rc = "no"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.name, rc, p.short)
	}
	_ = w.Flush()
	fmt.Fprintf(&b, "\nTotal: %d providers\n", len(factories))
	return b.String()
}
