# 设计：README 总览 + docs/ 正式中文手册

日期：2026-09-14  
仓库：`github.com/notes-bin/ddns6`  
状态：待用户审阅 spec 后实施

## 背景与目标

用户选择：

- **文档风格**：正式项目手册（非口语精简版）
- **语言**：仅简体中文
- **形态**：`README.md` 作总览 + `docs/` 专题（轻量拆分）

成功标准：

1. GitHub 首页 `README.md` 可在短时间内完成「是什么 → 怎么装 → 怎么跑 → 去哪看细节」。
2. 四个专题文档覆盖运营商、部署、架构、开发，事实与当前代码一致（Go 1.27.1、23 家运营商、CI/Docker/Netlink 行为）。
3. 不改动运行时行为；仅文档变更（可顺带修正 README 中与代码不一致的表述）。
4. 文风正式、可读，避免宣传腔与空话；技术术语保留英文并在首次可附中文。

## 方案选择

采用 **方案 1：轻量拆分**（已确认）：

| 文件 | 职责 |
|------|------|
| `README.md` | 首页总览 |
| `docs/providers.md` | DNS 运营商 |
| `docs/deployment.md` | 部署与运维 |
| `docs/architecture.md` | 架构与行为 |
| `docs/development.md` | 开发与贡献 |

不采用「按使用路径拆很多文件」或「仅拆 providers」：前者维护成本高，后者首页仍过长。

明确不做：

- 不新增英文 README / 双语并列
- 不把 `docs/superpowers/` 流程文档改写成用户手册
- 不修改 Go 代码、Makefile、Docker 行为（除非文档发现事实错误且需在正文中纠正描述）
- 不为每个运营商单独建文件

## README.md 大纲

1. **标题与徽章**  
   - 项目名 + 一句话定位  
   - Badges：Go（读 `go.mod`）、License、Test workflow；可选 Release

2. **简介**  
   - 本机 IPv6 变化时更新 DNS AAAA  
   - 适用场景（如 PPPoE 重拨、家庭宽带）  
   - 明确不支持 IPv4 / A 记录

3. **特性**（条目列表，不堆实现细节）  
   - Linux Netlink + 非 Linux 轮询 + Netlink 失败回退  
   - 10 秒防抖  
   - 23 家运营商、多子域名  
   - 配置优先级：CLI > 环境变量 > 配置文件  
   - Docker 硬化、`check` / `list` / `records` / `clean`

4. **快速开始**  
   - 安装：GitHub Releases、`go build`、`make build`/`install`、`go install`（若适用）  
   - 临时 `run`  
   - `init` → `run`  
   - `check`  
   - 链到专题文档

5. **命令速查**  
   - 子命令一行说明表  
   - 全局参数精简表（与 `cmd/root.go` 的 `persistentFlags` 对齐）  
   - 细节指向 `ddns6 <cmd> --help`

6. **文档目录**  
   - 四个 `docs/*.md` 链接 + 各一句话说明  
   - 简短 FAQ（3–5 条高频）+ 其余指向专题

7. **安全要点**（短）  
   - `chmod 600`、最小权限令牌、Docker 优先挂载配置文件

8. **许可证**  
   - MIT → `LICENSE`

## docs/providers.md 大纲

1. 总览：`ddns6 list`；CLI 名与 `auth` 字段（`-` → `_`）对应关系  
2. 完整对照表（23 家，与 `cmd/providers.go` 的 `providerFactories` 一致）  
3. 受限运营商：`duckdns` / `he` / `noip`（无 records/clean）  
4. 配置通例 + 常见 YAML 示例（至少：tencent、cloudflare、alicloud 含 `sign_version: v3`）  
5. 注意事项：Token 权限、Namecheap 白名单 IP、AWS `session_token`、gcloud access token 等

## docs/deployment.md 大纲

1. 配置文件路径、字段、权限（`~/.ddns6/config.yaml`）  
2. systemd 单元与启停、日志查看  
3. Docker：多阶段镜像、非 root、`network_mode: host`、挂配置 vs Compose 环境变量、安全参数表、`docker run` / Makefile 目标  
4. 日志：`--log-file`、仅 stderr、logrotate 示例  
5. Shell 补全：bash / zsh / fish / powershell

## docs/architecture.md 大纲

1. 总体数据流（地址变化 → 取 IPv6 → 对比缓存 → 同步子域名）  
2. 触发器：Netlink / 轮询 / 回退  
3. 防抖：10 秒窗口重置  
4. 同步：GetRecords → 相同跳过 / Modify / Add；启动全量同步失败则退出；运行中错误只记日志  
5. IPv6 多源并发表（与 `pkg/ipaddr` / `DefaultIPv6Fetchers` 一致）  
6. SIGINT/SIGTERM 与退出等待（约 5 秒）

## docs/development.md 大纲

1. Go 1.27.1+（`go.mod`）  
2. Makefile：`build` / `test` / `fmt` / `cross-build` / `release` / Docker 目标  
3. 与 CI 对齐的 vet + race + cover 命令  
4. Workflow：`test.yml`、`release.yml`  
5. 项目结构树（反映当前目录，含 `cmd`、`internal`、`pkg`、workflows）  
6. 新增运营商步骤（与 `providers.go` 文件头注释一致）  
7. 简要贡献约定（测试、gofmt、文档同步）

## 事实来源（实施时核对）

| 主题 | 权威来源 |
|------|----------|
| Go 版本 | `go.mod` |
| 运营商列表与 flag | `cmd/providers.go` |
| 全局 flag / 环境变量 | `cmd/root.go` `persistentFlags` |
| 配置结构 | `internal/config/` |
| Netlink / 防抖 / 轮询 | `internal/ddns/service_linux.go`、`service_other.go`、`service.go` |
| IPv6 源 | `internal/ddns` 默认 fetcher / `pkg/ipaddr` |
| Docker | `Dockerfile`、`docker-compose.yml`、`.env.example` |
| CI | `.github/workflows/test.yml`、`release.yml` |
| 构建 | `Makefile` |

## 文风约定

- 简体中文；标识符与命令保持英文  
- 正式手册语气：完整句子、清晰小标题、表格优先于长段落  
- 避免宣传腔、空泛形容词、无信息的「强大/轻松/一站式」等套话  
- 示例域名统一 `example.com`；密钥用占位符 `xxx` / `YOUR_*`

## 实施顺序

1. 新建四个 `docs/*.md`（按大纲写全，并与代码核对）  
2. 重写 `README.md`（总览 + 链接）  
3. 通读交叉链接与目录是否可达  
4. （可选）请用户确认后单独提交文档 commit

## 验收清单

- [ ] README 含文档目录且四个链接有效  
- [ ] 运营商表 23 家，CLI 名与代码一致  
- [ ] 全局参数默认值与 `root.go` 一致  
- [ ] Docker / systemd / 架构描述与实现一致  
- [ ] 开发章节 Go 版本与 CI 描述正确  
- [ ] 全文简体中文，无英文平行章节  
