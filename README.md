# DDNS6 简易命令行工具

## 简介

这是一个简单的DDNS6（动态域名解析系统，支持IPv6）命令行工具。它可以帮助用户轻松地更新其域名的IPv6地址。

## 功能

- 更新域名的IPv6地址
- 支持定时更新
- 提供命令行选项以及环境变量以配置更新参数

## 安装

确保你的系统已经安装了Go语言环境（版本1.20或更高）。然后，你可以使用以下命令克隆并构建项目：

```bash
git clone https://github.com/your-repo/ddns6.git
cd ddns6/cmd
go build -o ddns6
```

## 使用

使用`-h`或`--help`选项查看帮助信息：

```bash
./ddns6 -h
```

### 示例

以下命令将使用指定的域名和服务提供商更新IPv6地址：

```bash
./ddns6 -domain your-domain.com -service tencent
```

## 配置

工具支持以下命令行选项和环境变量：

### 命令行选项

 - `-debug`: 开启调试模式
 - `-domain`: 设置域名
 - `-iface`: 设备的物理网卡名称 (default "eth0")
 - `-init`: 生成 systemd 服务
 - `-interval`: 定时任务时间间隔（例如 1s、2m、3h、5m2s、1h15m） (default 5m0s)
 - `-ipv6`: 选择一个IPv6 获取方式(可选值: [dns site]) (default dns)
 - `-public-dns`: 添加自定义公共IPv6 DNS, 多个DNS用逗号分隔 (default 2400:3200:baba::1, 2001:4860:4860::8888)
 - `-service`: 选择一个 ddns 服务商(可选值: [tencent cloudflare]) (default tencent)
 - `-site`: 添加一个可以查询IPv6地址的自定义网站, 多个网站用逗号分隔 (default https://6.ipw.cn)
 - `-subdomain`: 设置子域名 (default "@")
 - `-version`: 显示版本信息

### 环境变量

- `DDNS6_DOMAIN`：与`-domain`选项相同，用于设置需要更新IPv6地址的域名
- `DDNS6_SERVICE`：与`-service`选项相同，用于设置使用的DDNS服务提供商
- `DDNS6_INTERVAL`：与`-interval`选项相同，用于设置更新间隔（单位为秒）

**注意**：如果命令行选项和环境变量同时存在，命令行选项将优先生效。

## 注意事项

- 确保你有权限更新指定的域名DNS记录
- 根据你的DDNS服务提供商，可能需要提供额外的认证信息
- 在使用前，请确保已正确配置所有必要的选项和环境变量

## 贡献

如果你有任何问题或建议，请提交一个issue或pull request。

## 许可证

MIT许可证。详情请参见LICENSE文件。