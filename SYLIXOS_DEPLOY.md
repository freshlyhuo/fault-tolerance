# SylixOS 部署指南

## 快速开始

### 1. 编译

**重要变更**: 已将 BoltDB 替换为 etcd，解决了交叉编译兼容性问题。现在支持：
- ✅ 交叉编译到 SylixOS
- ✅ 纯内存模式（不需要 etcd）
- ✅ etcd 持久化模式（可选）

#### 方法 A: 交叉编译到 SylixOS（推荐）

```bash
# 1. 安装依赖
cd /home/yzj/fault-tolerance/health-monitor
go mod tidy

# 2. 交叉编译 SylixOS ARM64
mkdir -p build
GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" -o build/health-monitor-sylixos ./cmd/integration_test
GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" -o build/health-monitor-sylixos ./cmd/integration_test_microservice
GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" -o build/health-monitor-sylixos ./cmd/integration_test_business
# 3. 检查编译结果
ls -lh build/health-monitor-sylixos
```

#### 方法 B: 在 SylixOS 上直接编译

```bash
# 1. 上传源码到 SylixOS
tar -czf health-monitor-src.tar.gz --exclude='.git' --exclude='build' .
scp health-monitor-src.tar.gz root@192.168.1.100:/tmp/

# 2. SSH 登录 SylixOS 并编译
ssh root@192.168.1.100
cd /tmp && tar -xzf health-monitor-src.tar.gz
go mod tidy
go build -o health-monitor ./cmd/monitor
```

#### 方法 C: 在开发机测试

```bash
# 本地编译测试
go build -o build/health-monitor ./cmd/monitor
./build/health-monitor --ecsm-url=http://localhost:8080 --interval=10
```

### 2. 安装到系统目录

```bash
# 在 SylixOS 上，将编译好的程序安装到系统目录
cd /usr/local/src/health-monitor
cp health-monitor /usr/local/bin/
chmod +x /usr/local/bin/health-monitor

# 验证安装
which health-monitor
health-monitor --help
```

### 3. 在 SylixOS 上运行

```bash
# SSH 登录到 SylixOS
ssh root@192.168.1.100

# 添加执行权限
chmod +x /usr/local/bin/health-monitor

# 运行监控程序（纯内存模式）
/usr/local/bin/health-monitor \
  --ecsm-url=http://your-ecsm-platform:8080 \
  --interval=30

# 或使用 etcd 持久化（如果有 etcd 服务）
/usr/local/bin/health-monitor \
  --ecsm-url=http://your-ecsm-platform:8080 \
  --etcd=192.168.1.200:2379 \
  --interval=30
```

## 运行模式

### 前台运行（调试）

```bash
# 直接运行，查看实时输出
./health-monitor --ecsm-url=http://192.168.1.50:8080 --interval=10
```
./health-monitor-sylixos --ecsm-url=http://192.168.31.127:3001 --interval=10
输出示例：
```
========== 健康监控系统启动 ==========
容器平台地址: http://192.168.1.50:8080
数据库路径: ./health-monitor.db
采集间隔: 10秒
======================================

初始化状态管理器...
初始化告警生成器（含趋势分析）...
初始化业务层监控...
初始化微服务层监控...
启动微服务层定期采集...

✅ 系统运行中，按 Ctrl+C 停止

✅ [14:30:15] 采集成功 (耗时: 234ms)
✅ [14:30:45] 采集成功 (耗时: 189ms)
```

### 后台运行（生产）

```bash
# 方法1: 使用 nohup
nohup ./health-monitor \
  --ecsm-url=http://192.168.1.50:8080 \
  --interval=30 \
  > /var/log/health-monitor.log 2>&1 &

# 查看日志
tail -f /var/log/health-monitor.log

# 方法2: 使用 screen (如果 SylixOS 支持)
screen -S monitor
./health-monitor --ecsm-url=http://192.168.1.50:8080 --interval=30
# 按 Ctrl+A, D 分离会话

# 恢复会话
screen -r monitor
```

### 停止程序

```bash
# 查找进程
ps aux | grep health-monitor

# 优雅停止（发送 SIGTERM）
kill <PID>

# 强制停止
kill -9 <PID>
```

## 命令行参数

