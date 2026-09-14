# README 与 docs 正式手册 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将用户文档重写为「`README.md` 总览 + `docs/` 四专题」的正式简体中文手册，并与当前代码事实一致。

**Architecture:** 按设计稿轻量拆分：首页只保留定位、快速开始、命令速查、文档索引与安全要点；运营商、部署、架构、开发分别写入 `docs/*.md`。实施顺序先专题后 README，保证首页链接指向已存在文件。

**Tech Stack:** Markdown；事实来源为仓库内 Go/YAML/Makefile/Workflow（见 Global Constraints）。

## Global Constraints

- 语言：仅简体中文；标识符与命令保持英文
- 风格：正式手册；禁止宣传腔与空话
- 不改运行时行为：仅文档（`README.md`、`docs/*.md`）
- Go 版本：`1.27.1`（`go.mod`）
- 运营商数量：23（`cmd/providers.go` `providerFactories`）
- 受限 records/clean：`duckdns`、`he`、`noip`
- 配置路径：`~/.ddns6/config.yaml`
- 优先级：CLI > `DDNS6_*` 环境变量 > 配置文件
- 防抖：`10 * time.Second`（`internal/ddns/service_linux.go`）
- 优雅退出等待：最多 5 秒（`internal/ddns/service.go`）
- 设计稿：`docs/superpowers/specs/2026-09-14-readme-docs-handbook-design.md`

## File Map

| 文件 | 操作 | 职责 |
|------|------|------|
| `docs/providers.md` | Create | 23 家运营商对照与配置示例 |
| `docs/deployment.md` | Create | 配置、systemd、Docker、日志、补全 |
| `docs/architecture.md` | Create | 触发、防抖、同步、IPv6 源 |
| `docs/development.md` | Create | 构建、CI、结构、加运营商 |
| `README.md` | Rewrite | 首页总览与文档索引 |

---

### Task 1: 创建 `docs/providers.md`

**Files:**
- Create: `docs/providers.md`
- Verify against: `cmd/providers.go`、`cmd/list.go`

**Interfaces:**
- Consumes: 无
- Produces: `docs/providers.md`（供 README 链接）

- [ ] **Step 1: 从代码导出运营商清单**

Run:

```bash
rg -n 'name: "' cmd/providers.go | head -40
rg -n 'noListClean: true' cmd/providers.go
```

Expected: 23 个 `name:` 条目；`duckdns`/`noip`/`he` 带 `noListClean: true`。

- [ ] **Step 2: 写入完整 `docs/providers.md`**

文件须包含以下章节与对照表列（与设计稿一致）：

1. 标题与说明：`ddns6 list`；CLI flag 名中 `-` 在 `auth` 中变为 `_`
2. 完整表列：`运营商 | CLI 名称 | 必填参数 | 配置文件字段 (auth) | records/clean | 说明`
3. 23 行数据与 `providerFactories` 顺序可不同，但名称集合必须一致
4. 「受限运营商」专节说明三家仅更新 API
5. 配置通例 YAML
6. 至少三个完整示例：`tencent`、`cloudflare`、`alicloud`（含 `sign_version: "v3"`）
7. 「注意事项」：Cloudflare DNS:Edit；Namecheap `client_ip` 白名单；AWS 可选 `session_token`；gcloud 的 `access_token`；Azure Service Principal 四字段

写入文件后可用下列内容作为结构骨架（实施时填满表格，勿留空行占位）：

```markdown
# DNS 运营商

本文说明 ddns6 支持的 DNS 运营商、认证参数与配置文件字段。

运行 `ddns6 list` 可在本地查看内置名单（不读配置、不访问网络）。

## 参数对应关系

命令行 flag 使用连字符（如 `--secret-id`），写入 `~/.ddns6/config.yaml` 的 `auth` 时改为下划线（如 `secret_id`）。

## 对照表

| 运营商 | CLI 名称 | 必填参数 | 配置文件字段 (`auth`) | records/clean | 说明 |
|--------|---------|---------|----------------------|---------------|------|
| … | … | … | … | 支持/受限 | … |

## 受限运营商

`duckdns`、`he`、`noip` 的 API 仅提供更新接口。对这三家执行 `records` 或 `clean` 会返回错误，请在官网面板管理记录。

## 配置示例

### 通例

### 腾讯云 DNSPod（tencent）

### Cloudflare（cloudflare）

### 阿里云 DNS（alicloud，V3 签名）

## 注意事项

…
```

