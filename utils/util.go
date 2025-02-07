package utils

import (
	"fmt"
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
