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
│   │   └── receiver.go    # etcd/网络接收
│   └── utils/             # 工具包
│       └── logger.go      # 日志工具
├── configs/               # 配置文件
│   ├── fault_tree_business.json    # 业务层故障树
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
./fault-diagnosis -config ./configs/fault_tree_business.json
```

### 配置参数

- `-config`: 故障树配置文件路径
- `-etcd-endpoints`: etcd集群地址（用于接收告警）
- `-log-level`: 日志级别（debug/info/warn/error）

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
