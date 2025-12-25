package models

// AlertEvent 告警事件（与健康监测模块兼容）
type AlertEvent struct {
	AlertID       string                 // 告警唯一标识
	Type          string                 // 告警类型
	Severity      string                 // 严重程度 (info/warning/critical)
	Source        string                 // 告警源（组件名称）
	Message       string                 // 告警消息
	Timestamp     int64                  // 时间戳
	FaultCode     string                 // 故障编号
	MetricValue   float64                // 触发告警的指标值
	RelatedAlerts []string               // 关联的其他告警ID
	Metadata      map[string]interface{} // 额外的元数据信息
}

// EventState 事件状态
type EventState int

const (
	StateUnknown EventState = iota // 未知状态
	StateFalse                     // 假（未触发）
	StateTrue                      // 真（已触发）
)

func (s EventState) String() string {
	switch s {
	case StateTrue:
		return "TRUE"
	case StateFalse:
		return "FALSE"
	default:
		return "UNKNOWN"
	}
}

// Bool 转换为布尔值
func (s EventState) Bool() bool {
	return s == StateTrue
}
