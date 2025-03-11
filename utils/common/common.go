package common

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// 安全获取环境变量，如果不存在则返回错误
func GetEnvSafe(keys ...string) (map[string]string, error) {
	result := make(map[string]string)
	for _, key := range keys {
		if value, exists := os.LookupEnv(key); !exists {
			return nil, fmt.Errorf("环境变量 %s 无法获取", key)
		} else {
			result[key] = value
		}
	}
	slog.Debug("获取环境变量成功", "result", result)
	return result, nil
}

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
