package logger

import (
	"io"
	"log/slog"
	"path/filepath"
)

// 日志记录器
func SetLogger(debug bool, out io.Writer) {
	opts := new(slog.HandlerOptions)
	if debug {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
		opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				source.File = filepath.Base(source.File)
			}
			return a
		}
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(out, opts)))
}
