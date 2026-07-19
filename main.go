package main

import (
	"log/slog"
	"os"

	"github.com/notes-bin/ddns6/cmd"
)

var log = slog.With("module", "main")

func main() {
	if err := cmd.Execute(); err != nil {
		log.Error("command execution failed", "err", err)
		os.Exit(1)
	}
}
