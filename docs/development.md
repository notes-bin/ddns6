# 开发指南

本文说明本地开发、构建与测试、CI/Release、目录结构，以及如何新增 DNS 运营商。架构见 [`docs/architecture.md`](architecture.md)，运营商见 [`docs/providers.md`](providers.md)，部署见 [`docs/deployment.md`](deployment.md)。

## 开发环境

| 依赖 | 版本 / 说明 |
|------|-------------|
| Go | **1.27.1+**（与 `go.mod`、CI 一致） |
| Git | 版本号注入（`git describe`） |
| Docker（可选） | 镜像与 Compose |
| Make | `Makefile` 常用目标 |
| golangci-lint / govulncheck（可选） | 与 CI 对齐；`go.mod` 已声明相关 tool |

```bash
go version   # 应显示 go1.27.1 或更高
```

## 常用命令

### Makefile

| 命令 | 说明 |
|------|------|
| `make build` | 编译到 `bin/ddns6`，注入 Version / Commit / buildAt |
| `make test` | `go test -v ./...` |
| `make fmt` | `go fmt ./...` |
| `make cross-build` | linux/amd64、darwin/amd64、darwin/arm64 |
| `make release` | 交叉编译并打包到 `dist/`（`COPYFILE_DISABLE=1`，避免 macOS `._*` 元数据） |
| `make install` | 安装到 `$GOPATH/bin` 或 `$GOBIN` |
| `make run` / `make clean` / `make help` | 运行、清理 `bin/`+`dist/`、帮助 |

### Docker

| 命令 | 说明 |
|------|------|
| `make docker-build` | 构建镜像 `ddns6` |
| `make docker-run` | host 网络 + `NET_ADMIN`，挂载 `~/.ddns6` |
| `make docker-up` / `logs` / `down` | Compose 启停与日志 |

### 与 CI 对齐的检查

推送前建议执行与 `.github/workflows/test.yml` 相同的检查：

```bash
go mod verify
go tool govulncheck ./...
golangci-lint run ./...
go vet ./...
go test ./... -race -count=1 -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | tail -n 1   # 总覆盖率须 ≥ 80%
```

## CI 与 Release

| Workflow | 触发 | 行为 |
|----------|------|------|
| `test.yml` | push / PR → `main` | `go mod verify`、`govulncheck`、`golangci-lint`、`go vet`、`go test -race -cover`，覆盖率 &lt; 80% 失败 |
| `release.yml` | tag `v*` | 质量门禁后矩阵构建并上传 GitHub Release |

- Go 版本：`1.27.1`
- Release 产物：`ddns6_<goos>_<goarch>.tar.gz`（含二进制、`LICENSE`、`README.md`）

## 项目结构

```
ddns6/
├── cmd/
│   └── ddns6/main.go                # 程序入口（仅调用 cli.Execute）
├── internal/
│   ├── cli/                         # Cobra：run / init / check / list / records / clean …
│   │   └── providers.go             # 23 家 providerFactories
│   ├── config/                      # 配置读写；Unix 0600 fail-closed
│   ├── crypto/                      # 签名辅助
│   ├── ddns/                        # 触发、单飞同步、records/clean
│   ├── httputil/                    # HTTP 客户端、脱敏、有界读体
│   ├── metrics/                     # 可选 loopback Prometheus
│   └── providers/                   # 各运营商 DNS API
├── pkg/
│   ├── domainutil/                  # SplitDomain、ZoneCandidates
│   ├── ipaddr/                      # IPv6 多源竞速
│   └── retry/
├── docs/                            # 用户手册（不含 superpowers 流程稿）
├── .github/workflows/
│   ├── test.yml
│   └── release.yml
├── Dockerfile
├── docker-compose.yml               # 默认 ddns6-config 挂载模式
├── Makefile
└── go.mod
```

| 路径 | 职责 |
|------|------|
| `internal/cli/providers.go` | 注册工厂；`optional` flag 不强制非空 |
| `internal/ddns` | 编排；同根域一次 `GetRecords` |
| `internal/httputil` | `NewHTTPClient`（同主机重定向）、`RedactSecrets` |
| `pkg/domainutil` | zone 后缀候选 `ZoneCandidates` |
| `internal/providers/*` | 实现 `ddns.DNSProvider` |

## 新增 DNS 运营商

遵循 `internal/cli/providers.go` 文件头注释。

### 1. 实现 Provider

在 `internal/providers/<name>/` 实现 `DNSProvider`。`GetRecords(ctx, root, type)` 应返回该 zone 下该类型的**全部**记录（编排层按子域名过滤）。zone 探测优先用 `domainutil.ZoneCandidates`；路径段使用 `url.PathEscape`；出站客户端优先 `httputil.NewHTTPClient`。

### 2. 注册工厂

在 `providerFactories` 追加：`name`、`flags`（可选参数设 `optional: true`）、`run`、`fromConfig`。

### 3. 受限 API

仅更新、无查询/删除时：`noListClean: true`，并加入 `restrictedProviders`。

### 4. 文档与部署

更新 `docs/providers.md`、`docker-compose.yml`、`.env.example`；CLI kebab-case ↔ 配置 snake_case。

### 5. 验证

```bash
make fmt
go vet ./...
go test ./... -race -count=1
golangci-lint run ./...
make build
```

## 贡献约定

- 提交前 `make fmt`；并发路径注意 `-race`
- 行为 / flag / 配置变更时同步 `README.md` 与 `docs/*`
- 覆盖率保持 ≥ 80%（与 CI 门禁一致）
- 发布：维护者打 `v*` tag 触发 `release.yml`
