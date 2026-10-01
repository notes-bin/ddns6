// Package main 是 ddns6 的程序入口。
//
// 业务命令定义在 internal/cli；此处仅调用 cli.Execute，失败时记录错误并以非零状态码退出。
package main

import (
	"log/slog"
	"os"

	"github.com/notes-bin/ddns6/internal/cli"
	"github.com/notes-bin/ddns6/internal/httputil"
)

func main() {
	if err := cli.Execute(); err != nil {
		slog.Error("command execution failed", "err", httputil.ErrForLog(err), "module", "main")
		os.Exit(1)
	}
}
