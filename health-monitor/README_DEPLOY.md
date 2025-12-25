# 健康监控系统部署指南

## 快速开始

### 1. 编译

```bash
# 查看所有编译选项
make help

# 编译 Linux x86_64 版本（最常用）
make linux-amd64

# 编译 Linux ARM64 版本（国产化平台）
make linux-arm64

# 编译所有平台
make all-platforms
```

### 2. 运行

```bash
# 基本运行
./build/health-monitor-linux-amd64 \
  --ecsm-url=http://your-ecsm-platform:8080 \
  --db-path=/var/lib/health-monitor/data.db \
  --interval=30

# 开发测试（快速采集）
make run-dev
```

### 3. 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--ecsm-url` | 容器平台 API 地址 | `http://localhost:8080` |
| `--db-path` | 数据库存储路径 | `/var/lib/health-monitor/data.db` |
| `--interval` | 监控采集间隔(秒) | `30` |
| `--business-port` | 业务层数据接收端口 | `9999` |

## 部署方案

### 方案 1: 直接运行

```bash
# 1. 上传二进制文件到目标机器
scp build/health-monitor-linux-amd64 user@target:/usr/local/bin/health-monitor

# 2. 创建数据目录
ssh user@target "mkdir -p /var/lib/health-monitor"

# 3. 运行
ssh user@target "nohup /usr/local/bin/health-monitor \
  --ecsm-url=http://ecsm-platform:8080 \
  --interval=30 \
  > /var/log/health-monitor.log 2>&1 &"
```

### 方案 2: systemd 服务

创建服务文件 `/etc/systemd/system/health-monitor.service`:

```ini
[Unit]
Description=Health Monitor Service
After=network.target

[Service]
Type=simple
User=monitor
Group=monitor
ExecStart=/usr/local/bin/health-monitor \
  --ecsm-url=http://ecsm-platform:8080 \
  --db-path=/var/lib/health-monitor/data.db \
  --interval=30
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启动服务:

```bash
# 重载 systemd
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start health-monitor

# 设置开机自启
sudo systemctl enable health-monitor

# 查看状态
sudo systemctl status health-monitor

# 查看日志
sudo journalctl -u health-monitor -f
```

### 方案 3: Docker 容器

创建 `Dockerfile`:

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY . .
RUN go mod download
RUN CGO_ENABLED=1 go build -o health-monitor ./cmd/monitor

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/health-monitor .
VOLUME ["/data"]
EXPOSE 9999
ENTRYPOINT ["./health-monitor"]
CMD ["--ecsm-url=http://ecsm-platform:8080", "--db-path=/data/data.db"]
```

构建并运行:

```bash
# 构建镜像
docker build -t health-monitor:1.0 .

# 运行容器
docker run -d \
  --name health-monitor \
  --restart=always \
  -v /var/lib/health-monitor:/data \
  -p 9999:9999 \
  health-monitor:1.0 \
  --ecsm-url=http://ecsm-platform:8080 \
  --interval=30
```

## 监控功能

### 微服务层监控

系统会定期采集以下指标：

- **节点监控**: CPU、内存、磁盘、网络
- **容器监控**: CPU、内存、重启次数
- **服务监控**: 健康状态、实例数量

### 告警机制

1. **阈值告警** (Critical 级别):
   - 节点 CPU > 90%
   - 节点内存 > 90%
   - 容器 CPU > 80%
   - 容器内存 > 80%
   - 容器重启次数 > 10

2. **趋势告警** (Warning 级别):
   - CPU 持续上升 (10% 变化)
   - 内存持续增长 (10% 变化)
   - 容器频繁重启 (3次趋势检测)
   - 服务校验失败率上升

### 数据存储

- **环形缓冲区**: 最近 600 条记录，实时查询
- **BoltDB 持久化**: 每分钟快照，历史数据分析
- **趋势分析窗口**: 5 分钟历史数据，10 个数据点

## 性能参数

- **采集间隔**: 建议 30 秒（生产环境）或 10 秒（开发环境）
- **内存占用**: 约 50-100 MB
- **磁盘占用**: 约 10-50 MB/天（取决于指标数量）
- **CPU 占用**: < 5%

## 故障排查

### 查看日志

```bash
# systemd 服务
sudo journalctl -u health-monitor -f

# 直接运行
tail -f /var/log/health-monitor.log
```

### 常见问题

1. **无法连接容器平台**:
   - 检查 `--ecsm-url` 参数是否正确
   - 检查网络连通性: `curl http://ecsm-platform:8080/api/v1/node`

2. **数据库权限问题**:
   - 确保数据目录存在: `mkdir -p /var/lib/health-monitor`
   - 检查文件权限: `chown -R monitor:monitor /var/lib/health-monitor`

3. **内存占用过高**:
   - 减少采集间隔: `--interval=60`
   - 清理历史数据: `rm /var/lib/health-monitor/data.db`

## 版本信息

- **版本**: 1.0.0
- **Go 版本**: 1.24.5
- **支持平台**: Linux (amd64/arm64/arm), Windows, macOS
