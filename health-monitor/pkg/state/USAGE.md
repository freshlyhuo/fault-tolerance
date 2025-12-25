# 状态管理器使用指南

## 为什么需要 BoltDB + Ring Buffer?

### Ring Buffer (内存环形缓冲区)
```
优势:
✅ 纳秒级读写速度 - 适合高频实时查询
✅ 固定内存大小 - 自动淘汰旧数据
✅ 完美的时序数据结构 - FIFO特性
✅ 无需GC压力 - 预分配数组

劣势:
❌ 易失性 - 程序重启丢失
❌ 容量有限 - 只能保存固定数量
```

### BoltDB (持久化存储)
```
优势:
✅ 持久化 - 程序重启数据不丢失
✅ ACID事务 - 数据一致性保证
✅ 零依赖 - 单文件数据库
✅ 快照恢复 - 故障快速恢复

劣势:
❌ 毫秒级延迟 - 比内存慢1000倍
❌ 磁盘IO - 高频写入影响性能
```

### 最佳实践组合
```
┌─────────────────────────────────────────────┐
│         高频实时数据 (每秒数百次)             │
│              ↓                               │
│        Ring Buffer (内存)                    │
│    - 保存最近10分钟数据                       │
│    - 纳秒级查询响应                           │
│    - 支持趋势分析、异常检测                    │
│              ↓                               │
│        定期快照 (每分钟1次)                    │
│              ↓                               │
│         BoltDB (磁盘)                        │
│    - 持久化状态快照                           │
│    - 故障恢复基准点                           │
│    - 历史数据归档                             │
└─────────────────────────────────────────────┘
```

## 架构设计

```
┌──────────────────────────────────────────────────────────┐
│                   StateManager                            │
│  ┌────────────────────────────────────────────────────┐  │
│  │  实时状态存储 (map[string]Metric)                   │  │
│  │  - latestStates: 保存所有组件的最新状态             │  │
│  │  - sync.RWMutex: 并发安全                          │  │
│  └────────────────────────────────────────────────────┘  │
│                           ↓                               │
│  ┌────────────────────────────────────────────────────┐  │
│  │  历史数据缓冲 (map[string]*RingBuffer)             │  │
│  │  - 每个组件独立的环形缓冲区                         │  │
│  │  - 保存最近600条记录 (10分钟@1秒采样)              │  │
│  │  - 自动淘汰过期数据                                 │  │
│  └────────────────────────────────────────────────────┘  │
│                           ↓                               │
│  ┌────────────────────────────────────────────────────┐  │
│  │  持久化层 (BoltDB)                                  │  │
│  │  - 每分钟保存快照                                   │  │
│  │  - 支持故障恢复                                     │  │
│  │  - 历史快照清理                                     │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

## 核心功能

### 1. 实时状态更新
```go
// 更新节点状态
nodeMetric := &state.NodeMetric{
    Data: &model.NodeMetrics{
        ID:               "node-001",
        Status:           "online",
        CPUUsage:         75.5,
        MemoryTotal:      16000000000,
        MemoryFree:       4000000000,
        ContainerRunning: 8,
        ContainerTotal:   10,
    },
    Timestamp: time.Now().Unix(),
}

sm.UpdateMetric(nodeMetric)
```

### 2. 查询最新状态
```go
// 查询单个组件状态
metric, exists := sm.GetLatestState(state.MetricTypeNode, "node-001")
if exists {
    nodeMetric := metric.(*state.NodeMetric)
    fmt.Printf("节点CPU: %.1f%%\n", nodeMetric.Data.CPUUsage.(float64))
}

// 查询所有节点状态
allNodes := sm.GetAllLatestStates(state.MetricTypeNode)
for _, metric := range allNodes {
    nm := metric.(*state.NodeMetric)
    fmt.Printf("节点 %s: %s\n", nm.Data.ID, nm.Data.Status)
}
```

### 3. 历史数据查询
```go
// 查询最近5分钟的CPU历史
history := sm.QueryHistory(state.MetricTypeNode, "node-001", 5*time.Minute)

