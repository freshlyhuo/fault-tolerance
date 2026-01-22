package models

import "time"

// DiagnosisResult 诊断结果
type DiagnosisResult struct {
	DiagnosisID   string                 // 诊断唯一标识
	FaultTreeID   string                 // 故障树ID
	TopEventID    string                 // 触发的顶层事件ID
	TopEventName  string                 // 顶层事件名称
	FaultCode     string                 // 故障码
	FaultReason   string                 // 故障原因
	Source        string                 // 诊断源（与告警Source一致）
	Timestamp     time.Time              // 诊断时间
	TriggerPath   []string               // 触发路径（事件ID列表）
	BasicEvents   []string               // 触发的基本事件列表
	Metadata      map[string]interface{} // 额外元数据
}

// NewDiagnosisResult 创建诊断结果
func NewDiagnosisResult(faultTreeID, topEventID, topEventName, faultCode, reason string) *DiagnosisResult {
	return &DiagnosisResult{
		DiagnosisID:  generateDiagnosisID(),
		FaultTreeID:  faultTreeID,
		TopEventID:   topEventID,
		TopEventName: topEventName,
		FaultCode:    faultCode,
		FaultReason:  reason,
		Timestamp:    time.Now(),
		TriggerPath:  make([]string, 0),
		BasicEvents:  make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
}

// AddTriggerPath 添加触发路径
func (d *DiagnosisResult) AddTriggerPath(eventID string) {
	d.TriggerPath = append(d.TriggerPath, eventID)
}

// AddBasicEvent 添加基本事件
func (d *DiagnosisResult) AddBasicEvent(eventID string) {
	d.BasicEvents = append(d.BasicEvents, eventID)
}

// generateDiagnosisID 生成诊断ID
func generateDiagnosisID() string {
	return "DIAG-" + time.Now().Format("20060102150405")
}
