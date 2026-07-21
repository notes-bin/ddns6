package ddns

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// FormatRecords 将 DNS 记录列表格式化为表格文本。
//
// 输出格式：
//
//	ID          Name             Type    Value                      TTL
//	12345678    www              AAAA    240e:xxx::1                600
func FormatRecords(records []RecordInfo) string {
	if len(records) == 0 {
		return "No records found."
	}

	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	// 表头
	fmt.Fprintln(w, "ID\tName\tType\tValue\tTTL")

	for _, r := range records {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			r.ID, r.Name, r.Type, r.Value, r.TTL)
	}

	w.Flush()
	return b.String()
}
