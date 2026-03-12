# Health Monitor - 健康监测模块

## 模块概述

完整的健康监测模块，支持业务层和微服务层的指标监控、状态管理、告警生成。

## 核心功能

### 1. 多层监控
- **业务层**: 二进制报文解析 (供电、热控、通信、姿控等15+组件)
- **微服务层**: ECSM API 监控 (节点、容器、服务指标)

业务层报文规范与 Pub/Sub 推送接口说明：见 `pkg/business/BUSINESS_PACKET_PUBSUB_SPEC.md`。

### 2. 状态管理
- **实时状态**: 内存 Map，100ns 查询
- **历史数据**: Ring Buffer (600条/指标)，10μs 查询


## 项目结构

```
health-monitor/
├── cmd/
│   ├── monitor/           # 主程序
│   ├── integration_demo/  # 完整集成演示
│   └── trend_demo/        # 趋势分析演示
│
├── pkg/
│   ├── alert/            # 告警模块
│   │   ├── generator.go      # 告警生成器
│   │   ├── threshold.go      # 阈值检查
│   │   ├── trend.go          # 趋势分析 ⭐
│   │   ├── correlate.go      # 关联分析
│   │   ├── debounce.go       # 去抖动
│   │   └── TREND_ANALYSIS.md # 趋势分析文档
│   │
│   ├── business/         # 业务层
│   │   ├── receiver.go       # 报文接收解析
│   │   └── dispatcher.go     # 指标派发
│   │   └── BUSINESS_PACKET_PUBSUB_SPEC.md # 业务层报文规范 + Pub/Sub接口
│   │
│   ├── microservice/     # 微服务层
│   │   ├── fetcher.go        # 指标采集
│   │   ├── extractor.go      # 指标提取
│   │   └── dispatcher.go     # 指标派发
│   │
│   ├── state/            # 状态管理 ⭐
│   │   ├── state_manager.go  # 核心状态管理器
│   │   ├── types.go          # 指标类型定义
│   │   ├── storage.go        # 存储接口
│   │   ├── USAGE.md          # 使用说明
│   │   └── function.md       # 功能设计
│   │
│   ├── models/           # 数据模型
│   │   ├── metrics.go        # 指标结构
│   │   ├── alert.go          # 告警结构
│   │   └── topology.go       # 拓扑结构
│   │
│   ├── config/           # 配置管理
│   └── utils/            # 工具函数
│
├── INTEGRATION.md        # 完整集成架构文档 ⭐
└── go.mod
```

## 核心组件说明

### StateManager (状态管理器)

负责所有指标的存储和查询:

```go
// 初始化（纯内存模式）
sm, _ := state.NewStateManager()

// 或者使用 etcd 持久化
sm, _ := state.NewStateManager("localhost:2379")

// 更新指标
sm.UpdateMetric(nodeMetric)

// 查询最新状态
metric, exists := sm.GetLatestState(state.MetricTypeNode, "node-001")

// 查询历史数据 (用于趋势分析)
history := sm.QueryHistory(state.MetricTypeNode, "node-001", 5*time.Minute)

// 保存快照
sm.SaveSnapshot()
```


### Generator (告警生成器)

统一的告警生成入口:

```go
// 创建不带趋势分析的生成器
generator := alert.NewGenerator()

// 创建带趋势分析的生成器 (推荐)
generator := alert.NewGeneratorWithStateManager(sm)

// 处理业务层指标 (仅阈值告警)
generator.ProcessBusinessMetrics(ctx, businessMetrics)

// 处理微服务层指标 (阈值 + 趋势告警)
generator.ProcessMicroserviceMetrics(ctx, microserviceMetrics)
```

## 告警分类

### Critical (严重) - 立即干预
- 已经超过阈值
- 故障已经发生
- 需要立即处理

**示例**:
- CPU > 90%
- 内存使用 > 95%
- 容器运行率 < 70%
- 服务校验失败率 > 20%

### Warning (警告) - 趋势预警
- 尚未超过阈值
- 但指标持续恶化
- 需要关注和准备

**示例**:
- CPU 从 60% 持续上升到 85%
- 内存使用率连续10次递增
- 容器 5 分钟内重启 2 次
- 业务校验失败率从 1% 上升到 8%

## 数据流

```
业务层:
  报文 → Receiver → Dispatcher → StateManager + Alert (阈值)

微服务层:
  ECSM → Fetcher → Extractor → Dispatcher → StateManager + Alert (阈值+趋势)

StateManager:
  Ring Buffer (实时) + BoltDB (持久化)

Alert:
  Threshold (阈值) + Trend (趋势) → AlertEvent
```

