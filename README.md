# ddns6 — 动态 DNS 更新工具

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](go.mod)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

ddns6 是一个用 Go 编写的 CLI 工具，**自动检测本机 IPv6 地址变化并实时更新到 DNS 服务商的 AAAA 记录**。

适用于需要 IPv6 DDNS 的场景：PPPoE 重拨后地址变更自动更新、家庭服务器域名绑定等。

---

## 功能特性

| 特性 | 说明 |
|------|------|
| **Netlink 事件驱动** | Linux 上通过内核 Netlink 实时监听地址变化（非轮询），**PPPoE 重拨后秒级触发更新** |
| **双模式降级** | macOS/Windows 自动回退到定时轮询，无需额外配置 |
| **10 秒防抖** | 地址变化后等待 10 秒再更新，确保 PPPoE 重拨过程中的地址稳定性 |
| **13 个运营商** | 腾讯云、阿里云、Cloudflare、华为云、GoDaddy、DuckDNS、No-IP、HE、Dynv6、Porkbun、DigitalOcean、百度云、DNSPod |
| **多子域名支持** | 一次命令更新多个子域名：`--subdomain www --subdomain @ --subdomain api` |
| **配置文件模式** | `ddns6 init [provider]` 一键生成完整配置，长期运行无需反复输入参数 |
| **智能更新** | 仅在 IPv6 地址变化时才调用 DNS API，避免无效请求 |
| **优雅关闭** | 收到 SIGINT/SIGTERM 后等待当前操作完成再退出（最长 5 秒） |

---

## 快速开始

### 安装

```bash
git clone https://github.com/notes-bin/ddns6.git
cd ddns6
make build
make install      # 安装到 $GOPATH/bin
```

### 临时运行（单次测试）

```bash
# 腾讯云 DNSPod 示例
./bin/ddns6 run tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY
```

首次运行会立即获取 IPv6 地址并更新 DNS 记录。后续在 Linux 上通过 Netlink 实时监听变化，在其他平台上每 5 分钟轮询一次。

### 使用配置文件（长期运行）

#### 方式一：一键生成完整配置（推荐）

```bash
# 直接生成完整配置（含 provider 和认证凭据）
./bin/ddns6 init tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY

# 直接运行（从配置文件读取参数）
./bin/ddns6 run
```

#### 方式二：生成模板后手动编辑

```bash
# 1. 生成配置文件模板
./bin/ddns6 init

# 2. 编辑配置文件
vim ~/.ddns6/config.yaml

# 3. 直接运行（从配置文件读取参数）
./bin/ddns6 run
```

---

## 配置说明

### 配置文件位置

```
~/.ddns6/
├── config.yaml       ← 主配置文件
```

通过 `ddns6 init` 生成模板或手动创建。

### 配置格式

```yaml
# 运营商名称（必填）
provider: "tencent"

# 认证凭据（必填，不同运营商字段不同）
auth:
  secret_id: "your-secret-id"
  secret_key: "your-secret-key"

# 根域名（必填）
domain: "example.com"

# 子域名列表（必填，可多个）
subdomains:
  - "www"
  - "@"
  - "api"

# 非 Linux 轮询间隔（可选，默认 5m）
interval: 10m

# 监听的网络接口（可选，仅 Linux Netlink 模式）
# interface: ppp0

# DNS 记录 TTL（可选，默认 600 秒）
# ttl: 600
```

### 优先级规则

命令行参数 > 配置文件。例如：

```bash
# 配置文件设定了 interval: 10m，但以下命令临时改为 30s
./bin/ddns6 run --interval 30s
```

---

## 命令参考

### `ddns6 init [provider]`

生成 `~/.ddns6/config.yaml` 配置文件。指定 provider 名称和认证参数可直接生成完整配置。

#### 生成完整配置（含 provider 和认证凭据）

```bash
./bin/ddns6 init tencent --domain example.com --secret-id xxx --secret-key yyy
# → Configuration file created at: /home/user/.ddns6/config.yaml
# → Configuration is complete. Run: ddns6 run

./bin/ddns6 init cloudflare --domain example.com --api-token xxx
./bin/ddns6 init alicloud --domain example.com --access-key-id xxx --access-key-secret yyy
```

#### 生成模板后手动编辑

```bash
./bin/ddns6 init
# → Configuration file created at: /home/user/.ddns6/config.yaml
# → Edit it with your provider details, then run: ddns6 run
```

#### 可用认证参数

各 provider 的认证参数与运行模式一致，参见下方"支持的运营商"表格。例如：