// 计算平均CPU使用率
var sum float64
for _, entry := range history {
    nodeData := entry.Data.(*model.NodeMetrics)
    sum += nodeData.CPUUsage.(float64)
}
avg := sum / float64(len(history))
fmt.Printf("最近5分钟平均CPU: %.1f%%\n", avg)
```

### 4. 趋势分析示例
```go
// 检测CPU持续上升趋势
func detectCPUTrend(sm *state.StateManager, nodeID string) bool {
    history := sm.QueryHistory(state.MetricTypeNode, nodeID, 3*time.Minute)
    
    if len(history) < 10 {
        return false
    }
    
    // 检查是否持续上升
    increasing := 0
    for i := 1; i < len(history); i++ {
        prevCPU := history[i-1].Data.(*model.NodeMetrics).CPUUsage.(float64)
        currCPU := history[i].Data.(*model.NodeMetrics).CPUUsage.(float64)
        
        if currCPU > prevCPU {
            increasing++
        }
    }
    
    // 如果80%的采样点都在上升，认为是趋势
    return float64(increasing) / float64(len(history)-1) > 0.8
}
```

### 5. 持久化和恢复
```go
// 手动保存快照
err := sm.SaveSnapshot()

// 程序重启后自动加载最新快照
sm, err := state.NewStateManager("/data/state.db")
// 快照会自动加载

// 后台自动持久化（每分钟一次）
// 无需手动调用，StateManager自动处理
```

## 集成示例

### 与业务层集成
```go
package business

import (
    "context"
    "state"
    "alert"
)

type Dispatcher struct {
    receiver     *Receiver
    stateManager *state.StateManager
    generator    *alert.Generator
}

func (d *Dispatcher) HandleBusinessMetrics(ctx context.Context, bm *model.BusinessMetrics) {
    // 1. 保存到状态管理器
    businessMetric := &state.BusinessMetric{
        Data:      bm,
        Timestamp: time.Now().Unix(),
    }
    d.stateManager.UpdateMetric(businessMetric)
    
    // 2. 发送到告警生成器
    d.generator.ProcessBusinessMetrics(ctx, bm)
    
    // 3. 其他处理（健康评分、可视化等）
}
```

### 与微服务层集成
```go
package microservice

import (
    "context"
    "state"
    "alert"
)

type Dispatcher struct {
    fetcher      *Fetcher
    extractor    *Extractor
    stateManager *state.StateManager
    generator    *alert.Generator
}

func (d *Dispatcher) RunOnce(ctx context.Context) error {
    // 1. 采集和提取指标
    raw, _ := d.fetcher.GatherRawMetrics(ctx)
    metrics := d.extractor.Extract(raw)
    
    // 2. 保存到状态管理器
    for _, node := range metrics.NodeMetrics {
        nodeCopy := node
        nodeMetric := &state.NodeMetric{
            Data:      &nodeCopy,
            Timestamp: time.Now().Unix(),
        }
        d.stateManager.UpdateMetric(nodeMetric)
    }
    
    for _, container := range metrics.ContainerMetrics {
        containerCopy := container
        containerMetric := &state.ContainerMetric{
            Data:      &containerCopy,
            Timestamp: time.Now().Unix(),
        }
        d.stateManager.UpdateMetric(containerMetric)
    }
    
    // 3. 发送到告警生成器
    d.generator.ProcessMicroserviceMetrics(ctx, metrics)
    
    return nil
}
```

### 告警生成器使用状态管理器
```go
package alert

import (
    "state"
    "time"
)

type Generator struct {
    stateManager *state.StateManager
}

// 检测持续高CPU
func (g *Generator) checkSustainedHighCPU(nodeID string) bool {
    // 查询最近60秒的历史
    history := g.stateManager.QueryHistory(state.MetricTypeNode, nodeID, 60*time.Second)
    
    if len(history) < 6 { // 至少6个采样点
        return false
    }
    
    // 检查是否持续>85%
    highCount := 0
    for _, entry := range history {
        nodeData := entry.Data.(*model.NodeMetrics)
        if cpuUsage, ok := nodeData.CPUUsage.(float64); ok && cpuUsage > 85.0 {
            highCount++
        }
    }
    
    // 如果90%以上的时间都>85%，触发告警
    return float64(highCount)/float64(len(history)) > 0.9
}

