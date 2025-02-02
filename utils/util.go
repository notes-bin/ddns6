package utils

import (
	"fmt"
	"os"
)

// 安全获取环境变量，如果不存在则返回错误
func GetEnvSafe(key ...string) (map[string]string, error) {
	value := make(map[string]string)
	for _, key := range key {
		if _, exists := os.LookupEnv(key); !exists {
			return nil, fmt.Errorf("environment variable %s not found", key)
		}
	}
	return value, nil
}
