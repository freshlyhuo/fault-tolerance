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

## 核心组件

### Dispatcher (业务层分发器)
```go
type Dispatcher struct {
    generator *alert.Generator  // 告警生成器
}

func (d *Dispatcher) HandleBusinessMetrics(ctx context.Context, bm *model.BusinessMetrics) {
    // 直接转发给 Generator
    d.generator.ProcessBusinessMetrics(ctx, bm)
    
    // 其他处理：健康分计算、持久化、可视化等
}
```

### Generator (告警生成器)
```go
type Generator struct {
    // 可扩展配置
}

func (g *Generator) ProcessBusinessMetrics(ctx context.Context, bm *model.BusinessMetrics) {
    var alerts []*model.AlertEvent
    
    // 根据组件类型调用对应的阈值检查
    switch bm.ComponentType {
    case 0x03: // 供电服务
        alerts = CheckPowerThresholds(bm.Data.(*model.PowerMetrics))
    case 0x06: // 热控服务
        alerts = CheckThermalThresholds(bm.Data.(*model.ThermalMetrics))
    // ... 其他组件
    }
    
    // 直接输出告警
    if len(alerts) > 0 {
        g.outputAlerts(alerts)
    }
}

func (g *Generator) outputAlerts(alerts []*model.AlertEvent) {
    // 告警去重
    // 按严重程度分类
    // 输出到控制台/日志/MQ/数据库等
}
```

### Threshold (阈值检查函数)
```go
// 每个组件有独立的阈值检查函数
func CheckPowerThresholds(metrics *model.PowerMetrics) []*model.AlertEvent {
    var alerts []*model.AlertEvent
    
    // 检查各项指标
    if metrics.BatteryVoltage < 21.0 || metrics.BatteryVoltage > 29.4 {
        alerts = append(alerts, &model.AlertEvent{
            AlertID:     "PWR-BAT-xxx",
            Type:        "VoltageAbnormal",
            Severity:    model.SeverityCritical,
            Source:      "BatteryVoltage",
            Message:     "蓄电池电压异常: 19.00V (正常[21,29.4]V)",
            FaultCode:   "CJB-RG-ZD-3",
            MetricValue: 19.0,
        })
    }
    
    return alerts
}
```

## 使用示例

### 完整流程
```go
package main

import (
    "context"
    "business"
)

func main() {
    // 1. 创建组件
    dispatcher := business.NewDispatcher()
    receiver := business.NewReceiver(dispatcher)
    
    ctx := context.Background()
    
    // 2. 接收原始报文
    packet := getPacketFromNetwork() // 从网络/总线接收
    
    // 3. 解析报文
    metrics, err := receiver.ParsePacket(packet)
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. 处理指标（自动触发告警检查和输出）
    dispatcher.HandleBusinessMetrics(ctx, metrics)
    
    // Generator 会自动：
    // - 调用对应的阈值检查函数
    // - 生成告警事件
    // - 输出告警到控制台/日志
}
```

### 告警输出示例

当检测到异常时，Generator 会输出：

```
========== 告警事件 ==========

【严重告警】共 2 个:
  [PWR-BAT-1732867200] VoltageAbnormal
    故障码: CJB-RG-ZD-3
    来源: BatteryVoltage
    消息: 蓄电池电压异常: 19.00V (正常[21,29.4]V)
    指标值: 19.00
    时间戳: 1732867200

  [PWR-CPU-1732867200] VoltageAbnormal
    故障码: CJB-RG-ZD-3
    来源: CPUVoltage
    消息: CPU板电压异常: 2.80V (正常[3.1,3.5]V)
    指标值: 2.80
    时间戳: 1732867200

【警告告警】共 2 个:
  [PWR-12V-1732867200] VoltageAbnormal
    故障码: CJB-RG-ZD-1
    来源: PowerModule12V
    消息: 12V功率模块电压异常: 11.00V (正常约13V)
    指标值: 11.00
    时间戳: 1732867200

  [PWR-LOAD-1732867200] CurrentAbnormal
    故障码: CJB-O2-CS-1
    来源: LoadCurrent
    消息: 负载电流异常: 6.00A (正常[0.5,5]A)
    指标值: 6.00
    时间戳: 1732867200

==============================
```

