# 趋势分析快速参考

## 一图看懂趋势分析

```
┌─────────────────────────────────────────────────────────────────┐
│                    微服务指标采集 (每30秒)                       │
│  NodeMetrics | ContainerMetrics | ServiceMetrics                │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ↓ UpdateMetric()
┌─────────────────────────────────────────────────────────────────┐
│                   StateManager 状态管理                          │
│                                                                  │
│  latestStates: {"node:001" → NodeMetric}  ← 最新值 (~100ns)    │
│                                                                  │
│  historyBuffers: {"node:001" → RingBuffer[600]}                │
│    [0] {t=1732896000, CPU=60%}                                  │
│    [1] {t=1732896030, CPU=62%}                                  │
│    [2] {t=1732896060, CPU=65%}                                  │
│    [3] {t=1732896090, CPU=68%}  ← 历史序列 (~10μs)             │
│    [4] {t=1732896120, CPU=72%}                                  │
│    ...                                                           │
│    [9] {t=1732896300, CPU=88%}  ← 查询最近5分钟                │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ↓ QueryHistory(TypeNode, "001", 5min)
┌─────────────────────────────────────────────────────────────────┐
│               Generator → TrendAnalyzer 趋势分析                 │
│                                                                  │
│  1. 提取指标序列                                                 │
│     cpuValues = [60, 62, 65, 68, 72, 75, 78, 82, 85, 88]       │
│                                                                  │
│  2. 计算趋势 calculateTrend()                                   │
│     ┌─────────────────────────────────────┐                    │
│     │ for i=1 to len(values):             │                    │
│     │   if values[i] > values[i-1]:       │                    │
│     │     increases++                     │                    │
│     └─────────────────────────────────────┘                    │
│     结果: {                                                      │
│       IsIncreasing: true,                                       │
│       ContinuousCount: 9,  ← 连续9次上升                       │
│       ChangeRate: 0.046    ← 平均4.6%增长                      │
│     }                                                            │
│                                                                  │
│  3. 判断是否异常                                                 │
│     if IsIncreasing && Count>=3 && Rate>10%:                   │
│       ✓ 触发告警                                                │
│                                                                  │
│  4. 预测未来                                                     │
│     currentCPU = 88%                                            │
│     avgChange = +2.8% per 30s                                  │
│     prediction: 88% + 2.8%*10 = 116% (超过100%)               │
│     → "可能在未来5分钟内达到100%"                               │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ↓
┌─────────────────────────────────────────────────────────────────┐
│                      告警事件输出                                │
│                                                                  │
│  AlertEvent {                                                   │
│    Type: "Node-CPU-Trend"                                       │
│    Severity: "Warning"         ← 趋势告警 (不是Critical)        │
│    FaultCode: "TREND_CPU_INCREASE"                              │
│    Message: "CPU使用率持续上升，当前88.0%，变化率4.6%"          │
│    Metadata: {                                                  │
│      "trend_type": "increasing",                                │
│      "change_rate": 0.046,                                      │
│      "prediction": "可能在未来5分钟内达到100%"                  │
│    }                                                             │
│  }                                                               │
└─────────────────────────────────────────────────────────────────┘
```

## 关键代码片段

### 1. 查询历史数据
```go
// 在 TrendAnalyzer 中
history := ta.stateManager.QueryHistory(
    state.MetricTypeNode, 
    "node-001", 
    5*time.Minute  // 回溯5分钟
)
// 返回: []HistoryEntry (最近10个数据点)
```

### 2. 提取指标序列
```go
cpuValues := make([]float64, 0, len(history))
for _, entry := range history {
    nodeMetric := entry.Data.(*model.NodeMetrics)
    cpuValues = append(cpuValues, nodeMetric.CPUUsage.(float64))
}
// cpuValues = [60, 62, 65, 68, 72, 75, 78, 82, 85, 88]
```

### 3. 计算趋势
```go
func (ta *TrendAnalyzer) calculateTrend(values []float64) *TrendInfo {
    increases := 0
    totalChange := 0.0
    
    for i := 1; i < len(values); i++ {
        diff := values[i] - values[i-1]
        if diff > 0 {
            increases++
            totalChange += diff / values[i-1]  // 相对变化率
        }
    }
    
    return &TrendInfo{
        IsIncreasing:    increases > len(values)/2,
        ContinuousCount: increases,
        ChangeRate:      totalChange / float64(increases),
    }
}
```

