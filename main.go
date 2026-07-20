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
