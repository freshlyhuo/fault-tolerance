.PHONY: build test clean run demo docker docker-up docker-down help

# 变量定义
BINARY_NAME=fault-diagnosis
DEMO_BINARY_NAME=fault-diagnosis-demo
BUILD_DIR=./build
CMD_DIR=./cmd/diagnosis
DEMO_DIR=./cmd/demo

# 默认目标
all: build

# 构建主程序
build:
	@echo "构建故障诊断模块..."
	@mkdir -p $(BUILD_DIR)
	@cd $(CMD_DIR) && go build -o ../../$(BUILD_DIR)/$(BINARY_NAME) .
	@cd $(DEMO_DIR) && go build -o ../../$(BUILD_DIR)/$(DEMO_BINARY_NAME) .
	@echo "✅ 构建完成: $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "✅ 构建完成: $(BUILD_DIR)/$(DEMO_BINARY_NAME)"

# 运行测试
test:
	@echo "运行测试..."
	@cd test && go test -v -count=1 ./...

# 运行主程序
run: build
	@echo "启动故障诊断模块..."
	@$(BUILD_DIR)/$(BINARY_NAME) \
		-config ./configs/fault_tree_business.json \
		-etcd localhost:2379 \
		-prefix /alerts/business/ \
		-log-level info

# 运行演示程序
demo: build
	@echo "运行演示程序..."
	@$(BUILD_DIR)/$(DEMO_BINARY_NAME)

# 清理构建产物
clean:
	@echo "清理构建产物..."
	@rm -rf $(BUILD_DIR)
	@echo "✅ 清理完成"

# 代码格式化
fmt:
	@echo "格式化代码..."
	@go fmt ./...
	@echo "✅ 格式化完成"

# 代码检查
lint:
	@echo "检查代码..."
	@go vet ./...
	@echo "✅ 检查完成"

# 下载依赖
deps:
	@echo "下载依赖..."
	@go mod download
	@echo "✅ 依赖下载完成"

# 更新依赖
deps-update:
	@echo "更新依赖..."
	@go get -u ./...
	@go mod tidy
	@echo "✅ 依赖更新完成"

# Docker构建
docker:
	@echo "构建Docker镜像..."
	@docker build -t fault-diagnosis:latest .
	@echo "✅ Docker镜像构建完成"

# 启动Docker Compose
docker-up:
	@echo "启动Docker Compose..."
	@docker-compose up -d
	@echo "✅ 服务已启动"
	@echo "查看日志: docker-compose logs -f"

# 停止Docker Compose
docker-down:
	@echo "停止Docker Compose..."
	@docker-compose down
	@echo "✅ 服务已停止"

# 查看Docker日志
docker-logs:
	@docker-compose logs -f

# 安装到系统
install: build
	@echo "安装到系统..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "✅ 安装完成: /usr/local/bin/$(BINARY_NAME)"

# 卸载
uninstall:
	@echo "从系统卸载..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✅ 卸载完成"

# 帮助信息
help:
	@echo "故障诊断模块 - Makefile命令"
	@echo ""
	@echo "使用方法: make [target]"
	@echo ""
	@echo "可用目标:"
	@echo "  build         - 编译项目"
	@echo "  test          - 运行测试"
	@echo "  run           - 运行主程序"
	@echo "  demo          - 运行演示程序"
	@echo "  clean         - 清理构建产物"
	@echo "  fmt           - 格式化代码"
	@echo "  lint          - 代码检查"
	@echo "  deps          - 下载依赖"
	@echo "  deps-update   - 更新依赖"
	@echo "  docker        - 构建Docker镜像"
	@echo "  docker-up     - 启动Docker Compose"
	@echo "  docker-down   - 停止Docker Compose"
	@echo "  docker-logs   - 查看Docker日志"
	@echo "  install       - 安装到系统"
	@echo "  uninstall     - 从系统卸载"
	@echo "  help          - 显示此帮助信息"
	@echo ""
	@echo "示例:"
	@echo "  make build    # 编译项目"
	@echo "  make demo     # 运行演示"
	@echo "  make test     # 运行测试"
