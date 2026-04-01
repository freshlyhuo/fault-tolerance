# 故障诊断模块 (Fault Diagnosis Module)

## 概述

故障诊断模块是容错监控系统的核心组件，负责接收健康监测模块的告警事件，通过故障树分析（FTA）模型进行逻辑推理，判断系统级故障并生成诊断结论。

## 核心功能

1. **故障树管理（Fault Tree Analysis, FTA）**
   - 支持配置化的故障树定义
   - 支持与门（AND）、或门（OR）、非门（NOT）等逻辑门
   - 支持顶层事件、中间事件、基本事件的层级结构
   - 支持热加载配置

2. **事件驱动诊断**
   - 接收来自健康监测模块的告警事件
   - 基本事件状态实时更新
   - 自底向上的逻辑求值
   - 顶层故障事件触发和诊断生成

3. **双层支持**
   - 业务层故障诊断（如蓄电池异常、AD模块异常）
   - 微服务层故障诊断（如服务性能下降、容器崩溃）

## 项目结构

```
fault-diagnosis/
├── cmd/
│   └── diagnosis/          # 主程序入口
│       └── main.go
├── pkg/
│   ├── models/            # 数据模型
│   │   ├── event.go       # 事件定义
│   │   ├── fault_tree.go  # 故障树结构定义
│   │   └── diagnosis.go   # 诊断结果定义
│   ├── engine/            # FTA推理引擎
│   │   ├── evaluator.go   # 故障树求值器
│   │   ├── state.go       # 事件状态管理
│   │   └── engine.go      # 诊断引擎主逻辑
│   ├── config/            # 配置管理
│   │   ├── loader.go      # 故障树配置加载
│   │   └── watcher.go     # 配置热加载监听
│   ├── receiver/          # 告警接收器
│   │   └── receiver.go    # 接收
│   └── utils/             # 工具包
│       └── logger.go      # 日志工具
├── configs/               # 配置文件
│   ├── fault_trees_multi_template.json # 业务层多故障树模板（默认）
│   ├── fault_tree_business.json    # 单业务故障树示例
│   └── fault_tree_microservice.json # 微服务层故障树
└── test/                  # 测试
    └── integration_test.go
```

## 工作流程

### 1. 事件接收与状态更新
- 持续监听健康监测模块的告警事件
- 将告警事件映射为故障树中的基本事件
- 在内存中更新基本事件状态为 TRUE

### 2. 诊断求值（自底向上）
- 基本事件状态变化触发求值流程
- 从触发的基本事件开始，逐级向上计算
- **或门（OR）**: 任一输入为 TRUE，输出为 TRUE
- **与门（AND）**: 所有输入为 TRUE，输出才为 TRUE
- **非门（NOT）**: 输入为 FALSE，输出为 TRUE

### 3. 顶层事件触发
- 当顶层事件求值为 TRUE 时，确认系统级故障
- 提取故障码、故障原因等诊断信息

### 4. 诊断输出
- 生成诊断报告
- 发送至故障修复模块或告警系统

## 主要函数说明

以下为 fault-diagnosis 模块主链路中的核心函数说明，统一包含：函数名、URL（无则写无）、功能、工作流程（简版）、功能解释。




| 函数名 | URL | 功能 | 工作流程（简版） | 功能解释 |
|------|-----|------|------------------|----------|
| LoadFaultTrees | 无 | 加载多故障树配置 | 读取 JSON 文件 -> 校验每棵树 -> 返回故障树列表 | 支持多种配置格式，保证运行时装载灵活性。 |

| NewChannelReceiver | 无 | 创建内存通道接收器 | 初始化通道、上下文、缓冲区与日志 -> 返回接收器 | 资源受限场景下的轻量告警接入实现。 |
| ChannelReceiver.Start | 无 | 启动告警消费循环 | 校验 handler 已设置 -> 启动消费协程 consume | 接收器运行入口，开始处理告警队列。 |
| ChannelReceiver.consume | 无 | 持续消费队列中的告警 | 监听停止信号和告警通道 -> 逐条调用 handleAlert | 将异步告警输入转为稳定的串行处理流程。 |
| ChannelReceiver.SendAlert | 无 | 向队列投递告警 | 非阻塞写入告警通道 -> 队列满时告警并返回错误 | 供外部模块提交告警，具备背压保护。 |