## 性能指标

| 指标 | 性能 |
|------|------|
| 状态更新延迟 | ~10μs |
| 最新状态查询 | ~100ns |
| 历史数据查询 | ~10μs |
| 阈值告警延迟 | <1ms |
| 趋势分析延迟 | ~1ms |
| 内存占用 | ~60MB (100组件) |
| 磁盘占用 | <100MB |

## 配置文件

```yaml
# config/config.yaml
state_manager:
  mode: "memory"                       # 固定内存缓存模式（不依赖etcd）
  ring_buffer_size: 600
  history_retention: "10m"

trend_analyzer:
  window_size: 10
  threshold: 0.1
  continuous_count: 3
  lookback_duration: "5m"

alert:
  deduplication_window: "5m"
  output_channels:
    - console
    - mq
    - database
```
# 业务层告警处理流程

## 架构概览

```
原始报文 → Receiver.ParsePacket() → BusinessMetrics (结构化数据)
                                            ↓
                            Dispatcher.HandleBusinessMetrics()
                                            ↓
                            Generator.ProcessBusinessMetrics()
                                            ↓
                            Threshold 阈值检查函数
                                            ↓
                            AlertEvent[] (告警事件)
                                            ↓
                            Generator.outputAlerts() → 直接输出
```

## 数据流说明

### 1. Receiver 解析报文
- **输入**: 原始二进制报文
- **输出**: `BusinessMetrics` 结构体
- **职责**: 将报文解析为对应的组件指标结构体

```go
metrics, err := receiver.ParsePacket(packet)
// metrics.Data 包含具体组件的指标结构体
// 例如: *PowerMetrics, *ThermalMetrics, *CommMetrics 等
```

### 2. Dispatcher 分发指标
- **输入**: `BusinessMetrics` 结构体
- **输出**: 无（将指标转发给 Generator）
- **职责**: 接收解析后的指标，转发给告警生成器

```go
dispatcher.HandleBusinessMetrics(ctx, metrics)
// 内部调用: generator.ProcessBusinessMetrics(ctx, metrics)
```

### 3. Generator 生成告警
- **输入**: `BusinessMetrics` 结构体
- **输出**: 直接输出告警到控制台（可扩展到其他输出）
- **职责**:
  - 根据组件类型调用对应的阈值检查函数
  - 对告警进行去重、分类
  - 输出告警事件

```go
generator.ProcessBusinessMetrics(ctx, bm)
// 内部调用 threshold 检查函数
// 直接输出告警，不返回给 dispatcher
```

### 4. Threshold 阈值检查
- **输入**: 具体组件的指标结构体（如 `*PowerMetrics`）
- **输出**: `[]*AlertEvent` 告警事件列表
- **职责**: 根据 metrics.md 中定义的阈值判断是否异常

```go
alerts := CheckPowerThresholds(powerMetrics)
// 返回所有超过阈值的告警
```

# 微服务层告警集成流程

## 架构设计

```
微服务监控数据采集
       ↓
Fetcher (GatherRawMetrics)
       ↓
Extractor (Extract)
       ↓
Dispatcher (RunOnce)
       ↓
alert.Generator (ProcessMicroserviceMetrics)
       ↓
alert.Threshold (CheckNodeThresholds/CheckContainerThresholds/CheckServiceThresholds)
       ↓
AlertEvent 生成与输出
```

## 数据流程

### 1. 采集阶段
- **Fetcher**: 从ECSM API采集原始指标数据
- **输出**: 原始JSON数据

### 2. 提取阶段
- **Extractor**: 解析原始数据,提取结构化指标
- **输出**: `MicroServiceMetricsSet` 包含:
  - `[]NodeMetrics` - 节点指标列表
  - `[]ContainerMetrics` - 容器指标列表
  - `[]ServiceMetrics` - 服务指标列表

### 3. 派发阶段
- **Dispatcher**: 统一派发指标到告警模块
- **功能**:
  - 调用 `generator.ProcessMicroserviceMetrics()`
  - 后续可扩展: StateManager存储、数据库持久化、可视化推送

### 4. 告警生成阶段
- **Generator**: 处理微服务指标,生成告警事件
- **流程**:
  1. 遍历所有节点指标 → `CheckNodeThresholds()`
  2. 遍历所有容器指标 → `CheckContainerThresholds()`
  3. 遍历所有服务指标 → `CheckServiceThresholds()`
  4. 收集所有告警事件
  5. 告警去重 (`deduplicateAlerts`)
  6. 按严重程度分类输出

