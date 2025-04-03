package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// 设置测试环境变量
	os.Setenv("DOMAIN", "test.example.com")
	os.Setenv("TENCENT_SECRET_ID", "test-id")
	os.Setenv("TENCENT_SECRET_KEY", "test-key")
	os.Setenv("CLOUDFLARE_API_TOKEN", "test-token")

	// 运行测试
	code := m.Run()

	// 清理环境变量
	os.Unsetenv("DOMAIN")
	os.Unsetenv("TENCENT_SECRET_ID")
	os.Unsetenv("TENCENT_SECRET_KEY")
	os.Unsetenv("CLOUDFLARE_API_TOKEN")

	os.Exit(code)
}

func TestCreateProvider(t *testing.T) {
	tests := []struct {
		name    string
		service string
		wantErr bool
	}{
		{"tencent", "tencent", false},
		{"cloudflare", "cloudflare", false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := createProvider(tt.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("createProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetEnvWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		want         string
	}{
		{"existing", "DOMAIN", "default.com", "test.example.com"},
		{"non-existing", "NON_EXISTENT", "default.com", "default.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getEnvWithDefault(tt.key, tt.defaultValue); got != tt.want {
				t.Errorf("getEnvWithDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}