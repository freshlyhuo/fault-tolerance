# StateManager 数据集成 + 趋势分析完整架构

## 系统架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                         数据源层                                 │
│  ┌──────────────┐              ┌──────────────┐                │
│  │  业务层报文   │              │  ECSM API    │                │
│  │  (二进制)     │              │  (微服务)     │                │
│  └──────┬───────┘              └──────┬───────┘                │
└─────────┼──────────────────────────────┼─────────────────────────┘
          │                              │
          ↓                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                         解析层                                   │
│  ┌──────────────┐              ┌──────────────┐                │
│  │   Receiver   │              │  Fetcher     │                │
│  │   (解析报文)  │              │  + Extractor │                │
│  └──────┬───────┘              └──────┬───────┘                │
└─────────┼──────────────────────────────┼─────────────────────────┘
          │                              │
          ↓                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                       派发层 (Dispatcher)                        │
│  ┌──────────────────────────────────────────────────────┐      │
│  │            业务层 Dispatcher                          │      │
│  │  HandleBusinessMetrics(BusinessMetrics)              │      │
│  └──────────────────────────────────────────────────────┘      │
│  ┌──────────────────────────────────────────────────────┐      │
│  │            微服务层 Dispatcher                        │      │
│  │  RunOnce() → saveToStateManager()                    │      │
│  └──────────────────────────────────────────────────────┘      │
│         ┌────────────────┬────────────────┐                    │
└─────────┼────────────────┼────────────────┼─────────────────────┘
          │                │                │
          ↓                ↓                ↓
┌─────────────────────────────────────────────────────────────────┐
│                      核心处理层                                  │
│  ┌─────────────────┐  ┌─────────────────────────────────────┐ │
│  │  StateManager   │  │     Alert Generator                  │ │
│  │  ┌───────────┐  │  │  ┌────────────┐  ┌──────────────┐  │ │
│  │  │Ring Buffer│  │  │  │ Threshold  │  │TrendAnalyzer │  │ │
│  │  │  (实时)    │  │←─┤  │ (阈值检查) │  │  (趋势分析)   │  │ │
│  │  └───────────┘  │  │  └────────────┘  └──────────────┘  │ │
│  │  ┌───────────┐  │  │         ↓               ↓          │ │
│  │  │  BoltDB   │  │  │      Critical        Warning        │ │
│  │  │  (持久化) │  │  │    (已发生故障)    (趋势预警)       │ │
│  │  └───────────┘  │  └─────────────────────────────────────┘ │
│  └─────────────────┘                                          │
└─────────────────────────────────────────────────────────────────┘
          │                              
          ↓                              
┌─────────────────────────────────────────────────────────────────┐
│                    输出层 (AlertEvent)                           │
│  → 故障诊断模块                                                  │
│  → 消息队列 (MQ)                                                │
│  → 可视化平台 (Grafana)                                         │
│  → 告警通知 (邮件/短信)                                          │
└─────────────────────────────────────────────────────────────────┘
```

## 数据流向架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         数据源层                                 │
│                                                                  │
│  ┌──────────────┐              ┌──────────────┐                │
│  │  业务层报文   │              │  ECSM API    │                │
│  │  (二进制)     │              │  (微服务)     │                │
│  └──────┬───────┘              └──────┬───────┘                │
│         │                              │                         │
└─────────┼──────────────────────────────┼─────────────────────────┘
          │                              │
          ↓                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                         解析层                                   │
│                                                                  │
│  ┌──────────────┐              ┌──────────────┐                │
│  │   Receiver   │              │  Fetcher     │                │
│  │   (解析报文)  │              │  + Extractor │                │
│  └──────┬───────┘              └──────┬───────┘                │
│         │                              │                         │
└─────────┼──────────────────────────────┼─────────────────────────┘
          │                              │
          ↓                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                       派发层 (Dispatcher)                        │
│                                                                  │
│  ┌──────────────────────────────────────────────────────┐      │
│  │            业务层 Dispatcher                          │      │
│  │  HandleBusinessMetrics(BusinessMetrics)              │      │
│  └──────────────────────────────────────────────────────┘      │
│                                                                  │
│  ┌──────────────────────────────────────────────────────┐      │
│  │            微服务层 Dispatcher                        │      │
│  │  RunOnce() → saveToStateManager()                    │      │
│  └──────────────────────────────────────────────────────┘      │
│                                                                  │
│         ┌────────────────┬────────────────┐                    │
│         ↓                ↓                ↓                    │
└─────────┼────────────────┼────────────────┼─────────────────────┘
          │                │                │
          ↓                ↓                ↓
┌─────────────────────────────────────────────────────────────────┐
│                      核心处理层                                  │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐│
│  │  StateManager   │  │  Alert Generator│  │  其他处理模块    ││
│  │  (状态存储)      │  │  (告警生成)     │  │  (DB/MQ/可视化)  ││
│  └─────────────────┘  └─────────────────┘  └─────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## 详细数据流

相关规范：业务层二进制报文格式与 Pub/Sub 推送接口见 `pkg/business/BUSINESS_PACKET_PUBSUB_SPEC.md`。

### 业务层数据流
```
1. 报文接收
   业务报文 → Receiver.Submit(packet)
   
