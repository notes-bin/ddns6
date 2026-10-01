# 部署与运维

本文说明 ddns6 的配置文件、systemd 服务、Docker 部署、日志与 Shell 自动补全。运营商与认证字段见 [`docs/providers.md`](providers.md)。

## 配置文件

默认路径：`~/.ddns6/config.yaml`。可用 `ddns6 init <provider>` 按 flag 生成配置模板，或手动编辑。

### 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `provider` | 是 | DNS 运营商 CLI 名称（如 `tencent`、`cloudflare`） |
| `auth` | 是 | 认证凭据，键名为 snake_case（如 `secret_id`）；各运营商字段见 [`docs/providers.md`](providers.md) |
| `domain` | 是 | 根域名（如 `example.com`） |
| `subdomains` | 否 | 子域名列表（如 `["www", "@"]`）；可选，缺省时默认为 `["@"]` |
| `interval` | 否 | 轮询间隔（如 `5m`、`10m`）；默认 `5m`。非 Linux 始终使用；Linux 在正常 Netlink 事件模式下不使用，当 Netlink 失败并回退轮询时生效（见 [`docs/architecture.md`](architecture.md)） |
| `interface` | 否 | 监听的网络接口（仅 Linux Netlink 模式，如 `ppp0`）；留空则监听所有接口 |
| `ttl` | 否 | DNS 记录 TTL（秒）；默认 `600` |

示例：

```yaml
provider: "tencent"
auth:
  secret_id: "YOUR_SECRET_ID"
  secret_key: "YOUR_SECRET_KEY"
domain: "example.com"
subdomains:
  - "www"
  - "@"
# interval: 5m
# interface: ppp0
# ttl: 600
```

### 文件权限

配置含 API 密钥。程序行为：

- 写入：配置目录权限 **`0700`**，配置文件 **`0600`**；
- 加载：Unix 上若文件对 group/others 可读（含 `0077` 位），**拒绝启动**（fail-closed），并提示执行 `chmod 600`。

```bash
chmod 700 ~/.ddns6
chmod 600 ~/.ddns6/config.yaml
```

Windows 不按 Unix 位图做同样检查。

## systemd

Linux 主机上可将 ddns6 注册为 systemd 服务，开机自启并在崩溃后自动重启。

### 单元文件示例

将二进制安装至 `/usr/local/bin/ddns6` 后，创建 `/etc/systemd/system/ddns6.service`：

```ini
[Unit]
Description=DDNS6 IPv6 Dynamic DNS
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
# 可选：User=ddns6 以非 root 用户运行（需保证该用户可读配置与二进制）
ExecStart=/usr/local/bin/ddns6 run
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

`ddns6 run` 无参数时从 `~/.ddns6/config.yaml` 读取配置。若以专用用户运行，请确保该用户的 `HOME` 下存在配置文件，或在 `ExecStart` 中改用 `ddns6 run <provider> ...` 显式传参。

### 启停与日志

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ddns6
sudo systemctl status ddns6
sudo journalctl -u ddns6 -f
```

停止服务：`sudo systemctl stop ddns6`。

## Docker

镜像采用多阶段构建：编译阶段 `golang:1.27.1-alpine`，运行阶段 `alpine:3.21`。运行用户为 `ddns6`，uid/gid 均为 `10001`，非 root。

镜像与 Compose 默认将 `DDNS6_LOG_FILE` 设为空（仅 stderr）。只读根文件系统下无法在 `WORKDIR` 创建默认的 `ddns6.log`；若需落盘日志，请挂载可写路径并设置例如 `-e DDNS6_LOG_FILE=/tmp/ddns6.log`。

### 为何使用 `network_mode: host`

ddns6 在 Linux 上通过 Netlink 监听 IPv6 地址变化。Bridge 网络模式下容器无法直接接收宿主机网卡事件，因此 Compose 与 `make docker-run` 均使用 `network_mode: host`。**注意：** `network_mode: host` 仅在 Linux 上有效；macOS Docker Desktop 不支持，Netlink 功能在 macOS 上不可用。

### 方式一：挂载配置文件（推荐，Compose 默认）

将主机 `~/.ddns6` 只读挂载到容器内 `HOME`（`/home/ddns6/.ddns6`），密钥保留在文件中，不进入进程命令行参数。