- [ ] **Step 3: 核对名称集合**

Run:

```bash
python3 - <<'PY'
import re, pathlib
src = pathlib.Path("cmd/providers.go").read_text()
names = re.findall(r'name:\s*"([a-z0-9]+)"', src)
# providerFactory 结构里 name 字段；过滤非工厂（应约 23）
# 更稳妥：只取 providerFactories 块内
start = src.index("var providerFactories")
block = src[start:src.index("\n}", start)+2]
factory_names = re.findall(r'\n\t\tname:\s*"([^"]+)"', block)
doc = pathlib.Path("docs/providers.md").read_text()
missing = [n for n in factory_names if f"`{n}`" not in doc and n not in doc]
print("count", len(factory_names))
print("missing_in_doc", missing)
assert len(factory_names) == 23
assert not missing
print("OK")
PY
```

Expected: `count 23`、`missing_in_doc []`、`OK`。

- [ ] **Step 4: Commit**

```bash
git add docs/providers.md
git commit -m "$(cat <<'EOF'
docs: 新增运营商专题文档

将 23 家 DNS 运营商对照表与配置示例拆至 docs/providers.md。
EOF
)"
```

---

### Task 2: 创建 `docs/deployment.md`

**Files:**
- Create: `docs/deployment.md`
- Verify against: `internal/config/config.go`、`Dockerfile`、`docker-compose.yml`、`.env.example`、`Makefile`、`cmd/root.go`（completion）

**Interfaces:**
- Consumes: 无
- Produces: `docs/deployment.md`

- [ ] **Step 1: 核对配置字段与 Docker 事实**

Run:

```bash
rg -n 'type Config struct' -A20 internal/config/config.go
rg -n 'network_mode|cap_|read_only|ddns6-config|10001' Dockerfile docker-compose.yml
```

Expected: Config 含 `provider/auth/domain/subdomains/interval/interface/ttl`；镜像非 root uid 10001；compose 使用 host 网络等。

- [ ] **Step 2: 写入 `docs/deployment.md`**

必须章节：

1. **配置文件**：路径 `~/.ddns6/config.yaml`；字段表；`chmod 600`；权限过宽警告（Unix）
2. **systemd**：单元文件示例（`After=network-online.target`、`ExecStart=/usr/local/bin/ddns6 run`、`Restart=always`）；`daemon-reload` / `enable --now` / `journalctl`
3. **Docker**：
   - 构建：`golang:1.27.1-alpine` → `alpine:3.21`；用户 `ddns6` uid/gid 10001
   - 为何需要 `network_mode: host`（Netlink）
   - 方式一：挂载 `~/.ddns6` → `/home/ddns6/.ddns6:ro`（推荐）
   - 方式二：`.env` + Compose CLI（密钥进 argv，仅可信环境）
   - 安全参数表：`cap_drop ALL`、`cap_add NET_ADMIN`、`read_only`、`tmpfs /tmp`、`no-new-privileges`
   - `make docker-build` / `docker-run` / `docker-up` / `docker-logs` / `docker-down`
4. **日志**：默认 `ddns6.log`；`--log-file ""` 仅 stderr；logrotate 示例
5. **Shell 补全**：bash / zsh / fish / powershell（与 `cmd/root.go` completion Long 一致）

- [ ] **Step 3: 链接与路径自检**

Run:

```bash
test -f docs/deployment.md
rg -n 'network_mode|chmod 600|completion' docs/deployment.md
```

Expected: 文件存在；命中上述关键词。

- [ ] **Step 4: Commit**

```bash
git add docs/deployment.md
git commit -m "$(cat <<'EOF'
docs: 新增部署与运维专题文档

覆盖配置、systemd、Docker、日志与 Shell 补全。
EOF
)"
```

