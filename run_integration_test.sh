#!/bin/bash

# 集成测试运行脚本

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     健康监测 + 故障诊断 集成测试                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# 检查是否在正确的目录
if [ ! -d "fault-diagnosis" ] || [ ! -d "health-monitor" ]; then
    echo "❌ 错误: 请在项目根目录下运行此脚本"
    exit 1
fi

# 检查配置文件
echo "📋 检查配置文件..."
if [ ! -f "fault-diagnosis/configs/fault_tree_business.json" ]; then
    echo "❌ 缺少业务层故障树配置文件"
    exit 1
fi

if [ ! -f "fault-diagnosis/configs/fault_tree_microservice.json" ]; then
    echo "❌ 缺少微服务层故障树配置文件"
    exit 1
fi
echo "✓ 配置文件检查通过"
echo ""

# 创建 go.mod（如果不存在）
if [ ! -f "go.mod" ]; then
    echo "📦 初始化 Go 模块..."
    go mod init integration-test
    echo "✓ Go 模块已初始化"
    echo ""
fi

# 设置 Go workspace（如果需要）
if [ ! -f "go.work" ]; then
    echo "📦 设置 Go workspace..."
    go work init
    go work use ./fault-diagnosis
    go work use ./health-monitor
    go work use .
    echo "✓ Go workspace 已设置"
    echo ""
fi

# 构建
echo "🔨 构建集成测试程序..."
go build -o build/integration-test cmd/integration_test/main.go
if [ $? -ne 0 ]; then
    echo "❌ 构建失败"
    exit 1
fi
echo "✓ 构建成功"
echo ""

# 运行
echo "🚀 启动集成测试..."
echo ""
./build/integration-test

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     测试结束                                                  ║"
echo "╚══════════════════════════════════════════════════════════════╝"
