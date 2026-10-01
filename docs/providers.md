# DNS 运营商

本文说明 ddns6 支持的 DNS 运营商、认证参数与配置文件字段。

运行 `ddns6 list` 可在本地查看内置名单（不读配置、不访问网络）。

## 参数对应关系

命令行 flag 使用连字符（如 `--secret-id`），写入 `~/.ddns6/config.yaml` 的 `auth` 时改为下划线（如 `secret_id`）。

可选参数同样遵循此规则（如 `--sign-version` → `sign_version`）。`ddns6 init <provider>` 会按所选运营商生成对应的 `auth` 字段。

更细的 flag 说明见 `ddns6 run <provider> --help`。

## 对照表

共 23 家运营商，与 `cmd/providers.go` 中 `providerFactories` 一致。

| 运营商 | CLI 名称 | 必填参数 | 配置文件字段 (`auth`) | records/clean | 说明 |
|--------|---------|---------|----------------------|---------------|------|
| 腾讯云 DNSPod | `tencent` | `--secret-id` `--secret-key` | `secret_id` `secret_key` | 支持 | DNSPod API v3 |
| Cloudflare | `cloudflare` | `--api-token` | `api_token` | 支持 | API Token 需 DNS:Edit 权限 |
| 阿里云 DNS | `alicloud` | `--access-key-id` `--access-key-secret` | `access_key_id` `access_key_secret` | 支持 | 默认 ACS3 **v3**；可选 `--sign-version v1` |
| GoDaddy | `godaddy` | `--api-key` `--api-secret` | `api_key` `api_secret` | 支持 | 凭据来自 GoDaddy Developer Portal |
| 华为云 DNS | `huaweicloud` | `--access-key` `--secret-key` | `access_key` `secret_key` | 支持 | IAM 用户 Access Key |
| DuckDNS | `duckdns` | `--token` | `token` | 受限 | 免费 DDNS，API 仅更新 |
| No-IP | `noip` | `--username` `--password` | `username` `password` | 受限 | 经典 DDNS，API 仅更新 |
| Hurricane Electric | `he` | `--password` | `password` | 受限 | HE DNS DDNS Key，API 仅更新 |
| Dynv6 | `dynv6` | `--token` | `token` | 支持 | 免费 IPv6 DDNS |
| Porkbun | `porkbun` | `--api-key` `--api-secret` | `api_key` `api_secret` | 支持 | API Key + Secret Key |
| DigitalOcean | `digitalocean` | `--token` | `token` | 支持 | API Token 需 write 权限 |
| 百度云 BCD | `baiducloud` | `--access-key` `--secret-key` | `access_key` `secret_key` | 支持 | |
| DNSPod 旧版 | `dnspod` | `--login-token` | `login_token` | 支持 | 格式 `ID,Token` |
| deSEC.io | `desec` | `--token` | `token` | 支持 | |
| Linode (Akamai) | `linode` | `--api-key` | `api_key` | 支持 | Personal Access Token，DNS API v4 |
| NameSilo | `namesilo` | `--api-key` | `api_key` | 支持 | |
| IONOS | `ionos` | `--prefix` `--secret` | `prefix` `secret` | 支持 | API Key 前缀与 Secret |
| Hetzner Cloud | `hetzner` | `--token` | `token` | 支持 | API Token 需 DNS 权限 |
| AWS Route 53 | `aws` | `--access-key-id` `--secret-access-key` | `access_key_id` `secret_access_key` | 支持 | 可选 `--session-token`（IAM Role / STS） |
| Google Cloud DNS | `gcloud` | `--project` `--access-token` | `project` `access_token` | 支持 | REST 调用，不依赖 gcloud CLI |
| Azure DNS | `azure` | `--subscription-id` `--tenant-id` `--client-id` `--client-secret` | `subscription_id` `tenant_id` `client_id` `client_secret` | 支持 | Service Principal 四字段 |
| Namecheap | `namecheap` | `--api-key` `--username` `--client-ip` | `api_key` `username` `client_ip` | 支持 | `client_ip` 须在 Namecheap API 白名单 |
| DNSPod 国际版 | `dpi` | `--login-token` | `login_token` | 支持 | 格式 `ID,Key` |

## 受限运营商

`duckdns`、`he`、`noip` 的 API 仅提供更新接口。对这三家执行 `records` 或 `clean` 会返回错误，请在官网面板管理记录。

## 配置示例

### 通例

默认配置文件路径：`~/.ddns6/config.yaml`。`provider` 填上表 CLI 名称；`auth` 填对应 snake_case 字段。

