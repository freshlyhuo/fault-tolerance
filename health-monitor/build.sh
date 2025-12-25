#!/bin/bash

# 健康监控系统交叉编译脚本
# 支持 SylixOS ARM64 平台

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}健康监控系统 - 交叉编译脚本${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# 配置
BINARY_NAME="health-monitor"
BUILD_DIR="build"
MAIN_PKG="./cmd/monitor"

# 创建构建目录
mkdir -p ${BUILD_DIR}

# 函数：编译指定平台
build_platform() {
    local GOOS=$1
    local GOARCH=$2
    local OUTPUT_NAME=$3
    
    echo -e "${YELLOW}编译 ${GOOS}/${GOARCH}...${NC}"
    
    # 设置 CGO（根据平台决定）
    local CGO_ENABLED=0
    if [ "$GOOS" = "sylixos" ]; then
        CGO_ENABLED=1
    fi
    
    # 编译
    CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} go build \
        -ldflags="-s -w" \
        -o ${BUILD_DIR}/${OUTPUT_NAME} \
        ${MAIN_PKG}
    
    if [ $? -eq 0 ]; then
        local SIZE=$(du -h ${BUILD_DIR}/${OUTPUT_NAME} | cut -f1)
        echo -e "${GREEN}✅ 编译成功: ${BUILD_DIR}/${OUTPUT_NAME} (${SIZE})${NC}"
    else
        echo -e "${RED}❌ 编译失败${NC}"
        exit 1
    fi
    echo ""
}

# 显示菜单
echo "请选择编译目标:"
echo "  1) SylixOS ARM64 (目标平台)"
echo "  2) Linux AMD64 (开发/测试)"
echo "  3) Linux ARM64 (国产化平台)"
echo "  4) 所有平台"
echo "  5) 仅本地编译"
echo ""
read -p "请输入选项 [1-5]: " choice

case $choice in
    1)
        echo -e "${YELLOW}编译 SylixOS ARM64...${NC}"
        build_platform "sylixos" "arm64" "${BINARY_NAME}-sylixos-arm64"
        ;;
    2)
        echo -e "${YELLOW}编译 Linux AMD64...${NC}"
        build_platform "linux" "amd64" "${BINARY_NAME}-linux-amd64"
        ;;
    3)
        echo -e "${YELLOW}编译 Linux ARM64...${NC}"
        build_platform "linux" "arm64" "${BINARY_NAME}-linux-arm64"
        ;;
    4)
        echo -e "${YELLOW}编译所有平台...${NC}"
        build_platform "sylixos" "arm64" "${BINARY_NAME}-sylixos-arm64"
        build_platform "linux" "amd64" "${BINARY_NAME}-linux-amd64"
        build_platform "linux" "arm64" "${BINARY_NAME}-linux-arm64"
        build_platform "linux" "arm" "${BINARY_NAME}-linux-arm"
        ;;
    5)
        echo -e "${YELLOW}本地编译...${NC}"
        go build -o ${BUILD_DIR}/${BINARY_NAME} ${MAIN_PKG}
        echo -e "${GREEN}✅ 编译成功: ${BUILD_DIR}/${BINARY_NAME}${NC}"
        ;;
    *)
        echo -e "${RED}无效选项${NC}"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}编译完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "构建目录: ${BUILD_DIR}/"
ls -lh ${BUILD_DIR}/
echo ""
echo -e "${YELLOW}使用方法:${NC}"
echo "  # 上传到目标机器"
echo "  scp ${BUILD_DIR}/${BINARY_NAME}-sylixos-arm64 user@target:/usr/local/bin/${BINARY_NAME}"
echo ""
echo "  # 在目标机器上运行"
echo "  ${BINARY_NAME} --ecsm-url=http://your-platform:8080 --interval=30"
echo ""