| 参数 | 说明 | 默认值 | 示例 |
|------|------|--------|------|
| `--ecsm-url` | 容器平台 API 地址 | `http://localhost:8080` | `http://192.168.1.50:8080` |
| `--etcd` | etcd 集群地址 | `` (纯内存) | `localhost:2379` |
| `--interval` | 采集间隔（秒） | `30` | `10` (测试) / `60` (生产) |

## 监控功能

### 微服务层自动监控

程序启动后会自动持续监控：

1. **节点指标采集**（每 interval 秒）
   - CPU 使用率、内存使用率
   - 磁盘空间、网络流量
   - 容器总数、运行状态

2. **容器指标采集**
   - CPU/内存使用情况
   - 重启次数统计
   - 部署状态监控

3. **服务指标采集**
   - 服务健康状态
   - 实例在线数量
   - 负载因子

### 告警机制

#### 阈值告警（Critical 级别）
- 节点 CPU > 90%
- 节点内存 > 90%
- 容器 CPU > 80%
- 容器内存 > 80%
- 容器重启次数 > 10

#### 趋势告警（Warning 级别）
- CPU 持续上升趋势（10% 变化率）
- 内存持续增长趋势（10% 变化率）
- 容器频繁重启趋势（3 次连续检测）
- 服务校验失败率上升

### 数据存储

- **内存缓冲**：最近 600 条记录，快速查询
- **持久化存储**：BoltDB 文件，每分钟快照
- **趋势分析**：5 分钟历史窗口，10 个数据点

## 完整部署流程

### 步骤 1: 上传源码到 SylixOS

```bash
cd /path/to/health-monitor

# 确保依赖完整
go mod tidy

# 打包源码
tar -czf health-monitor-src.tar.gz \
  --exclude='.git' \
  --exclude='build' \
  --exclude='*.db' \
  .

# 上传到 SylixOS
scp health-monitor-src.tar.gz root@192.168.1.100:/tmp/
```

### 步骤 2: 在 SylixOS 上编译

```bash
# SSH 登录 SylixOS
ssh root@192.168.1.100

# 解压源码
cd /tmp
tar -xzf health-monitor-src.tar.gz -C /usr/local/src/
cd /usr/local/src/health-monitor

# 检查 Go 环境
go version
go env

# 编译程序
go mod tidy
go build -o health-monitor ./cmd/monitor

# 检查编译结果
ls -lh health-monitor
file health-monitor
./health-monitor --help
```

### 步骤 3: 配置运行环境

```bash
# 创建数据和日志目录
mkdir -p /var/lib/health-monitor
mkdir -p /var/log

# 设置权限
chmod 755 /var/lib/health-monitor
chmod 755 /var/log
```

### 步骤 4: 首次测试运行

```bash
# 前台运行，验证功能
/usr/local/bin/health-monitor \
  --ecsm-url=http://192.168.1.50:8080 \
  --db-path=/var/lib/health-monitor/data.db \
  --interval=10

# 观察输出，确认：
# 1. 能成功连接容器平台
# 2. 能采集到指标数据
# 3. 没有报错信息
```

### 步骤 5: 后台持续运行

```bash
# 使用 nohup 后台运行
nohup /usr/local/bin/health-monitor \
  --ecsm-url=http://192.168.1.50:8080 \
  --db-path=/var/lib/health-monitor/data.db \
  --interval=30 \
  > /var/log/health-monitor.log 2>&1 &

# 记录 PID
echo $! > /var/run/health-monitor.pid

# 验证运行
ps aux | grep health-monitor
tail -f /var/log/health-monitor.log
```

### 步骤 6: 设置开机自启（可选）

创建启动脚本 `/etc/init.d/health-monitor`:

```bash
#!/bin/sh

DAEMON=/usr/local/bin/health-monitor
PIDFILE=/var/run/health-monitor.pid
LOGFILE=/var/log/health-monitor.log

case "$1" in
  start)
    echo "Starting health monitor..."
    nohup $DAEMON \
      --ecsm-url=http://192.168.1.50:8080 \
      --db-path=/var/lib/health-monitor/data.db \
      --interval=30 \
      > $LOGFILE 2>&1 &
    echo $! > $PIDFILE
    echo "Started"
    ;;
  stop)
    echo "Stopping health monitor..."
    if [ -f $PIDFILE ]; then
      kill $(cat $PIDFILE)
      rm -f $PIDFILE
      echo "Stopped"
    else
      echo "Not running"
    fi
    ;;
  restart)
    $0 stop
    sleep 2
    $0 start
    ;;
  *)
    echo "Usage: $0 {start|stop|restart}"
    exit 1
    ;;
esac
```

