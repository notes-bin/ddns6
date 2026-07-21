// Package config 配置文件管理测试
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configDirForTest 在测试中临时替换 HOME 来获取配置目录。
func configDirForTest(t *testing.T, dir string) string {
	t.Helper()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })
	os.Setenv("HOME", dir)
	return filepath.Join(dir, ".ddns6")
}

// writeConfig 在测试目录中写入 config.yaml。
func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	cfgDir := filepath.Join(dir, ".ddns6")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// yamlLines 将多行字符串拼接为 YAML 内容。
// 使用 strings.Join 避免 Go 原始字符串中的缩进问题。
func yamlLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// ============================================================
// ConfigDir / ConfigPath 测试
// ============================================================

func TestConfigDir(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() 不应返回错误: %v", err)
	}
	if dir == "" {
		t.Error("ConfigDir() 不应返回空字符串")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("ConfigDir() = %q, 期望绝对路径", dir)
	}
}

func TestConfigPath(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() 不应返回错误: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("ConfigPath() = %q, 期望绝对路径", path)
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("ConfigPath() 文件名应为 config.yaml, 得到 %q", filepath.Base(path))
	}
}

// ============================================================
// Load 测试
// ============================================================

func TestLoad_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() 应返回错误当文件不存在")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("错误信息应提示文件不存在, 得到: %v", err)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)
	writeConfig(t, tmpDir, "invalid: [yaml: broken")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() 应返回错误当 YAML 解析失败")
	}
	if !strings.Contains(err.Error(), "cannot parse config") {
		t.Errorf("错误信息应提示解析失败, 得到: %v", err)
	}
}

func TestLoad_MissingProvider(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)
	writeConfig(t, tmpDir, "domain: example.com\nsubdomains:\n  - www\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() 应返回错误当 provider 缺失")
	}
	if !strings.Contains(err.Error(), "provider") {
		t.Errorf("错误信息应提示 provider 缺失, 得到: %v", err)
	}
}

func TestLoad_MissingDomain(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)
	writeConfig(t, tmpDir, "provider: tencent\nsubdomains:\n  - www\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() 应返回错误当 domain 缺失")
	}
	if !strings.Contains(err.Error(), "domain") {
		t.Errorf("错误信息应提示 domain 缺失, 得到: %v", err)
	}
}

func TestLoad_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)
	writeConfig(t, tmpDir, yamlLines(
		"provider: tencent",
		"domain: example.com",
		"subdomains:",
		"  - www",
		`  - "@"`,
		"auth:",
		`  secret_id: "my-id"`,
		`  secret_key: "my-key"`,
	))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 不应返回错误: %v", err)
	}
	if cfg.Provider != "tencent" {
		t.Errorf("Provider = %q, 期望 %q", cfg.Provider, "tencent")
	}
	if cfg.Domain != "example.com" {
		t.Errorf("Domain = %q, 期望 %q", cfg.Domain, "example.com")
	}
	if len(cfg.Subdomains) != 2 {
		t.Errorf("Subdomains 数量 = %d, 期望 2", len(cfg.Subdomains))
	}
	if cfg.Auth["secret_id"] != "my-id" {
		t.Errorf("Auth.secret_id = %q, 期望 %q", cfg.Auth["secret_id"], "my-id")
	}
	if cfg.Auth["secret_key"] != "my-key" {
		t.Errorf("Auth.secret_key = %q, 期望 %q", cfg.Auth["secret_key"], "my-key")
	}
}

func TestLoad_DefaultSubdomains(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)
	writeConfig(t, tmpDir, "provider: tencent\ndomain: example.com\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 不应返回错误: %v", err)
	}
	if len(cfg.Subdomains) != 1 || cfg.Subdomains[0] != "@" {
		t.Errorf("Subdomains 默认应为 [@], 得到 %v", cfg.Subdomains)
	}
}

func TestLoad_DefaultAuth(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)
	writeConfig(t, tmpDir, "provider: tencent\ndomain: example.com\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 不应返回错误: %v", err)
	}
	if cfg.Auth == nil {
		t.Error("Auth 不应为 nil, Load 应初始化为空 map")
	}
	if len(cfg.Auth) != 0 {
		t.Errorf("Auth 应为空 map, 得到 %v", cfg.Auth)
	}
}

