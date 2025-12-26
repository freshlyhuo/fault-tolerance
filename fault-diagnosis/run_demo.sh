#!/bin/bash

# 故障诊断模块演示运行脚本

echo "════════════════════════════════════════════════════════════"
echo "    故障诊断模块 - 快速演示脚本"
echo "════════════════════════════════════════════════════════════"
echo ""

# 检查是否在正确的目录
if [ ! -f "go.mod" ]; then
    echo "❌ 错误: 请在 fault-diagnosis 目录下运行此脚本"
    exit 1
fi

# 检查配置文件
echo "📋 检查配置文件..."
if [ ! -f "configs/fault_tree_business.json" ]; then
    echo "❌ 缺少业务层故障树配置文件"
    exit 1
fi

if [ ! -f "configs/fault_tree_microservice.json" ]; then
    echo "❌ 缺少微服务层故障树配置文件"
    exit 1
fi
echo "✓ 配置文件检查通过"
echo ""

# 构建
echo "🔨 构建演示程序..."
go build -o build/demo cmd/demo/main.go
if [ $? -ne 0 ]; then
    echo "❌ 构建失败"
    exit 1
fi
echo "✓ 构建成功"
echo ""

# 运行
echo "🚀 启动故障诊断演示..."
echo ""
./build/demo

echo ""
echo "════════════════════════════════════════════════════════════"
echo "    演示结束"
echo "════════════════════════════════════════════════════════════"
