# 趋势分析模块说明

## 概述

趋势分析模块 (`pkg/alert/trend.go`) 通过分析历史指标数据的变化趋势，提前预警可能发生的故障，实现**预测性告警**。

## 核心功能

### 1. 连续上升检测
- 检测指标值是否持续上升
- 判断依据：连续N个采样点的值递增
- 默认阈值：连续3次上升，变化率>10%

### 2. 连续下降检测
- 检测指标值是否持续下降
- 适用于服务可用性、容器运行数等指标

### 3. 重启次数增长趋势
- 通过容器Uptime的变化检测重启
- Uptime减少 = 发生重启
- 统计时间窗口内的重启频率

### 4. 业务校验失败率上升
- 监控业务校验成功/失败比例
- 失败率持续上升触发告警
- 默认阈值：失败率>5%且持续上升

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    Dispatcher 调度层                        │
│  (定期采集微服务指标)                                        │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────────────────────┐
│                 StateManager 状态管理                        │
│                                                              │
│  ┌──────────────┐                                           │
│  │ Ring Buffer  │  保留最近10分钟历史数据                   │
│  │  600条/指标  │  支持快速时间窗口查询                     │
│  └──────────────┘                                           │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓ QueryHistory(metricType, id, duration)
┌─────────────────────────────────────────────────────────────┐
│                Generator (告警生成器)                        │
│                                                              │
│  ┌─────────────────────────────────────────────────┐       │
│  │          TrendAnalyzer (趋势分析器)              │       │
│  │                                                  │       │
│  │  1. AnalyzeNodeTrends()      - 节点趋势        │       │
│  │  2. AnalyzeContainerTrends() - 容器趋势        │       │
│  │  3. AnalyzeServiceTrends()   - 服务趋势        │       │
│  │                                                  │       │
│  │  分析方法:                                       │       │
│  │  - analyzeCPUTrend()         - CPU趋势         │       │
│  │  - analyzeMemoryTrend()      - 内存趋势        │       │
│  │  - analyzeRestartTrend()     - 重启趋势        │       │
│  │  - analyzeValidationTrend()  - 校验失败趋势    │       │
│  └─────────────────────────────────────────────────┘       │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────────────────────┐
│              AlertEvent 告警事件输出                         │
│                                                              │
│  Severity: Warning (趋势告警)                               │
│  Type: Node-CPU-Trend / Container-Restart-Trend / ...       │
│  FaultCode: TREND_CPU_INCREASE / TREND_RESTART_INCREASE     │
│  Metadata: {trend_type, change_rate, prediction}            │
└─────────────────────────────────────────────────────────────┘
```

## 数据流

```
1. 微服务指标采集
   Fetcher → Extractor → Dispatcher
   
2. 状态存储
   Dispatcher → StateManager.UpdateMetric()
   ├─ 更新实时状态 (latestStates map)
   └─ 追加历史记录 (Ring Buffer)
   
3. 趋势分析触发
   Dispatcher → Generator.ProcessMicroserviceMetrics()
   
4. 历史数据查询
   Generator → TrendAnalyzer → StateManager.QueryHistory()
   返回: []HistoryEntry (最近5分钟的数据点)
   
5. 趋势计算
   TrendAnalyzer.calculateTrend(values)
   ├─ 计算相邻点变化方向
   ├─ 统计连续上升/下降次数
   └─ 计算平均变化率
   
6. 告警生成
   如果检测到趋势异常:
   ├─ 创建 AlertEvent (Severity=Warning)
   ├─ 添加预测信息 (Metadata.prediction)
   └─ 输出告警事件
```

## 代码示例

### 1. 创建带趋势分析的生成器

```go
// 创建状态管理器
sm, _ := state.NewStateManager("/data/state.db")

// 创建生成器（自动启用趋势分析）
generator := alert.NewGeneratorWithStateManager(sm)

