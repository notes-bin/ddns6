package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configDirForTest 临时替换 HOME，使配置路径落在测试目录下。
func configDirForTest(t *testing.T, dir string) string {
	t.Helper()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })
	os.Setenv("HOME", dir)
	return filepath.Join(dir, ".ddns6")
}

// writeConfig 在测试目录写入 config.yaml。
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

// yamlLines 将多行拼接为 YAML，避免原始字符串缩进干扰。
func yamlLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// TestConfigDir 验证返回非空绝对路径。
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

// TestConfigPath 验证路径为绝对路径且文件名为 config.yaml。
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

// TestLoad_FileNotFound 验证配置文件缺失时返回明确错误。
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

// TestLoad_InvalidYAML 验证 YAML 损坏时返回解析错误。
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

// TestLoad_MissingProvider 验证缺少 provider 时失败。
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

// TestLoad_MissingDomain 验证缺少 domain 时失败。
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

// TestLoad_Success 验证合法配置可完整解析。
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

// TestLoad_DefaultSubdomains 验证未配置子域名时默认为根域名 "@"。
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

// TestLoad_DefaultAuth 验证缺失 auth 时初始化为空 map。
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

// TestLoad_ConfigDirPermissions 验证权限警告不导致 Load 失败。
func TestLoad_ConfigDirPermissions(t *testing.T) {
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

// TestGetInterval_Default 验证零值 Config 的间隔默认为 5m。
func TestGetInterval_Default(t *testing.T) {
	c := &Config{}
	d, err := c.GetInterval()
	if err != nil {
		t.Fatalf("GetInterval() 不应返回错误: %v", err)
	}
	if d != 5*60*1000000000 {
		t.Errorf("GetInterval() 默认应为 5m, 得到 %v", d)
	}
}

// TestGetInterval_Empty 验证空字符串间隔回退到 5m。
func TestGetInterval_Empty(t *testing.T) {
	c := &Config{Interval: ""}
	d, err := c.GetInterval()
	if err != nil {
		t.Fatalf("GetInterval() 不应返回错误: %v", err)
	}
	if d != 5*60*1000000000 {
		t.Errorf("GetInterval() 默认为空时应返回 5m, 得到 %v", d)
	}
}

// TestGetInterval_Custom 验证自定义间隔字符串可正确解析。
func TestGetInterval_Custom(t *testing.T) {
	c := &Config{Interval: "10m"}
	d, err := c.GetInterval()
	if err != nil {
		t.Fatalf("GetInterval() 不应返回错误: %v", err)
	}
	if d != 10*60*1000000000 {
		t.Errorf("GetInterval() 应为 10m, 得到 %v", d)
	}
}

// TestGetInterval_Invalid 验证非法间隔回退 5m 并返回错误。
func TestGetInterval_Invalid(t *testing.T) {
	c := &Config{Interval: "invalid"}
	d, err := c.GetInterval()
	if err == nil {
		t.Error("GetInterval() 无效格式时应返回错误")
	}
	if d != 5*60*1000000000 {
		t.Errorf("GetInterval() 无效格式时应回退到 5m, 得到 %v", d)
	}
}

// TestGetTTL_Default 验证未设置 TTL 时使用默认值 600。
func TestGetTTL_Default(t *testing.T) {
	c := &Config{}
	ttl := c.GetTTL()
	if ttl != 600 {
		t.Errorf("GetTTL() 默认应为 600, 得到 %d", ttl)
	}
}

// TestGetTTL_Zero 验证 TTL 为 0 时回退默认值。
func TestGetTTL_Zero(t *testing.T) {
	c := &Config{TTL: 0}
	ttl := c.GetTTL()
	if ttl != 600 {
		t.Errorf("GetTTL() 零值时应返回 600, 得到 %d", ttl)
	}
}

// TestGetTTL_Custom 验证自定义正数 TTL 原样返回。
func TestGetTTL_Custom(t *testing.T) {
	c := &Config{TTL: 300}
	ttl := c.GetTTL()
	if ttl != 300 {
		t.Errorf("GetTTL() 应为 300, 得到 %d", ttl)
	}
}

// TestGetTTL_Negative 验证负 TTL 回退默认值。
func TestGetTTL_Negative(t *testing.T) {
	c := &Config{TTL: -1}
	ttl := c.GetTTL()
	if ttl != 600 {
		t.Errorf("GetTTL() 负值时应返回 600, 得到 %d", ttl)
	}
}

// TestGenerate_Success 验证 Generate 可创建配置文件。
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

	path := filepath.Join(tmpDir, ".ddns6", "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Generate() 应创建 config.yaml")
	}
}

// TestGenerate_AlreadyExists 验证已存在配置时拒绝覆盖。
func TestGenerate_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	configDirForTest(t, tmpDir)

	if err := Generate(InitParams{Domain: "example.com"}); err != nil {
		t.Fatalf("首次 Generate() 不应返回错误: %v", err)
	}

	err := Generate(InitParams{Domain: "example.com"})
	if err == nil {
		t.Fatal("重复 Generate() 应返回错误")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("错误应提示文件已存在, 得到: %v", err)
	}
}

// TestGenerate_DefaultParams 验证预填字段写入生成内容。
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