// 检测容器频繁重启
func (g *Generator) checkFrequentRestart(containerID string) bool {
    // 查询最近10分钟的历史
    history := g.stateManager.QueryHistory(state.MetricTypeContainer, containerID, 10*time.Minute)
    
    if len(history) < 2 {
        return false
    }
    
    // 统计重启次数变化
    firstRestart := history[0].Data.(*model.ContainerMetrics).RestartCount
    lastRestart := history[len(history)-1].Data.(*model.ContainerMetrics).RestartCount
    
    restartDelta := lastRestart - firstRestart
    
    // 10分钟内重启超过3次
    return restartDelta > 3
}
```

## 性能特性

### 写入性能
```
Ring Buffer写入:   ~50ns/op   (2000万次/秒)
Map更新:           ~100ns/op  (1000万次/秒)
BoltDB写入:        ~2ms/op    (500次/秒)

结论: Ring Buffer + Map 组合可支持每秒百万级指标更新
```

### 查询性能
```
最新状态查询:      ~100ns/op  (1000万次/秒)
历史数据查询:      ~10μs/op   (10万次/秒)
BoltDB查询:        ~500μs/op  (2000次/秒)

结论: 内存查询比BoltDB快1000倍以上
```

### 内存占用
```
每个指标约1KB
600条历史 × 100个组件 = 60MB
实时状态100个组件 = 100KB
总计: ~60-100MB (可接受范围)
```

## 配置调优

### 调整Ring Buffer大小
```go
// 根据采样频率和保留时长调整
// 如果每秒采样1次，保留10分钟
const RingBufferSize = 10 * 60 = 600

// 如果每10秒采样1次，保留1小时
const RingBufferSize = 6 * 60 = 360

// 如果每秒采样1次，保留1小时
const RingBufferSize = 60 * 60 = 3600
```

### 调整快照间隔
```go
// 高可靠性场景：更频繁快照
const SnapshotInterval = 30 * time.Second

// 高性能场景：降低快照频率
const SnapshotInterval = 5 * time.Minute
```

### 调整历史保留时长
```go
// 只需要短期趋势分析
const HistoryRetention = 5 * time.Minute

// 需要更长时间的分析
const HistoryRetention = 1 * time.Hour
```

## 最佳实践

1. **写入优先级**: 先写Ring Buffer(快)，再写BoltDB(慢)
2. **查询优先级**: 优先查Ring Buffer，失败才查BoltDB
3. **并发控制**: 使用RWMutex，读多写少场景性能最优
4. **定期清理**: 自动清理过期快照，防止磁盘爆满
5. **优雅关闭**: 程序退出前保存最终快照
6. **故障恢复**: 启动时自动加载最新快照

## 故障场景处理

### 场景1: 程序崩溃
```
1. Ring Buffer数据丢失（最近1分钟）
2. 启动时从BoltDB加载最新快照
3. 快照最多丢失1分钟数据
4. 重新开始采集后快速恢复
```

### 场景2: 磁盘满
```
1. BoltDB写入失败
2. Ring Buffer继续工作
3. 告警系统正常运行
4. 清理磁盘后恢复持久化
```

### 场景3: 内存不足
```
1. 减小RingBufferSize
2. 增加清理频率
3. 只保留关键指标的历史
```

## 监控指标

```go
// 定期输出统计信息
ticker := time.NewTicker(1 * time.Minute)
for range ticker.C {
    stats := sm.GetStats()
    fmt.Printf("StateManager统计: %+v\n", stats)
}

// 输出示例:
// {
//   "latest_states": 150,
//   "history_buffers": 120,
//   "ring_buffer_size": 600,
//   "retention": "10m0s"
// }
```
