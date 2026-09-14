# 开发指南

本文说明本地开发环境、常用构建与测试命令、CI/Release 流程、项目目录结构，以及如何新增 DNS 运营商。架构原理见 [`docs/architecture.md`](architecture.md)，运营商字段见 [`docs/providers.md`](providers.md)，部署方式见 [`docs/deployment.md`](deployment.md)。

## 开发环境

| 依赖 | 版本 / 说明 |
|------|-------------|
| Go | **1.27.1+**（与 `go.mod`、CI 工作流一致） |
| Git | 用于版本号注入（`git describe`） |
| Docker（可选） | 本地镜像构建与 Compose 验证 |
| Make | 执行 `Makefile` 中的常用目标 |

克隆仓库后，确认 Go 版本：

```bash
go version   # 应显示 go1.27.1 或更高
```

## 常用命令

项目根目录提供 `Makefile`，本地开发与 CI 分工如下：日常构建与格式化走 `make`，与 GitHub Actions 一致的静态分析与竞态测试需手动执行（见下一节）。

### Makefile 目标

| 命令 | 说明 |
|------|------|
| `make build` | 编译二进制到 `bin/ddns6`，注入 `Version` / `Commit` / `buildAt` |
| `make test` | 运行全部单元测试（`go test -v ./...`） |
| `make fmt` | 格式化代码（`go fmt ./...`） |
| `make cross-build` | 交叉编译 linux/amd64、darwin/amd64、darwin/arm64 |
| `make release` | 清理后交叉编译并生成各平台 `.tar.gz` 发布包 |
| `make install` | 安装到 `$GOPATH/bin` 或 `$GOBIN` |
| `make run` | 构建并运行 `bin/ddns6` |
| `make clean` | 删除 `bin/`、本地压缩包及 Go 缓存 |
| `make help` | 列出全部 Make 目标 |

### Docker 目标

| 命令 | 说明 |
|------|------|
| `make docker-build` | 构建镜像 `ddns6` |
| `make docker-run` | 以 `docker run` 启动（`--network host`、`--cap-add NET_ADMIN`，挂载 `~/.ddns6`） |
| `make docker-up` | `docker compose up -d` |
| `make docker-logs` | `docker compose logs -f` |
| `make docker-down` | `docker compose down` |

### 与 CI 一致的测试命令

推送前建议执行与 `.github/workflows/test.yml` 相同的检查：

```bash
go vet ./...
go test ./... -race -count=1 -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | tail -n 1
```

## CI 与 Release

| Workflow | 触发 | 行为 |
|----------|------|------|
| `test.yml` | push / PR → `main` | `go vet ./...` + `go test -race -cover` + 输出覆盖率摘要 |
| `release.yml` | 推送 tag `v*` | 测试与 vet 通过后，矩阵构建 linux/amd64、darwin/amd64、darwin/arm64，打包并上传 GitHub Release |

### test.yml 要点

- Go 版本：`1.27.1`
- 分支：`main`
- 权限：`contents: read`

### release.yml 要点

- 触发：`v*` 标签（如 `v0.0.190`）
- 构建产物：`ddns6_<goos>_<goarch>.tar.gz`（含二进制、`LICENSE`、`README.md`）
- Release 由 `softprops/action-gh-release` 创建，并自动生成变更日志

## 项目结构

```
ddns6/
├── main.go                          # 程序入口，调用 cmd.Execute()
├── cmd/                             # CLI（Cobra）：run / records / clean / init / list 等
│   ├── root.go
│   ├── providers.go                 # 23 家运营商工厂注册（providerFactories）
│   ├── records.go / clean.go / ...
│   └── *_test.go
├── internal/
│   ├── config/                      # 配置文件读写与校验
│   ├── crypto/                      # 签名等密码学辅助
│   ├── ddns/                        # 核心服务：触发、同步、records/clean 逻辑
│   └── providers/                   # 各 DNS 运营商 API 实现
│       ├── tencent/ cloudflare/ aws/ ...
│       └── <name>/                  # 新增运营商在此建子目录
├── pkg/
│   ├── domainutil/                  # 域名拆分等工具
│   ├── ipaddr/                      # IPv6 多源获取
│   └── retry/                       # 重试辅助
├── docs/                            # 项目文档
│   ├── architecture.md
│   ├── deployment.md
│   ├── providers.md
│   └── development.md               # 本文
├── .github/workflows/
│   ├── test.yml                     # PR / main 分支 CI
│   └── release.yml                  # 版本 tag 发布
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── Makefile
└── go.mod                           # module github.com/notes-bin/ddns6, go 1.27.1
```

