# DDNS6 — IPv6 动态域名解析工具

[![Go Version](https://img.shields.io/badge/Go-1.27.1+-00ADD8?logo=go)](go.mod)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Test](https://github.com/notes-bin/ddns6/actions/workflows/test.yml/badge.svg)](https://github.com/notes-bin/ddns6/actions/workflows/test.yml)

自动检测本机 IPv6 地址变化，实时更新到 DNS 服务商的 AAAA 记录。支持 **23 家 DNS 运营商**；Linux 通过 Netlink 事件驱动，其他平台定时轮询。

---

## 快速开始

### 安装

```bash
git clone https://github.com/notes-bin/ddns6.git
cd ddns6
go build -o ddns6 .
sudo mv ddns6 /usr/local/bin/
```

或使用 Makefile：

```bash
make build          # 输出到 bin/ddns6
sudo make install   # 安装到 GOPATH/bin
```

也可直接下载 [GitHub Releases](https://github.com/notes-bin/ddns6/releases) 的预编译二进制（由 `release.yml` 在推送 `v*` tag 时自动构建）。

### 临时运行（单次测试）

```bash
# 腾讯云 DNSPod
ddns6 run tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY
```

首次运行会立即获取 IPv6 并更新 DNS。Linux 后续由 Netlink 实时监听；其他平台默认每 5 分钟轮询一次。

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
| `--interface` | `DDNS6_INTERFACE` | string | — | 网络接口（仅 Linux Netlink） |
| `--log-file` | `DDNS6_LOG_FILE` | string | `ddns6.log` | 日志路径，`""`=仅 stderr |
| `--debug` | `DDNS6_DEBUG` | bool | `false` | 调试日志 |
| `-V` / `--version` | — | bool | `false` | 版本信息（等同于 `ddns6 version`） |

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
ddns6 check
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

检查项：配置文件解析 → Provider 名称 → 认证参数完整性 → API 连通性。

### `ddns6 list`

列出全部可用 DNS 运营商（不读配置、不访问网络）。

输出表格包含 CLI 名称、是否支持 `records`/`clean`，以及简要说明。

```bash
ddns6 list
```

### `ddns6 records [provider]`

列出 DNS 记录。

命令专属参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--type` | `AAAA` | 记录类型过滤；设为 `""` 表示不过滤类型 |

```bash
# 列出 AAAA 记录
ddns6 records tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

# 列出所有类型
ddns6 records tencent --domain example.com --type "" --secret-id xxx --secret-key yyy

# 不带 --subdomain 则展示该域名下匹配类型的全部记录
ddns6 records tencent --domain example.com --secret-id xxx --secret-key yyy
```

**受限运营商**：`duckdns`、`he`、`noip` 的 API 仅提供更新端点，不支持记录查询。对上述运营商执行 `records` 会返回明确错误提示，请使用各服务商 Web 面板管理记录。

### `ddns6 clean [provider]`

删除 DNS 记录。

命令专属参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--type` | `AAAA` | 记录类型过滤 |
| `--dry-run` | `false` | 仅预览，不实际删除 |
| `--yes` | `false` | 跳过交互确认（适合脚本） |

```bash
# 预览
ddns6 clean tencent --domain example.com --subdomain www --dry-run \
  --secret-id xxx --secret-key yyy

# 交互式删除
ddns6 clean tencent --domain example.com --subdomain www \
  --secret-id xxx --secret-key yyy

# 自动删除
ddns6 clean tencent --domain example.com --subdomain www --yes \
  --secret-id xxx --secret-key yyy
```

安全特性：删除前列表确认、`--dry-run` 预览、`--yes` 跳过确认、并发限流（最多 5）。

**受限运营商**：`duckdns`、`he`、`noip` 同样不支持 `clean`。

### `ddns6 init [provider]`

生成 `~/.ddns6/config.yaml`。

```bash
ddns6 init
ddns6 init --domain example.com --subdomain www --subdomain @
ddns6 init tencent --domain example.com --secret-id xxx --secret-key yyy
```

### `ddns6 version`

显示版本、提交与构建时间（也可用 `-V` / `--version`）。

```bash
ddns6 version
ddns6 -V
```

### `ddns6 completion [bash|zsh|fish|powershell]`

生成 Shell 自动补全脚本。

```bash
# Bash（需已安装 bash-completion）
ddns6 completion bash > /etc/bash_completion.d/ddns6
source /etc/bash_completion.d/ddns6
```

```bash
# Debian/Ubuntu
apt install bash-completion -y

# RHEL/CentOS
yum install bash-completion -y
```

---

## 支持的 DNS 运营商

共 **23 家**，注册于 `cmd/providers.go` 的 `providerFactories`。

| 运营商 | CLI 名称 | 必填参数 | 配置文件字段 (`auth`) | records/clean | 说明 |
|--------|---------|---------|----------------------|------------|------|
| 腾讯云 DNSPod | `tencent` | `--secret-id` `--secret-key` | `secret_id` `secret_key` | 支持 | API v3 |
| Cloudflare | `cloudflare` | `--api-token` | `api_token` | 支持 | 需 DNS:Edit 权限 |
| 阿里云 DNS | `alicloud` | `--access-key-id` `--access-key-secret` | `access_key_id` `access_key_secret` | 支持 | 可选 `sign_version`（见下文） |
| GoDaddy | `godaddy` | `--api-key` `--api-secret` | `api_key` `api_secret` | 支持 | |
| 华为云 DNS | `huaweicloud` | `--access-key` `--secret-key` | `access_key` `secret_key` | 支持 | |
| 百度云 BCD | `baiducloud` | `--access-key` `--secret-key` | `access_key` `secret_key` | 支持 | |
| DigitalOcean | `digitalocean` | `--token` | `token` | 支持 | |
| DNSPod 旧版 | `dnspod` | `--login-token` | `login_token` | 支持 | 格式 `ID,Token` |
| Porkbun | `porkbun` | `--api-key` `--api-secret` | `api_key` `api_secret` | 支持 | |
| DuckDNS | `duckdns` | `--token` | `token` | **受限** | 仅更新端点 |
| Hurricane Electric | `he` | `--password` | `password` | **受限** | DDNS Key，仅更新端点 |
| No-IP | `noip` | `--username` `--password` | `username` `password` | **受限** | 经典 DDNS，仅更新端点 |
| Dynv6 | `dynv6` | `--token` | `token` | 支持 | |
| deSEC.io | `desec` | `--token` | `token` | 支持 | |
| Linode (Akamai) | `linode` | `--api-key` | `api_key` | 支持 | DNS API v4 |
| NameSilo | `namesilo` | `--api-key` | `api_key` | 支持 | |
| IONOS | `ionos` | `--prefix` `--secret` | `prefix` `secret` | 支持 | |
| Hetzner Cloud | `hetzner` | `--token` | `token` | 支持 | 需 DNS 权限 |
| AWS Route 53 | `aws` | `--access-key-id` `--secret-access-key` | `access_key_id` `secret_access_key` | 支持 | 可选 `session_token` |
| Google Cloud DNS | `gcloud` | `--project` `--access-token` | `project` `access_token` | 支持 | REST API，不依赖 gcloud CLI |
| Azure DNS | `azure` | `--subscription-id` `--tenant-id` `--client-id` `--client-secret` | 同左 snake_case | 支持 | Service Principal |
| Namecheap | `namecheap` | `--api-key` `--username` `--client-ip` | `api_key` `username` `client_ip` | 支持 | 需 API 白名单 IP |
| DNSPod 国际版 | `dpi` | `--login-token` | `login_token` | 支持 | 格式 `ID,Key` |

各运营商详细参数运行 `ddns6 run <name> --help` 查看。

### 阿里云 V3 签名

阿里云默认为 V1（HMAC-SHA1），可切换到 V3（ACS3-HMAC-SHA256）：

```bash
ddns6 run alicloud --domain example.com --sign-version v3 \
  --access-key-id xxx --access-key-secret yyy
```

配置文件：

```yaml
auth:
  access_key_id: "xxx"
  access_key_secret: "yyy"
  sign_version: "v3"
```

---

## 配置文件格式

默认路径：`~/.ddns6/config.yaml`。

```yaml
provider: "tencent"          # 必填：运营商名称（见上表 CLI 名称）
auth:                        # 必填：认证凭据
  secret_id: "xxx"
  secret_key: "xxx"
domain: "example.com"        # 必填：根域名
subdomains:                  # 必填：子域名列表
  - "www"
  - "@"                      # "@" 表示根域名本身
# interval: 5m               # 可选：非 Linux 轮询间隔
# interface: ppp0            # 可选：网络接口（仅 Linux）
# ttl: 600                   # 可选：TTL（默认 600 秒）
```

建议：`chmod 600 ~/.ddns6/config.yaml`。

---

## Docker 部署

镜像为多阶段构建（`golang:1.27.1-alpine` → 固定 `alpine:3.21`），以非 root 用户 `ddns6`（uid 10001）运行，支持 `TARGETARCH` 多架构。Linux 上使用 Netlink 时需要主机网络命名空间。

### 前置条件

- Linux 主机（`network_mode: host` 在 Docker Desktop / macOS 上无效或意义不同）
- 已安装 Docker 与 Compose

### 方式一：配置文件挂载（推荐）

密钥留在 `~/.ddns6/config.yaml`，不会出现在 `docker inspect` / 进程参数中。

```bash
ddns6 init tencent --domain example.com --subdomain www \
  --secret-id xxx --secret-key yyy
chmod 600 ~/.ddns6/config.yaml
```

在 `docker-compose.yml` 中取消注释 `ddns6-config` 服务（挂载路径为 `/home/ddns6/.ddns6`），然后：

```bash
make docker-build
docker compose up -d ddns6-config
```

### 方式二：环境变量 + Compose

```bash
cp .env.example .env
# 编辑 .env：填写 DOMAIN、SUBDOMAIN 以及所选运营商的凭证
# 默认启用 ddns6-tencent；其他运营商服务需在 docker-compose.yml 中取消注释
# 注意：CLI 模式会把密钥放进容器命令行，仅适合受控环境

make docker-up      # docker compose up -d
make docker-logs    # 跟踪日志
make docker-down    # 停止并删除
```

### 安全要点

| 项 | 说明 |
|----|------|
| 非 root 用户 | 镜像内 `ddns6:ddns6`（uid/gid 10001） |
| `network_mode: host` | 与主机共用网络栈，Netlink 才能感知地址变化 |
| `cap_drop: ALL` + `cap_add: NET_ADMIN` | 仅保留订阅地址事件所需能力 |
| `read_only` + `tmpfs /tmp` | 只读根文件系统，降低容器被篡改风险 |
| `no-new-privileges` | 禁止提权 |
| 配置挂载 | `~/.ddns6` → `/home/ddns6/.ddns6:ro`（与镜像用户 HOME 一致） |
| 多子域名 | Compose 命令行模式通常只传单个 `--subdomain`；多子域名请用配置文件模式 |

### 直接 `docker run`

```bash
make docker-build
# 或 make docker-run（等价于下方，依赖已有 ~/.ddns6）
docker run -d --name ddns6 --restart unless-stopped \
  --network host --read-only --tmpfs /tmp:size=16m,mode=1777 \
  --security-opt no-new-privileges:true \
  --cap-drop ALL --cap-add NET_ADMIN \
  -v ~/.ddns6:/home/ddns6/.ddns6:ro \
  ddns6 run
```

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
ddns6 init tencent --domain example.com --subdomain www \
  --secret-id xxx --secret-key yyy

sudo systemctl daemon-reload
sudo systemctl enable --now ddns6
sudo journalctl -u ddns6 -f
```

---

## 安全注意事项

- 配置文件含 API 密钥，建议 `chmod 600 ~/.ddns6/config.yaml`
- 运行 `check` / `run` 时若权限过松会输出警告
- 日志不记录 secret key、token 等敏感信息
- 建议为 DDNS 创建专用 API 令牌，仅授予 DNS 编辑权限
- Docker CLI 模式会将凭据写入容器命令行，生产环境优先使用配置文件挂载

---

## 架构说明

### 触发器

| 平台 | 模式 | 说明 |
|------|------|------|
| Linux | Netlink 事件驱动 | 实时监听内核 `RTM_NEWADDR`；PPPoE 重拨后秒级触发 |
| macOS / Windows 等 | 定时轮询 | 默认每 5 分钟（`--interval` 可配） |
| Linux（回退） | 定时轮询 | Netlink 不可用时自动回退 |

### 防抖（Debounce）

PPPoE 重拨时地址可能短时间内多次变化。Linux Netlink 模式下检测到新地址后等待 **10 秒**防抖窗口，窗口内每次新事件重置计时器，地址稳定后再触发 DNS 同步。

### 同步流程

```
地址变化 → GetIPv6Addr（多源并发竞速） → 对比缓存
  ├─ 未变化 → 跳过
  └─ 已变化 → 并发同步所有子域名
        ├─ GetRecords（查询现有记录）
        ├─ 遍历：IP 相同 → 跳过；不同 → ModifyRecord
        └─ 无记录 → AddRecord
```

启动时会立即执行一次完整同步（fail-fast）；后续触发中的失败仅记日志，不终止服务。收到 SIGINT/SIGTERM 后优雅关闭（最多等待 5 秒）。

### IPv6 获取源

每次随机打乱顺序后并发竞速，首个成功即返回：

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
│   ├── providers.go           # 23 家 provider 工厂注册
│   ├── check.go / list.go / records.go / clean.go / init.go
│   └── ...
├── internal/
│   ├── config/                # 配置加载与生成
│   ├── crypto/                # 签名用哈希工具
│   ├── ddns/                  # 核心服务编排
│   │   ├── types.go           # RecordInfo、DNSProvider、Domain
│   │   ├── service.go         # RunService 主循环
│   │   ├── service_linux.go   # Netlink 触发 + 防抖（Linux）
│   │   ├── service_other.go   # 轮询触发（非 Linux）
│   │   ├── record.go          # DNS 记录同步
│   │   ├── match.go           # 记录名匹配、地址比较
│   │   ├── processor.go       # CollectMatchingRecords
│   │   └── display.go         # 表格输出
│   └── providers/             # 各运营商实现（23 个子包）
├── pkg/
│   ├── domainutil/            # SplitDomain
│   ├── ipaddr/                # IPv6 获取（HTTP / DNS）
│   └── retry/                 # 指数退避重试
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── Makefile
└── .github/workflows/
    ├── test.yml               # push/PR 到 main：vet + race + cover
    └── release.yml            # 推送 v* tag：跨平台构建与 GitHub Release
```

---

## 开发

要求 Go **1.27.1+**（见 `go.mod`）。

### 本地命令

```bash
make build            # 编译到 bin/ddns6
make test             # go test -v ./...
go vet ./...          # 静态分析
make fmt              # go fmt
make cross-build      # linux/darwin 交叉编译
make release          # 打包发布产物
make help             # 查看全部目标
```

带 race 与覆盖率（与 CI 一致）：

```bash
go vet ./...
go test ./... -race -count=1 -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | tail -n 1
```

### CI / Release

| Workflow | 触发条件 | 内容 |
|----------|---------|------|
| [`test.yml`](.github/workflows/test.yml) | `push` / `pull_request` → `main` | `go vet ./...`；`go test ./... -race -count=1 -coverprofile=coverage.out -covermode=atomic` |
| [`release.yml`](.github/workflows/release.yml) | 推送 `v*` tag | 测试 + vet；构建 `linux/amd64`、`darwin/amd64`、`darwin/arm64`；上传 tar.gz（含二进制、LICENSE、README）至 GitHub Release |

### 添加新运营商

1. 在 `internal/providers/<name>/` 实现 `ddns.DNSProvider`
2. 在 `cmd/providers.go` 的 `providerFactories` 追加一条（flags、run、fromConfig）
3. 若 API 仅支持更新、不支持查询/删除，设置 `noListClean: true` 并加入 `restrictedProviders`
4. 同步更新 `docker-compose.yml`、`.env.example` 与本 README
5. 运行 `go test ./...` 确认通过

---

## 常见问题

**Q: 怎么查看当前公网 IPv6？**

```bash
curl -6 https://6.ipw.cn
```

**Q: 如何验证配置？**

```bash
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

**Q: Netlink 需要 root 吗？**  
读取 `NETLINK_ROUTE` 通常不需要 root。权限不足时会自动回退到定时轮询。

**Q: 支持 A 记录（IPv4）吗？**  
不支持。本项目专注 IPv6 DDNS（名称中的「6」即此意）。

**Q: 为什么 duckdns / he / noip 不能 records 或 clean？**  
这三家 API 仅提供 DDNS 更新端点，无记录查询/删除接口；CLI 会注册占位命令并返回明确错误。

**Q: Docker 为什么要 `--network host`？**  
Netlink 需要主机网络命名空间，才能感知本机 IPv6 变化。

**Q: 日志文件过大怎么办？**  
使用 `--log-file ""` 仅输出到 stderr，或配合 logrotate：

```bash
# /etc/logrotate.d/ddns6
/var/log/ddns6.log {
    daily
    rotate 7
    compress
    missingok
    copytruncate
}
```

---

## 许可证

MIT License — 详见 [LICENSE](LICENSE)