2. 报文解析
   Receiver.ParsePacket(packet) → BusinessMetrics
   
3. 派发处理
   Receiver → Dispatcher.HandleBusinessMetrics()
   
4. 状态存储
   ┌─ BusinessMetric 封装
   │  Data: BusinessMetrics (原始数据)
   │  Timestamp: 当前时间戳
   │
   └─ StateManager.UpdateMetric(businessMetric)
      ├─ 保存到实时状态 map
      ├─ 追加到 Ring Buffer (历史)
      └─ 定期持久化到 BoltDB
   
5. 告警检查 (阈值)
   Generator.ProcessBusinessMetrics()
   ├─ 根据 ComponentType 调用对应的阈值检查
   ├─ CheckPowerThresholds()      → Critical/Warning
   ├─ CheckThermalThresholds()    → Critical/Warning
   ├─ CheckCommThresholds()       → Critical/Warning
   └─ 生成告警并直接输出

注: 业务层目前只做阈值告警，不做趋势分析
```

### 微服务层数据流 (含趋势分析)
```
1. 指标采集
   Fetcher.GatherRawMetrics() → 原始JSON
   
2. 指标提取
   Extractor.Extract(raw) → MicroServiceMetricsSet
   ├─ NodeMetrics[]
   ├─ ContainerMetrics[]
   └─ ServiceMetrics[]
   
3. 派发处理
   Dispatcher.RunOnce()
   
4. 状态存储
   saveToStateManager(metrics)
   ├─ 遍历所有节点
   │  └─ NodeMetric 封装 → UpdateMetric()
   │     ├─ latestStates["node:id"] = metric
   │     └─ historyBuffers["node:id"].Append(entry)
   │
   ├─ 遍历所有容器
   │  └─ ContainerMetric 封装 → UpdateMetric()
   │     └─ 同上
   │
   └─ 遍历所有服务
      └─ ServiceMetric 封装 → UpdateMetric()
         └─ 同上
   
5. 告警检查 - 阶段1: 阈值告警 (Critical/Warning)
   Generator.ProcessMicroserviceMetrics()
   ├─ CheckNodeThresholds()      → 已发生故障
   ├─ CheckContainerThresholds() → 已发生故障
   ├─ CheckServiceThresholds()   → 已发生故障
   └─ 生成 Critical 告警
   
6. 告警检查 - 阶段2: 趋势告警 (Warning)
   Generator.trendAnalyzer (如果已启用)
   │
   ├─ AnalyzeNodeTrends(nodeID)
   │  ├─ QueryHistory(TypeNode, nodeID, 5分钟)
   │  │  └─ 返回 Ring Buffer 中的历史数据
   │  │
   │  ├─ analyzeCPUTrend(历史数据)
   │  │  ├─ 提取 CPU 值序列
   │  │  ├─ calculateTrend(cpuValues)
   │  │  │  ├─ 判断连续上升/下降
   │  │  │  └─ 计算变化率
   │  │  │
   │  │  └─ 如果连续上升 && 变化率>10%
   │  │     └─ 返回 TrendResult{预测信息}
   │  │
   │  └─ analyzeMemoryTrend(历史数据)
   │     └─ 同上
   │
   ├─ AnalyzeContainerTrends(containerID)
   │  └─ analyzeRestartTrend(历史数据)
   │     ├─ 检测 Uptime 减少 = 重启
   │     └─ 统计重启频率
   │
   └─ AnalyzeServiceTrends(serviceID)
      └─ analyzeValidationTrend(历史数据)
         ├─ 计算失败率序列
         └─ 检测失败率上升趋势
   
7. 告警输出
   outputAlerts(alerts)
   ├─ 去重和压缩
   ├─ 按严重程度分类
   │  ├─ Critical: 已发生故障，需要立即干预
   │  └─ Warning: 趋势异常，可能即将发生故障
   │
   └─ 输出到各种通道
      ├─ 控制台打印
      ├─ 消息队列
      ├─ 数据库
      └─ 可视化平台
```

## StateManager 存储结构

### 实时状态存储 (内存 Map)
```go
latestStates: map[string]Metric
  "node:node-001"        → NodeMetric
  "node:node-002"        → NodeMetric
  "container:cnt-001"    → ContainerMetric
  "service:svc-001"      → ServiceMetric
  "business:\x03"        → BusinessMetric (供电)
  "business:\x06"        → BusinessMetric (热控)
  ...
