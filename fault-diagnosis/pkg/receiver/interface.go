package receiver

import "fault-diagnosis/pkg/models"

// Receiver 告警接收器接口
// 支持多种实现：内存队列、etcd、Kafka等
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