| Provider | 可用参数 |
|----------|---------|
| tencent | `--secret-id`, `--secret-key` |
| cloudflare | `--api-token` |
| alicloud | `--access-key-id`, `--access-key-secret` |

### `ddns6 run [provider]`

启动 DDNS 服务。

- **指定 provider**：从命令行参数读取配置
- **不指定 provider**：从 `~/.ddns6/config.yaml` 读取配置

```bash
# 指定 provider
./bin/ddns6 run tencent --domain example.com --subdomain www --secret-id xxx --secret-key yyy

# 多子域名
./bin/ddns6 run cloudflare --domain example.com --subdomain www --subdomain @ --api-token xxx

# 指定网络接口
./bin/ddns6 run duckdns --domain example.com --interface ppp0 --token xxx

# 调试模式
./bin/ddns6 run --debug tencent --domain example.com --secret-id xxx --secret-key yyy
```

### `ddns6 version`

显示版本、Git 提交和构建时间。也可用 `ddns6 -V` 或 `ddns6 --version`。

```bash
./bin/ddns6 version
# → Version: 0.0.173
# → Commit:  abc1234
# → BuildAt: 2026-07-19T10:00:00Z
```

### `ddns6 help [command]`

查看子命令的详细帮助。

```bash
./bin/ddns6 help run                  # 查看 run 命令帮助
./bin/ddns6 help run tencent          # 查看 tencent 运营商帮助
./bin/ddns6 run tencent --help        # 同上
```

### 全局参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--domain` | string | — | 根域名（必填，如 `example.com`）|
| `--subdomain` | string[] | `@` | 子域名，可多次指定 |
| `--interval` | duration | `5m` | 非 Linux 平台轮询间隔 |
| `--interface` | string | — | 监听的网络接口（仅 Linux）|
| `--ttl` | int | `600` | DNS 记录 TTL（秒）|
| `-V, --version` | bool | `false` | 显示版本、Git 提交、构建时间 |
| `--debug` | bool | `false` | 开启调试日志 |

---

## 支持的运营商

| 运营商 | 命令名 | 认证方式 | 必填参数 |
|--------|--------|---------|---------|
| 腾讯云 DNSPod | `tencent` | SecretID + SecretKey | `--secret-id`, `--secret-key` |
| Cloudflare | `cloudflare` | API Token | `--api-token` |
| 阿里云 DNS | `alicloud` | AccessKey | `--access-key-id`, `--access-key-secret` |
| GoDaddy | `godaddy` | API Key | `--api-key`, `--api-secret` |
| 华为云 DNS | `huaweicloud` | Username + Password | `--username`, `--password`, `--domain-name` |
| DuckDNS | `duckdns` | Token | `--token` |
| No-IP | `noip` | Username + Password | `--username`, `--password` |
| Hurricane Electric | `he` | DDNS Key | `--password` |
| Dynv6 | `dynv6` | Token | `--token` |
| Porkbun | `porkbun` | API Key | `--api-key`, `--api-secret` |
| DigitalOcean | `digitalocean` | Token | `--token` |
| 百度云 DNS | `baiducloud` | AccessKey + SecretKey | `--access-key`, `--secret-key` |
| DNSPod (旧版) | `dnspod` | Login Token | `--login-token` |

详细参数请查看各运营商帮助：

```bash
./bin/ddns6 run tencent --help
./bin/ddns6 run cloudflare --help
# ...
```

---

## 架构设计

### 地址变化监听

```
RunService(domains, p, interval, fetchers, iface)
│
├─ [Linux] ── Netlink 监听 (RTM_NEWADDR)
│                 │
│            IPv6 地址变化事件
│                 │
│            [过滤] 全局单播 IPv6 + 指定接口
│                 │
│            [防抖] 10 秒等待窗口，每次新事件重置
│                 │     ← PPPoE 重拨时可能连续触发多个事件
│                 ▼
│            GetIPv6Addr(fetchers...)     ← 随机顺序，并发竞速
│                 │
│                 ▼
│            循环 for _, d := range domains:
│                SyncRecord(ctx, d, ip, p)   ← 同步每个子域名
│
├─ [非 Linux] ── cron 定时轮询 (默认 5 分钟)
│
└─ signal.Notify(SIGINT, SIGTERM)
      │
      ▼
    cancel context → 等待最多 5s → exit
```

### IP 获取策略

每次触发时，**随机打乱** fetchers 顺序后并发请求，首个成功即返回：

1. HTTP 端点（https://6.ipw.cn、https://ifconfig.co、https://v6.ident.me）
2. DNS 直连（2402:4e00::、2400:3200:baba::1、Google DNS、Cloudflare DNS）