```

### 历史数据存储 (Ring Buffer)
```go
historyBuffers: map[string]*RingBuffer
  "node:node-001" → RingBuffer[600] {
    [0]: {Timestamp: t0, Data: NodeMetrics}
    [1]: {Timestamp: t1, Data: NodeMetrics}
    ...
    [599]: {Timestamp: t599, Data: NodeMetrics}
  }
  
每个 RingBuffer:
  - 固定大小: 600条记录
  - 保留时长: 10分钟 (假设1秒采样1次)
  - 自动淘汰: FIFO, 新数据覆盖最旧数据
```

### 持久化存储 (BoltDB)
```
/tmp/integration_demo.db
├─ bucket: snapshots
│  ├─ snapshot_1732896000 → StateSnapshot JSON
│  ├─ snapshot_1732896060 → StateSnapshot JSON
│  └─ snapshot_1732896120 → StateSnapshot JSON
│
└─ bucket: history (预留)
   └─ (可用于长期历史数据归档)
```

## 代码变更说明

### 1. pkg/alert/trend.go (新增)
```go
// 趋势分析器
type TrendAnalyzer struct {
    stateManager     *state.StateManager
    trendWindowSize  int           // 10个数据点
    trendThreshold   float64       // 10%变化率
    continuousCount  int           // 连续3次
    lookbackDuration time.Duration // 回溯5分钟
}

// 核心方法
func (ta *TrendAnalyzer) AnalyzeNodeTrends(ctx, nodeID) []*AlertEvent
func (ta *TrendAnalyzer) AnalyzeContainerTrends(ctx, containerID) []*AlertEvent
func (ta *TrendAnalyzer) AnalyzeServiceTrends(ctx, serviceID) []*AlertEvent

// 趋势分析算法
func (ta *TrendAnalyzer) analyzeCPUTrend(metrics) *TrendResult
func (ta *TrendAnalyzer) analyzeMemoryTrend(metrics) *TrendResult
func (ta *TrendAnalyzer) analyzeRestartTrend(metrics) *TrendResult
func (ta *TrendAnalyzer) analyzeValidationTrend(metrics) *TrendResult
func (ta *TrendAnalyzer) calculateTrend(values) *TrendInfo
```

### 2. pkg/alert/generator.go (更新)
```go
// 新增字段
type Generator struct {
    trendAnalyzer *TrendAnalyzer  // 新增趋势分析器
}

// 新增构造函数
func NewGeneratorWithStateManager(sm *state.StateManager) *Generator {
    return &Generator{
        trendAnalyzer: NewTrendAnalyzer(sm),  // 启用趋势分析
    }
}

// 更新方法 - 增加趋势分析
func (g *Generator) ProcessMicroserviceMetrics(ctx, ms) {
    // 1. 阈值告警 (原有逻辑)
    alerts := CheckNodeThresholds()
    
    // 2. 趋势告警 (新增逻辑)
    if g.trendAnalyzer != nil {
        trendAlerts := g.trendAnalyzer.AnalyzeNodeTrends(ctx, nodeID)
        alerts = append(alerts, trendAlerts...)
    }
    
    // 3. 输出告警
    g.outputAlerts(alerts)
}
```

### 3. pkg/microservice/dispatcher.go (更新)
```go
// 新增字段
type Dispatcher struct {
    stateManager *state.StateManager  // 新增
}

// 更新构造函数 - 使用带趋势分析的Generator
func NewDispatcher(fetcher, stateManager) *Dispatcher {
    return &Dispatcher{
        fetcher:      fetcher,
        extractor:    NewExtractor(),
        generator:    alert.NewGeneratorWithStateManager(stateManager),  // 变更
        stateManager: stateManager,
    }
}

// 新增方法
func (d *Dispatcher) saveToStateManager(metrics) error {
    // 批量保存节点/容器/服务指标到 StateManager
}
```

### 4. pkg/business/dispatcher.go (更新)
```go
// 新增字段
type Dispatcher struct {
    stateManager *state.StateManager  // 新增
}

// 更新构造函数 - 使用带趋势分析的Generator
func NewDispatcher(stateManager) *Dispatcher {
    return &Dispatcher{
        generator:    alert.NewGeneratorWithStateManager(stateManager),  // 变更
        stateManager: stateManager,
    }
}

// 更新方法
func (d *Dispatcher) HandleBusinessMetrics() {
    // 1. 保存到 StateManager (新增)
    // 2. 告警生成 (原有)
    // 3. 其他处理
}
```

## 使用示例

### 初始化系统 (含趋势分析)
```go
// 1. 创建 StateManager
sm, _ := state.NewStateManager("/data/state.db")

// 2. 创建业务层 (自动启用趋势分析)
businessDispatcher := business.NewDispatcher(sm)
businessReceiver := business.NewReceiver(businessDispatcher)

