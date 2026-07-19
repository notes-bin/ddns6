// Package config 管理 DDNS6 配置文件 (~/.ddns6/config.yaml)。
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
//
// 配置文件通过 ddns6 init 生成模板，或手动创建。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/notes-bin/ddns6/internal/providers"
	"gopkg.in/yaml.v3"
)

// Config 表示 ~/.ddns6/config.yaml 的完整配置结构。
type Config struct {
	Provider   string            `yaml:"provider"`             // DNS 运营商名称（如 tencent、cloudflare）
	Auth       map[string]string `yaml:"auth"`                 // 运营商认证凭据（不同运营商字段不同）
	Domain     string            `yaml:"domain"`               // 根域名（如 example.com）
	Subdomains []string          `yaml:"subdomains"`           // 子域名列表（如 ["www", "@"]）
	Interval   string            `yaml:"interval"`             // 轮询间隔字符串（如 "10m"、"5m"）
	Interface  string            `yaml:"interface,omitempty"`  // 监听的网络接口（可选，仅 Linux）
	TTL        int               `yaml:"ttl,omitempty"`        // DNS 记录 TTL（可选，默认 600）
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

// Load 读取并解析 ~/.ddns6/config.yaml，返回 Config 结构体。
//
// 如果文件不存在或格式错误，返回错误。
// 调用方可根据错误类型判断是"文件不存在"还是"解析错误"。
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

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file %s: %w", path, err)
	}

	// 必填字段校验
	if cfg.Provider == "" {
		return nil, fmt.Errorf("config field 'provider' is required")
	}
	if cfg.Domain == "" {
		return nil, fmt.Errorf("config field 'domain' is required")
	}
	if len(cfg.Subdomains) == 0 {
		cfg.Subdomains = []string{"@"} // 默认根域名
	}
	if cfg.Auth == nil {
		cfg.Auth = make(map[string]string)
	}

	return &cfg, nil
}

// GetInterval 解析轮询间隔字符串为 time.Duration。
// 如果未设置或解析失败，返回默认值 5 分钟。
func (c *Config) GetInterval() time.Duration {
	if c.Interval == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

// GetTTL 返回 TTL 值，未设置时返回默认值。
func (c *Config) GetTTL() int {
	if c.TTL <= 0 {
		return providers.DefaultTTL
	}
	return c.TTL
}

// Generate 创建 ~/.ddns6/ 目录并写入带注释的 config.yaml 模板。
//
// 如果目录已存在但配置文件已存在，不会覆盖。
// 首次使用 ddns6 init 命令时会调用此函数。
func Generate() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	// 创建目录（如果不存在）
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory %s: %w", dir, err)
	}

	// 检查配置文件是否已存在
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists at %s", path)
	}

	// 生成带详细注释的模板
	template := `# DDNS6 配置文件
# 编辑后执行 ddns6 run 即可启动服务
#
# 各字段说明见下方注释，更多信息请参考 ddns6 run --help

# 必填：DNS 运营商名称
# 支持: tencent, cloudflare, alicloud, godaddy, huaweicloud, duckdns,
#       noip, he, dynv6, porkbun, digitalocean, baiducloud, dnspod
provider: ""

# 必填：运营商认证凭据（不同运营商字段不同）
# 查看各运营商所需字段请运行: ddns6 run <provider> --help
auth: {}
  # tencent 示例：
  # secret_id: "your-secret-id"
  # secret_key: "your-secret-key"
  # cloudflare 示例：
  # api_token: "your-api-token"
  # 阿里云示例：
  # access_key_id: "your-access-key-id"
  # access_key_secret: "your-access-key-secret"

# 必填：根域名
domain: ""

# 必填：子域名列表（可多个，每个占一行）
# 使用 "@" 表示根域名
subdomains:
  - "@"

# 可选：非 Linux 平台的轮询间隔
# 格式：数字+单位（s=秒, m=分, h=时），默认 5m
# Linux 平台由 Netlink 事件驱动，此选项无效
# interval: 5m

# 可选：监听的网络接口（仅 Linux Netlink 模式有效）
# 指定后只监听该接口的 IPv6 地址变化
# 不指定则监听所有接口
# interface: ppp0

# 可选：DNS 记录 TTL，单位秒，默认 600
# ttl: 600
`

	if err := os.WriteFile(path, []byte(template), 0644); err != nil {
		return fmt.Errorf("cannot write config file %s: %w", path, err)
	}

	fmt.Printf("Configuration file created at: %s\n", path)
	fmt.Println("Edit it with your provider details, then run: ddns6 run")
	return nil
}