---

### Task 3: 创建 `docs/architecture.md`

**Files:**
- Create: `docs/architecture.md`
- Verify against: `internal/ddns/service.go`、`service_linux.go`、`service_other.go`、`processor.go` / `record.go`

**Interfaces:**
- Consumes: 无
- Produces: `docs/architecture.md`

- [ ] **Step 1: 核对行为常量**

Run:

```bash
rg -n 'debounceDuration|DefaultIPv6Fetchers|5 \* time\.Second|startTrigger' internal/ddns/
```

Expected: debounce 10s；7 个 fetcher；shutdown 5s。

- [ ] **Step 2: 写入 `docs/architecture.md`**

必须内容：

1. 总体数据流（ASCII 或 mermaid 均可）：

```
地址变化 → GetIPv6Addr（多源并发） → 对比缓存
  ├─ 未变 → 跳过
  └─ 已变 → 并发同步各子域名
        ├─ GetRecords
        ├─ IP 相同 → 跳过；不同 → ModifyRecord
        └─ 无记录 → AddRecord
```

2. 触发器表：Linux Netlink（`RTM_NEWADDR`）/ 其他平台轮询（默认 5m）/ Linux Netlink 失败回退轮询
3. 防抖：新全局单播 IPv6 后等 10 秒；窗口内再事件则重置计时
4. 启动：先完整同步，失败则退出；运行中失败只记日志
5. IPv6 源表：

| 来源 | 类型 |
|------|------|
| `https://6.ipw.cn` | HTTP |
| `https://ifconfig.co` | HTTP |
| `https://v6.ident.me` | HTTP |
| `2402:4e00::` | DNS |
| `2400:3200:baba::1` | DNS |
| `2001:4860:4860::8888` | DNS |
| `2606:4700:4700::1111` | DNS |

6. 信号：SIGINT/SIGTERM；最多等待 5 秒结束当前同步

- [ ] **Step 3: 核对 IPv6 URL 出现在文档中**

Run:

```bash
for u in '6.ipw.cn' 'ifconfig.co' 'v6.ident.me' '2402:4e00::' '2400:3200:baba::1' '2001:4860:4860::8888' '2606:4700:4700::1111'; do
  rg -q "$u" docs/architecture.md || { echo "missing $u"; exit 1; }
done
echo OK
```

Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add docs/architecture.md
git commit -m "$(cat <<'EOF'
docs: 新增架构专题文档

说明触发器、防抖、同步流程与 IPv6 多源获取。
EOF
)"
```

---

### Task 4: 创建 `docs/development.md`

**Files:**
- Create: `docs/development.md`
- Verify against: `Makefile`、`.github/workflows/test.yml`、`.github/workflows/release.yml`、`cmd/providers.go` 文件头、仓库树

**Interfaces:**
- Consumes: 无
- Produces: `docs/development.md`

- [ ] **Step 1: 列出目录骨架**

Run:

```bash
find . -maxdepth 3 \( -name '*.go' -o -type d \) \
  ! -path './.git*' ! -path './bin*' ! -path './.superpowers*' \
  | head -80
```

- [ ] **Step 2: 写入 `docs/development.md`**

必须章节：

1. 环境：Go 1.27.1+
2. 常用命令：`make build` / `test` / `fmt` / `cross-build` / `release` / Docker 目标；并给出与 CI 一致的：

```bash
go vet ./...
go test ./... -race -count=1 -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | tail -n 1
```

3. CI 表：

| Workflow | 触发 | 行为 |
|----------|------|------|
| `test.yml` | push/PR → `main` | vet + test -race + cover |
| `release.yml` | tag `v*` | 测试后构建 linux/amd64、darwin/amd64、darwin/arm64 并上传 Release |

4. 项目结构树（含 `main.go`、`cmd/`、`internal/{config,crypto,ddns,providers}`、`pkg/{domainutil,ipaddr,retry}`、Docker、workflows）
5. 新增运营商五步（与 `cmd/providers.go` 顶部注释一致）：实现 Provider → 注册工厂 → `noListClean`/`restrictedProviders` → 更新 compose/env/README/docs → `go test`
6. 贡献约定：`gofmt`、补测试、文档与代码同步

- [ ] **Step 3: 确认 Go 版本与 workflow 文件名出现在文档**

Run:

```bash
rg -n '1\.27\.1|test\.yml|release\.yml|providerFactories' docs/development.md
```

Expected: 均有命中。

- [ ] **Step 4: Commit**

```bash
git add docs/development.md
git commit -m "$(cat <<'EOF'
docs: 新增开发专题文档

