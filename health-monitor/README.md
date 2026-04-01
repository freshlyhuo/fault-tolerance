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

## 主要函数说明

以下为 health-monitor 模块主链路中的核心函数说明，统一包含：函数名、URL（无则写无）、功能、工作流程、功能解释。

### 启动与调度层

| 函数名 | URL | 功能 | 工作流程（简版） | 功能解释 |
|------|-----|------|------------------|----------|
| `main` | 无 | 启动健康监控系统 | 初始化参数和上下文 -> 初始化 StateManager -> 启动业务层与微服务层 -> 监听退出信号 | 系统总入口，串联两条监控链路。 |
| `microServiceMonitorLoop` | 无 | 周期驱动微服务采集 | 立即采集一次 -> ticker 周期触发 -> 调用 `collectAndReport` | 微服务监控的定时执行器。 |
| `collectAndReport` | 无 | 执行单次采集并打印结果 | 调用 `Dispatcher.RunOnce` -> 记录成功/失败与耗时 | 将一次采集过程封装为标准动作，便于循环调用。 |
| `businessTestLoop` | 无 | 测试模式周期发报文 | 立即发一次 -> ticker 周期触发 -> 调用 `sendTestPackets` | 用于联调验证业务层解析和告警链路。 |
| `sendTestPackets` | 无 | 发送模拟业务报文 | 构造供电/热控/通信报文 -> 调用 `Receiver.Submit` 投递 | 提供可控输入，验证阈值触发与恢复场景。 |

### 业务层链路

| 函数名 | URL | 功能 | 工作流程（简版） | 功能解释 |
|------|-----|------|------------------|----------|
| `Receiver.Submit` | 无 | 接收业务二进制报文并入队 | 校验报文长度 -> 写入 `inputChan` | 业务层数据入口，做最基础的合法性检查。 |
| `Receiver.Start` | 无 | 启动业务报文监听循环 | 监听 `inputChan/ctx/stopChan` -> 解析报文 -> 分发给 Dispatcher | 将收包、解析、分发串联为实时流水线。 |
| `Receiver.ParsePacket` | 无 | 解析业务报文为结构化指标 | 读取组件类型和长度 -> 按组件调用 `parseXxx` -> 生成 `BusinessMetrics` | 业务侧核心解析器，为阈值判断提供标准输入。 |
| `Dispatcher.HandleBusinessMetrics` | 无 | 处理业务层指标 | 写入 `StateManager` -> 调用 `Generator.ProcessBusinessMetrics` | 业务指标统一派发点，连接存储与告警。 |

### 微服务采集链路

| 函数名 | URL | 功能 | 工作流程（简版） | 功能解释 |
|------|-----|------|------------------|----------|
| `Fetcher.ListNode` | `/api/v1/node` | 分页获取节点列表 | 组装分页参数 -> GET 请求 -> 解析返回 | 节点基础清单来源。 |
| `Fetcher.ListContainerByNodePage` | `/api/v1/container/node` | 按节点分页获取容器列表 | 组装 `nodeIds[]` 和分页参数 -> GET 请求 -> 解析返回 | 容器数据采集关键入口。 |
| `Fetcher.ListService` | `/api/v1/service` | 分页获取服务列表 | 组装分页参数 -> GET 请求 -> 解析返回 | 服务清单来源。 |
| `Fetcher.ListNodeStatus` | `/api/v1/node/status` | 批量获取节点状态 | 组装 `ids[]` 参数 -> GET 请求 -> 解析 `nodes` | 获取节点在线状态和资源状态。 |
| `Fetcher.ListContainerStatus` | `/api/v1/container/{taskID}` | 获取容器详细状态 | 遍历容器逐个请求 -> 解析详情 -> 汇总结果 | 用详情补齐容器运行时状态。 |
| `Fetcher.ListServiceStatus` | `/api/v1/service/{id}` | 获取服务详细状态 | 遍历服务逐个请求 -> 解析详情 -> 汇总结果 | 获取服务健康与实例状态。 |
| `Fetcher.GatherRawMetrics` | 组合调用：`/api/v1/node`、`/api/v1/node/status`、`/api/v1/container/node`、`/api/v1/container/{taskID}`、`/api/v1/service`、`/api/v1/service/{id}` | 统一采集全部原始指标 | 依次采集 nodes/containers/services -> 组装 `RawMetrics` 返回 | 微服务采集总入口。 |
| `Extractor.Extract` | 无 | 原始指标标准化 | 调用 `ExtractNodeMetrics`、`ExtractContainerMetrics`、`ExtractServiceMetrics` | 将采集结果转换为统一的 `MicroServiceMetricsSet`。 |
| `Dispatcher.RunOnce` | 无 | 执行一轮完整微服务监控 | `GatherRawMetrics` -> `Extract` -> 保存状态 -> 阈值告警 | 微服务链路单次闭环入口。 |

