# DDNS6：IPv6 动态域名解析

[![Go Version](https://img.shields.io/badge/Go-1.27.1+-00ADD8?logo=go)](go.mod)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Test](https://github.com/notes-bin/ddns6/actions/workflows/test.yml/badge.svg)](https://github.com/notes-bin/ddns6/actions/workflows/test.yml)

本机 IPv6 变了就更新 DNS 上的 AAAA。目前接了 23 家运营商。Linux 用 Netlink 盯地址变化；别的系统按间隔轮询。

---

## 快速开始

### 安装

```bash
git clone https://github.com/notes-bin/ddns6.git
cd ddns6
go build -o ddns6 .
sudo mv ddns6 /usr/local/bin/
```

或用 Makefile：

```bash
make build          # 输出到 bin/ddns6
sudo make install   # 安装到 GOPATH/bin
```

也可以从 [GitHub Releases](https://github.com/notes-bin/ddns6/releases) 下预编译包（推送 `v*` tag 时由 `release.yml` 构建）。

### 临时跑一下

```bash
# 腾讯云 DNSPod
ddns6 run tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY
```

起来会先拉一次 IPv6 并写 DNS。Linux 之后靠 Netlink；其他平台默认每 5 分钟查一次。

### 配置文件（长期跑）

```bash
ddns6 init tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY

ddns6 run
```

### 先检查再上线

```bash
# 只检查配置和 API，不改记录
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

---

## 命令参考

### 全局参数

下面这些参数多数命令都能用，也可用 `DDNS6_*` 环境变量：

| 参数 | 环境变量 | 类型 | 默认值 | 说明 |
|------|---------|------|--------|------|
| `--domain` | `DDNS6_DOMAIN` | string | | 根域名（如 example.com） |
| `--subdomain` | `DDNS6_SUBDOMAIN` | string[] | `@` | 子域名，可写多次 |
| `--ttl` | `DDNS6_TTL` | int | `600` | TTL（秒） |
| `--interval` | `DDNS6_INTERVAL` | duration | `5m` | 非 Linux 轮询间隔 |
| `--interface` | `DDNS6_INTERFACE` | string | | 网卡名（仅 Linux Netlink） |
| `--log-file` | `DDNS6_LOG_FILE` | string | `ddns6.log` | 日志路径；`""` 表示只打 stderr |
| `--debug` | `DDNS6_DEBUG` | bool | `false` | 调试日志 |
| `-V` / `--version` | | bool | `false` | 版本（同 `ddns6 version`） |

优先级：命令行 > 环境变量 > 配置文件。

### `ddns6 run [provider]`

开 DDNS 服务。

```bash
# 读配置文件
ddns6 run

# 命令行
ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

# 多个子域名
ddns6 run cloudflare --domain example.com --subdomain www --subdomain @ --api-token xxx

# 调试
ddns6 run --debug tencent --domain example.com --secret-id xxx --secret-key yyy

# 环境变量
export DDNS6_DOMAIN=example.com DDNS6_SUBDOMAIN=www
ddns6 run tencent --secret-id xxx --secret-key yyy
```

### `ddns6 check [provider]`

检查配置和 API 能不能通，不改 DNS。

```bash
ddns6 check
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

顺序大致是：读配置 → 认 provider → 看认证字段 → 打 API。

### `ddns6 list`

打印内置运营商名单（不读配置、不联网）。表格里有 CLI 名、能不能用 `records`/`clean`，以及一句说明。

```bash
ddns6 list
```

### `ddns6 records [provider]`

查 DNS 记录。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--type` | `AAAA` | 按类型过滤；`""` 表示不过滤 |

```bash
# AAAA
ddns6 records tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

# 全部类型
ddns6 records tencent --domain example.com --type "" --secret-id xxx --secret-key yyy

# 不传 --subdomain：该域名下匹配类型的记录都会列出来
ddns6 records tencent --domain example.com --secret-id xxx --secret-key yyy
```

`duckdns`、`he`、`noip` 的 API 只能更新，查不了记录。对这些家跑 `records` 会直接报错，去官网面板改。

### `ddns6 clean [provider]`

删 DNS 记录。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--type` | `AAAA` | 类型过滤 |
| `--dry-run` | `false` | 只预览 |
| `--yes` | `false` | 跳过确认（脚本用） |

```bash
# 预览
ddns6 clean tencent --domain example.com --subdomain www --dry-run \
  --secret-id xxx --secret-key yyy

# 交互确认后删
ddns6 clean tencent --domain example.com --subdomain www \
  --secret-id xxx --secret-key yyy

# 直接删
ddns6 clean tencent --domain example.com --subdomain www --yes \
  --secret-id xxx --secret-key yyy
```

删之前会先列出目标；可用 `--dry-run` 看一眼。并发最多 5 个。`duckdns` / `he` / `noip` 同样不支持 `clean`。

### `ddns6 init [provider]`

写出 `~/.ddns6/config.yaml`。

```bash
ddns6 init
ddns6 init --domain example.com --subdomain www --subdomain @
ddns6 init tencent --domain example.com --secret-id xxx --secret-key yyy
```

### `ddns6 version`

版本、提交、构建时间（或用 `-V`）。

```bash
ddns6 version
ddns6 -V
```

### `ddns6 completion [bash|zsh|fish|powershell]`

生成补全脚本。

```bash
# Bash（需要 bash-completion）
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

一共 23 家，注册在 `cmd/providers.go` 的 `providerFactories`。完整名单也可以 `ddns6 list`。

| 运营商 | CLI 名称 | 必填参数 | 配置文件字段 (`auth`) | records/clean | 说明 |
|--------|---------|---------|----------------------|------------|------|
| 腾讯云 DNSPod | `tencent` | `--secret-id` `--secret-key` | `secret_id` `secret_key` | 支持 | API v3 |
| Cloudflare | `cloudflare` | `--api-token` | `api_token` | 支持 | 需 DNS:Edit |
| 阿里云 DNS | `alicloud` | `--access-key-id` `--access-key-secret` | `access_key_id` `access_key_secret` | 支持 | 可选 `sign_version`（见下） |
| GoDaddy | `godaddy` | `--api-key` `--api-secret` | `api_key` `api_secret` | 支持 | |
| 华为云 DNS | `huaweicloud` | `--access-key` `--secret-key` | `access_key` `secret_key` | 支持 | |
| 百度云 BCD | `baiducloud` | `--access-key` `--secret-key` | `access_key` `secret_key` | 支持 | |
| DigitalOcean | `digitalocean` | `--token` | `token` | 支持 | |
| DNSPod 旧版 | `dnspod` | `--login-token` | `login_token` | 支持 | 格式 `ID,Token` |
| Porkbun | `porkbun` | `--api-key` `--api-secret` | `api_key` `api_secret` | 支持 | |
| DuckDNS | `duckdns` | `--token` | `token` | 受限 | 只有更新接口 |
| Hurricane Electric | `he` | `--password` | `password` | 受限 | DDNS Key，只有更新 |
| No-IP | `noip` | `--username` `--password` | `username` `password` | 受限 | 经典 DDNS，只有更新 |
| Dynv6 | `dynv6` | `--token` | `token` | 支持 | |
| deSEC.io | `desec` | `--token` | `token` | 支持 | |
| Linode (Akamai) | `linode` | `--api-key` | `api_key` | 支持 | DNS API v4 |
| NameSilo | `namesilo` | `--api-key` | `api_key` | 支持 | |
| IONOS | `ionos` | `--prefix` `--secret` | `prefix` `secret` | 支持 | |
| Hetzner Cloud | `hetzner` | `--token` | `token` | 支持 | 要有 DNS 权限 |
| AWS Route 53 | `aws` | `--access-key-id` `--secret-access-key` | `access_key_id` `secret_access_key` | 支持 | 可选 `session_token` |
| Google Cloud DNS | `gcloud` | `--project` `--access-token` | `project` `access_token` | 支持 | REST，不依赖 gcloud CLI |
| Azure DNS | `azure` | `--subscription-id` `--tenant-id` `--client-id` `--client-secret` | 同左 snake_case | 支持 | Service Principal |
| Namecheap | `namecheap` | `--api-key` `--username` `--client-ip` | `api_key` `username` `client_ip` | 支持 | API 要加白名单 IP |
| DNSPod 国际版 | `dpi` | `--login-token` | `login_token` | 支持 | 格式 `ID,Key` |

更细的 flag 看 `ddns6 run <name> --help`。

### 阿里云 V3 签名

默认 V1（HMAC-SHA1）。要 V3（ACS3-HMAC-SHA256）可以这样：

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

默认：`~/.ddns6/config.yaml`。

```yaml
provider: "tencent"          # 必填：运营商（上表 CLI 名）
auth:                        # 必填：凭据
  secret_id: "xxx"
  secret_key: "xxx"
domain: "example.com"        # 必填：根域名
subdomains:                  # 必填：子域名
  - "www"
  - "@"                      # 根域名本身
# interval: 5m               # 可选：非 Linux 轮询间隔
# interface: ppp0            # 可选：网卡（仅 Linux）
# ttl: 600                   # 可选：TTL，默认 600
```

建议：`chmod 600 ~/.ddns6/config.yaml`。

---

## Docker 部署

镜像是多阶段构建（`golang:1.27.1-alpine` → 固定 `alpine:3.21`），进程以用户 `ddns6`（uid 10001）跑，支持 `TARGETARCH`。Linux 上要走 Netlink 的话，容器得进主机网络命名空间。

### 前置

- Linux 主机（`network_mode: host` 在 Docker Desktop / macOS 上基本没用）
- 已装 Docker 和 Compose

### 方式一：挂配置文件（更稳妥）

密钥放在 `~/.ddns6/config.yaml`，不会出现在 `docker inspect` 或进程参数里。

```bash
ddns6 init tencent --domain example.com --subdomain www \
  --secret-id xxx --secret-key yyy
chmod 600 ~/.ddns6/config.yaml
```

在 `docker-compose.yml` 里打开 `ddns6-config` 服务（挂载到 `/home/ddns6/.ddns6`），然后：

```bash
make docker-build
docker compose up -d ddns6-config
```

### 方式二：环境变量 + Compose

```bash
cp .env.example .env
# 填 DOMAIN、SUBDOMAIN 和对应运营商凭证
# 默认起 ddns6-tencent；其他运营商服务要在 compose 里取消注释
# CLI 模式会把密钥写进容器命令行，只适合你信得过的机器

make docker-up      # docker compose up -d
make docker-logs
make docker-down
```

### 容器安全相关设置

| 项 | 说明 |
|----|------|
| 非 root | 镜像内 `ddns6:ddns6`（uid/gid 10001） |
| `network_mode: host` | 跟主机共用网络，Netlink 才能看到地址变化 |
| `cap_drop: ALL` + `cap_add: NET_ADMIN` | 只留订阅地址事件需要的能力 |
| `read_only` + `tmpfs /tmp` | 根只读 |
| `no-new-privileges` | 禁止提权 |
| 配置挂载 | `~/.ddns6` → `/home/ddns6/.ddns6:ro` |
| 多子域名 | Compose 命令行模式一般只带一个 `--subdomain`；多个请用配置文件 |

### 直接 `docker run`

```bash
make docker-build
# 或 make docker-run（等价于下面，需要已有 ~/.ddns6）
docker run -d --name ddns6 --restart unless-stopped \
  --network host --read-only --tmpfs /tmp:size=16m,mode=1777 \
  --security-opt no-new-privileges:true \
  --cap-drop ALL --cap-add NET_ADMIN \
  -v ~/.ddns6:/home/ddns6/.ddns6:ro \
  ddns6 run
```

---

## 部署为 systemd 服务

`/etc/systemd/system/ddns6.service`：

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

## 安全

- 配置里有 API 密钥：`chmod 600 ~/.ddns6/config.yaml`
- `check` / `run` 发现权限太松会警告
- 日志不写 secret key、token
- 最好单独建一把只开了 DNS 编辑权限的令牌
- Docker CLI 模式会把凭据放进容器命令行；线上优先挂配置文件

---

## 架构说明

### 触发器

| 平台 | 模式 | 说明 |
|------|------|------|
| Linux | Netlink | 听 `RTM_NEWADDR`；PPPoE 重拨后很快会动 |
| macOS / Windows 等 | 轮询 | 默认 5 分钟（`--interval`） |
| Linux（回退） | 轮询 | Netlink 挂了就退回轮询 |

### 防抖

PPPoE 重拨时地址可能连跳好几次。Linux Netlink 路径上，见到新地址会等 10 秒；窗口里又来事件就重新计时，稳定了再同步 DNS。

### 同步流程

```
地址变化 → GetIPv6Addr（多源并发） → 对比缓存
  ├─ 没变 → 跳过
  └─ 变了 → 并发同步各子域名
        ├─ GetRecords
        ├─ IP 相同 → 跳过；不同 → ModifyRecord
        └─ 没有记录 → AddRecord
```

启动时会先完整同步一次（失败就停）。之后触发里出错只记日志，服务继续跑。收到 SIGINT/SIGTERM 后最多等 5 秒再退出。

### IPv6 从哪拿

每次打乱顺序后并发抢，谁先成功用谁：

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
├── main.go                    # 入口
├── cmd/                       # CLI
│   ├── root.go                # 根命令、全局参数、环境变量
│   ├── providers.go           # 23 家工厂
│   ├── check.go / list.go / records.go / clean.go / init.go
│   └── ...
├── internal/
│   ├── config/                # 配置
│   ├── crypto/                # 签名用哈希
│   ├── ddns/                  # 主循环
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── service_linux.go   # Netlink + 防抖
│   │   ├── service_other.go   # 轮询
│   │   ├── record.go
│   │   ├── match.go
│   │   ├── processor.go
│   │   └── display.go
│   └── providers/             # 各运营商（23 个子包）
├── pkg/
│   ├── domainutil/
│   ├── ipaddr/
│   └── retry/
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── Makefile
└── .github/workflows/
    ├── test.yml               # push/PR → main：vet + race + cover
    └── release.yml            # v* tag：跨平台构建并发 Release
```

---

## 开发

需要 Go 1.27.1+（见 `go.mod`）。

### 本地命令

```bash
make build
make test             # go test -v ./...
go vet ./...
make fmt
make cross-build
make release
make help
```

跟 CI 一样带 race 和覆盖率：

```bash
go vet ./...
go test ./... -race -count=1 -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | tail -n 1
```

### CI / Release

| Workflow | 触发 | 做什么 |
|----------|------|--------|
| [`test.yml`](.github/workflows/test.yml) | `push` / `pull_request` → `main` | `go vet`；`go test -race -count=1` + cover |
| [`release.yml`](.github/workflows/release.yml) | 推送 `v*` tag | 测完后编 `linux/amd64`、`darwin/amd64`、`darwin/arm64`，打成 tar.gz 上传 Release |

### 加一家运营商

1. `internal/providers/<name>/` 实现 `ddns.DNSProvider`
2. 在 `cmd/providers.go` 的 `providerFactories` 加一条
3. 若只能更新、不能查/删：`noListClean: true`，并加入 `restrictedProviders`
4. 改 `docker-compose.yml`、`.env.example`、本 README
5. `go test ./...`

---

## 常见问题

**怎么看当前公网 IPv6？**

```bash
curl -6 https://6.ipw.cn
```

**怎么验证配置？**

```bash
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

**Netlink 要 root 吗？**  
读 `NETLINK_ROUTE` 一般不用。权限不够就自动改轮询。

**支持 A 记录（IPv4）吗？**  
不支持。就做 IPv6（名字里的 6）。

**为什么 duckdns / he / noip 不能 records / clean？**  
这三家只有 DDNS 更新接口。CLI 会挂占位命令并报错。

**Docker 为什么要 `--network host`？**  
Netlink 要进主机网络命名空间，才能看到本机 IPv6。

**日志太大怎么办？**  
`--log-file ""` 只打 stderr，或配 logrotate：

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

MIT，见 [LICENSE](LICENSE)。