首个成功即返回，全部失败则跳过本次同步。

### DNS 同步策略

```
SyncRecord:
  ├─ Lock Domain → defer Unlock
  ├─ ctx.Done() 检查 → 已取消则跳过
  ├─ hasAddressChanged? → 地址未变则跳过
  └─ syncDNSRecord:
        ├─ GetRecords(fqdn, AAAA)
        ├─ 遍历记录:
        │   ├─ 筛选: r.Name == 目标子域名 ∧ r.Type == AAAA
        │   ├─ IP 相同 → 更新缓存（continue）
        │   └─ IP 不同 → ModifyRecord（continue 处理全部）
        └─ 无匹配记录 → AddRecord
```

### 防抖（Debounce）设计

PPPoE 重拨过程中地址可能短时间内多次变化：

```
时间线:
T+0s   RTM_NEWADDR 240e:100::1     ← 第一次收到新地址
T+0.2s RTM_DELADDR 240e:100::1     ← 地址被回收
T+0.5s RTM_NEWADDR 240e:200::2     ← PPPoE 重新协商后的正式地址
T+0.6s RTM_NEWADDR 240e:200::3     ← SLAAC 生成新临时地址
                                   ← 10 秒内无新事件
T+10s  执行 GetIPv6Addr + SyncRecord ← 地址稳定后才更新
```

---

## 项目结构

```
ddns6/
├── main.go                    # 程序入口
├── cmd/                       # CLI 命令定义（cobra）
│   ├── root.go                #   根命令 + init/version 子命令 + 全局参数
│   └── providers.go           #   run 命令 + 13 个运营商注册 + config 加载
├── internal/
│   ├── config/                # 配置文件 (~/.ddns6/config.yaml)
│   │   └── config.go          #   加载、解析、生成配置模板
│   ├── ddns/                  # DDNS 服务编排
│   │   ├── service.go         #   RunService 主入口 + 事件循环
│   │   ├── service_linux.go   #   Linux Netlink 实现（//go:build linux）
│   │   ├── service_fallback.go#   macOS/Windows 定时轮询（//go:build !linux）
│   │   └── sync.go            #   DNS 记录同步逻辑 (SyncRecord)
│   └── providers/             # DNS 运营商接口与实现
│       ├── record.go          #   Domain 结构体 + DNSProvider 接口 + 通用 CRUD
│       ├── domain.go          #   SplitDomain 辅助函数
│       ├── tencent/           #   腾讯云 DNSPod (API v3)
│       ├── cloudflare/        #   Cloudflare DNS
│       ├── alicloud/          #   阿里云 DNS
│       └── ...                #   其余 10 个运营商
└── pkg/
    └── ipaddr/                # IPv6 地址获取
        ├── ipv6.go            #   GetIPv6Addr（随机顺序逐个尝试）
        ├── site.go            #   HTTP 端点 Fetcher
        └── dns.go             #   DNS 直连 Fetcher
```

---

## 开发指南

### 编译与测试

```bash
make test            # 运行全部测试
go build ./...       # 编译
go vet ./...         # 静态分析
go test -v -cover ./...  # 测试并查看覆盖率
```

### 跨平台编译

```bash
make cross-build     # 生成 linux_amd64 + darwin_amd64 + darwin_arm64
make release         # 打包为 tar.gz
```

### Docker 部署

```bash
docker build -t ddns6 .
docker run -d --name ddns6 --restart always --network host ddns6 \
  run tencent \
  --secret-id ID --secret-key KEY \
  --domain example.com --subdomain www
```

> `--network host` 是必要的，因为 Netlink 需要主机的网络命名空间。

---

## 常见问题

**如何验证配置是否正确？**
```bash
./bin/ddns6 --debug run tencent --domain example.com --secret-id xxx --secret-key yyy
```
首次运行会立即更新，日志中可看到详细过程。

**Netlink 需要 root 权限吗？**
大多数 Linux 发行版上，`NETLINK_ROUTE` 的读取操作无需 root。如果遇到权限错误，工具会自动回退到定时轮询模式。

**支持 A 记录（IPv4）吗？**
暂不支持。本项目专注于 IPv6 DDNS 场景。

**如何更新多个子域名？**
```bash
# 命令行模式
./bin/ddns6 run tencent --domain example.com --subdomain www --subdomain @ --subdomain api ...

# 配置文件模式
# 在 config.yaml 的 subdomains 列表中添加多个子域名
```

---

## 许可证

MIT License — 详见 [LICENSE](LICENSE)
