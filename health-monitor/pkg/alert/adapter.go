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
	return map[string]interface{}{
		"AlertID":       alert.AlertID,
		"Type":          alert.Type,
		"Status":        string(alert.Status),   // 告警状态 (firing/resolved)
		"Source":        alert.Source,
		"Message":       alert.Message,
		"Timestamp":     alert.Timestamp,
		"FaultCode":     alert.FaultCode,
		"MetricValue":   alert.MetricValue,
		"RelatedAlerts": alert.RelatedAlerts,
		"Metadata":      alert.Metadata,
	}
}


func ConvertToDiagnosisAlertDirect(alert *model.AlertEvent) interface{} {
	return struct {
		AlertID       string
		Type          string
		Status        string // 告警状态 (firing/resolved)
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
		Source:        alert.Source,
		Message:       alert.Message,
		Timestamp:     alert.Timestamp,
		FaultCode:     alert.FaultCode,
		MetricValue:   alert.MetricValue,
		RelatedAlerts: alert.RelatedAlerts,
		Metadata:      alert.Metadata,
	}
}