### 模块职责速查

| 路径 | 职责 |
|------|------|
| `cmd/providers.go` | 集中注册 `providerFactories`，挂载各运营商到 CLI 子命令 |
| `internal/ddns` | DDNS 服务编排、记录同步、Linux Netlink / 轮询触发 |
| `internal/providers/*` | 实现 `ddns.DNSProvider` 接口，对接各云厂商 DNS API |
| `pkg/ipaddr` | 公网 IPv6 多 Fetcher 并发竞速 |
| `pkg/domainutil` | 根域名与子域名解析 |
| `pkg/retry` | HTTP 等操作的重试封装 |

## 新增 DNS 运营商

新增运营商时，遵循 `cmd/providers.go` 文件头注释中的流程，并补充文档与部署示例。完整步骤如下。

### 第一步：实现 Provider

在 `internal/providers/<name>/` 新建包，实现 `ddns.DNSProvider` 接口（`GetRecords`、`AddRecord`、`ModifyRecord`、`DeleteRecord` 等，视 API 能力而定）。

建议同目录添加 `<name>_test.go`，对请求构造、响应解析等可测逻辑编写单元测试。

### 第二步：注册工厂

在 `cmd/providers.go` 的 `providerFactories` 切片中追加一条 `providerFactory`：

- `name`：CLI 与配置文件中的运营商标识
- `flags`：命令行认证参数
- `run`：从 flag 构造 `[]*ddns.Domain` 与 `ddns.DNSProvider`
- `fromConfig`：从 `config.Config` 构造 `ddns.DNSProvider`

注册后，`run`、`records`、`clean`、`init` 等子命令会自动识别新运营商。

### 第三步：受限 API 标记

若目标 API **仅支持更新、不支持查询或删除**（如 DuckDNS、HE、No-IP）：

1. 在对应 `providerFactory` 上设置 `noListClean: true`
2. 将运营商名加入 `restrictedProviders` map

此时 `records` / `clean` 子命令会注册提示性占位，避免用户误以为支持列表或清理。

### 第四步：更新部署与文档

同步修改以下文件，保证用户能配置并运行新运营商：

| 文件 | 内容 |
|------|------|
| `docker-compose.yml` | 新增 provider 对应的 Compose 服务示例（如适用） |
| `.env.example` | 补充环境变量占位与注释 |
| `docs/providers.md` | 新增运营商认证字段、CLI 示例、配置 YAML 示例 |
| `README.md` | 更新运营商列表、快速入门示例及相关 Compose 说明 |

字段命名约定：CLI 使用 kebab-case（如 `--secret-id`），配置文件 `auth` 块使用 snake_case（如 `secret_id`）。

### 第五步：验证

```bash
make fmt
go vet ./...
go test ./... -race -count=1
make build
```

可选：用 `ddns6 init <provider>` 或 `docker compose` 做端到端冒烟验证。

## 贡献约定

- **格式化**：提交前执行 `make fmt`（或 `gofmt -w`），保持与仓库风格一致
- **测试**：新逻辑应补充或更新 `_test.go`；涉及并发路径时注意 `-race` 可通过
- **文档同步**：代码行为、CLI flag、配置字段变更时，同步更新 `README.md`、`docs/providers.md`、`docs/deployment.md`
- **提交前检查**：至少运行与 CI 相同的 `go vet` 与 `go test -race` 命令
- **版本发布**：合并到 `main` 后由维护者打 `v*` tag 触发 `release.yml` 自动构建 Release
