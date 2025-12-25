package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"fault-diagnosis/pkg/models"
	"go.uber.org/zap"
)

// UDPReceiver 基于UDP的轻量级告警接收器
// 适用于资源极度受限的环境，零依赖，最小开销
type UDPReceiver struct {
	conn         *net.UDPConn
	address      string
	alertHandler AlertHandler
	logger       *zap.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewUDPReceiver 创建UDP接收器
// address: 监听地址，如 ":9999"
func NewUDPReceiver(address string, logger *zap.Logger) *UDPReceiver {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	receiver := &UDPReceiver{
		address: address,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	logger.Info("UDP告警接收器创建成功", zap.String("address", address))

	return receiver
}

// SetHandler 设置告警处理函数
func (r *UDPReceiver) SetHandler(handler AlertHandler) {
	r.alertHandler = handler
}

// Start 启动接收器
func (r *UDPReceiver) Start() error {
	if r.alertHandler == nil {
		return fmt.Errorf("未设置告警处理函数")
	}

	// 解析UDP地址
	addr, err := net.ResolveUDPAddr("udp", r.address)
	if err != nil {
		return fmt.Errorf("解析UDP地址失败: %w", err)
	}

	// 监听UDP端口
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("监听UDP端口失败: %w", err)
	}

	r.conn = conn
	r.logger.Info("UDP告警接收器已启动", zap.String("address", r.address))

	// 启动接收协程
	r.wg.Add(1)
	go r.receive()

	return nil
}

// Stop 停止接收器
func (r *UDPReceiver) Stop() {
	r.logger.Info("停止UDP告警接收器")
	r.cancel()
	if r.conn != nil {
		r.conn.Close()
	}
	r.wg.Wait()
	r.logger.Info("UDP告警接收器已停止")
}

// receive 接收UDP数据包
func (r *UDPReceiver) receive() {
	defer r.wg.Done()

	buffer := make([]byte, 4096) // 4KB缓冲区

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			n, addr, err := r.conn.ReadFromUDP(buffer)
			if err != nil {
				if r.ctx.Err() != nil {
					// 已停止
					return
				}
				r.logger.Error("读取UDP数据失败", zap.Error(err))
				continue
			}

			r.logger.Debug("接收到UDP数据包",
				zap.String("from", addr.String()),
				zap.Int("size", n),
			)

			// 解析告警
			var alert models.AlertEvent
			if err := json.Unmarshal(buffer[:n], &alert); err != nil {
				r.logger.Error("解析告警失败",
					zap.Error(err),
					zap.String("data", string(buffer[:n])),
				)
				continue
			}

			// 处理告警
			if r.alertHandler != nil {
				r.alertHandler(&alert)
			}
		}
	}
}

// SendAlert 发送告警到UDP接收器（供健康监测模块调用）
func SendAlertViaUDP(alert *models.AlertEvent, targetAddr string) error {
	// 序列化告警
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("序列化告警失败: %w", err)
	}

	// 解析目标地址
	addr, err := net.ResolveUDPAddr("udp", targetAddr)
	if err != nil {
		return fmt.Errorf("解析UDP地址失败: %w", err)
	}

	// 发送UDP数据包
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("连接UDP失败: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("发送UDP数据失败: %w", err)
	}

	return nil
}