| NewMultiDiagnosisEngine | 无 | 初始化多故障树诊断引擎 | 遍历故障树创建单树引擎 -> 构建 alert_id 到引擎列表路由表 | 多树并行诊断入口，支持告警 fanout。 |
| MultiDiagnosisEngine.ProcessAlert | 无 | 告警路由与并发分发 | 用 alert_id 查路由 -> 并发调用每棵目标树引擎 -> 等待全部完成 | 同一告警可同时触发多棵故障树评估。 |
| DiagnosisEngine.buildTree | 无 | 构建故障树运行时结构 | 创建基本/中间/顶层节点 -> 建立子节点关系 -> 处理 NOT 前缀节点 | 将静态配置转换为可求值的内存树结构。 |
| DiagnosisEngine.ProcessAlert | 无 | 处理单条告警并更新事件状态 | 判断 firing/resolved -> alert_id 映射到基本事件 -> 更新状态 -> 调用 diagnose | 告警到诊断的关键入口。 |
| DiagnosisEngine.diagnose | 无 | 执行顶层故障求值并输出结果 | 遍历顶层事件求值 -> 比较前后状态 -> 生成触发或恢复诊断 -> 回调输出 | 负责状态迁移判断与诊断结果触发。 |
| generateDiagnosisResult | 无 | 生成标准诊断结果对象 | 基于顶层事件创建 DiagnosisResult -> 收集触发路径和基本事件 | 统一诊断结果格式，便于下游消费。 |

| EvaluateNode | 无 | 递归求值单节点状态 | 基本事件直接读状态 -> 非基本事件按逻辑门调用对应求值函数 | FTA 推理核心入口。 |
| evaluateAND / evaluateOR / evaluateNOT | 无 | 逻辑门求值 | 递归读取子节点状态 -> 按门规则输出当前节点状态 | 实现故障树逻辑门语义。 |
| CollectTriggerPath | 无 | 收集触发路径 | 从顶层真值节点递归向下收集已触发节点 | 解释故障因果链路。 |
| CollectTriggeredBasicEvents | 无 | 收集触发的基本事件 | 从顶层真值节点递归提取 IsBasic 且为真节点 | 输出最小触发证据集合。 |
| NewStateManagerWithTTL | 无 | 创建带 TTL 的状态管理器 | 初始化状态表和默认 TTL -> 启动过期清理协程 | 保障事件状态随时间自动失效。 |
| SetState / GetState | 无 | 写入和读取事件状态 | 写入时记录时间和 TTL -> 读取时自动判定过期 | 维护基本事件与中间状态的一致性。 |

### 主要函数相关变量说明

以下变量是主要函数链路中最关键的运行时数据，按“变量名 + 作用”做简要说明。

| 变量名 | 作用 |
|------|------|
| configPath | 启动参数中的配置文件路径，决定加载哪份故障树。 |
| outputPath | 诊断结果输出文件路径，控制是否落盘。 |
| logger | 全链路日志对象，负责记录启动、路由、诊断和异常信息。 |
| faultTrees | 从配置文件加载出的故障树集合，是多树引擎初始化输入。 |
| diagnosisEngine | 多故障树诊断引擎实例，负责告警路由与并发评估。 |
| alertReceiver | 告警接收器实例，负责从队列接收告警并触发处理函数。 |
| configPath（Loader.configPath） | Loader 内部保存的配置路径，用于读取 JSON 配置。 |
| engines | MultiDiagnosisEngine 内部单树引擎列表。 |
| alertToEngines | alert_id 到目标引擎列表的路由表，支持一条告警命中多棵树。 |
| faultTree | DiagnosisEngine 绑定的当前故障树配置。 |
| eventNodes | event_id 到运行时事件节点的索引表，用于快速建树和求值。 |
| alertToEvent | alert_id 到基本事件 event_id 的映射表，用于把告警映射到树节点。 |
| topEvents | 顶层事件节点列表，是每轮诊断求值的入口集合。 |
| stateManager（DiagnosisEngine） | 事件状态管理器，保存基本事件和中间事件状态。 |
| evaluator | 逻辑门求值器，执行 AND/OR/NOT 规则计算。 |
| callback | 诊断结果回调函数，触发时将结果输出到日志或下游模块。 |
| topEventSource / topEventServiceID / topEventServiceName | 顶层事件上下文缓存，用于恢复告警时保留来源和服务信息。 |
| alertChan | ChannelReceiver 内部告警队列通道。 |
| alertHandler | 告警处理函数，接收器消费后调用该函数进入诊断流程。 |
| bufferSize | 接收器队列容量，用于限制告警堆积。 |
| states（StateManager） | event_id 到状态记录的表，保存事件状态、更新时间和 TTL。 |
| defaultTTL | 事件默认过期时间，超时后状态会自动清理。 |
| stopClean | 状态管理器后台清理协程停止信号。 |