// 创建微服务调度器
dispatcher := microservice.NewDispatcher(fetcher, sm)
```

### 2. 处理微服务指标（自动触发趋势分析）

```go
func (d *Dispatcher) RunOnce(ctx context.Context) {
    // 1. 采集指标
    metrics := d.extractor.Extract(raw)
    
    // 2. 保存到StateManager
    d.saveToStateManager(metrics)
    
    // 3. 告警检查（阈值 + 趋势）
    d.generator.ProcessMicroserviceMetrics(ctx, metrics)
    //          ↓
    //  自动调用 TrendAnalyzer.AnalyzeNodeTrends()
    //          ↓
    //  查询历史: QueryHistory(TypeNode, nodeID, 5分钟)
    //          ↓
    //  计算趋势: calculateTrend(cpuValues)
    //          ↓
    //  生成告警: AlertEvent (如果异常)
}
```

### 3. 趋势判断逻辑

```go
func (ta *TrendAnalyzer) analyzeCPUTrend(metrics) *TrendResult {
    // 提取CPU值序列
    cpuValues := []float64{60, 62, 65, 68, 72, 75, 78, 82, 85, 88}
    
    // 计算趋势
    trend := ta.calculateTrend(cpuValues)
    // trend = {
    //   IsIncreasing: true,
    //   ContinuousCount: 9,  // 连续9次上升
    //   ChangeRate: 0.046,   // 平均每次增长4.6%
    // }
    
    // 判断是否异常
    if trend.IsIncreasing && trend.ContinuousCount >= 3 {
        currentValue := cpuValues[len(cpuValues)-1] // 88%
        
        // 预测
        prediction := "可能在未来5分钟内达到90%"
        
        return &TrendResult{
            Type: "increasing",
            Message: "CPU使用率持续上升，当前88.0%，变化率4.6%",
            Value: 88.0,
            ChangeRate: 0.046,
            Prediction: prediction,
        }
    }
    
    return nil
}
```

### 4. 查询历史数据

```go
// 查询最近5分钟的节点数据
history := sm.QueryHistory(state.MetricTypeNode, "node-001", 5*time.Minute)

// history = []HistoryEntry{
//   {Timestamp: 1732896000, Data: NodeMetrics{CPUUsage: 60.0}},
//   {Timestamp: 1732896030, Data: NodeMetrics{CPUUsage: 62.0}},
//   {Timestamp: 1732896060, Data: NodeMetrics{CPUUsage: 65.0}},
//   ...
// }

// 提取指标序列
cpuValues := make([]float64, len(history))
for i, entry := range history {
    cpuValues[i] = entry.Data.(*model.NodeMetrics).CPUUsage.(float64)
}
```

## 告警类型

### 节点趋势告警

| 告警类型 | FaultCode | 触发条件 | Severity |
|---------|-----------|---------|----------|
| CPU上升趋势 | TREND_CPU_INCREASE | 连续3次上升，变化率>10% | Warning |
| 内存上升趋势 | TREND_MEMORY_INCREASE | 连续3次上升，变化率>10% | Warning |

### 容器趋势告警

| 告警类型 | FaultCode | 触发条件 | Severity |
|---------|-----------|---------|----------|
| 频繁重启 | TREND_RESTART_INCREASE | 5分钟内重启>=2次 | Warning |

### 服务趋势告警

| 告警类型 | FaultCode | 触发条件 | Severity |
|---------|-----------|---------|----------|
| 校验失败率上升 | TREND_VALIDATION_FAILURE | 失败率>5%且连续上升 | Warning |

## 告警事件格式

```go
AlertEvent {
    AlertID: "TREND-NODE-CPU-node-001-1732896120",
    Type: "Node-CPU-Trend",
    FaultCode: "TREND_CPU_INCREASE",
    Severity: "Warning",
    Source: "node:node-001",
    Message: "CPU使用率持续上升，当前85.0%，变化率4.6%",
    MetricValue: 85.0,
    Timestamp: 1732896120,
    Metadata: {
        "trend_type": "increasing",
        "change_rate": 0.046,
        "prediction": "可能在未来5分钟内达到90%"
    }
}
```

## 参数配置

### TrendAnalyzer 配置

```go
type TrendAnalyzer struct {
    trendWindowSize   int           // 趋势窗口大小：10个数据点
    trendThreshold    float64       // 变化率阈值：0.1 (10%)
    continuousCount   int           // 连续次数阈值：3次
    lookbackDuration  time.Duration // 回溯时长：5分钟
}
```

### 调整参数

```go
// 创建自定义配置的分析器
analyzer := &TrendAnalyzer{
    stateManager:     sm,
    trendWindowSize:  15,              // 增加窗口大小
    trendThreshold:   0.05,            // 降低阈值（更敏感）
    continuousCount:  5,               // 需要更多连续点
    lookbackDuration: 10 * time.Minute, // 回溯更长时间
}
```

## 性能考虑

### 1. 查询效率
```
Ring Buffer查询: ~10μs
- 固定内存访问
- 无需磁盘IO
- 适合高频趋势分析
```

### 2. 分析频率
```
建议: 每30秒分析一次
- 过于频繁: 浪费CPU，增加告警噪音
- 过于稀疏: 错过趋势预警
```

### 3. 数据量
```
单节点历史数据: ~600条 × 1KB = 600KB
100个节点: ~60MB
完全可接受
```

## 预测算法

### 线性趋势预测

```go
// 简单线性外推
func predictFutureValue(values []float64, minutesAhead int) float64 {
    n := len(values)
    if n < 2 {
        return values[n-1]
    }
    
    // 计算平均变化率
    totalChange := 0.0
    for i := 1; i < n; i++ {
        totalChange += (values[i] - values[i-1])
    }
    avgChange := totalChange / float64(n-1)
    
    // 外推
    currentValue := values[n-1]
    predictedValue := currentValue + avgChange*float64(minutesAhead)
    
    return predictedValue
}

