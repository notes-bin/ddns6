# 基础镜像，使用官方的 Go 镜像
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制项目文件
COPY . .

# 构建项目
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ddns6 .

# 最终镜像，使用轻量级的 alpine 镜像
FROM alpine:latest

# 安装必要的依赖
RUN apk --no-cache add ca-certificates

# 设置工作目录
WORKDIR /root/

# 从 builder 阶段复制构建好的二进制文件
COPY --from=builder /app/ddns6 .

# 启动应用
CMD ["./ddns6"]