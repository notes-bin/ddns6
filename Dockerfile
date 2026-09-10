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
      -X github.com/notes-bin/ddns6/cmd.Version=${VERSION} \
      -X github.com/notes-bin/ddns6/cmd.Commit=${COMMIT} \
      -X github.com/notes-bin/ddns6/cmd.buildAt=${BUILD_TIME}" \
    -o /out/ddns6 .

# ---------- 运行阶段 ----------
# 固定 alpine 小版本，避免 :latest 漂移
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -g 10001 -S ddns6 \
    && adduser -u 10001 -S -G ddns6 -H -D ddns6

WORKDIR /app

COPY --from=builder --chown=ddns6:ddns6 /out/ddns6 /app/ddns6

USER ddns6:ddns6

# 只读根文件系统下可能需要临时目录（compose 可挂 tmpfs）
ENV HOME=/home/ddns6 \
    TZ=UTC

ENTRYPOINT ["/app/ddns6"]
CMD ["--help"]
