package recovery

import "time"

const (
	EventStatusFiring   = "FIRING"
	EventStatusResolved = "RESOLVED"
)

// DiagnosisResult 与故障诊断模块结构一致
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

// DiagnosisStatus 从诊断结果中推断状态（默认视为触发）
func DiagnosisStatus(result DiagnosisResult) string {
	if result.Metadata == nil {
		return EventStatusFiring
	}
	if v, ok := result.Metadata["status"].(string); ok && v != "" {
		return v
	}
	if v, ok := result.Metadata["resolved"].(bool); ok && v {
		return EventStatusResolved
	}
	return EventStatusFiring
}

// DiagnosisTargetID 诊断目标（优先Source，其次metadata.source）
func DiagnosisTargetID(result DiagnosisResult) string {
	if result.Source != "" {
		return result.Source
	}
	if result.Metadata != nil {
		if v, ok := result.Metadata["source"].(string); ok {
			return v
		}
	}
	return ""
}

// RecoveryResult 修复执行结果
// status: SUCCESS | FAILED | TIMEOUT | REJECTED | NO_ACTION
type RecoveryResult struct {
	TargetID   string `json:"target_id"`
	FaultCode  string `json:"fault_code"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	Error      string `json:"error,omitempty"`
}

const (
	ResultSuccess  = "SUCCESS"
	ResultFailed   = "FAILED"
	ResultTimeout  = "TIMEOUT"
	ResultRejected = "REJECTED"
	ResultNoAction = "NO_ACTION"
)

func nowUnix() int64 {
	return time.Now().Unix()
}
