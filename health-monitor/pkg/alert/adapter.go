package alert

import (
	"health-monitor/pkg/models"
)

// DiagnosisReceiver 故障诊断接收器接口
// 这个接口定义了故障诊断模块需要实现的方法
// 避免直接依赖 fault-diagnosis 模块，实现松耦合
type DiagnosisReceiver interface {
	SendAlert(alert interface{}) error
}

// AlertAdapter 告警适配器
// 用于将健康监测的告警事件转换为故障诊断模块可接收的格式
type AlertAdapter struct {
	receiver DiagnosisReceiver // 故障诊断接收器
}

// NewAlertAdapter 创建告警适配器
func NewAlertAdapter(receiver DiagnosisReceiver) *AlertAdapter {
	return &AlertAdapter{
		receiver: receiver,
	}
}

// SendAlert 发送告警到故障诊断模块
func (a *AlertAdapter) SendAlert(alert *model.AlertEvent) error {
	// 转换为故障诊断模块的格式
	diagnosisAlert := ConvertToDiagnosisAlert(alert)
	
	// 发送到故障诊断接收器
	return a.receiver.SendAlert(diagnosisAlert)
}

// SendAlerts 批量发送告警
func (a *AlertAdapter) SendAlerts(alerts []*model.AlertEvent) error {
	for _, alert := range alerts {
		if err := a.SendAlert(alert); err != nil {
			return err
		}
	}
	return nil
}

// ConvertToDiagnosisAlert 转换告警事件格式
// 从健康监测的 model.AlertEvent 转换为故障诊断的格式
func ConvertToDiagnosisAlert(alert *model.AlertEvent) map[string]interface{} {
	// 由于不直接依赖 fault-diagnosis 模块，这里返回通用的 map 格式
	// 故障诊断模块的接收器会将其反序列化为 models.AlertEvent
	// 注意：map 的键必须与故障诊断 models.AlertEvent 的字段名完全匹配（首字母大写）
	return map[string]interface{}{
		"AlertID":       alert.AlertID,
		"Type":          alert.Type,
		"Status":        string(alert.Status),   // 告警状态 (firing/resolved)
		"Severity":      string(alert.Severity), // 转换枚举类型为字符串
		"Source":        alert.Source,
		"Message":       alert.Message,
		"Timestamp":     alert.Timestamp,
		"FaultCode":     alert.FaultCode,
		"MetricValue":   alert.MetricValue,
		"RelatedAlerts": alert.RelatedAlerts,
		"Metadata":      alert.Metadata,
	}
}

// ConvertToDiagnosisAlertDirect 直接转换为目标结构体
// 如果在同一进程中使用，可以直接创建目标类型的实例
// 需要在编译时导入 fault-diagnosis/pkg/models 包
func ConvertToDiagnosisAlertDirect(alert *model.AlertEvent) interface{} {
	// 创建一个与 fault-diagnosis/pkg/models.AlertEvent 兼容的结构
	// 由于字段完全相同，只是类型略有差异，这里返回通用结构
	return struct {
		AlertID       string
		Type          string
		Status        string // 告警状态 (firing/resolved)
		Severity      string // 注意：故障诊断模块使用 string，而不是 AlertSeverity
		Source        string
		Message       string
		Timestamp     int64
		FaultCode     string
		MetricValue   float64
		RelatedAlerts []string
		Metadata      map[string]interface{}
	}{
		AlertID:       alert.AlertID,
		Type:          alert.Type,
		Status:        string(alert.Status),
		Severity:      string(alert.Severity),
		Source:        alert.Source,
		Message:       alert.Message,
		Timestamp:     alert.Timestamp,
		FaultCode:     alert.FaultCode,
		MetricValue:   alert.MetricValue,
		RelatedAlerts: alert.RelatedAlerts,
		Metadata:      alert.Metadata,
	}
}
