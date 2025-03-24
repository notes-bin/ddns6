# 定义变量
BINARY_NAME=ddns6
VERSION=$(shell git describe --tags --always --dirty)
BUILD_FLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(shell git rev-parse HEAD)"
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
	$(GO) clean

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
	@echo "  help        显示帮助信息"

.PHONY: all build install run test clean cross-build release fmt help