设置权限并启用：

```bash
chmod +x /etc/init.d/health-monitor
# 添加到启动项（根据 SylixOS 的具体机制）
```

## 监控验证

### 查看运行状态

```bash
# 检查进程
ps aux | grep health-monitor

# 查看实时日志
tail -f /var/log/health-monitor.log

# 检查数据库
ls -lh /var/lib/health-monitor/data.db
```

### 测试告警功能

程序会自动检测异常并打印告警，例如：

```
✅ [14:30:15] 采集成功 (耗时: 234ms)
⚠️  告警: [CRITICAL] 节点 node-001 CPU 使用率过高: 95.3%
⚠️  告警: [WARNING] 容器 container-abc CPU 呈上升趋势: +12.5%
✅ [14:30:45] 采集成功 (耗时: 189ms)
```

## 故障排查

### 问题 1: 无法连接容器平台

```bash
# 检查网络连通性
ping 192.168.1.50

# 测试 API 可访问性
curl http://192.168.1.50:8080/api/v1/node

# 检查程序日志
tail -50 /var/log/health-monitor.log | grep "采集失败"
```

### 问题 2: 数据库权限错误

```bash
# 检查目录权限
ls -ld /var/lib/health-monitor

# 修改权限
chmod 755 /var/lib/health-monitor
chown root:root /var/lib/health-monitor
```

### 问题 3: 内存占用过高

```bash
# 调整采集间隔
kill <PID>
./health-monitor --interval=60  # 增加到 60 秒

# 清理旧数据库
rm /var/lib/health-monitor/data.db
```

### 问题 4: 程序崩溃

```bash
# 查看崩溃日志
tail -100 /var/log/health-monitor.log

# 使用更详细的日志重新运行
./health-monitor --ecsm-url=... --interval=30 2>&1 | tee debug.log
```

### 问题 5: 编译错误（BoltDB 相关）

如果遇到 `undefined: unix.Mmap` 等错误，说明交叉编译不支持，请：

```bash
# 方案1: 在 SylixOS 上直接编译（推荐）
# 上传源码后在目标机器上编译

# 方案2: 检查 CGO 设置
go env CGO_ENABLED  # 应该是 1

# 方案3: 如果 SylixOS 不支持 BoltDB，可以考虑纯内存模式
# 修改 state_manager.go 使用纯内存存储
```

## 性能指标

- **内存占用**: 约 30-80 MB
- **CPU 占用**: < 5%（采集时瞬时升高到 10-20%）
- **磁盘占用**: 约 5-30 MB/天（取决于监控对象数量）
- **网络流量**: 约 10-50 KB/次采集

## 注意事项

1. **编译方式**: 强烈建议在 SylixOS 上直接编译，因为 BoltDB 依赖底层系统调用（Mmap, Madvise 等），交叉编译可能遇到兼容性问题

2. **Go 环境**: 确保 SylixOS 上已安装 Go 1.18+ 和必要的开发工具链

3. **首次运行**: 建议使用较短的采集间隔（10-15秒）测试，确认功能正常后再设置为 30-60 秒

4. **数据库位置**: 如果 SylixOS 存储空间有限，建议将 `--db-path` 设置为临时目录或外部存储

5. **网络稳定性**: 如果容器平台网络不稳定，建议增加采集间隔，避免频繁失败

6. **系统资源**: 建议预留至少 100MB 内存和 100MB 磁盘空间

## 升级方法

```bash
# 1. 停止旧版本
kill $(cat /var/run/health-monitor.pid)

# 2. 备份数据库
cp /var/lib/health-monitor/data.db /var/lib/health-monitor/data.db.backup

# 3. 上传新版本
scp build/health-monitor-sylixos root@192.168.1.100:/usr/local/bin/health-monitor.new

# 4. 替换并重启
mv /usr/local/bin/health-monitor.new /usr/local/bin/health-monitor
chmod +x /usr/local/bin/health-monitor
/etc/init.d/health-monitor start
```
