# 故障诊断模块架构文档

## 架构概述

故障诊断模块采用事件驱动的FTA（故障树分析）架构，通过自底向上的逻辑推理，将基本告警事件转化为系统级故障诊断。

## 核心组件

### 1. 数据模型层 (pkg/models/)

#### event.go - 事件定义
- `AlertEvent`: 告警事件结构，与健康监测模块兼容
- `EventState`: 事件状态枚举（TRUE/FALSE/UNKNOWN）

#### fault_tree.go - 故障树结构
- `FaultTree`: 故障树配置结构
- `Event`: 顶层/中间事件节点
- `BasicEvent`: 基本事件（叶子节点）
- `EventNode`: 运行时事件节点，用于求值
- `GateType`: 逻辑门类型（AND/OR/NOT/BASIC）

#### diagnosis.go - 诊断结果
- `DiagnosisResult`: 诊断结果结构
- 包含故障码、触发路径、基本事件列表等信息

### 2. 推理引擎层 (pkg/engine/)

#### state.go - 状态管理器
```
StateManager
├── states: map[eventID]EventState
├── SetState()    - 设置事件状态
├── GetState()    - 获取事件状态
└── GetTrueEvents() - 获取所有为真的事件
```

#### evaluator.go - 求值器
```
Evaluator
├── EvaluateNode()   - 求值单个节点（递归）
├── evaluateAND()    - 与门求值
├── evaluateOR()     - 或门求值
├── evaluateNOT()    - 非门求值
└── EvaluateTree()   - 求值整个故障树
```

求值算法：
- **与门（AND）**: 所有子节点为TRUE时，输出TRUE
- **或门（OR）**: 任一子节点为TRUE时，输出TRUE
- **非门（NOT）**: 子节点为FALSE时，输出TRUE

#### engine.go - 诊断引擎
```
DiagnosisEngine
├── faultTree      - 故障树配置
├── topEvents      - 顶层事件节点列表
├── eventNodes     - 事件ID到节点的映射
├── alertToEvent   - 告警ID到基本事件的映射
├── stateManager   - 状态管理器
├── evaluator      - 求值器
├── ProcessAlert() - 处理告警事件
└── diagnose()     - 执行诊断推理
```

诊断流程：
1. 接收告警事件
2. 映射到基本事件ID
3. 更新基本事件状态为TRUE
4. 触发自底向上的故障树求值
5. 检查顶层事件是否被触发
6. 生成诊断结果并调用回调函数

### 3. 配置管理层 (pkg/config/)

#### loader.go - 配置加载器
- `LoadFaultTree()`: 从JSON文件加载故障树
- `validateFaultTree()`: 验证配置完整性

### 4. 接收器层 (pkg/receiver/)

#### receiver.go - 告警接收器
```
AlertReceiver
├── etcdClient     - etcd客户端
├── watchPrefix    - 监听键前缀
├── alertHandler   - 告警处理函数
├── Start()        - 启动监听
└── watch()        - 监听etcd变化
```

通过etcd的Watch机制实时接收健康监测模块发送的告警事件。

### 5. 工具层 (pkg/utils/)

#### logger.go - 日志工具
使用zap提供高性能结构化日志。

## 数据流

```
健康监测模块
    |
    | (告警事件)
    ↓
etcd (/alerts/)
    |
    | (Watch)
    ↓
AlertReceiver
    |
    | AlertEvent
    ↓
DiagnosisEngine.ProcessAlert()
    |
    | 1. 映射告警ID → 基本事件ID
    ↓
StateManager.SetState(eventID, TRUE)
    |
    | 2. 更新事件状态
    ↓
Evaluator.EvaluateTree()
    |
    | 3. 自底向上求值
    | - 基本事件
    | - 中间事件（逻辑门）
    | - 顶层事件
    ↓
DiagnosisResult
    |
    | 4. 生成诊断报告
    ↓
Callback / 故障修复模块
```

## 故障树求值算法

### 递归求值过程

```python
def EvaluateNode(node):
    if node.IsBasic:
        # 基本事件：直接从状态管理器获取
        return StateManager.GetState(node.EventID)
    
    # 中间/顶层事件：根据逻辑门求值
    if node.GateType == AND:
        return all(EvaluateNode(child) for child in node.Children)
    
    elif node.GateType == OR:
        return any(EvaluateNode(child) for child in node.Children)
    
    elif node.GateType == NOT:
        return not EvaluateNode(node.Children[0])
    
    return False
```

### 示例：蓄电池异常诊断

故障树结构：
```
TOP-001 (OR门)
├── MID-001 (AND门) - 蓄电池异常
│   ├── EVT-001 (基本事件) - 蓄电池电压异常
│   ├── EVT-002 (基本事件) - 母线电压异常
│   └── NOT(MID-002) - 排除AD模块异常
└── MID-002 (BASIC门) - AD模块异常
    └── EVT-003 (基本事件) - CPU板电压异常
```

