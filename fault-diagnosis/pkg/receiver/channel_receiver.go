package receiver

import (
	"context"
	"fmt"
	"sync"

	"fault-diagnosis/pkg/models"
	"go.uber.org/zap"
)

// ChannelReceiver 基于Go Channel的内存消息队列接收器
// 适用于资源受限环境，无需依赖外部组件
type ChannelReceiver struct {
	alertChan    chan *models.AlertEvent
	alertHandler AlertHandler
	logger       *zap.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	bufferSize   int
}

// NewChannelReceiver 创建Channel接收器
// bufferSize: 队列缓冲大小，建议100-1000
func NewChannelReceiver(bufferSize int, logger *zap.Logger) *ChannelReceiver {
	if logger == nil {
		logger = zap.NewNop()
	}

	if bufferSize <= 0 {
		bufferSize = 500 // 默认缓冲500条告警
	}

	ctx, cancel := context.WithCancel(context.Background())

	receiver := &ChannelReceiver{
		alertChan:  make(chan *models.AlertEvent, bufferSize),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		bufferSize: bufferSize,
	}

	logger.Info("Channel告警接收器创建成功",
		zap.Int("buffer_size", bufferSize),
	)

	return receiver
}

// SetHandler 设置告警处理函数
func (r *ChannelReceiver) SetHandler(handler AlertHandler) {
	r.alertHandler = handler
}

// Start 启动接收器
func (r *ChannelReceiver) Start() error {
	if r.alertHandler == nil {
		return fmt.Errorf("未设置告警处理函数")
	}

	r.logger.Info("启动Channel告警接收器")

	// 启动消费协程
	r.wg.Add(1)
	go r.consume()

	return nil
}

// Stop 停止接收器
func (r *ChannelReceiver) Stop() {
	r.logger.Info("停止Channel告警接收器")
	r.cancel()
	close(r.alertChan)
	r.wg.Wait()
	r.logger.Info("Channel告警接收器已停止")
}

// consume 消费告警消息
func (r *ChannelReceiver) consume() {
	defer r.wg.Done()

	r.logger.Info("开始消费告警消息")

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Info("接收到停止信号，停止消费")
			return
		case alert, ok := <-r.alertChan:
			if !ok {
				r.logger.Info("告警通道已关闭")
				return
			}
			r.handleAlert(alert)
		}
	}
}

// handleAlert 处理单个告警
func (r *ChannelReceiver) handleAlert(alert *models.AlertEvent) {
	r.logger.Debug("接收到告警",
		zap.String("alert_id", alert.AlertID),
		zap.String("severity", alert.Severity),
	)

	if r.alertHandler != nil {
		r.alertHandler(alert)
	}
}

// SendAlert 发送告警到队列（供健康监测模块调用）
// 这个方法让健康监测模块可以直接调用，无需依赖etcd
func (r *ChannelReceiver) SendAlert(alert *models.AlertEvent) error {
	select {
	case r.alertChan <- alert:
		r.logger.Debug("告警已加入队列",
			zap.String("alert_id", alert.AlertID),
		)
		return nil
	case <-r.ctx.Done():
		return fmt.Errorf("接收器已停止")
	default:
		// 队列已满
		r.logger.Warn("告警队列已满，丢弃告警",
			zap.String("alert_id", alert.AlertID),
			zap.Int("buffer_size", r.bufferSize),
		)
		return fmt.Errorf("告警队列已满")
	}
}

// GetQueueLength 获取当前队列长度
func (r *ChannelReceiver) GetQueueLength() int {
	return len(r.alertChan)
}

// GetQueueCapacity 获取队列容量
func (r *ChannelReceiver) GetQueueCapacity() int {
	return r.bufferSize
}
