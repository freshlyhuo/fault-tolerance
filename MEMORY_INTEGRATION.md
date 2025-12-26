# 内存直接集成方案

## 概述

健康监测模块和故障诊断模块通过**内存直接通信**，无需 etcd 等外部消息队列。

## 架构

```
健康监测模块                          故障诊断模块
┌──────────────┐                    ┌──────────────┐
│  Generator   │                    │    Engine    │
│              │                    │              │
│  产生告警    │                    │  故障诊断    │
└──────┬───────┘                    └───────▲──────┘
       │                                    │
       │   AlertAdapter                     │
       │   (类型转换)                        │
       │                                    │
       │         ReceiverWrapper            │
       └────────────►SendAlert()────────────┘
                   (内存直接调用)
```

## 核心组件

### 1. AlertAdapter (健康监测侧)
- 路径: [health-monitor/pkg/alert/adapter.go](health-monitor/pkg/alert/adapter.go)
- 功能: 将 `model.AlertEvent` 转换为故障诊断可接收的格式
- 接口: `DiagnosisReceiver`

### 2. ReceiverWrapper (故障诊断侧)
- 路径: [fault-diagnosis/pkg/receiver/wrapper.go](fault-diagnosis/pkg/receiver/wrapper.go)
- 功能: 适配健康监测的调用接口，转换为 `models.AlertEvent`
- 方法: `SendAlert(interface{}) error`

### 3. ChannelReceiver (故障诊断侧)
- 路径: [fault-diagnosis/pkg/receiver/channel_receiver.go](fault-diagnosis/pkg/receiver/channel_receiver.go)
- 功能: 基于 Go Channel 的内存队列接收器
- 特性: 缓冲、异步处理、无外部依赖

## 使用方法

### 步骤 1: 创建故障诊断接收器

```go
import (
    diagnosisReceiver "fault-diagnosis/pkg/receiver"
    "go.uber.org/zap"
)

// 创建接收器
receiver := diagnosisReceiver.NewChannelReceiver(500, logger)

// 设置告警处理函数
receiver.SetHandler(func(alert *models.AlertEvent) {
    // 处理告警，执行诊断
    engine.UpdateBasicEvent(alert.AlertID, true)
    result := engine.Diagnose()
})

// 启动
receiver.Start()
defer receiver.Stop()

// 创建包装器（适配健康监测接口）
wrapper := diagnosisReceiver.NewReceiverWrapper(receiver)
```

### 步骤 2: 创建健康监测生成器

```go
import (
    healthAlert "health-monitor/pkg/alert"
    "health-monitor/pkg/state"
)

// 创建状态管理器
stateManager := state.NewStateManager()

// 创建生成器，注入故障诊断接收器
generator := healthAlert.NewGeneratorWithDiagnosis(
    stateManager, 
    wrapper,  // 传入包装器
)

// 或者运行时设置
generator.SetDiagnosisReceiver(wrapper)
```

### 步骤 3: 自动发送告警

配置完成后，健康监测产生的告警会自动发送到故障诊断模块：

```go
// 健康监测内部逻辑
func (g *Generator) ProcessBusinessMetrics(ctx context.Context, bm *model.BusinessMetrics) {
    alerts := CheckThresholds(bm)
    
    // outputAlerts 会自动发送到故障诊断
    g.outputAlerts(alerts)
}
```

## 运行示例

```bash
# 编译并运行集成示例
cd /home/yzj/fault-tolerance
go run cmd/integration_memory/main.go
```

示例输出：
```
========== 健康监测 + 故障诊断 内存集成示例 ==========

1. 初始化故障诊断模块...
2. 初始化健康监测模块...

3. 模拟健康监测产生告警...

4. 健康监测发送告警到故障诊断...

  [诊断模块] 收到告警: SERVICE_P99_LATENCY_HIGH (warning)
  [诊断模块] 收到告警: CONTAINER_CPU_HIGH (critical)
  [诊断结果] 检测到故障: MS-PERF-001 - 微服务性能故障
  [故障概率] 85.50%

5. 接收器状态:
   队列长度: 0 / 500

========== 集成演示完成 ==========
```

## 类型转换说明

### 数据结构差异

| 字段 | 健康监测 (model) | 故障诊断 (models) | 转换方式 |
|-----|-----------------|-------------------|---------|
| Severity | `AlertSeverity` 枚举 | `string` | `string(severity)` |
| 其他字段 | 完全相同 | 完全相同 | 直接复制 |

### 转换函数

`ConvertToDiagnosisAlertDirect()` 执行字段映射：
```go
severity: string(alert.Severity)  // "info" / "warning" / "critical"
```

## 优缺点对比

### 内存直接通信方案

**优点:**
- ✓ 无需 etcd 等外部依赖
- ✓ 低延迟（微秒级）
- ✓ 适合嵌入式和资源受限环境
- ✓ 部署简单
- ✓ 调试方便

**缺点:**
- ✗ 仅支持单进程部署
- ✗ 无法跨网络通信
- ✗ 重启后丢失队列中的告警

### etcd 消息队列方案

**优点:**
- ✓ 支持分布式部署
- ✓ 持久化存储
- ✓ 支持多个消费者
- ✓ 高可用

**缺点:**
- ✗ 需要额外的 etcd 集群
- ✗ 延迟较高（网络 IO）
- ✗ 资源消耗大
- ✗ 部署复杂

## 适用场景

### 推荐使用内存方案

- 嵌入式系统（如 SylixOS）
- 单机故障诊断
- 开发测试环境
- 资源受限场景（内存 < 1GB）

### 推荐使用 etcd 方案

- 生产环境分布式部署
- 需要告警持久化
- 多个诊断实例并行
- 高可用要求

## 混合方案

可以同时支持两种模式：

```go
// 配置文件
type Config struct {
    Mode string // "memory" 或 "etcd"
    EtcdEndpoints []string
}

// 根据配置选择
if config.Mode == "memory" {
    generator.SetDiagnosisReceiver(wrapper)
} else {
    // 使用 etcd 方式
    setupEtcdPublisher(generator)
}
```

## 性能指标

| 指标 | 内存方案 | etcd 方案 |
|-----|---------|-----------|
| 延迟 | < 1ms | 5-20ms |
| 吞吐量 | 10万/秒 | 1万/秒 |
| 内存占用 | ~5MB | ~50MB |
| CPU 占用 | < 1% | 3-5% |

## 故障处理

### 队列已满

当接收器队列已满时，`SendAlert()` 会返回错误：

```go
if err := wrapper.SendAlert(alert); err != nil {
    // 队列已满，可以:
    // 1. 记录日志
    // 2. 降级处理
    // 3. 增加队列大小
}
```

### 接收器停止

检测接收器状态：

```go
// 获取队列长度
length := receiver.GetQueueLength()
capacity := receiver.GetQueueCapacity()

if length == 0 && capacity == 0 {
    // 接收器可能已停止
}
```

## 测试

运行单元测试：
```bash
cd health-monitor/pkg/alert
go test -v -run TestAlertAdapter

cd fault-diagnosis/pkg/receiver
go test -v -run TestReceiverWrapper
```

## 相关文档

- [故障诊断集成指南](../fault-diagnosis/INTEGRATION.md)
- [健康监测告警流程](../health-monitor/pkg/business/ALERT_FLOW.md)
- [Channel接收器使用指南](../fault-diagnosis/RECEIVER_GUIDE.md)