### 告警与阈值链路

| 函数名 | URL | 功能 | 工作流程（简版） | 功能解释 |
|------|-----|------|------------------|----------|
| `Generator.ProcessBusinessMetrics` | 无 | 生成业务层告警 | 按组件类型调用阈值检查 -> 汇总告警 -> `outputAlerts` | 业务层告警主入口。 |
| `Generator.ProcessMicroserviceMetrics` | 无 | 生成微服务层告警 | 遍历 node/container/service -> 阈值检查 -> `outputAlerts` | 微服务层告警主入口。 |
| `Generator.outputAlerts` | 无 | 输出并下发告警 | 去重 -> 过滤 firing 输出 -> 可选发送诊断模块 | 告警统一出口。 |
| `CheckPowerThresholdsWithState` | 无 | 电源阈值检查（支持恢复） | 加载阈值配置 -> 比较电压/电流 -> 状态变更触发告警 | 电源类异常与恢复判定核心。 |
| `CheckNodeThresholdsWithState` | 无 | 节点阈值检查（支持恢复） | 检查在线状态、CPU、内存、磁盘 -> 生成告警 | 节点健康判定核心。 |
| `CheckContainerThresholdsWithState` | 无 | 容器阈值检查（支持恢复） | 检查部署状态、CPU、内存、磁盘 -> 生成告警 | 容器稳定性判定核心。 |
| `CheckServiceThresholdsWithState` | 无 | 服务阈值检查（支持恢复） | 检查健康状态和在线实例数 -> 生成告警 | 服务可用性判定核心。 |

### 状态与配置层

| 函数名 | URL | 功能 | 工作流程（简版） | 功能解释 |
|------|-----|------|------------------|----------|
| `NewStateManager` | 无 | 初始化状态管理器 | 创建 latest/history/alert 三类内存结构 -> 返回管理器 | 状态中心初始化入口。 |
| `StateManager.UpdateMetric` | 无 | 更新实时状态并追加历史 | 时间戳对齐 -> 更新最新状态 -> 追加 RingBuffer 历史 | 指标写入统一入口。 |
| `StateManager.GetLatestState` | 无 | 查询指标最新状态 | 通过 `metricType:id` 键读取最新状态映射 | 提供低延迟实时查询。 |
| `StateManager.QueryHistory` | 无 | 查询窗口历史数据 | 定位指标 RingBuffer -> 按时间窗口过滤返回 | 支持趋势分析与回溯。 |
| `GetThresholdConfig` | 无 | 加载阈值配置 | 初始化默认值 -> 读取 JSON 覆盖 -> 进程内缓存复用 | 阈值配置统一入口，保障默认可用。 |


## 主要变量与数据结构

以下内容用于快速理解 health-monitor 模块的数据组织方式，分为“主要变量（含常量）”和“主要数据结构”。

### 主要变量（含常量）