### 5. 阈值检查阶段
根据 `microservice/metrics.md` 中定义的阈值进行判断:

#### 节点指标检查 (CheckNodeThresholds)
| 指标 | 正常阈值 | 故障判据 | 故障编号 | 严重程度 |
|------|----------|----------|----------|----------|
| 节点状态 | online | offline | MS-NO-FL-1 | Critical |
| CPU使用率 | ≤75% | >85% | MS-NO-FL-2 | Warning |
| 内存使用率 | ≤80% | >90% | MS-NO-FL-3 | Critical |
| 磁盘使用率 | ≤80% | >90% | MS-NO-FL-4 | Critical |
| 容器运行比例 | ≥0.9 | <0.8 | MS-NO-FL-6 | Warning |

#### 容器指标检查 (CheckContainerThresholds)
| 指标 | 正常阈值 | 故障判据 | 故障编号 | 严重程度 |
|------|----------|----------|----------|----------|
| 部署状态 | success | failure | MS-CN-FL-1 | Critical |
| 启动状态 | running | exited/paused | MS-CN-FL-2 | Critical/Warning |
| 运行时长 | ≥300s | <60s | MS-CN-FL-3 | Warning |
| CPU使用率 | ≤80% | >90% | MS-CN-FL-5 | Warning |
| 内存使用率 | ≤85% | >90% | MS-CN-FL-5 | Critical |
| 磁盘占用率 | ≤80% | >90% | MS-CN-FL-6 | Warning |

#### 服务指标检查 (CheckServiceThresholds)
| 指标 | 正常阈值 | 故障判据 | 故障编号 | 严重程度 |
|------|----------|----------|----------|----------|
| 健康状态 | TRUE | FALSE | MS-SV-FL-1 | Critical |
| 节点数量 | ≥1 | 0 | MS-SV-FL-5 | Critical |
| 容器运行比例 | ≥0.9 | <0.8 | MS-SV-FL-4 | Warning |

### 6. 告警输出阶段
- **输出格式**: 控制台打印,按严重程度分类
  - 【严重告警】Critical
  - 【警告告警】Warning
  - 【信息告警】Info
- **告警信息包含**:
  - AlertID: 唯一标识
  - Type: 告警类型
  - Severity: 严重程度
  - Source: 来源 (节点/容器/服务ID)
  - Message: 描述信息
  - FaultCode: 故障编号
  - MetricValue: 指标值
  - Timestamp: 时间戳



## 扩展功能

### 1. 添加趋势分析指标

```go
// 在 trend.go 中添加
func (ta *TrendAnalyzer) analyzeDiskTrend(metrics) *TrendResult {
    diskValues := extractDiskUsage(metrics)
    trend := ta.calculateTrend(diskValues)
    
    if trend.IsIncreasing && trend.ContinuousCount >= 3 {
        return &TrendResult{
            Type: "increasing",
            Message: "磁盘使用率持续上升...",
            ...
        }
    }
    return nil
}
```

### 2. 集成消息队列

```go
// 在 generator.go 的 outputAlerts 中
func (g *Generator) outputAlerts(alerts) {
    // 发送到 Kafka
    for _, alert := range alerts {
        g.mqProducer.Send("alerts", alert)
    }
}
```

### 3. 添加可视化

```go
// 导出 Prometheus 指标
func (sm *StateManager) ExportMetrics() {
    for id, metric := range sm.latestStates {
        prometheus.Gauge("node_cpu_usage").Set(metric.CPUUsage)
    }
}
```

## 常见问题

### Q: 趋势分析会不会产生很多误报？

A: 有完善的过滤机制:
- 需要连续多次上升（默认3次）
- 变化率需要超过阈值（默认10%）
- 只有严重程度为 Warning，不是 Critical
- 可以通过调整参数降低敏感度

### Q: 历史数据会占用多少内存？

A: Ring Buffer 固定大小:
- 每个指标 600 条记录 ≈ 600KB
- 100 个组件 ≈ 60MB
- 内存占用可控

### Q: 程序崩溃后数据会丢失吗？

A: 取决于存储模式:
- 当前实现为**纯内存缓存模式**，进程重启后缓存数据会丢失
- 如需持久化，可在后续版本增加外部存储适配层

### Q: 如何调整趋势分析的敏感度？

A: 修改 TrendAnalyzer 参数:
```go
analyzer := &TrendAnalyzer{
    trendWindowSize:  15,  // 增加窗口 = 降低敏感度
    trendThreshold:   0.2, // 增加阈值 = 降低敏感度
    continuousCount:  5,   // 增加次数 = 降低敏感度
}
```
