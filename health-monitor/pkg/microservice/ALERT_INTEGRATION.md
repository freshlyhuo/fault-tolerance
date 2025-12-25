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

## 使用示例

### 1. 初始化组件
```go
import (
    "context"
    "alert"
    "microservice"
)

// 创建告警生成器
generator := alert.NewGenerator()

// 创建Fetcher (需要ECSM API配置)
fetcher := microservice.NewFetcher(ecsmConfig)

// 创建Dispatcher,注入Generator
dispatcher := microservice.NewDispatcher(fetcher, generator)
```

### 2. 执行监控与告警
```go
ctx := context.Background()

// 运行一次监控周期
metrics, err := dispatcher.RunOnce(ctx)
if err != nil {
    log.Printf("监控失败: %v", err)
    return
}

// 告警会自动生成并输出到控制台
// 可以在Generator中扩展输出目标:
// - 消息队列 (Kafka/RabbitMQ)
// - 数据库存储
// - 告警通知 (邮件/短信/钉钉)
// - 故障诊断模块
```

### 3. 周期性监控
```go
import "time"

ticker := time.NewTicker(30 * time.Second)
defer ticker.Stop()

for {
    select {
    case <-ticker.C:
        dispatcher.RunOnce(ctx)
    case <-ctx.Done():
        return
    }
}
```

## 扩展点

### 1. 添加新的阈值检查
在 `alert/threshold.go` 中添加新的检查函数:
```go
func CheckCustomThresholds(metrics *model.CustomMetrics) []*model.AlertEvent {
    var alerts []*model.AlertEvent
    // 实现自定义阈值逻辑
    return alerts
}
```

### 2. 自定义告警输出
在 `Generator.outputAlerts()` 中添加新的输出通道:
```go
// 发送到消息队列
mqClient.Publish("alerts", alerts)

// 存储到数据库
db.SaveAlerts(alerts)

// 发送通知
notifier.Send(alerts)
```

### 3. 趋势分析
可以在Generator中添加趋势分析逻辑:
- 持续高CPU使用率
- 内存泄漏检测
- 磁盘增长速率

### 4. 关联分析
- 同一服务下多个容器同时异常
- 同一节点下多个容器异常
- 时间窗口内的告警聚合

## 测试

运行集成测试:
```bash
cd pkg/microservice
go test -v -run TestMicroserviceAlertIntegration
```

运行单元测试:
```bash
go test -v -run TestNodeThresholdChecking
go test -v -run TestContainerThresholdChecking
go test -v -run TestServiceThresholdChecking
```

## 注意事项

1. **性能考虑**: 大量指标时要注意告警检查的性能
2. **告警风暴**: 实现告警压缩和去重机制
3. **误报处理**: 可以添加时间窗口和连续N次检查
4. **告警恢复**: 需要实现告警恢复机制
5. **配置管理**: 阈值可以从配置文件加载,支持动态调整