| 名称 | 所在位置 | 类型 | 作用 |
|------|----------|------|------|
| `HistoryRetention` | `pkg/state/state_manager.go` | `time.Duration` | 历史数据窗口保留时长基线（当前为10分钟）。 |
| `RingBufferSize` | `pkg/state/state_manager.go` | `int` | 每个指标的环形缓冲区容量（当前600条）。 |
| `latestStates` | `StateManager` | `map[string]Metric` | 保存每个指标的最新状态，支撑低延迟查询。 |
| `historyBuffers` | `StateManager` | `map[string]*RingBuffer` | 保存每个指标的历史窗口数据。 |
| `alertStates` | `StateManager` | `map[string]bool` | 记录告警当前状态（firing/resolved），用于去重与恢复判断。 |
| `stopChan` | `StateManager`/`Receiver` | `chan struct{}` | 协程停止信号，实现优雅退出。 |
| `inputChan` | `Receiver` | `chan []byte` | 业务层原始报文输入队列。 |
| `thresholdOnce` | `pkg/config/thresholds_loader.go` | `sync.Once` | 确保阈值配置只初始化一次。 |
| `thresholdCfg` | `pkg/config/thresholds_loader.go` | `*ThresholdConfig` | 进程内缓存阈值配置。 |
| `CompRunMgr`...`CompEPS` | `pkg/business/receiver.go` | `const` | 业务层组件类型编号，驱动报文解析分发。 |
| `AlertStatusFiring`/`AlertStatusResolved` | `pkg/models/alert.go` | `const` | 告警状态枚举值。 |
| `MetricTypeNode`/`MetricTypeContainer`/`MetricTypeService`/`MetricTypeBusiness` | `pkg/state/types.go` | `const` | 状态管理中的指标类型标签。 |
| `BaseURL` | `SimpleHTTPClient` | `string` | 微服务采集 API 的基础地址。 |
| `Client` | `SimpleHTTPClient` | `*http.Client` | 发起 ECSM HTTP 请求的客户端（含超时）。 |
| `HM_THRESHOLD_CONFIG` | 环境变量 | `string` | 自定义阈值配置路径（优先级最高）。 |

### 主要数据结构

| 数据结构 | 所在位置 | 核心字段 | 说明 |
|----------|----------|----------|------|
| `MicroServiceMetricsSet` | `pkg/models/metrics.go` | `NodeMetrics`、`ContainerMetrics`、`ServiceMetrics` | 微服务层统一指标聚合对象。 |
| `BusinessMetrics` | `pkg/models/metrics.go` | `ComponentType`、`Timestamp`、`Data` | 业务层解析后的统一指标封装。 |
| `NodeMetrics` | `pkg/models/metrics.go` | `ID`、`Status`、`CPUUsage`、内存/磁盘字段 | 节点监控指标模型。 |
| `ContainerMetrics` | `pkg/models/metrics.go` | `ID`、`DeployStatus`、`CPUUsage`、内存/磁盘字段 | 容器监控指标模型。 |
| `ServiceMetrics` | `pkg/models/metrics.go` | `ID`、`Healthy`、`InstanceOnline`、`Status` | 服务监控指标模型。 |
| `PowerMetrics`/`ThermalMetrics`/`CommMetrics`/`ActuatorMetrics` | `pkg/models/metrics.go` | 电压电流、温度、通信状态、动量轮转速等字段 | 业务层关键组件指标模型。 |
| `AlertEvent` | `pkg/models/alert.go` | `AlertID`、`Status`、`Source`、`FaultCode`、`MetricValue` | 告警事件标准结构（触发与恢复共用）。 |
| `StateManager` | `pkg/state/state_manager.go` | `latestStates`、`historyBuffers`、`alertStates` | 状态中心，负责实时状态、历史窗口与告警状态管理。 |
| `RingBuffer`/`HistoryEntry` | `pkg/state/state_manager.go` | `head`、`tail`、`data` / `Timestamp`、`Data` | 历史数据窗口缓存的底层结构。 |
| `Metric` 接口及 `NodeMetric`/`ContainerMetric`/`ServiceMetric`/`BusinessMetric` | `pkg/state/types.go` | `GetID`、`GetType`、`GetTimestamp`、`GetData` | 统一指标抽象与包装层，便于状态管理通用处理。 |
| `ThresholdConfig` | `pkg/config/thresholds_loader.go` | `Power`、`Thermal`、`Comm`、`Node`、`Container`、`Service` | 阈值配置总模型，支持默认值与 JSON 覆盖。 |
| `Fetcher`/`RawMetrics` | `pkg/microservice/fetcher.go` | `http` / `Nodes`、`Containers`、`Services` | 微服务采集器及其原始采集结果封装。 |
| `NodeStatus`/`ContainerInfo`/`ServiceGet` | `pkg/microservice/*.go` | 运行状态、资源使用、部署状态等字段 | ECSM API 返回对象的原始映射结构。 |



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