容器内进程 uid/gid 为 `10001`。主机上 `chmod 600` 的配置文件若属其他用户，容器将无法读取；放宽 other 可读又会被 fail-closed 权限检查拒绝。部署前请将配置目录属主改为容器用户：

```bash
sudo chown -R 10001:10001 ~/.ddns6
chmod 700 ~/.ddns6
chmod 600 ~/.ddns6/config.yaml
```

仓库 `docker-compose.yml` **默认启用** `ddns6-config` 服务；CLI 凭据模式的各 provider 服务默认注释掉。

**docker run：**

```bash
make docker-build
make docker-run
```

等效命令：

```bash
docker run -d --name ddns6 --restart unless-stopped \
  --network host \
  --read-only \
  --tmpfs /tmp:size=16m,mode=1777 \
  --security-opt no-new-privileges:true \
  --cap-drop ALL \
  --cap-add NET_ADMIN \
  -e DDNS6_LOG_FILE= \
  -v ${HOME}/.ddns6:/home/ddns6/.ddns6:ro \
  ddns6 run
```

**docker compose：**

```bash
# 确保主机已有 ~/.ddns6/config.yaml，且属主为容器用户 10001（权限 0700/0600）
sudo chown -R 10001:10001 ~/.ddns6
docker compose up -d
```

`ddns6-config` 片段示例：

```yaml
ddns6-config:
  <<: *ddns6
  volumes:
    - ${HOME}/.ddns6:/home/ddns6/.ddns6:ro
  command: ["run"]
```

### 方式二：`.env` + Compose CLI 模式（密钥进 argv，不推荐）

复制环境变量模板并填入凭据后，取消注释对应 provider 服务、注释掉 `ddns6-config`：

```bash
cp .env.example .env
# 编辑 .env 与 docker-compose.yml
docker compose up -d
```

Compose 将变量展开为 `ddns6 run <provider> --secret-id=...` 等形式。**密钥会出现在容器命令行参数中**（`docker inspect` / `ps` 可见），仅适合受控环境。多子域名请改用配置文件模式。

### 安全参数

Compose 与 `make docker-run` 采用以下约束（对齐最小权限原则）：

| 参数 | 值 | 说明 |
|------|-----|------|
| `cap_drop` | `ALL` | 丢弃全部 capability |
| `cap_add` | `NET_ADMIN` | Netlink 监听 IPv6 地址变化所需 |
| `read_only` | `true` | 根文件系统只读 |
| `tmpfs` | `/tmp:size=16m,mode=1777` | 只读根 FS 下的临时目录 |
| `security_opt` | `no-new-privileges:true` | 禁止进程提权 |
| 运行用户 | uid/gid `10001` | 非 root 用户 `ddns6` |

### Makefile 目标

| 命令 | 说明 |
|------|------|
| `make docker-build` | 构建镜像 `ddns6` |
| `make docker-run` | 以配置文件模式直接运行容器（推荐） |
| `make docker-up` | `docker compose up -d` |
| `make docker-logs` | `docker compose logs -f` |
| `make docker-down` | `docker compose down` |

## 日志

全局 flag `--log-file` 指定日志文件路径，默认 `ddns6.log`（相对当前工作目录）。日志同时写入 stderr 与文件。

设为**空字符串**时仅输出到 stderr，不写文件：

```bash
ddns6 run --log-file ""
```

也可通过环境变量 `DDNS6_LOG_FILE` 覆盖默认值。

### logrotate 示例

长期运行时可将日志交给 logrotate 轮转。创建 `/etc/logrotate.d/ddns6`：

```
/path/to/ddns6.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
```

将 `/path/to/ddns6.log` 替换为实际路径。`copytruncate` 适用于 ddns6 持续持有文件句柄的场景。

## Shell 自动补全

`ddns6 completion` 生成各 Shell 的补全脚本。将输出写入对应配置位置即可启用：

```bash
# Bash
ddns6 completion bash > /etc/bash_completion.d/ddns6

# Zsh
ddns6 completion zsh > "${fpath[1]}/_ddns6"

# Fish
ddns6 completion fish > ~/.config/fish/completions/ddns6.fish

# PowerShell
ddns6 completion powershell > ddns6.ps1
```

PowerShell 需在 profile 中 `source` 生成的 `ddns6.ps1`。支持的 shell：`bash`、`zsh`、`fish`、`powershell`。
