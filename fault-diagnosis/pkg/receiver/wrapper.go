package receiver

import (
	"encoding/json"
	"fault-diagnosis/pkg/models"
)

// ReceiverWrapper 接收器包装器
// 用于适配健康监测模块的告警发送接口
// 将通用的 interface{} 类型转换为 models.AlertEvent
type ReceiverWrapper struct {
	receiver *ChannelReceiver
}

// NewReceiverWrapper 创建接收器包装器
func NewReceiverWrapper(receiver *ChannelReceiver) *ReceiverWrapper {
	return &ReceiverWrapper{
		receiver: receiver,
	}
}

// SendAlert 接收并转换告警
// 这个方法实现了健康监测模块期望的接口
func (w *ReceiverWrapper) SendAlert(alert interface{}) error {
	// 将 interface{} 转换为 models.AlertEvent
	var alertEvent *models.AlertEvent
	
	// 尝试直接类型断言
	if ae, ok := alert.(*models.AlertEvent); ok {
		alertEvent = ae
	} else {
		// 通过 JSON 序列化/反序列化转换
		// 这种方式可以处理结构体字段相同但类型不同的情况
		data, err := json.Marshal(alert)
		if err != nil {
			return err
		}
		
		alertEvent = &models.AlertEvent{}
		if err := json.Unmarshal(data, alertEvent); err != nil {
			return err
		}
	}
	
	// 发送到内部的 ChannelReceiver
	return w.receiver.SendAlert(alertEvent)
}

// GetReceiver 获取内部的 ChannelReceiver
// 用于直接访问接收器的其他方法
func (w *ReceiverWrapper) GetReceiver() *ChannelReceiver {
	return w.receiver
}
