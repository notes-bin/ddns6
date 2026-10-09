# DDNS6

[![Go Version](https://img.shields.io/badge/Go-1.27.1+-00ADD8?logo=go)](go.mod)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Test](https://github.com/notes-bin/ddns6/actions/workflows/test.yml/badge.svg)](https://github.com/notes-bin/ddns6/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/notes-bin/ddns6)](https://github.com/notes-bin/ddns6/releases)

本机 IPv6 动态域名解析（DDNS）工具：检测到本机公网 IPv6 变化后，更新 DNS 上的 AAAA 记录。

## 简介

适合 PPPoE 重拨、家庭宽带等本机 IPv6 会变动的场景。Linux 通过 Netlink 监听地址变化；其他平台按间隔轮询。内置 23 家 DNS 运营商。

本工具只更新 AAAA（IPv6），不支持 A 记录或 IPv4。同步前会过滤 link-local / ULA 等非公网地址，只写入全局公网 IPv6。

## 特性

- Linux Netlink 事件驱动；非 Linux 定时轮询；Netlink 不可用时回退轮询
- 地址变化后 10 秒防抖；同步单飞，避免堆积
- 同根域名多子域合并为一次 `GetRecords`；同名多条记录全部成功后才更新本地缓存
- 多源并发获取 IPv6（HTTP + DNS 拨号），取首个公网结果
- 23 家运营商；一次可更新多个子域名
- 配置文件 Unix 权限 fail-closed（`0600`）；Docker 默认挂载配置、非 root
- `check` / `list` / `records` / `clean`；可选 loopback Prometheus `--metrics-addr`

## 快速开始

### 安装

从 [GitHub Releases](https://github.com/notes-bin/ddns6/releases) 下载预编译包，或本地构建：

```bash
git clone https://github.com/notes-bin/ddns6.git
cd ddns6
go build -o ddns6 ./cmd/ddns6
# 或
make build          # 输出 bin/ddns6
sudo make install   # 安装到 GOPATH/bin
```

也可用：

```bash
go install github.com/notes-bin/ddns6/cmd/ddns6@latest
```

### 临时运行（命令行模式）

```bash
ddns6 run tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY
```

启动后会先同步一次 IPv6。Linux 之后依赖 Netlink；其他平台默认每 5 分钟轮询（`--interval`；非法或非正值回退为 `5m`）。

### 配置文件模式

```bash
ddns6 init tencent \
  --domain example.com \
  --subdomain www \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY

chmod 700 ~/.ddns6
chmod 600 ~/.ddns6/config.yaml   # init 已写 0600；过宽权限将拒绝加载

ddns6 run
```

`ddns6 run`（不带 provider）时，**域名与子域名以配置文件为准**；命令行可覆盖 `--interval`、`--interface` 等运行参数。临时改域名请用带 provider 的命令行模式。

字段与权限见 [部署与运维](docs/deployment.md)。

### 上线前检查

```bash
# 读 ~/.ddns6/config.yaml
ddns6 check

# 或用命令行参数直接探测（需对应运营商的认证 flag）
ddns6 check tencent --domain example.com --secret-id xxx --secret-key yyy
```

`check` 只验证配置与 API，不修改 DNS 记录。`duckdns` / `he` / `noip` 仅提供更新接口，不支持经 API 做连通性探测。

## 命令速查

| 子命令 | 说明 |
|--------|------|
| `run [provider]` | 启动 DDNS 服务 |
| `init [provider]` | 生成 `~/.ddns6/config.yaml`（已存在则拒绝覆盖） |
| `check [provider]` | 校验配置与 API，不改记录 |
| `list` | 列出内置运营商（不读配置、不联网） |
| `records [provider]` | 查询 DNS 记录（默认 AAAA） |
| `clean [provider]` | 删除 DNS 记录（支持 `--dry-run` / `--yes`） |
| `version` | 打印版本、提交与构建时间 |
| `completion …` | 生成 bash / zsh / fish / powershell 补全 |

全局参数（多数子命令可用；也可用对应环境变量）：

| 参数 | 环境变量 | 默认值 |
|------|---------|--------|
| `--domain` | `DDNS6_DOMAIN` | （空） |
| `--subdomain` | `DDNS6_SUBDOMAIN` | `@` |
| `--ttl` | `DDNS6_TTL` | `600` |
| `--interval` | `DDNS6_INTERVAL` | `5m` |
| `--interface` | `DDNS6_INTERFACE` | （空） |
| `--log-file` | `DDNS6_LOG_FILE` | `ddns6.log` |
| `--debug` | `DDNS6_DEBUG` | `false` |
| `--metrics-addr` | `DDNS6_METRICS_ADDR` | （空=禁用；仅 loopback） |
| `-V` / `--version` | — | — |

更多细节见 `ddns6 <command> --help`。

## 文档目录

| 文档 | 内容 |
|------|------|
| [DNS 运营商](docs/providers.md) | 23 家对照表、认证字段、配置示例 |
| [部署与运维](docs/deployment.md) | 配置权限、systemd、Docker、日志、补全 |
| [架构与运行原理](docs/architecture.md) | 触发、防抖、合并查询、IPv6 来源、metrics |
| [开发指南](docs/development.md) | 构建、CI（govulncheck / lint / 覆盖率）、新增运营商 |

### 常见问题

**如何确认配置可用？**  
运行 `ddns6 check`（读配置文件）或 `ddns6 check <provider> …`（命令行参数）。通过后再 `run`。

**配置权限过宽会怎样？**  
Unix 上若 `config.yaml` 对 group/others 可读，或无法读取权限信息，程序 **拒绝加载** 并提示 `chmod 600`。

**配置优先级是什么？**  
- **命令行模式**（`run/check/records/clean <provider>`）：flag / `DDNS6_*` 环境变量决定域名与认证。  
- **配置文件模式**（`run` 不带 provider）：域名与子域名来自 `~/.ddns6/config.yaml`；`--interval`、`--interface` 等若在命令行显式传入则可覆盖配置。

**Netlink 监听是否需要 root？**  
一般不需要。若使用 `--interface` 或 Docker 中监听网卡事件，可能需要相应网络能力，见 [部署与运维](docs/deployment.md)。

**为何 Docker 要用 host 网络？**  
Linux 的 Netlink 地址事件依赖宿主机网络命名空间；`network_mode: host` 才能收到本机 IPv6 变化。

**为何 duckdns / he / noip 不能用 records、clean 或 check？**  
这三家 API 只提供更新接口。完整说明见 [DNS 运营商](docs/providers.md)。

## 安全要点

- 配置目录 `0700`、文件 `0600`；过宽权限 fail-closed；`init` 不会覆盖已有配置
- 令牌按最小权限签发（例如 Cloudflare 仅 DNS:Edit）
- Docker 优先挂载配置目录只读（Compose 默认 `ddns6-config`），避免密钥进 argv
- `--metrics-addr` 仅允许绑定 loopback

## 许可证

[MIT](LICENSE)