## 配置示例

### 业务层故障树示例（蓄电池异常）

```json
{
  "fault_tree_id": "business_battery_fault",
  "description": "业务层蓄电池和母线电压遥测异常诊断",
  "top_events": [
    {
      "event_id": "TOP-001",
      "name": "CJB-RG-ZD-3",
      "description": "蓄电池、母线电压遥测异常",
      "fault_code": "CJB-RG-ZD-3",
      "gate_type": "OR",
      "children": ["MID-001", "MID-002"]
    }
  ],
  "intermediate_events": [
    {
      "event_id": "MID-001",
      "name": "蓄电池异常",
      "gate_type": "AND",
      "children": ["EVT-001", "EVT-002", "NOT-MID-002"]
    },
    {
      "event_id": "MID-002",
      "name": "AD模块异常",
      "gate_type": "BASIC",
      "children": ["EVT-003"]
    }
  ],
  "basic_events": [
    {
      "event_id": "EVT-001",
      "name": "蓄电池电压异常",
      "alert_id": "BATTERY_VOLTAGE_ALERT"
    },
    {
      "event_id": "EVT-002",
      "name": "母线电压异常",
      "alert_id": "BUS_VOLTAGE_ALERT"
    },
    {
      "event_id": "EVT-003",
      "name": "CPU板电压异常",
      "alert_id": "CPU_VOLTAGE_ALERT"
    }
  ]
}
```

### 微服务层故障树示例（服务性能下降）

```json
{
  "fault_tree_id": "microservice_performance_fault",
  "description": "微服务层性能严重下降诊断",
  "top_events": [
    {
      "event_id": "TOP-MS-001",
      "name": "服务性能严重下降",
      "fault_code": "SVC-PERF-001",
      "gate_type": "AND",
      "children": ["EVT-MS-001", "EVT-MS-002"]
    }
  ],
  "basic_events": [
    {
      "event_id": "EVT-MS-001",
      "name": "P99延迟过高",
      "alert_id": "SERVICE_P99_LATENCY_HIGH"
    },
    {
      "event_id": "EVT-MS-002",
      "name": "错误率过高",
      "alert_id": "SERVICE_ERROR_RATE_HIGH"
    }
  ]
}
```

## 使用方法

### 编译

```bash
./build.sh
```

### 运行

```bash
go run ./cmd/diagnosis -config ./configs/fault_trees_multi_template.json
```

不传 `-config` 时，默认读取 `./configs/fault_trees_multi_template.json`。

### 多故障树 fanout 行为

- 启动时一次性加载配置中的全部故障树。
- 收到告警后按 `alert_id` 路由到所有命中的故障树并行评估。
- 命中多树时会实时逐条回调输出诊断结果。
- 共享 `alert_id`（跨树）是合法配置；同一棵树内 `event_id` 重复会在启动时 fail-fast。

### 配置参数

- `-config`: 故障树配置文件路径
- `-etcd`: etcd集群地址（用于接收告警）
- `-prefix`: 监听的 etcd 键前缀
- `-log-level`: 日志级别（debug/info/warn/error）
- `-output`: 诊断结果输出文件（为空则仅日志输出）

## 与健康监测模块集成

故障诊断模块通过 etcd 接收来自健康监测模块的告警事件：

1. 健康监测模块将告警写入 etcd 的 `/alerts/` 路径
2. 故障诊断模块监听该路径，接收告警事件
3. 将告警 ID 映射为故障树中的基本事件
4. 触发诊断推理流程

## 扩展性

- **新增故障树**: 在 `configs/` 目录添加新的 JSON 配置文件
- **自定义逻辑门**: 在 `engine/evaluator.go` 中扩展门类型
- **多数据源**: 在 `receiver/` 中实现新的接收器

## 技术栈

- Go 1.24.5
- etcd v3（事件通信）
- zap（日志）

## 参考文档

- [故障树分析（FTA）理论](https://en.wikipedia.org/wiki/Fault_tree_analysis)
- [健康监测模块文档](../health-monitor/README.md)
