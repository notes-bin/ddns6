# 定义变量
BINARY_NAME=ddns6
BIN_DIR=bin
DIST_DIR=dist
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S%z)
BUILD_FLAGS=-ldflags "-X github.com/notes-bin/ddns6/cmd.Version=$(VERSION) -X github.com/notes-bin/ddns6/cmd.Commit=$(shell git rev-parse HEAD) -X github.com/notes-bin/ddns6/cmd.buildAt=$(BUILD_TIME)"
GO=go
# COPYFILE_DISABLE=1 阻止 macOS 将 AppleDouble (._*) 与扩展属性写入归档
TAR=COPYFILE_DISABLE=1 tar

# 默认目标
all: build

# 构建二进制文件到 bin/ 目录
build: $(BIN_DIR)
	$(GO) build $(BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME) .

# 安装到系统路径
install:
	$(GO) install $(BUILD_FLAGS) .

# 运行程序（编译后运行）
run: build
	./$(BIN_DIR)/$(BINARY_NAME)

# 运行测试
test:
	$(GO) test -v ./...

# 创建 bin / dist 目录
$(BIN_DIR) $(DIST_DIR):
	mkdir -p $@

# 清理构建文件
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
	rm -f *.tar.gz *.zip
	$(GO) clean --cache --testcache

# 交叉编译
cross-build: $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME)_linux_amd64 .
	GOOS=darwin GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME)_darwin_amd64 .
	GOOS=darwin GOARCH=arm64 $(GO) build $(BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME)_darwin_arm64 .

# 打包单个平台：二进制 + LICENSE + README → dist/
# 用法: $(call pack-release,linux_amd64)
define pack-release
	$(TAR) -czf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_$(1).tar.gz \
		-C $(BIN_DIR) $(BINARY_NAME)_$(1) \
		-C $(CURDIR) LICENSE README.md
endef

# 生成发布包（输出到 dist/，禁用 macOS 归档元数据）
release: clean
	$(MAKE) cross-build
	mkdir -p $(DIST_DIR)
	$(call pack-release,linux_amd64)
	$(call pack-release,darwin_amd64)
	$(call pack-release,darwin_arm64)

# Docker 构建
docker-build:
	docker build -t ddns6 .

# Docker 直接运行（推荐挂载配置文件，避免密钥进 argv）
docker-run:
	docker run -d --name ddns6 --restart unless-stopped \
	  --network host \
	  --read-only \
	  --tmpfs /tmp:size=16m,mode=1777 \
	  --security-opt no-new-privileges:true \
	  --cap-drop ALL \
	  --cap-add NET_ADMIN \
	  -v $${HOME}/.ddns6:/home/ddns6/.ddns6:ro \
	  ddns6 run

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
	@echo "  build       构建到 bin/ddns6"
	@echo "  install     安装到系统路径"
	@echo "  run         构建并运行（bin/ddns6）"
	@echo "  test        运行测试"
	@echo "  clean       清理 bin/、dist/ 与缓存"
	@echo "  cross-build 交叉编译"
	@echo "  release     生成发布包到 dist/（禁用 macOS 元数据）"
	@echo "  fmt         格式化代码"
	@echo "  docker-build  构建 Docker 镜像"
	@echo "  docker-run    直接运行容器（docker run --network host --cap-add=NET_ADMIN）"
	@echo "  docker-up     启动容器（docker compose up -d）"
	@echo "  docker-logs   查看容器日志"
	@echo "  docker-down   停止并删除容器"
	@echo "  help          显示帮助信息"

.PHONY: all build install run test clean cross-build release docker-build docker-run docker-up docker-logs docker-down fmt help