```yaml
provider: "tencent"          # 运营商 CLI 名称
auth:                        # 凭据（字段因运营商而异）
  secret_id: "xxx"
  secret_key: "xxx"
domain: "example.com"        # 根域名
subdomains:                  # 子域名列表
  - "www"
  - "@"                      # 根域名本身
# ttl: 600                   # 可选，默认 600
# interval: 5m               # 可选；非 Linux 轮询，或 Linux Netlink 回退时使用
# interface: ppp0            # 可选，Linux Netlink 监听网卡
```

建议目录与文件权限：

```bash
chmod 700 ~/.ddns6
chmod 600 ~/.ddns6/config.yaml
```

Unix 上过宽权限会 **拒绝加载** 配置（见 [`docs/deployment.md`](deployment.md)）。也可用 `ddns6 init <provider>` 生成模板（写入即为 `0600`）。

### 腾讯云 DNSPod（tencent）

```yaml
provider: "tencent"
auth:
  secret_id: "YOUR_SECRET_ID"
  secret_key: "YOUR_SECRET_KEY"
domain: "example.com"
subdomains:
  - "www"
  - "@"
```

命令行等效：

```bash
ddns6 run tencent --domain example.com --subdomain www --subdomain @ \
  --secret-id YOUR_SECRET_ID --secret-key YOUR_SECRET_KEY
```

SecretID / SecretKey 可在 [腾讯云 CAM 控制台](https://console.cloud.tencent.com/cam) 创建。

### Cloudflare（cloudflare）

```yaml
provider: "cloudflare"
auth:
  api_token: "YOUR_API_TOKEN"
domain: "example.com"
subdomains:
  - "www"
```

命令行等效：

```bash
ddns6 run cloudflare --domain example.com --subdomain www \
  --api-token YOUR_API_TOKEN
```

API Token 须包含目标 Zone 的 **DNS:Edit** 权限（见下文「注意事项」）。

### 阿里云 DNS（alicloud）

默认签名为 **V3**（ACS3-HMAC-SHA256）。若需兼容旧版 V1（HMAC-SHA1），显式指定 `sign_version: v1`。

```yaml
provider: "alicloud"
auth:
  access_key_id: "YOUR_ACCESS_KEY_ID"
  access_key_secret: "YOUR_ACCESS_KEY_SECRET"
  # sign_version: "v1"   # 可选；省略则默认 v3
domain: "example.com"
subdomains:
  - "www"
```

命令行等效：

```bash
ddns6 run alicloud --domain example.com --subdomain www \
  --access-key-id YOUR_ACCESS_KEY_ID \
  --access-key-secret YOUR_ACCESS_KEY_SECRET
# 可选：--sign-version v1
```

Access Key 建议使用 RAM 子用户，并仅授予 DNS 相关最小权限。

## 注意事项

**Cloudflare API Token 权限**  
创建 Token 时须勾选目标 Zone 的 **DNS:Edit**。权限不足时更新会失败。

**Namecheap API 白名单 IP**  
Namecheap API 要求调用来源 IP 预先加入白名单。`--client-ip` / `auth.client_ip` 须填写 Namecheap 账户中已登记的外网 IP；若出口 IP 变化，需先在面板更新白名单。

**AWS 临时凭据**  
使用 IAM Role 或 STS 时，除 `--access-key-id` 与 `--secret-access-key` 外，可传入可选的 `--session-token`（配置字段 `session_token`）。

**Google Cloud DNS access token**  
`gcloud` 运营商通过 REST API 调用，需 `--access-token`（配置字段 `access_token`）。可用 `gcloud auth print-access-token` 获取 OAuth2 Token；Token 有过期时间，长期运行需自行刷新或配合外部凭据轮换。

**Azure Service Principal**  
Azure DNS 须同时提供四个字段：`subscription_id`、`tenant_id`、`client_id`、`client_secret`。在 Azure AD 中注册应用并授予 DNS Zone Contributor（或等效）角色后使用。

**配置文件安全**  
凭据仅存于配置文件或环境变量，勿提交至版本库。Unix 加载配置时对过宽权限 fail-closed。Docker 优先挂载配置目录（Compose 默认 `ddns6-config`），避免密钥出现在容器 argv（详见 `docs/deployment.md`）。

**GetRecords 语义**  
编排层对同一根域名只查询一次；各运营商实现应返回该 zone 下指定类型的记录列表，由客户端按子域名匹配。
