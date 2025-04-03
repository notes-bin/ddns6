package logging

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"testing"
)

// TestSetLogger 测试 SetLogger 函数
func TestSetLogger(t *testing.T) {
	tests := []struct {
		name    string
		debug   bool
		wantKey string
	}{
		{
			name:    "debug_enabled",
			debug:   true,
			wantKey: slog.SourceKey,
		},
		{
			name:    "debug_disabled",
			debug:   false,
			wantKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建一个缓冲区来捕获日志输出
			var buf bytes.Buffer
			out := io.Writer(&buf)

			// 调用 SetLogger 函数
			SetLogger(tt.debug, out)

			// 记录一条日志
			slog.Info("test log message")

			// 检查日志输出中是否包含预期的键
			output := buf.String()
			if tt.wantKey != "" && output != "" && output != "\n" {
				if output != "" && output != "\n" {
					if !containsKey(output, tt.wantKey) {
						t.Errorf("日志输出中未找到预期的键 %q", tt.wantKey)
					}
				}
			}
		})
	}
}

// containsKey 检查字符串中是否包含指定的键
func containsKey(s, key string) bool {
	return bytes.Contains([]byte(s), []byte(key))
}

// TestSetLoggerWithFile 测试将日志输出到文件
func TestSetLoggerWithFile(t *testing.T) {
	// 创建一个临时文件
	file, err := os.CreateTemp("", "test_logger_*.log")
	if err != nil {
		t.Fatalf("无法创建临时文件: %v", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	// 调用 SetLogger 函数，将日志输出到临时文件
	SetLogger(true, file)

	// 记录一条日志
	slog.Info("test log message to file")

	// 读取文件内容
	_, err = file.Seek(0, 0)
	if err != nil {
		t.Fatalf("无法定位文件指针: %v", err)
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, file)
	if err != nil {
		t.Fatalf("无法读取文件内容: %v", err)
	}

	// 检查日志输出中是否包含预期的消息
	output := buf.String()
	if !containsKey(output, "test log message to file") {
		t.Errorf("文件中未找到预期的日志消息")
	}
}
