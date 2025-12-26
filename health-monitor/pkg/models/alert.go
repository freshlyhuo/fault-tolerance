/* 
告警相关（考虑要不要和alert/generator合并
定义：

AlertEvent：

AlertID

Type

Severity

Source （业务/容器/节点/服务）

Timestamp ？

关联事件（related alerts）？ */
package model

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"     // 信息级别
	SeverityWarning  AlertSeverity = "warning"  // 警告级别
	SeverityCritical AlertSeverity = "critical" // 严重级别
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"   // 告警触发中
	AlertStatusResolved AlertStatus = "resolved" // 告警已恢复
)

// AlertEvent 告警事件
type AlertEvent struct {
	AlertID       string        // 告警唯一标识
	Type          string        // 告警类型
	Status        AlertStatus   // 告警状态（firing/resolved）
	Severity      AlertSeverity // 严重程度
	Source        string        // 告警源（组件名称）
	Message       string        // 告警消息
	Timestamp     int64                  // 时间戳
	FaultCode     string                 // 故障编号
	MetricValue   float64                // 触发告警的指标值
	RelatedAlerts []string               // 关联的其他告警ID
	Metadata      map[string]interface{} // 额外的元数据信息
}

// IsFiring 判断是否为触发告警
func (e *AlertEvent) IsFiring() bool {
	return e.Status == "" || e.Status == AlertStatusFiring
}

// IsResolved 判断是否为恢复告警
func (e *AlertEvent) IsResolved() bool {
	return e.Status == AlertStatusResolved
}