# ============================================================
# 多阶段构建：编译静态二进制，运行镜像仅含 CA / 时区与非 root 用户
# ============================================================

# ---------- 构建阶段 ----------
FROM golang:1.27.1-alpine AS builder

WORKDIR /src

# 先复制依赖清单，利用层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 支持 docker buildx 多架构；本地 docker build 默认为 amd64
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags "-s -w \
      -X github.com/notes-bin/ddns6/internal/cli.Version=${VERSION} \
      -X github.com/notes-bin/ddns6/internal/cli.Commit=${COMMIT} \
      -X github.com/notes-bin/ddns6/internal/cli.buildAt=${BUILD_TIME}" \
    -o /out/ddns6 ./cmd/ddns6

# ---------- 运行阶段 ----------
# 固定 alpine 小版本，避免 :latest 漂移
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -g 10001 -S ddns6 \
    && adduser -u 10001 -S -G ddns6 -H -D ddns6 \
    && mkdir -p /home/ddns6 \
    && chown -R 10001:10001 /home/ddns6

WORKDIR /app

COPY --from=builder --chown=10001:10001 /out/ddns6 /app/ddns6

# 使用数字 UID/GID，避免不同发行版用户名解析差异
USER 10001:10001

# 只读根文件系统下可能需要临时目录（compose 可挂 tmpfs）
# 默认禁用落盘日志：只读根 FS 无法在 WORKDIR 创建 ddns6.log
ENV HOME=/home/ddns6 \
    TZ=UTC \
    DDNS6_LOG_FILE=

ENTRYPOINT ["/app/ddns6"]
CMD ["--help"]
