package receiver

import "fault-diagnosis/pkg/models"

// Receiver 告警接收器接口
// 当前实现为内存队列接收器
type Receiver interface {
	// Start 启动接收器
	Start() error

	// Stop 停止接收器
	Stop()

	// SetHandler 设置告警处理函数
	SetHandler(handler AlertHandler)
}

// AlertHandler 告警处理函数类型
type AlertHandler func(*models.AlertEvent)