说明构建测试、CI/Release、项目结构与新增运营商步骤。
EOF
)"
```

---

### Task 5: 重写 `README.md` 并验收交叉链接

**Files:**
- Modify: `README.md`（整体重写）
- Verify: `docs/providers.md`、`docs/deployment.md`、`docs/architecture.md`、`docs/development.md` 均已存在

**Interfaces:**
- Consumes: 四个专题文档路径
- Produces: 首页总览

- [ ] **Step 1: 确认四个专题已存在**

Run:

```bash
test -f docs/providers.md && test -f docs/deployment.md && \
  test -f docs/architecture.md && test -f docs/development.md && echo OK
```

Expected: `OK`

- [ ] **Step 2: 按设计稿重写 `README.md`**

章节顺序固定为：

1. 标题 + badges（Go 1.27.1+、MIT、Test；可选 Release）
2. 简介（含「不支持 A/IPv4」）
3. 特性列表
4. 快速开始（Releases / build / make / `go install github.com/notes-bin/ddns6@latest`；run；init；check）
5. 命令速查（子命令表 + 全局参数表，字段与 `cmd/root.go` `persistentFlags` 一致）
6. 文档目录（四个相对链接）+ 简短 FAQ（3–5 条）
7. 安全要点
8. 许可证

全局参数表必须包含：

| 参数 | 环境变量 | 默认值 |
|------|---------|--------|
| `--domain` | `DDNS6_DOMAIN` | （空） |
| `--subdomain` | `DDNS6_SUBDOMAIN` | `@` |
| `--ttl` | `DDNS6_TTL` | `600` |
| `--interval` | `DDNS6_INTERVAL` | `5m` |
| `--interface` | `DDNS6_INTERFACE` | （空） |
| `--log-file` | `DDNS6_LOG_FILE` | `ddns6.log` |
| `--debug` | `DDNS6_DEBUG` | `false` |
| `-V` / `--version` | — | — |

FAQ 至少覆盖：如何 `check`；Netlink 是否要 root；为何 Docker 要 host 网络；duckdns 等不能 records/clean。

- [ ] **Step 3: 交叉链接与事实验收**

Run:

```bash
python3 - <<'PY'
from pathlib import Path
readme = Path("README.md").read_text()
for p in [
    "docs/providers.md",
    "docs/deployment.md",
    "docs/architecture.md",
    "docs/development.md",
]:
    assert p in readme, p
    assert Path(p).is_file(), p
assert "1.27.1" in readme
assert "DDNS6_DOMAIN" in readme
assert "不支持" in readme or "IPv4" in readme
print("OK")
PY
```

Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs: 重写 README 为总览手册并链接专题文档

精简首页为快速开始与索引，详细内容指向 docs/ 专题。
EOF
)"
```

---

## Self-Review

1. **Spec coverage:** README 八段、四个专题大纲、事实来源表、文风与「明确不做」均有对应 Task；验收清单映射到 Task 5 Step 3 与各 Task 核对步骤。
2. **Placeholder scan:** 无 TBD/TODO；表格要求写明列名与核对命令。
3. **Consistency:** 文档路径与设计稿一致；Go 1.27.1、23 家、三家受限、防抖 10s、退出 5s 与代码一致。

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-14-readme-docs-handbook.md`.

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 每个 Task 派一个新子代理，任务间复核  
2. **Inline Execution** — 本会话按 `executing-plans` 连续执行并设检查点  

你选哪一种？