## 阈值配置

### 供电服务阈值（CheckPowerThresholds）
| 指标 | 正常范围 | 告警级别 | 故障编号 |
|------|----------|----------|----------|
| 12V功率模块电压 | 12.5-13.5V | Warning | CJB-RG-ZD-1 |
| 蓄电池电压 | [21, 29.4]V | Critical | CJB-RG-ZD-3 |
| CPU板电压 | [3.1, 3.5]V | Critical | CJB-RG-ZD-3 |
| 负载电流 | [0.5, 5]A | Warning | CJB-O2-CS-1 |

### 热控服务阈值（CheckThermalThresholds）
| 指标 | 正常范围 | 告警级别 | 故障编号 |
|------|----------|----------|----------|
| cjb热控温度1-10 | [-20, 50]℃ | Warning | CJB-RG-ZD-4 |
| 蓄电池温度1 | [0, 45]℃ | Warning | CJB-RG-ZD-4 |

### 通信服务阈值（CheckCommThresholds）
| 指标 | 检查条件 | 告警级别 | 故障编号 |
|------|----------|----------|----------|
| CAN通信状态 | 0=无应答 | Critical | CJB-RG-ZD-2 |
| 串口通信状态 | 0=无遥测 | Warning | CJB-O2-CS-1 |

### 姿态控制机构阈值（CheckActuatorThresholds）
| 指标 | 正常范围 | 告警级别 | 故障编号 |
|------|----------|----------|----------|
| X/Y/Z轴动量轮转速 | 90-110转 | Warning | CJB-O2-CS-16 |

## 扩展指南

### 添加新组件的阈值检查

1. **在 threshold.go 中添加检查函数**:
```go
func CheckNewComponentThresholds(metrics *model.NewComponentMetrics) []*model.AlertEvent {
    var alerts []*model.AlertEvent
    
    // 添加阈值检查逻辑
    if metrics.SomeValue > threshold {
        alerts = append(alerts, &model.AlertEvent{
            // 填充告警信息
        })
    }
    
    return alerts
}
```

2. **在 generator.go 中添加处理分支**:
```go
case 0x12: // CompNewComponent
    if newData, ok := bm.Data.(*model.NewComponentMetrics); ok {
        alerts = CheckNewComponentThresholds(newData)
    }
```

### 自定义告警输出

修改 `generator.go` 中的 `outputAlerts` 函数：

```go
func (g *Generator) outputAlerts(alerts []*model.AlertEvent) {
    // 1. 输出到控制台（已实现）
    g.printToConsole(alerts)
    
    // 2. 发送到消息队列
    g.sendToMQ(alerts)
    
    // 3. 写入数据库
    g.saveToDatabase(alerts)
    
    // 4. 发送到可视化平台
    g.sendToVisualization(alerts)
    
    // 5. 触发告警通知
    g.sendNotifications(alerts)
}
```

## 测试

运行集成测试：
```bash
cd pkg/business
go test -v -run TestDispatcherToGeneratorFlow
```

测试覆盖：
- ✅ 供电服务异常检测
- ✅ 热控服务异常检测
- ✅ 通信服务异常检测
- ✅ 姿态控制机构异常检测
- ✅ 正常数据（无告警）

## 关键特点

1. **单向流动**: Dispatcher → Generator → 输出，Generator 不返回给 Dispatcher
2. **自动触发**: 只需调用 `HandleBusinessMetrics()`，告警自动生成和输出
3. **可扩展**: 易于添加新的组件类型和阈值检查
4. **分类输出**: 按严重程度（Critical/Warning/Info）分类展示
5. **去重机制**: 自动去除重复告警

## 架构优势

- **解耦**: Dispatcher 只负责转发，告警逻辑完全在 Generator 中
- **职责清晰**: 
  - Receiver: 报文解析
  - Dispatcher: 指标分发和其他业务处理
  - Generator: 告警生成和输出
  - Threshold: 阈值检查逻辑
- **易于测试**: 每个组件可独立测试
- **灵活输出**: Generator 可以输出到多个目标（控制台、MQ、数据库等）
