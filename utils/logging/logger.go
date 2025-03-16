package logging

import (
	"io"
	"log/slog"
)

// 日志记录器
func Logger(debug bool, out io.Writer) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if debug {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(out, opts)))
}