求值过程：
1. 接收 EVT-001, EVT-002 告警
2. 设置 EVT-001 = TRUE, EVT-002 = TRUE
3. 求值 MID-002: EVT-003 = FALSE → MID-002 = FALSE
4. 求值 NOT(MID-002): FALSE → TRUE
5. 求值 MID-001: EVT-001 ∧ EVT-002 ∧ NOT(MID-002) = TRUE ∧ TRUE ∧ TRUE = TRUE
6. 求值 TOP-001: MID-001 ∨ MID-002 = TRUE ∨ FALSE = TRUE
7. 触发顶层故障 "CJB-RG-ZD-3"

## 配置格式

### 故障树JSON结构

```json
{
  "fault_tree_id": "唯一标识",
  "description": "描述",
  "top_events": [
    {
      "event_id": "TOP-001",
      "name": "事件名称",
      "fault_code": "故障码",
      "gate_type": "OR|AND|NOT",
      "children": ["子事件ID列表"]
    }
  ],
  "intermediate_events": [
    {
      "event_id": "MID-001",
      "name": "中间事件名称",
      "gate_type": "OR|AND|NOT|BASIC",
      "children": ["子事件ID列表"]
    }
  ],
  "basic_events": [
    {
      "event_id": "EVT-001",
      "name": "基本事件名称",
      "alert_id": "对应的告警ID"
    }
  ]
}
```

### NOT门的表示

在 `children` 数组中使用 `"NOT-<eventID>"` 前缀表示对子事件的非操作：

```json
{
  "event_id": "MID-001",
  "gate_type": "AND",
  "children": ["EVT-001", "NOT-MID-002"]
}
```

引擎会自动创建一个NOT门节点包装 `MID-002`。

## 扩展点

### 1. 新增逻辑门类型

在 `pkg/models/fault_tree.go` 中添加新的 `GateType` 常量，在 `pkg/engine/evaluator.go` 中实现对应的求值逻辑。

示例：添加异或门（XOR）
```go
// models/fault_tree.go
const GateXOR GateType = "XOR"

// engine/evaluator.go
case models.GateXOR:
    return e.evaluateXOR(node)

func (e *Evaluator) evaluateXOR(node *models.EventNode) models.EventState {
    trueCount := 0
    for _, child := range node.Children {
        if e.EvaluateNode(child) == models.StateTrue {
            trueCount++
        }
    }
    if trueCount == 1 {
        node.SetState(models.StateTrue)
        return models.StateTrue
    }
    node.SetState(models.StateFalse)
    return models.StateFalse
}
```

### 2. 多数据源接收

在 `pkg/receiver/` 中实现新的接收器：

```go
type KafkaReceiver struct {
    consumer KafkaConsumer
    handler  AlertHandler
}

func (r *KafkaReceiver) Start() error {
    // 从Kafka消费告警消息
}
```

### 3. 诊断结果输出

在主程序的回调函数中实现多种输出方式：
- 写入文件
- 发送到etcd
- 推送到消息队列
- 调用HTTP API

### 4. 时间窗口和去重

在 `DiagnosisEngine` 中添加时间窗口逻辑，避免短时间内重复触发相同故障：

```go
type DiagnosisEngine struct {
    // ...
    recentDiagnoses map[string]time.Time // faultCode -> lastDiagnosisTime
    cooldownPeriod  time.Duration        // 冷却期
}

func (e *DiagnosisEngine) shouldTrigger(faultCode string) bool {
    if lastTime, ok := e.recentDiagnoses[faultCode]; ok {
        if time.Since(lastTime) < e.cooldownPeriod {
            return false // 在冷却期内，不触发
        }
    }
    return true
}
```

## 性能优化

### 1. 增量求值

当前实现在每次基本事件状态变化时，对所有顶层事件进行求值。对于大型故障树，可以优化为：
- 仅对受影响的子树进行求值
- 使用缓存避免重复计算

### 2. 并发处理

为每个故障树创建独立的 `DiagnosisEngine` 实例，支持并发处理多个故障树。

### 3. 状态持久化

将 `StateManager` 的状态持久化到etcd或数据库，支持服务重启后恢复状态。

## 测试策略

### 单元测试
- `engine/evaluator_test.go`: 测试各种逻辑门的求值
- `engine/state_test.go`: 测试状态管理器
- `config/loader_test.go`: 测试配置加载和验证

### 集成测试
- `test/diagnosis_test.go`: 测试完整的诊断流程
- 模拟不同场景的告警序列
- 验证诊断结果的正确性

### 性能测试
- 测试大规模故障树的求值性能
- 测试高频告警场景下的响应时间

## 监控指标

建议收集以下指标：
- 接收的告警事件数量
- 触发的诊断次数（按故障码分类）
- 求值延迟
- 内存使用情况
