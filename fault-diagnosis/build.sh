#!/bin/bash

set -e

echo "======================================"
echo "构建故障诊断模块"
echo "======================================"

# 设置变量
MODULE_NAME="fault-diagnosis"
BUILD_DIR="./build"
CMD_DIR="./cmd/diagnosis"

# 创建构建目录
mkdir -p ${BUILD_DIR}

# 构建主程序
echo "正在编译主程序..."
cd ${CMD_DIR}
go build -o ../../${BUILD_DIR}/${MODULE_NAME} .
cd -

echo "✅ 构建成功: ${BUILD_DIR}/${MODULE_NAME}"

# 构建演示程序
echo "正在编译演示程序..."
cd ./cmd/demo
go build -o ../../${BUILD_DIR}/${MODULE_NAME}-demo .
cd -

echo "✅ 构建成功: ${BUILD_DIR}/${MODULE_NAME}-demo"

echo ""
echo "======================================"
echo "构建完成!"
echo "======================================"
echo "可执行文件:"
echo "  - ${BUILD_DIR}/${MODULE_NAME}         (主程序)"
echo "  - ${BUILD_DIR}/${MODULE_NAME}-demo    (演示程序)"
echo ""
echo "运行示例:"
echo "  ${BUILD_DIR}/${MODULE_NAME} -config ./configs/fault_tree_business.json"
echo "  ${BUILD_DIR}/${MODULE_NAME}-demo"
echo "======================================"
