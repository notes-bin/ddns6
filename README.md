# DDNS6 简易命令行工具

## 简介

这是一个简单的DDNS6（动态域名解析系统，支持IPv6）命令行工具。它可以帮助用户轻松地更新其域名的IPv6地址。

## 功能

- 更新域名的IPv6地址
- 支持定时更新
- 提供命令行选项以配置更新参数

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

工具支持以下命令行选项：

- `-domain`：需要更新IPv6地址的域名
- `-service`：使用的DDNS服务提供商（例如：tencent）
- `-interval`：更新间隔（默认为5分钟）

## 注意事项

- 确保你有权限更新指定的域名DNS记录
- 根据你的DDNS服务提供商，可能需要提供额外的认证信息

## 贡献

如果你有任何问题或建议，请提交一个issue或pull request。

## 许可证

MIT许可证。详情请参见LICENSE文件。