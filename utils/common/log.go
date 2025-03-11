package common

import (
	"io"
	"log/slog"
)

// 日志记录器
func Logger(w io.Writer, debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		AddSource: debug,
		Level:     level,
	}
	handler := slog.NewJSONHandler(w, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
