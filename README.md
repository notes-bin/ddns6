# ddns6 - 动态 DNS 更新工具

一个用 Go 编写的 CLI 工具，自动检测本机 IPv6 地址变化并更新到多个 DNS 服务商的 AAAA 记录。

## 功能特性

- **多服务商支持**：腾讯云 DNSPod、阿里云 DNS、华为云 DNS、Cloudflare、GoDaddy
- **并发竞速检测**：同时通过 HTTP 端点（6.ipw.cn、ifconfig.co、v6.ident.me）和 UDP DNS 直连（多个公共 DNS 服务器）获取本机 IPv6，取第一个成功结果
- **智能更新**：仅在 IPv6 地址变化时才调用 DNS API，避免无效请求
- **定时调度**：首次立即更新，之后按可配置的间隔周期检查
- **优雅退出**：监听系统信号（SIGINT/SIGTERM），安全关闭

## 安装

### 从源码编译

```bash
git clone https://github.com/notes-bin/ddns6.git
cd ddns6
make build        # 编译二进制
make install      # 安装到 $GOPATH/bin
```

### 交叉编译

```bash
make cross-build  # 生成 linux_amd64、darwin_amd64、darwin_arm64 三个平台二进制
make release      # 打包为 tar.gz 发布包
```

### Docker 部署

```bash
# 1. 复制环境变量模板，填入实际凭证
cp .env.example .env

# 2. 编辑 .env，填入你的域名和 provider 凭证
# 3. 在 docker-compose.yml 中取消注释你要用的 provider
# 4. 构建并启动
make docker-build
make docker-up

# 查看日志
make docker-logs

# 停止
make docker-down
```

也可直接使用 docker 命令：

```bash
docker build -t ddns6 .

# 腾讯云示例
docker run -d --name ddns6 --restart always ddns6 \
  run tencent \
  --secret-id ID --secret-key KEY \
  --domain example.com --subdomain www --interval 5m
```

## 使用说明

### 基本语法

```bash
./ddns6 [全局选项] run <provider> [provider 选项]
```

### 全局选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--debug` | 启用调试日志 | `false` |
| `--domain` | 根域名，如 `example.com` | — |
| `--subdomain` | 子域名，`@` 表示根域名 | `@` |
| `--interval` | 检查间隔，如 `5m`、`1h` | `5m` |

### 各服务商参数

#### 腾讯云 DNSPod

```bash
./ddns6 run tencent \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY \
  --domain example.com \
  --subdomain www
```

| 参数 | 说明 |
|------|------|
| `--secret-id` | 腾讯云 SecretId |
| `--secret-key` | 腾讯云 SecretKey |

#### 阿里云 DNS

```bash
./ddns6 run alicloud \
  --access-key-id YOUR_ACCESS_KEY_ID \
  --access-key-secret YOUR_ACCESS_KEY_SECRET \
  --domain example.com \
  --subdomain www
```

| 参数 | 说明 |
|------|------|
| `--access-key-id` | 阿里云 AccessKeyId |
| `--access-key-secret` | 阿里云 AccessKeySecret |

#### 华为云 DNS

```bash
./ddns6 run huaweicloud \
  --username YOUR_USERNAME \
  --password YOUR_PASSWORD \
  --domain-name YOUR_DOMAIN_NAME \
  --domain example.com \
  --subdomain www
```

| 参数 | 说明 |
|------|------|
| `--username` | 华为云用户名 |
| `--password` | 华为云密码 |
| `--domain-name` | 华为云账户域名 |

#### Cloudflare

```bash
./ddns6 run cloudflare \
  --api-token YOUR_API_TOKEN \
  --domain example.com \
  --subdomain www
```

| 参数 | 说明 |
|------|------|
| `--api-token` | Cloudflare API Token |

#### GoDaddy

```bash
./ddns6 run godaddy \
  --api-key YOUR_API_KEY \
  --api-secret YOUR_API_SECRET \
  --domain example.com \
  --subdomain www
```

| 参数 | 说明 |
|------|------|
| `--api-key` | GoDaddy API Key |
| `--api-secret` | GoDaddy API Secret |

## 工作原理

1. 通过 HTTP 端点和 DNS 直连并发获取本机 IPv6 地址（5 秒超时，取最先成功的结果）
2. 与上次缓存的地址比较，无变化则跳过
3. 调用 DNS 服务商 API 查询当前 AAAA 记录
4. 已有相同 IP 则跳过，IP 不同则修改，无记录则新增
5. 按 `--interval` 间隔循环执行

## 项目结构

```
cmd/                  # CLI 入口（cobra 命令定义、provider 注册、调度循环）
internal/providers/   # 核心包
  record.go           # Domain 结构体 + DNSProvider 接口 + RecordInfo 通用类型
  tencent/            # 腾讯云 DNSPod（TC3-HMAC-SHA256 签名）
  alicloud/           # 阿里云 DNS（HMAC-SHA1 签名）
  cloudflare/         # Cloudflare DNS（Bearer Token 认证）
  godaddy/            # GoDaddy DNS（SSO Key 认证）
  huaweicloud/        # 华为云 DNS（IAM Token 认证）
pkg/
  ipaddr/             # IPv6 地址获取（并发竞速，HTTP + DNS 双通道）
  env/                # 环境变量解析工具
```

## 开发

```bash
make test         # 运行全部测试
make fmt          # 格式化代码
go test -v ./internal/providers/cloudflare/  # 运行单个包的测试
go test -v -cover ./...                       # 带覆盖率的测试
```

## 常见问题

**如何验证配置是否正确？**
```bash
./ddns6 --debug run tencent --secret-id ID --secret-key KEY --domain example.com
```
首次运行会立即尝试更新，日志中可看到详细过程。

**支持哪些记录类型？**
目前仅支持 AAAA（IPv6）记录。A 记录支持不在计划内。

## 注意事项

- 运行环境需要支持 IPv6 网络（本机和外网）
- 确保 DNS 服务商 API 地址可正常访问，凭证有效

## 许可证

MIT License — 详见 [LICENSE](LICENSE)
