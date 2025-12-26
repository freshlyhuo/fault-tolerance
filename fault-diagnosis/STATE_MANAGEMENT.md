# 故障诊断 - 状态管理与恢复机制

## 概述

故障诊断系统支持完善的状态管理和自动恢复机制，解决基本事件状态"一直为真"的问题。

## 核心机制

### 1. 恢复告警（推荐）⭐

**原理：** 健康监测发送两种类型的告警
- `firing` - 故障触发告警（将基本事件置为真）
- `resolved` - 故障恢复告警（将基本事件置为假）

**优点：**
- ✅ 准确反映真实状态
- ✅ 实时响应
- ✅ 最符合实际场景

**使用方法：**

```go
// 1. 触发告警
alert := &models.AlertEvent{
    AlertID:  "CONTAINER_CPU_HIGH",
    Status:   models.AlertStatusFiring,  // 或留空，默认为 firing
    Severity: "critical",
    Message:  "CPU使用率过高: 95%",
}
engine.ProcessAlert(alert)

// 2. 恢复告警（当指标恢复正常时）
recoveryAlert := &models.AlertEvent{
    AlertID:  "CONTAINER_CPU_HIGH",
    Status:   models.AlertStatusResolved, // 标记为已恢复
    Severity: "info",
    Message:  "CPU使用率已恢复正常: 45%",
}
engine.ProcessAlert(recoveryAlert)
```

### 2. TTL 自动过期（兜底）

**原理：** 每个基本事件有生存时间（Time To Live），超时自动恢复为假

**优点：**
- ✅ 防止恢复告警丢失导致状态僵死
- ✅ 自动清理
- ✅ 无需额外操作

**配置方法：**

```go
// 方式1: 使用默认TTL (5分钟)
stateManager := NewStateManager()

// 方式2: 自定义默认TTL
stateManager := NewStateManagerWithTTL(10 * time.Minute)

// 方式3: 为特定事件设置TTL
stateManager.SetStateWithTTL("EVT-001", models.StateTrue, 2 * time.Minute)

// 方式4: 设置永久状态（不过期）
stateManager.SetStatePermanent("EVT-001", models.StateTrue)
```

**自动清理：**
- 后台协程每 30 秒检查一次过期事件
- 自动删除过期状态记录

### 3. 主动查询（可选）

**原理：** 诊断引擎主动向健康监测查询最新状态

**适用场景：**
- 关键故障需要二次确认
- 高可靠性要求
- 人工介入场景

**实现示例：**

```go
// 定义查询接口
type HealthMonitorQuery interface {
    GetCurrentMetricValue(alertID string) (float64, error)
    IsAlertStillActive(alertID string) (bool, error)
}

// 在诊断引擎中使用
func (e *DiagnosisEngine) VerifyBasicEvent(eventID string, query HealthMonitorQuery) bool {
    basicEvent := e.getBasicEvent(eventID)
    isActive, err := query.IsAlertStillActive(basicEvent.AlertID)
    if err != nil {
        return false
    }
    
    // 更新状态
    if isActive {
        e.stateManager.SetState(eventID, models.StateTrue)
    } else {
        e.stateManager.SetState(eventID, models.StateFalse)
    }
    
    return isActive
}
```

## 完整工作流程

```
健康监测模块                    故障诊断模块
     │                              │
     │  1. 检测到CPU超过90%         │
     ├─────── firing 告警 ────────→ │ 
     │                              │ SetState(EVT, TRUE, TTL=5min)
     │                              │ → 执行诊断
     │                              │ → 触发故障回调
     │                              │
     │  2. CPU降至45%               │
     ├────── resolved 告警 ───────→ │
     │                              │ SetState(EVT, FALSE)
     │                              │ → 不再触发诊断
     │                              │
     │  3. 如果恢复告警丢失          │
     │                              │ TTL超时（5分钟后）
     │                              │ → 自动清理过期状态
     │                              │ → 状态恢复为 FALSE
```

## 健康监测模块集成

### 在告警生成器中支持恢复告警

