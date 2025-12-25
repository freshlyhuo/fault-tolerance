# 资源受限环境下的接收器选择指南

## 三种接收器对比

### 1. Channel接收器（推荐用于资源受限环境）

**适用场景**：
- ✅ 健康监测模块与故障诊断模块在同一进程或同一机器
- ✅ 资源受限，无法运行etcd
- ✅ 告警频率中等（< 10000/秒）
- ✅ 不需要持久化

**优势**：
- 零依赖，纯Go Channel实现
- 内存占用小（< 10MB）
- 延迟极低（< 1ms）
- 配置简单

**劣势**：
- 无持久化，重启丢失
- 单机部署，无法分布式

**使用方式**：
```bash
# 故障诊断模块
./fault-diagnosis -receiver channel -channel-buffer 100

# 健康监测模块集成
import "fault-diagnosis/pkg/receiver"

channelReceiver := receiver.NewChannelReceiver(100, logger)
channelReceiver.Start()

// 发送告警
channelReceiver.SendAlert(alert)
```

**资源消耗**：
- 内存: ~5MB + (缓冲数量 × 告警大小)
- CPU: < 1%
- 网络: 0（无网络通信）

---

### 2. UDP接收器（最轻量）

**适用场景**：
- ✅ 极度资源受限（嵌入式系统）
- ✅ 健康监测与故障诊断在不同进程
- ✅ 可以容忍少量丢包
- ✅ 告警频率低（< 1000/秒）

**优势**：
- 最轻量，零依赖
- 跨进程通信
- 协议简单
- 无连接开销

**劣势**：
- UDP不可靠，可能丢包
- 无持久化
- 需要防火墙配置

**使用方式**：
```bash
# 故障诊断模块
./fault-diagnosis -receiver udp -udp-addr :9999

# 健康监测模块发送告警
import "fault-diagnosis/pkg/receiver"

err := receiver.SendAlertViaUDP(alert, "localhost:9999")
```

**资源消耗**：
- 内存: ~2MB
- CPU: < 0.5%
- 网络: UDP协议开销（最小）

---

### 3. etcd接收器（原方案）

**适用场景**：
- ❌ 资源充足
- ✅ 需要持久化
- ✅ 分布式部署
- ✅ 高可用需求

**优势**：
- 数据持久化
- 分布式一致性
- 高可用

**劣势**：
- 需要运行etcd（资源开销大）
- 内存占用: etcd ~50MB + 数据
- 配置复杂

---

## 推荐方案（资源受限环境）

### 方案A：单机部署 → Channel接收器

```
┌─────────────────────────────────┐
│    同一进程/机器                │
│                                 │
│  ┌──────────────┐              │
│  │ 健康监测模块 │              │
│  └──────┬───────┘              │
│         │ Go Channel            │
│         ↓                       │
│  ┌──────────────┐              │
│  │ 故障诊断模块 │              │
│  └──────────────┘              │
└─────────────────────────────────┘

资源消耗: ~10MB 内存, < 1% CPU
```

**部署步骤**：
```bash
# 1. 编译
cd fault-diagnosis
./build.sh

# 2. 启动（使用Channel接收器）
./build/fault-diagnosis \
  -receiver channel \
  -channel-buffer 100 \
  -config ./configs/fault_tree_business.json
```

### 方案B：跨进程部署 → UDP接收器

```
┌──────────────┐          ┌──────────────┐
│ 健康监测模块 │          │ 故障诊断模块 │
│              │ ─UDP──→  │              │
│ 进程1        │          │ 进程2        │
└──────────────┘          └──────────────┘

资源消耗: ~5MB 内存, < 1% CPU
网络: 局域网UDP（极小开销）
```

**部署步骤**：
```bash
# 1. 启动故障诊断（UDP接收）
./build/fault-diagnosis \
  -receiver udp \
  -udp-addr :9999 \
  -config ./configs/fault_tree_business.json

# 2. 健康监测模块配置
# 发送告警到 localhost:9999
```

---

## 集成示例

### 健康监测模块集成（Channel方式）

```go
// health-monitor/pkg/diagnosis/client.go
package diagnosis

import (
	"fault-diagnosis/pkg/models"
	"fault-diagnosis/pkg/receiver"
	"go.uber.org/zap"
)

// DiagnosisClient 故障诊断客户端
type DiagnosisClient struct {
	channelReceiver *receiver.ChannelReceiver
	logger          *zap.Logger
}

// NewDiagnosisClient 创建客户端
func NewDiagnosisClient(bufferSize int, logger *zap.Logger) *DiagnosisClient {
	return &DiagnosisClient{
		channelReceiver: receiver.NewChannelReceiver(bufferSize, logger),
		logger:          logger,
	}
}

// SendAlert 发送告警
func (c *DiagnosisClient) SendAlert(alert *models.AlertEvent) error {
	return c.channelReceiver.SendAlert(alert)
}

// GetChannelReceiver 获取接收器（供故障诊断模块使用）
func (c *DiagnosisClient) GetChannelReceiver() *receiver.ChannelReceiver {
	return c.channelReceiver
}
```

### 健康监测模块集成（UDP方式）

```go
// health-monitor/pkg/diagnosis/client.go
package diagnosis

import (
	"fault-diagnosis/pkg/models"
	"fault-diagnosis/pkg/receiver"
	"go.uber.org/zap"
)

// DiagnosisClient 故障诊断客户端（UDP版）
type DiagnosisClient struct {
	diagnosisAddr string
	logger        *zap.Logger
}

// NewDiagnosisClient 创建客户端
func NewDiagnosisClient(diagnosisAddr string, logger *zap.Logger) *DiagnosisClient {
	return &DiagnosisClient{
		diagnosisAddr: diagnosisAddr, // "localhost:9999"
		logger:        logger,
	}
}

// SendAlert 发送告警
func (c *DiagnosisClient) SendAlert(alert *models.AlertEvent) error {
	return receiver.SendAlertViaUDP(alert, c.diagnosisAddr)
}
```

---

## 性能对比

| 指标 | Channel | UDP | etcd |
|------|---------|-----|------|
| 延迟 | < 1ms | < 2ms | < 10ms |
| 吞吐量 | 50000/s | 10000/s | 5000/s |
| 内存 | ~5MB | ~2MB | ~100MB |
| CPU | < 1% | < 0.5% | ~5% |
| 可靠性 | 高（内存） | 中（UDP） | 极高（持久化） |
| 部署复杂度 | 低 | 低 | 高 |

---

## 启动命令对比

```bash
# Channel接收器（最简单）
./fault-diagnosis -receiver channel

# UDP接收器（跨进程）
./fault-diagnosis -receiver udp -udp-addr :9999

# etcd接收器（原方案）
./fault-diagnosis -receiver etcd -etcd localhost:2379
```

---

## 故障转移方案

如果未来资源不再受限，可以平滑迁移：

```bash
# 1. 启动etcd
docker run -d -p 2379:2379 quay.io/coreos/etcd:latest

# 2. 切换到etcd接收器
./fault-diagnosis -receiver etcd -etcd localhost:2379

# 无需修改代码，只需修改启动参数
```

---

## 建议

**资源受限环境（嵌入式、IoT）**：
- 优先选择 **Channel接收器**
- 如需跨进程，选择 **UDP接收器**

**资源充足环境**：
- 选择 **etcd接收器**，获得持久化和高可用

**混合环境**：
- 可以同时运行多个接收器实例
- 边缘设备用Channel/UDP
- 中心节点用etcd