// 使用示例
cpuValues := []float64{60, 62, 65, 68, 72, 75, 78}
predicted := predictFutureValue(cpuValues, 5) // 预测5分钟后
// predicted ≈ 93% (如果趋势持续)
```

## 运行演示

```bash
# 运行趋势分析演示
cd cmd/trend_demo
go run main.go

# 输出示例:
========== 场景1: CPU持续上升趋势 ==========
模拟节点 node-cpu-trend 的CPU使用率持续上升...
  [01] CPU: 60.0%
  [02] CPU: 62.5%
  ...
  [12] CPU: 87.5%

执行趋势分析...
========== 告警事件 ==========

【警告告警】共 1 个:
  [TREND-NODE-CPU-node-cpu-trend-1732896120] Node-CPU-Trend
    故障码: TREND_CPU_INCREASE
    来源: node:node-cpu-trend
    消息: CPU使用率持续上升，当前87.5%，变化率2.8%
    指标值: 87.50
    时间戳: 1732896120
    预测: 可能在未来3分钟内达到90%
```

## 与阈值告警的区别

| 对比项 | 阈值告警 (Threshold) | 趋势告警 (Trend) |
|-------|---------------------|-----------------|
| 检测时机 | 已经超过阈值 | 接近阈值但尚未超过 |
| 严重程度 | Critical | Warning |
| 处理紧急度 | 需要立即干预 | 需要关注，提前准备 |
| 数据依赖 | 单个数据点 | 历史数据序列 |
| 查询开销 | 无 | QueryHistory (~10μs) |
| 误报率 | 低 | 中等（取决于参数） |

## 最佳实践

### 1. 分层告警策略

```go
if cpuUsage > 90 {
    // 阈值告警 - 立即处理
    alert := CreateCriticalAlert("CPU过高")
} else if detectCPUTrend() {
    // 趋势告警 - 提前预警
    alert := CreateWarningAlert("CPU持续上升")
}
```

### 2. 避免告警疲劳

```go
// 去重: 同一趋势不重复告警
if !alreadyAlerted(source, trendType) {
    sendAlert(alert)
    markAsAlerted(source, trendType, 5*time.Minute)
}
```

### 3. 结合业务场景

```go
// 非业务高峰期的CPU上升更值得关注
if isCPUIncreasing() && !isBusinessPeakTime() {
    alert.Severity = "Warning"
    alert.Message += " (非高峰期异常)"
}
```

## 未来扩展

1. **机器学习预测**
   - 使用历史数据训练模型
   - 预测更准确的故障时间点

2. **多指标关联分析**
   - CPU + 内存 + 磁盘综合分析
   - 发现关联故障模式

3. **自适应阈值**
   - 根据历史基线自动调整阈值
   - 适应业务波动特性

4. **季节性检测**
   - 识别周期性变化（日/周/月）
   - 避免误报正常波动
