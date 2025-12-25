#!/bin/bash

# 快速测试脚本 - 在开发机上编译并测试

echo "=========================================="
echo "健康监控系统 - 快速测试"
echo "=========================================="
echo ""

# 创建构建目录
mkdir -p build

# 编译本地版本
echo "编译本地测试版本..."
go build -o build/health-monitor ./cmd/monitor

if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi

echo "✅ 编译成功: build/health-monitor"
echo ""

# 显示使用方法
echo "使用方法:"
echo ""
echo "1. 本地测试（需要容器平台运行）:"
echo "   ./build/health-monitor --ecsm-url=http://localhost:8080 --interval=10"
echo ""
echo "2. 连接远程容器平台:"
echo "   ./build/health-monitor --ecsm-url=http://192.168.1.50:8080 --interval=30"
echo ""
echo "3. 上传到 SylixOS 后测试:"
echo "   # 打包源码"
echo "   tar -czf health-monitor-src.tar.gz --exclude='.git' --exclude='build' ."
echo "   "
echo "   # 上传到 SylixOS"
echo "   scp health-monitor-src.tar.gz root@<sylixos-ip>:/tmp/"
echo "   "
echo "   # 在 SylixOS 上编译"
echo "   ssh root@<sylixos-ip>"
echo "   cd /tmp && tar -xzf health-monitor-src.tar.gz"
echo "   go build -o health-monitor ./cmd/monitor"
echo "   ./health-monitor --ecsm-url=http://your-platform:8080"
echo ""
