// Package main 是 ddns6 的程序入口。
//
// 实际命令逻辑在 cmd 包中；此处仅调用 cmd.Execute 并在失败时以非零状态退出。
package main

import (
	"log/slog"
	"os"

	"github.com/notes-bin/ddns6/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		slog.Error("command execution failed", "err", err, "module", "main")
		os.Exit(1)
	}
}