```go
// health-monitor/pkg/alert/generator.go

func (g *Generator) CheckThreshold(metric float64, threshold float64, alertID string) *model.AlertEvent {
    // 当前是否超过阈值
    isFiring := metric > threshold
    
    // 检查上一次状态
    lastState := g.getLastAlertState(alertID)
    
    if isFiring && lastState != "firing" {
        // 触发告警
        return &model.AlertEvent{
            AlertID:  alertID,
            Status:   model.AlertStatusFiring,
            Severity: model.SeverityCritical,
            Message:  fmt.Sprintf("指标超过阈值: %.2f > %.2f", metric, threshold),
            MetricValue: metric,
        }
    } else if !isFiring && lastState == "firing" {
        // 恢复告警
        return &model.AlertEvent{
            AlertID:  alertID,
            Status:   model.AlertStatusResolved,
            Severity: model.SeverityInfo,
            Message:  fmt.Sprintf("指标已恢复正常: %.2f < %.2f", metric, threshold),
            MetricValue: metric,
        }
    }
    
    return nil // 状态无变化
}
```

## 最佳实践

### 推荐方案：混合机制

```go
// 1. 创建带TTL的状态管理器
stateManager := NewStateManagerWithTTL(5 * time.Minute)

// 2. 优先使用恢复告警
// 健康监测主动发送恢复告警

// 3. TTL作为兜底
// 防止恢复告警丢失

// 4. 关键场景使用主动查询
// 人工干预前二次确认
```

### TTL 配置建议

| 场景 | 推荐TTL | 说明 |
|------|---------|------|
| 微服务性能 | 3-5分钟 | 性能问题通常短暂 |
| 硬件故障 | 10-15分钟 | 硬件问题持续时间较长 |
| 网络抖动 | 1-2分钟 | 网络问题恢复快 |
| 关键业务 | 不过期 | 需人工确认恢复 |

### 不同场景的最佳实践

#### 场景1: 嵌入式系统（资源受限）
```go
// 使用恢复告警 + 短TTL
stateManager := NewStateManagerWithTTL(2 * time.Minute)
```

#### 场景2: 云原生微服务
```go
// 使用恢复告警 + 中等TTL + 自动清理
stateManager := NewStateManagerWithTTL(5 * time.Minute)
```

#### 场景3: 高可靠性系统
```go
// 使用恢复告警 + 主动查询
stateManager := NewStateManagerWithTTL(10 * time.Minute)
// 关键故障时主动查询二次确认
```

## 状态查询 API

```go
// 获取当前状态（自动检查过期）
state := stateManager.GetState("EVT-001")

// 获取状态和时间戳
state, timestamp := stateManager.GetStateWithTimestamp("EVT-001")

// 获取所有为真的事件
trueEvents := stateManager.GetTrueEvents()

// 手动触发清理
stateManager.cleanExpired()

// 停止状态管理器（停止自动清理协程）
defer stateManager.Stop()
```

## 监控和调试

### 查看状态管理器信息

```go
fmt.Println(stateManager.String())
```

输出示例：
```
StateManager{DefaultTTL: 5m0s, Events: 3
  EVT-001: TRUE (TTL: 5m0s, Updated: 14:30:15)
  EVT-002: FALSE (TTL: 5m0s, Updated: 14:28:42)
  EVT-003: TRUE (TTL: 2m0s, Updated: 14:32:01) [EXPIRED]
}
```

### 日志示例

```
[INFO] 接收到告警事件 alert_id=CONTAINER_CPU_HIGH status=firing
[INFO] 基本事件状态已更新 event_id=EVT-MS-003 state=TRUE
[INFO] 检测到系统级故障 fault_code=CONTAINER-RESOURCE-001

[INFO] 接收到告警事件 alert_id=CONTAINER_CPU_HIGH status=resolved
[INFO] 基本事件已恢复 event_id=EVT-MS-003 state=FALSE

[INFO] 清理了 1 个过期事件状态
```

## 测试示例

见 [cmd/demo/main.go](cmd/demo/main.go) 中的完整测试场景。

## 常见问题

**Q: 如果健康监测不支持恢复告警怎么办？**
A: 使用较短的 TTL（如 2-3 分钟），让状态自动过期。

**Q: TTL 过期后会重新触发诊断吗？**
A: 不会。过期后状态变为 FALSE，只有新的 firing 告警才会触发诊断。

**Q: 恢复告警丢失会有什么影响？**
A: TTL 机制会在超时后自动清理状态，影响有限。

**Q: 可以手动重置某个事件吗？**
A: 可以，使用 `stateManager.ResetState("EVT-001")` 或 `engine.ResetBasicEvent("EVT-001")`。

**Q: 如何禁用 TTL？**
A: 使用 `SetStatePermanent()` 设置永久状态，或将 TTL 设为 0。
