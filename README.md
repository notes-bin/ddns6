# ddns6 - 动态DNS更新工具

一个用Go编写的动态DNS更新工具，支持IPv6地址自动更新到多个DNS服务商。

## 功能特性

- 支持多种DNS服务提供商：
  - 阿里云DNS
  - 腾讯云DNS
  - 华为云DNS
  - Cloudflare DNS
  - GoDaddy DNS
- 自动检测本地IPv6地址变化
- 定时检查并更新DNS记录
- 简单的命令行界面
- 支持后台服务模式
- 完善的日志记录

## 工作原理

ddns6通过以下步骤工作：
1. 定期检测本机IPv6地址变化
2. 当检测到IP变化时，调用对应DNS服务商的API
3. 更新指定域名的AAAA记录
4. 记录操作日志并等待下次检查

## 安装

### 从源码安装

```bash
git clone https://github.com/notes-bin/ddns6.git
cd ddns6
go build -o ddns6
```
## 使用说明
### 基本命令
```bash
./ddns6 [全局选项] <命令> [命令选项]
```
## 配置参数
### 全局选项
- --debug : 启用调试日志
- --version : 显示版本信息
- --domain : 根域名 (如 example.com)
- --subdomain : 子域名 (如 www)
- --interval : 检查间隔 (如 5m)
### 提供商特定参数：
- 腾讯云: --secret-id , --secret-key
- 阿里云: --access-key-id , --access-key-secret
- 华为云: --username , --password , --domain-name
- Cloudflare: --api-token
- GoDaddy: --api-key , --api-secret
### 主要命令
- run : 运行DDNS服务

## 详细配置示例
### 腾讯云DNS配置
1. 获取API密钥：
   - 登录腾讯云控制台
   - 进入"访问管理" > "API密钥管理"
   - 创建新密钥
2. 运行命令：
```bash
./ddns6 run tencent \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY \
  --domain example.com \
  --subdomain www \
  --interval 5m
```

### 使用Docker
```bash
docker pull notes-bin/ddns6
docker run -d --name ddns6 \
  -e DOMAIN=example.com \
  -e SUBDOMAIN=www \
  notes-bin/ddns6 run [provider] [flags]
```

## 常见问题
### 如何验证配置是否正确？
使用 --debug 标志运行可以查看详细日志：
```bash
./ddns6 --debug run [provider] [flags]
```
### 支持的提供商
- tencent - 腾讯云DNS
- alicloud - 阿里云DNS
- huaweicloud - 华为云DNS
- cloudflare - Cloudflare DNS
- godaddy - GoDaddy DNS

### 示例
更新腾讯云DNS记录：
```bash
./ddns6 run tencent \
  --secret-id YOUR_SECRET_ID \
  --secret-key YOUR_SECRET_KEY \
  --domain example.com \
  --subdomain www \
  --interval 5m
```


## 注意事项
- 确保你的网络环境可以访问DNS服务提供商的API。
- 确保你的DNS服务提供商的API凭证正确。
- 确保你的DNS服务提供商支持IPv6记录。
- 确保你的DNS服务提供商支持更新记录的API。
- 确保你的DNS服务提供商支持更新记录的API。
- 确保你的终端设备支持IPv6。

## 开发贡献
欢迎提交Issue或Pull Request

## 许可证

MIT License - 详见 LICENSE 文件