// 3. 创建微服务层 (自动启用趋势分析)
fetcher := microservice.NewFetcher(ecsmConfig)
microDispatcher := microservice.NewDispatcher(fetcher, sm)
//                                                   ↑
//                                         内部自动创建带趋势分析的Generator
```

### 运行监控 (自动趋势分析)
```go
// 业务层监听
go businessReceiver.Start(ctx)

// 微服务层周期采集
ticker := time.NewTicker(30 * time.Second)
for range ticker.C {
    microDispatcher.RunOnce(ctx)
    // 每次RunOnce会自动:
    // 1. 采集指标
    // 2. 保存到StateManager (Ring Buffer)
    // 3. 阈值告警检查
    // 4. 趋势告警检查 (自动查询历史数据)
}
```

### 查询状态
```go
// 查询节点状态
metric, _ := sm.GetLatestState(state.MetricTypeNode, "node-001")
nodeMetric := metric.(*state.NodeMetric)

// 查询历史趋势
history := sm.QueryHistory(state.MetricTypeNode, "node-001", 5*time.Minute)

// 查询业务指标
metric, _ := sm.GetLatestState(state.MetricTypeBusiness, "\x03") // 供电服务
businessMetric := metric.(*state.BusinessMetric)
```

## 性能指标

### 写入性能
```
业务层报文:       ~1000/秒  (每个报文触发1次 UpdateMetric)
微服务层采集:     ~30/分钟  (每次采集100+ 指标)
StateManager写入: ~10μs/op  (内存操作)
总吞吐量:        ~10000+ 指标/秒
```

### 存储开销
```
单个指标内存:     ~1KB
600条历史记录:    ~600KB
100个组件:        ~60MB
BoltDB文件:       <100MB (定期清理旧快照)
```

### 查询性能
```
最新状态查询:     ~100ns   (Map查询)
历史数据查询:     ~10μs    (Ring Buffer遍历)
跨类型查询:       ~1ms     (遍历所有类型)
```

## 数据一致性保证

1. **时间戳对齐**: AlignTimestamp() 处理不同来源的时间偏差
2. **并发安全**: RWMutex 保护所有读写操作
3. **原子更新**: 每次 UpdateMetric 是原子操作
4. **快照一致性**: BoltDB ACID 事务保证
5. **故障恢复**: 程序重启自动加载最新快照

## 告警示例输出

### 阈值告警 (Critical)
```
========== 告警事件 ==========

【严重告警】共 3 个:
  [ALERT-NODE-002-CPU-1732896120] Node-CPU-High
    故障码: NODE_CPU_HIGH
    来源: node:node-002
    消息: CPU使用率过高: 92.0%
    指标值: 92.00
    时间戳: 1732896120
```

### 趋势告警 (Warning)
```
【警告告警】共 2 个:
  [TREND-NODE-CPU-node-001-1732896125] Node-CPU-Trend
    故障码: TREND_CPU_INCREASE
    来源: node:node-001
    消息: CPU使用率持续上升，当前85.0%，变化率4.6%
    指标值: 85.00
    时间戳: 1732896125
    元数据: 
      - trend_type: increasing
      - change_rate: 0.046
      - prediction: 可能在未来5分钟内达到90%

  [TREND-SERVICE-VALIDATION-svc-001-1732896126] Service-Validation-Trend
    故障码: TREND_VALIDATION_FAILURE
    来源: service:svc-001
    消息: 业务校验失败率持续上升，当前8.5%，变化率1.2%
    指标值: 8.50
    时间戳: 1732896126
```

## 监控建议

```go
// 定期输出统计信息
ticker := time.NewTicker(1 * time.Minute)
for range ticker.C {
    stats := sm.GetStats()
    log.Printf("StateManager: %+v", stats)
    
    // 检查状态数量
    if stats["latest_states"].(int) > 1000 {
        log.Warn("状态数量过多，可能存在内存泄漏")
    }
}
```

## 故障处理

### 场景1: StateManager 保存失败
```
原因: BoltDB 写入失败（磁盘满等）
影响: 快照无法持久化，程序重启丢失最近1分钟数据
处理: Ring Buffer 继续工作，告警系统正常，修复磁盘后自动恢复
```

### 场景2: 内存不足
```
原因: 组件数量过多
影响: OOM 风险
处理: 
  - 减小 RingBufferSize (600 → 300)
  - 增加清理频率
  - 只保留关键组件历史
```

### 场景3: 程序崩溃
```
原因: 任意原因导致程序退出
影响: Ring Buffer 数据丢失（最近1分钟）
恢复: 
  1. 重启程序
  2. 自动从 BoltDB 加载最新快照
  3. 重新开始采集
  4. 快速恢复到最新状态
```