### 4. 判断异常
```go
trend := ta.calculateTrend(cpuValues)

if trend.IsIncreasing && trend.ContinuousCount >= 3 {
    return &TrendResult{
        Type:       "increasing",
        Message:    fmt.Sprintf("CPU使用率持续上升，当前%.1f%%", currentValue),
        Value:      currentValue,
        ChangeRate: trend.ChangeRate,
        Prediction: predictFuture(cpuValues),
    }
}
```

## 4种趋势分析场景

### 场景1: CPU持续上升
```
数据: [60, 62, 65, 68, 72, 75, 78, 82, 85, 88] %
判断: 连续9次上升 ✓
告警: TREND_CPU_INCREASE
预测: 可能在未来5分钟达到90%
```

### 场景2: 内存持续增长
```
数据: [50, 53, 57, 62, 66, 71, 75, 80, 84, 88] %
判断: 连续9次上升 ✓
告警: TREND_MEMORY_INCREASE
预测: 可能在未来10分钟达到85%触发OOM
```

### 场景3: 容器频繁重启
```
Uptime序列: [3600, 3900, 60, 360, 660, 60, 360, 660, 60, 360] 秒
                            ↑重启    ↑重启    ↑重启
判断: 5分钟内重启3次 ✓
告警: TREND_RESTART_INCREASE
```

### 场景4: 业务校验失败率上升
```
失败率: [1.0, 1.5, 2.2, 3.0, 4.1, 5.5, 6.8, 8.2, 9.5, 11.0] %
判断: 连续上升且超过5% ✓
告警: TREND_VALIDATION_FAILURE
```

## 阈值 vs 趋势对比

| 场景 | CPU值 | 阈值告警 | 趋势告警 |
|------|-------|----------|----------|
| T1 | 60% | ✗ (未超90%) | ✗ (数据不足) |
| T2 | 62% | ✗ | ✗ |
| T3 | 65% | ✗ | ✗ |
| T4 | 68% | ✗ | ✗ |
| T5 | 72% | ✗ | ✓ **趋势告警** |
| T6 | 75% | ✗ | ✓ |
| T7 | 78% | ✗ | ✓ |
| T8 | 82% | ✗ | ✓ |
| T9 | 85% | ✗ | ✓ |
| T10 | 92% | ✓ **Critical** | ✓ Warning |

**关键优势**: 在T5就发出Warning，提前5个周期预警！

## 参数调优

### 默认配置 (平衡)
```go
trendWindowSize:  10     // 分析10个数据点
trendThreshold:   0.1    // 10%变化率
continuousCount:  3      // 连续3次确认
lookbackDuration: 5min   // 回溯5分钟
```

### 敏感配置 (早预警)
```go
trendWindowSize:  8      // 减少窗口
trendThreshold:   0.05   // 5%变化率
continuousCount:  2      // 只需2次
lookbackDuration: 3min   // 回溯3分钟
```

### 保守配置 (少误报)
```go
trendWindowSize:  15     // 增加窗口
trendThreshold:   0.15   // 15%变化率
continuousCount:  5      // 需要5次
lookbackDuration: 10min  // 回溯10分钟
```

## 性能开销

```
单次趋势分析:
  QueryHistory()        ~10μs   (Ring Buffer查询)
  提取指标序列           ~5μs    (数组遍历)
  calculateTrend()      ~2μs    (简单循环)
  生成告警              ~1μs    (创建对象)
  ─────────────────────────────
  总计                  ~20μs   (0.02毫秒)

100个节点每30秒分析一次:
  100 nodes × 20μs = 2ms
  CPU占用: 2ms / 30s = 0.006%
  
完全可以忽略不计！
```

## 常见问题速查

**Q: 为什么需要 Ring Buffer？**
- A: BoltDB 太慢 (~2ms)，Ring Buffer 快 1000 倍 (~10μs)

**Q: 历史数据保留多久？**
- A: 10分钟 (600条×1秒采样)，足够趋势分析

**Q: 趋势告警会重复发送吗？**
- A: 有去重机制，5分钟内同一趋势只告警1次

**Q: 可以关闭趋势分析吗？**
- A: 可以，使用 `NewGenerator()` 而不是 `NewGeneratorWithStateManager()`

**Q: 如何查看趋势分析效果？**
- A: 运行 `go run cmd/trend_demo/main.go`

## 运行演示

```bash
# 趋势分析演示
cd cmd/trend_demo
go run main.go

# 完整系统演示
cd cmd/integration_demo
go run main.go
```

## 更多文档

- [TREND_ANALYSIS.md](TREND_ANALYSIS.md) - 完整趋势分析文档
- [INTEGRATION.md](../../INTEGRATION.md) - 系统集成架构
- [README.md](../../README.md) - 项目总览
