# Health Monitor - 航天器健康监测系统

## 系统概述

完整的健康监测系统，支持业务层和微服务层的指标监控、状态管理、告警生成（阈值+趋势分析）。

## 核心功能

### 1. 多层监控
- **业务层**: 二进制报文解析 (供电、热控、通信、姿控等15+组件)
- **微服务层**: ECSM API 监控 (节点、容器、服务指标)

业务层报文规范与 Pub/Sub 推送接口说明：见 `pkg/business/BUSINESS_PACKET_PUBSUB_SPEC.md`。

### 2. 状态管理
- **实时状态**: 内存 Map，100ns 查询
- **历史数据**: Ring Buffer (600条/指标)，10μs 查询
- **持久化**: BoltDB 快照，每分钟保存

### 3. 告警生成
- **阈值告警**: 已发生故障 (Critical)
- **趋势告警**: 预测性预警 (Warning)
  - CPU/内存持续上升
  - 容器频繁重启
  - 业务校验失败率上升

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

## 快速开始

### 1. 运行完整集成演示

```bash
cd cmd/integration_demo
go run main.go
```

**演示内容**:
- 初始化 StateManager
- 业务层报文解析和告警
- 微服务层指标采集和告警
- 状态查询和历史数据分析
- 持久化快照

### 2. 运行趋势分析演示

```bash
cd cmd/trend_demo
go run main.go
```

**演示场景**:
1. CPU 持续上升趋势 (60% → 75%)
2. 内存持续增长趋势 (50% → 88%)
3. 容器频繁重启 (每3次采样重启1次)
4. 业务校验失败率上升 (1% → 15%)

**预期输出**:
```
========== 场景1: CPU持续上升趋势 ==========
模拟节点 node-cpu-trend 的CPU使用率持续上升...
  [01] CPU: 60.0%
  [02] CPU: 62.5%
  ...
  [12] CPU: 87.5%

执行趋势分析...
========== 告警事件 ==========

【警告告警】共 1 个:
  [TREND-NODE-CPU-node-cpu-trend-xxx] Node-CPU-Trend
    故障码: TREND_CPU_INCREASE
    来源: node:node-cpu-trend
    消息: CPU使用率持续上升，当前87.5%，变化率2.8%
    指标值: 87.50
    预测: 可能在未来3分钟内达到90%
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

### TrendAnalyzer (趋势分析器)

通过历史数据预测未来故障:

```go
// 自动创建 (在 NewGeneratorWithStateManager 中)
analyzer := alert.NewTrendAnalyzer(sm)

// 分析节点趋势
alerts := analyzer.AnalyzeNodeTrends(ctx, "node-001")
// 返回: CPU上升、内存上升等趋势告警

// 分析容器趋势
alerts := analyzer.AnalyzeContainerTrends(ctx, "container-001")
// 返回: 重启频率异常等告警

// 分析服务趋势
alerts := analyzer.AnalyzeServiceTrends(ctx, "service-001")
// 返回: 业务校验失败率上升等告警
```

**趋势判断参数**:
- `trendWindowSize`: 10个数据点
- `trendThreshold`: 10% 变化率
- `continuousCount`: 连续3次上升/下降
- `lookbackDuration`: 回溯5分钟历史

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
  etcd_endpoints: ["localhost:2379"]  # etcd 集群地址，留空则纯内存模式
  ring_buffer_size: 600
  snapshot_interval: "1m"
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

## 扩展功能

### 1. 添加新的趋势分析指标

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

## 文档

- [完整集成架构](INTEGRATION.md) - 数据流、组件交互、代码变更
- [趋势分析详解](pkg/alert/TREND_ANALYSIS.md) - 算法原理、使用示例
- [状态管理使用](pkg/state/USAGE.md) - StateManager 完整文档
- [业务层告警流程](pkg/business/ALERT_FLOW.md)
- [微服务层集成](pkg/microservice/ALERT_INTEGRATION.md)

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
- **纯内存模式**: 崩溃后数据全部丢失
- **etcd 模式**: Ring Buffer 丢失最近1分钟，历史快照保存在 etcd，重启后自动恢复
- 建议生产环境使用 etcd 模式

### Q: 如何调整趋势分析的敏感度？

A: 修改 TrendAnalyzer 参数:
```go
analyzer := &TrendAnalyzer{
    trendWindowSize:  15,  // 增加窗口 = 降低敏感度
    trendThreshold:   0.2, // 增加阈值 = 降低敏感度
    continuousCount:  5,   // 增加次数 = 降低敏感度
}
```

## 贡献

欢迎提交 Issue 和 PR！

## 许可证

MIT License