func TestLoad_ConfigDirPermissions(t *testing.T) {
	// 验证权限检查逻辑不会导致 Load 失败
	// （权限警告写入 stderr 而非返回错误）
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)
	writeConfig(t, tmpDir, "provider: tencent\ndomain: example.com\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 不应返回错误: %v", err)
	}
	if cfg.Provider != "tencent" {
		t.Errorf("Provider = %q, 期望 %q", cfg.Provider, "tencent")
	}
}

// ============================================================
// GetInterval / GetTTL 测试
// ============================================================

func TestGetInterval_Default(t *testing.T) {
	c := &Config{}
	d := c.GetInterval()
	if d != 5*60*1000000000 {
		t.Errorf("GetInterval() 默认应为 5m, 得到 %v", d)
	}
}

func TestGetInterval_Empty(t *testing.T) {
	c := &Config{Interval: ""}
	d := c.GetInterval()
	if d != 5*60*1000000000 {
		t.Errorf("GetInterval() 默认为空时应返回 5m, 得到 %v", d)
	}
}

func TestGetInterval_Custom(t *testing.T) {
	c := &Config{Interval: "10m"}
	d := c.GetInterval()
	if d != 10*60*1000000000 {
		t.Errorf("GetInterval() 应为 10m, 得到 %v", d)
	}
}

func TestGetInterval_Invalid(t *testing.T) {
	c := &Config{Interval: "invalid"}
	d := c.GetInterval()
	if d != 5*60*1000000000 {
		t.Errorf("GetInterval() 无效格式时应回退到 5m, 得到 %v", d)
	}
}

func TestGetTTL_Default(t *testing.T) {
	c := &Config{}
	ttl := c.GetTTL()
	if ttl != 600 {
		t.Errorf("GetTTL() 默认应为 600, 得到 %d", ttl)
	}
}

func TestGetTTL_Zero(t *testing.T) {
	c := &Config{TTL: 0}
	ttl := c.GetTTL()
	if ttl != 600 {
		t.Errorf("GetTTL() 零值时应返回 600, 得到 %d", ttl)
	}
}

func TestGetTTL_Custom(t *testing.T) {
	c := &Config{TTL: 300}
	ttl := c.GetTTL()
	if ttl != 300 {
		t.Errorf("GetTTL() 应为 300, 得到 %d", ttl)
	}
}

func TestGetTTL_Negative(t *testing.T) {
	c := &Config{TTL: -1}
	ttl := c.GetTTL()
	if ttl != 600 {
		t.Errorf("GetTTL() 负值时应返回 600, 得到 %d", ttl)
	}
}

// ============================================================
// Generate 测试
// ============================================================

func TestGenerate_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)

	err := Generate(InitParams{
		Provider: "tencent",
		Auth:     map[string]string{"secret_id": "my-id", "secret_key": "my-key"},
		Domain:   "example.com",
	})
	if err != nil {
		t.Fatalf("Generate() 不应返回错误: %v", err)
	}

	// 验证文件已创建
	path := filepath.Join(tmpDir, ".ddns6", "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Generate() 应创建 config.yaml")
	}
}

func TestGenerate_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)

	// 先创建一次
	if err := Generate(InitParams{Domain: "example.com"}); err != nil {
		t.Fatalf("首次 Generate() 不应返回错误: %v", err)
	}

	// 再次创建应返回错误
	err := Generate(InitParams{Domain: "example.com"})
	if err == nil {
		t.Fatal("重复 Generate() 应返回错误")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("错误应提示文件已存在, 得到: %v", err)
	}
}

func TestGenerate_DefaultParams(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)

	err := Generate(InitParams{
		Provider: "cloudflare",
		Domain:   "example.com",
	})
	if err != nil {
		t.Fatalf("Generate() 不应返回错误: %v", err)
	}

	// 读取并验证内容
	data, err := os.ReadFile(filepath.Join(tmpDir, ".ddns6", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "cloudflare") {
		t.Error("生成内容应包含 cloudflare")
	}
	if !strings.Contains(content, "example.com") {
		t.Error("生成内容应包含 example.com")
	}
}
