// Package main 是 ddns6 的程序入口。
//
// 业务命令定义在 internal/cli；此处仅调用 cli.Execute，失败时以非零状态码退出。
// 错误信息由 cli.Execute 统一打印，避免与 slog 重复输出。
package main

import (
	"os"

	"github.com/notes-bin/ddns6/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
