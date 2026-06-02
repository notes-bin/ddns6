# 定义变量
BINARY_NAME=ddns6
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S%z)
BUILD_FLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(shell git rev-parse HEAD) -X main.buildAt=$(BUILD_TIME)"
GO=go

# 默认目标
all: build

# 构建二进制文件
build:
	$(GO) build $(BUILD_FLAGS) -o $(BINARY_NAME) ./cmd

# 安装到系统路径
install:
	$(GO) install $(BUILD_FLAGS) ./cmd

# 运行程序
run:
	$(GO) run $(BUILD_FLAGS) ./cmd

# 运行测试
test:
	$(GO) test -v ./...

# 清理构建文件
clean:
	rm -f $(BINARY_NAME)
	$(GO) clean --cache --testcache

# 交叉编译
cross-build:
	GOOS=linux GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(BINARY_NAME)_linux_amd64 ./cmd
	GOOS=darwin GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(BINARY_NAME)_darwin_amd64 ./cmd
	GOOS=darwin GOARCH=arm64 $(GO) build $(BUILD_FLAGS) -o $(BINARY_NAME)_darwin_arm64 ./cmd

# 生成发布包
release: clean cross-build
	tar -czf $(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz $(BINARY_NAME)_linux_amd64 LICENSE README.md
	tar -czf $(BINARY_NAME)_$(VERSION)_darwin_amd64.tar.gz $(BINARY_NAME)_darwin_amd64 LICENSE README.md
	tar -czf $(BINARY_NAME)_$(VERSION)_darwin_arm64.tar.gz $(BINARY_NAME)_darwin_arm64 LICENSE README.md

# Docker 构建
docker-build:
	docker build -t ddns6 .

# Docker 运行（以腾讯云为例，需先在 .env 中配置凭证）
docker-up:
	docker compose up -d

# Docker 查看日志
docker-logs:
	docker compose logs -f

# Docker 停止
docker-down:
	docker compose down

# 格式化代码
fmt:
	$(GO) fmt ./...

# 显示帮助信息
help:
	@echo "Makefile 命令列表:"
	@echo "  all         默认目标，等同于 build"
	@echo "  build       构建二进制文件"
	@echo "  install     安装到系统路径"
	@echo "  run         运行程序"
	@echo "  test        运行测试"
	@echo "  clean       清理构建文件"
	@echo "  cross-build 交叉编译"
	@echo "  release     生成发布包"
	@echo "  fmt         格式化代码"
	@echo "  docker-build  构建 Docker 镜像"
	@echo "  docker-up     启动容器（docker compose up -d）"
	@echo "  docker-logs   查看容器日志"
	@echo "  docker-down   停止并删除容器"
	@echo "  help          显示帮助信息"

.PHONY: all build install run test clean cross-build release docker-build docker-up docker-logs docker-down fmt help