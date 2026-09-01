// Package config 管理 DDNS6 配置文件（~/.ddns6/config.yaml）。
//
// 配置由 ddns6 init 生成模板，或手动创建；ddns6 run/check/list/clean 等命令通过 Load 读取。
//
// 配置文件格式（YAML）：
//
//	provider: tencent          # 必须：DNS 运营商名称
//	auth:                      # 必须：运营商认证凭据
//	  secret_id: "xxx"
//	  secret_key: "xxx"
//	domain: example.com        # 必须：根域名
//	subdomains:                # 必须：子域名列表
//	  - www
//	  - @
//	interval: 10m              # 可选：非 Linux 轮询间隔（默认 5m）
//	interface: ppp0            # 可选：监听的网络接口（仅 Linux Netlink）
//	ttl: 600                   # 可选：DNS 记录 TTL（默认 600）
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"gopkg.in/yaml.v3"
)

// Config 表示 ~/.ddns6/config.yaml 的完整配置结构。
type Config struct {
	Provider   string            `yaml:"provider"`            // 必填：DNS 运营商名称（如 tencent、cloudflare）
	Auth       map[string]string `yaml:"auth"`                // 必填：运营商认证凭据（字段因运营商而异）
	Domain     string            `yaml:"domain"`              // 必填：根域名（如 example.com）
	Subdomains []string          `yaml:"subdomains"`          // 必填：子域名列表（如 ["www", "@"]）
	Interval   string            `yaml:"interval"`            // 可选：非 Linux 轮询间隔字符串（如 "10m"，默认 5m）
	Interface  string            `yaml:"interface,omitempty"` // 可选：监听的网络接口（仅 Linux Netlink）
	TTL        int               `yaml:"ttl,omitempty"`       // 可选：DNS 记录 TTL 秒数（默认 600）
}

// ConfigDir 返回配置目录路径 ~/.ddns6。
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".ddns6"), nil
}

// ConfigPath 返回配置文件路径 ~/.ddns6/config.yaml。
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load 读取并解析 ~/.ddns6/config.yaml。
//
// 文件不存在或格式错误时返回错误；调用方可据此区分「未初始化」与「解析失败」。
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s (use 'ddns6 init' to create one)", path)
		}
		return nil, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	// Unix 上权限过宽时仅警告，不阻断加载（凭据可能泄露）
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(path); err == nil {
			if fi.Mode().Perm()&0077 != 0 {
				fmt.Fprintf(os.Stderr, "Warning: config file %s has world/group-readable permissions (%03o), consider 'chmod 600'\n",
					path, fi.Mode().Perm())
			}
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file %s: %w", path, err)
	}

	if cfg.Provider == "" {
		return nil, fmt.Errorf("config field 'provider' is required")
	}
	if cfg.Domain == "" {
		return nil, fmt.Errorf("config field 'domain' is required")
	}
	if len(cfg.Subdomains) == 0 {
		cfg.Subdomains = []string{"@"} // 缺省同步根域名
	}
	if cfg.Auth == nil {
		cfg.Auth = make(map[string]string)
	}

	return &cfg, nil
}

// GetInterval 解析轮询间隔字符串为 time.Duration。
//
// 未设置或解析失败时返回默认 5 分钟；解析失败时同时返回错误说明。
func (c *Config) GetInterval() (time.Duration, error) {
	if c.Interval == "" {
		return 5 * time.Minute, nil
	}
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return 5 * time.Minute, fmt.Errorf("invalid interval '%s': %w, using default 5m", c.Interval, err)
	}
	return d, nil
}

// GetTTL 返回 TTL；未设置或非正数时使用 ddns.DefaultTTL。
func (c *Config) GetTTL() int {
	if c.TTL <= 0 {
		return ddns.DefaultTTL
	}
	return c.TTL
}

// InitParams 为 ddns6 init 的可选预填参数。
//
// 空值/零值表示不预填，相应字段在生成的配置中保持注释示例。
type InitParams struct {
	Provider   string            // DNS 运营商名称（如 tencent）
	Auth       map[string]string // 认证凭据（如 secret_id, secret_key）
	Domain     string
	Subdomains []string
	TTL        int
	Interval   string
	Interface  string
}

// configTemplate 配置模板：有值字段写入配置，空值保留为注释示例。
const configTemplate = `# DDNS6 配置文件
# 编辑后执行 ddns6 run 即可启动服务
#
# 各字段说明见下方注释，更多信息请参考 ddns6 run --help

# 必填：DNS 运营商名称
# 支持: tencent, cloudflare, alicloud, godaddy, huaweicloud, duckdns,
#       noip, he, dynv6, porkbun, digitalocean, baiducloud, dnspod
provider: "{{.Provider}}"

# 必填：运营商认证凭据（不同运营商字段不同）
{{if .Auth}}auth:
{{- range $k, $v := .Auth}}
  {{$k}}: "{{$v}}"{{end}}
{{else}}auth: {}
  # tencent 示例：
  # secret_id: "your-secret-id"
  # secret_key: "your-secret-key"
  # cloudflare 示例：
  # api_token: "your-api-token"
  # 阿里云示例：
  # access_key_id: "your-access-key-id"
  # access_key_secret: "your-access-key-secret"
{{end}}
# 必填：根域名
domain: "{{.Domain}}"

# 必填：子域名列表（可多个，每个占一行）
# 使用 "@" 表示根域名
subdomains:{{if .Subdomains}}{{range .Subdomains}}
  - "{{.}}"{{end}}{{else}}
  - "@"{{end}}

# 可选：非 Linux 平台的轮询间隔
# 格式：数字+单位（s=秒, m=分, h=时），默认 5m
# Linux 平台由 Netlink 事件驱动，此选项无效
{{if .Interval}}interval: {{.Interval}}{{else}}# interval: 5m{{end}}

# 可选：监听的网络接口（仅 Linux Netlink 模式有效）
# 指定后只监听该接口的 IPv6 地址变化
# 不指定则监听所有接口
{{if .Interface}}interface: {{.Interface}}{{else}}# interface: ppp0{{end}}

# 可选：DNS 记录 TTL，单位秒，默认 600
{{if .TTL}}ttl: {{.TTL}}{{else}}# ttl: 600{{end}}
`

// Generate 创建 ~/.ddns6/ 目录并写入 config.yaml（已存在则拒绝覆盖）。
//
// params 中非零字段预填入配置；零值字段保留为注释默认值。
func Generate(params InitParams) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory %s: %w", dir, err)
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists at %s", path)
	}

	tmpl, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return fmt.Errorf("internal error: failed to parse config template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return fmt.Errorf("cannot render config: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("cannot write config file %s: %w", path, err)
	}

	fmt.Printf("Configuration file created at: %s\n", path)
	switch {
	case params.Provider != "" && len(params.Auth) > 0:
		fmt.Println("Configuration is complete. Run: ddns6 run")
	case params.Provider != "":
		fmt.Println("Set your auth credentials in the config, then run: ddns6 run")
	case params.Domain != "":
		fmt.Println("Set your provider and auth in the config, then run: ddns6 run")
	default:
		fmt.Println("Edit it with your provider details, then run: ddns6 run")
	}
	return nil
}
