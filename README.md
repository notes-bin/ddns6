# DDNS6 — IPv6 动态域名解析工具

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](go.mod)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

自动检测本机 IPv6 地址变化，实时更新到 DNS 服务商的 AAAA 记录。支持 **13 个 DNS 运营商**，Linux 上通过 Netlink 事件驱动、其他平台定时轮询。

---

## 快速开始

### 安装

```bash
git clone https://github.com/notes-bin/ddns6.git
cd ddns6
go build -o ddns6 .
sudo mv ddns6 /usr/local/bin/
```

或者直接下载 [GitHub Releases](https://github.com/notes-bin/ddns6/releases) 的预编译二进制。

### 临时运行（单次测试）

```bash
# 腾讯云 DNSPod
ddns6 run tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY
```

首次运行会立即获取 IPv6 地址并更新 DNS 记录。Linux 上后续通过 Netlink 实时监听变化，其他平台每 5 分钟轮询一次。

### 使用配置文件（长期服务）

```bash
# 一键生成完整配置
ddns6 init tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY

# 启动服务
ddns6 run
```

### 验证配置

```bash
# 检查配置文件和 API 连通性（不会修改任何记录）
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

---

## 命令参考

### 全局参数

所有命令都支持以下参数（可通过 `DDNS6_*` 环境变量设置）：

| 参数 | 环境变量 | 类型 | 默认值 | 说明 |
|------|---------|------|--------|------|
| `--domain` | `DDNS6_DOMAIN` | string | — | 根域名（如 example.com） |
| `--subdomain` | `DDNS6_SUBDOMAIN` | string[] | `@` | 子域名，可多次指定 |
| `--ttl` | `DDNS6_TTL` | int | `600` | DNS 记录 TTL（秒） |
| `--interval` | `DDNS6_INTERVAL` | duration | `5m` | 非 Linux 轮询间隔 |
| `--interface` | `DDNS6_INTERFACE` | string | — | 网络接口（仅 Linux） |
| `--log-file` | `DDNS6_LOG_FILE` | string | `ddns6.log` | 日志路径，`""`=仅 stderr |
| `--debug` | `DDNS6_DEBUG` | bool | `false` | 调试日志 |
| `-V / --version` | — | bool | `false` | 版本信息 |

优先级：**命令行 > 环境变量 > 配置文件**。

### `ddns6 run [provider]`

启动 DDNS 服务。

```bash
# 从配置文件读取
ddns6 run

# 命令行参数
ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

# 多子域名
ddns6 run cloudflare --domain example.com --subdomain www --subdomain @ --api-token xxx

# 调试模式
ddns6 run --debug tencent --domain example.com --secret-id xxx --secret-key yyy

# 环境变量
export DDNS6_DOMAIN=example.com DDNS6_SUBDOMAIN=www
ddns6 run tencent --secret-id xxx --secret-key yyy
```

### `ddns6 check [provider]`

验证配置和 API 连通性，**不会修改任何 DNS 记录**。

```bash
# 验证配置文件
ddns6 check

# 验证命令行参数
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

检查项：配置文件解析 → Provider 名称 → 认证参数完整性 → API 连通性测试。

### `ddns6 list [provider]`

列出 DNS 记录。

```bash
# 列出 AAAA 记录
ddns6 list tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

# 列出所有类型
ddns6 list tencent --domain example.com --type "" --secret-id xxx --secret-key yyy

# 不带 --subdomain 则展示该域名下所有记录
ddns6 list tencent --domain example.com --secret-id xxx --secret-key yyy
```

> ⚠️ duckdns、he、noip 不支持 list（API 仅提供更新接口）。

### `ddns6 clean [provider]`

删除 DNS 记录。

```bash
# 预览（不实际执行）
ddns6 clean tencent --domain example.com --subdomain www --dry-run \
  --secret-id xxx --secret-key yyy

# 交互式删除
ddns6 clean tencent --domain example.com --subdomain www \
  --secret-id xxx --secret-key yyy

# 自动删除（跳过确认）
ddns6 clean tencent --domain example.com --subdomain www --yes \
  --secret-id xxx --secret-key yyy
```

安全特性：删除前列表确认、`--dry-run` 预览、`--yes` 跳过确认、并发限流。

> ⚠️ duckdns、he、noip 不支持 clean。

### `ddns6 init [provider]`

生成 `~/.ddns6/config.yaml`。

```bash
# 仅模板（手动编辑）
ddns6 init

# 预填域名
ddns6 init --domain example.com --subdomain www --subdomain @

# 完整配置
ddns6 init tencent --domain example.com --secret-id xxx --secret-key yyy
```

### `ddns6 completion [bash|zsh|fish|powershell]`

生成 Shell 自动补全脚本。

```bash
# Bash
ddns6 completion bash > /etc/bash_completion.d/ddns6
source /etc/bash_completion.d/ddns6
```

需要安装 `bash-completion` 包：

```bash
# Debian/Ubuntu
apt install bash-completion -y

# RHEL/CentOS
yum install bash-completion -y
```

---

## 支持的 DNS 运营商

| 运营商 | CLI 名称 | 必填参数 | 配置文件字段 |
|--------|---------|---------|-------------|
| **腾讯云 DNSPod** | `tencent` | `--secret-id` `--secret-key` | `secret_id` `secret_key` |
| **Cloudflare** | `cloudflare` | `--api-token` | `api_token` |
| **阿里云 DNS** | `alicloud` | `--access-key-id` `--access-key-secret` | `access_key_id` `access_key_secret` |
| **GoDaddy** | `godaddy` | `--api-key` `--api-secret` | `api_key` `api_secret` |
| **华为云 DNS** | `huaweicloud` | `--access-key` `--secret-key` | `access_key` `secret_key` |
| **百度云 BCD** | `baiducloud` | `--access-key` `--secret-key` | `access_key` `secret_key` |
| **DigitalOcean** | `digitalocean` | `--token` | `token` |
| **DNSPod 旧版** | `dnspod` | `--login-token` | `login_token`（格式：`ID,Token`）|
| **Porkbun** | `porkbun` | `--api-key` `--api-secret` | `api_key` `api_secret` |
| **DuckDNS** 🚫 | `duckdns` | `--token` | `token` |
| **HE** 🚫 | `he` | `--password` | `password` |
| **No-IP** 🚫 | `noip` | `--username` `--password` | `username` `password` |
| **Dynv6** | `dynv6` | `--token` | `token` |

🚫 = 受限 API（仅更新接口，不支持 list/clean）。

各运营商详细参数运行 `ddns6 run <name> --help` 查看。

### 阿里云 V3 签名

阿里云默认为 V1 签名（HMAC-SHA1），可切换到 V3（ACS3-HMAC-SHA256）：

```bash
ddns6 run alicloud --domain example.com --sign-version v3 \
  --access-key-id xxx --access-key-secret yyy
```

配置文件中设置：
```yaml
auth:
  access_key_id: "xxx"
  access_key_secret: "yyy"
  sign_version: "v3"
```

---

## 配置文件格式

`~/.ddns6/config.yaml`：

```yaml
provider: "tencent"          # 必填：运营商名称
auth:                        # 必填：认证凭据
  secret_id: "xxx"
  secret_key: "xxx"
domain: "example.com"        # 必填：根域名
subdomains:                  # 必填：子域名列表
  - "www"
  - "@"                      # "@" 表示根域名
# interval: 5m               # 可选：轮询间隔
# interface: ppp0            # 可选：网络接口（仅 Linux）
# ttl: 600                   # 可选：TTL（默认 600 秒）
```

---

## 配置 Shell 自动补全

```bash
# 1. 安装 bash-completion
apt install bash-completion -y
# 或 yum install bash-completion -y

# 2. 生成并安装补全脚本
ddns6 completion bash > /etc/bash_completion.d/ddns6

# 3. 重新加载（或新开 shell）
source /etc/bash_completion.d/ddns6
```

之后输入 `ddns6 ␣␣`（按两次 Tab）即可看到子命令列表，`ddns6 run ␣␣` 看到 13 个 provider 名称。

---

## 部署为 systemd 服务

创建 `/etc/systemd/system/ddns6.service`：

```ini
[Unit]
Description=DDNS6 IPv6 Dynamic DNS Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ddns6 run
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
# 先完成配置
ddns6 init tencent --domain example.com --subdomain www \
  --secret-id xxx --secret-key yyy

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable --now ddns6
sudo journalctl -u ddns6 -f   # 查看日志
```

---

## 安全注意事项

- **配置文件权限**：`~/.ddns6/config.yaml` 包含 API 密钥，建议 `chmod 600`
- 运行 `ddns6 check` 或 `ddns6 run` 时如果权限过松会输出警告
- 日志中不会记录 secret key、token 等敏感信息
- 建议为 DDNS 创建专用 API 令牌，仅授予 DNS 编辑权限

---

## 架构说明

### 触发器（地址变化怎么被发现的）

| 平台 | 模式 | 说明 |
|------|------|------|
| Linux | Netlink 事件驱动 | 实时监听内核地址变化，PPPoE 重拨后秒级触发 |
| macOS / Windows | 定时轮询 | 每 5 分钟检测一次（可配置） |

### 防抖（Debounce）

PPPoE 重拨时地址可能短时间内多次变化。DDNS6 检测到新地址后等待 10 秒防抖窗口，窗口内每次新事件重置计时器，地址稳定后才执行 DNS 更新。

### 同步流程

```
地址变化 → GetIPv6Addr(多源并发请求) → 对比缓存
  ├─ 未变化 → 跳过
  └─ 已变化 → 并发同步所有子域名
        ├─ GetRecords(查询现有记录)
        ├─ 遍历记录：IP 相同→跳过，不同→ModifyRecord
        └─ 无记录 → AddRecord
```

### IPv6 获取源

每次随机打乱顺序，多个来源并发竞速，首个成功即返回：

| 来源 | 类型 |
|------|------|
| `https://6.ipw.cn` | HTTP |
| `https://ifconfig.co` | HTTP |
| `https://v6.ident.me` | HTTP |
| `2402:4e00::`（AliDNS） | DNS |
| `2400:3200:baba::1`（BaiduDNS） | DNS |
| `2001:4860:4860::8888`（Google） | DNS |
| `2606:4700:4700::1111`（Cloudflare） | DNS |

---

## 项目结构

```
ddns6/
├── main.go                    # 程序入口
├── cmd/                       # CLI 命令定义
│   ├── root.go                # 根命令、全局参数、环境变量
│   ├── check.go               # ddns6 check
│   ├── list.go                # ddns6 list
│   ├── clean.go               # ddns6 clean
│   └── providers.go           # 13 个 provider 的工厂注册
├── internal/
│   ├── config/                # 配置加载、生成
│   ├── crypto/                # 密码学工具
│   └── ddns/                  # 核心服务编排
│       ├── types.go           # RecordInfo、DNSProvider 接口
│       ├── service.go         # RunService 主循环
│       ├── record.go          # DNS 记录同步
│       ├── match.go           # 记录名匹配、地址比较
│       ├── processor.go       # CollectMatchingRecords
│       └── display.go         # 格式化输出
│   └── providers/             # 13 个运营商实现
│       ├── tencent/           # 腾讯云 DNSPod
│       ├── alicloud/          # 阿里云 DNS
│       ├── baiducloud/        # 百度云 BCD
│       ├── cloudflare/        # Cloudflare DNS
│       ├── digitalocean/      # DigitalOcean
│       ├── dnspod/            # DNSPod 旧版
│       ├── duckdns/           # DuckDNS
│       ├── dynv6/             # Dynv6
│       ├── godaddy/           # GoDaddy
│       ├── he/                # Hurricane Electric
│       ├── huaweicloud/       # 华为云 DNS
│       ├── noip/              # No-IP
│       └── porkbun/           # Porkbun
├── pkg/
│   ├── domainutil/            # 域名工具（SplitDomain）
│   ├── ipaddr/                # IPv6 地址获取
│   └── retry/                 # 指数退避重试
└── .github/workflows/
    └── release.yml            # CI/CD 流水线
```

---

## 开发

```bash
go build ./...        # 编译
go test ./...         # 测试
go vet ./...          # 静态分析
go mod tidy           # 整理依赖

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o ddns6-linux .
GOOS=darwin GOARCH=arm64 go build -o ddns6-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o ddns6.exe .
```

### 添加新运营商

1. 在 `internal/providers/` 下创建新包
2. 实现 `ddns.DNSProvider` 接口（4 个 CRUD 方法）
3. 在 `cmd/providers.go` 的 `providerFactories` 列表注册
4. 运行 `go test ./...` 确认通过

---

## 常见问题

**Q: 怎么知道自己当前的 IPv6 地址？**
```bash
curl -6 https://6.ipw.cn
```

**Q: 如何测试配置是否有效？**  
```bash
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

**Q: Netlink 需要 root 权限吗？**  
读取 `NETLINK_ROUTE` 通常不需要 root。如果遇到权限错误，自动回退到定时轮询。

**Q: 支持 A 记录（IPv4）吗？**  
不支持。本项目专注 IPv6 DDNS（项目名称 ddns6 即反映此目标）。

**Q: Docker 部署需要 `--network host` 吗？**  
是的。Netlink 需要主机的网络命名空间。

**Q: 日志文件越来越大怎么办？**  
使用 `--log-file ""` 禁用文件日志，或配合 logrotate 轮转：
```bash
# /etc/logrotate.d/ddns6
/var/log/ddns6.log {
    daily; rotate 7; compress; missingok; copytruncate
}
```

---

## 许可证

MIT License — 详见 [LICENSE](LICENSE)
