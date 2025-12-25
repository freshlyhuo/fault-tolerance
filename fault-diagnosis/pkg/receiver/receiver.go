package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"fault-diagnosis/pkg/models"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// EtcdReceiver 基于etcd的告警接收器
// 适用于需要持久化和分布式场景
type EtcdReceiver struct {
	etcdClient   *clientv3.Client
	watchPrefix  string
	logger       *zap.Logger
	alertHandler AlertHandler
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewEtcdReceiver 创建etcd告警接收器
func NewEtcdReceiver(endpoints []string, watchPrefix string, logger *zap.Logger) (*EtcdReceiver, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 创建etcd客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("创建etcd客户端失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	receiver := &EtcdReceiver{
		etcdClient:  cli,
		watchPrefix: watchPrefix,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}

	logger.Info("告警接收器创建成功",
		zap.Strings("endpoints", endpoints),
		zap.String("watch_prefix", watchPrefix),
	)

	return receiver, nil
}

// SetHandler 设置告警处理函数
func (r *EtcdReceiver) SetHandler(handler AlertHandler) {
	r.alertHandler = handler
}

// Start 启动接收器
func (r *EtcdReceiver) Start() error {
	if r.alertHandler == nil {
		return fmt.Errorf("未设置告警处理函数")
	}

	r.logger.Info("启动告警接收器", zap.String("watch_prefix", r.watchPrefix))

	// 启动监听
	go r.watch()

	return nil
}

// Stop 停止接收器
func (r *EtcdReceiver) Stop() {
	r.logger.Info("停止etcd告警接收器")
	r.cancel()
	if r.etcdClient != nil {
		r.etcdClient.Close()
	}
}

// watch 监听etcd变化
func (r *EtcdReceiver) watch() {
	watchChan := r.etcdClient.Watch(r.ctx, r.watchPrefix, clientv3.WithPrefix())

	r.logger.Info("开始监听etcd变化")

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Info("监听已停止")
			return
		case watchResp := <-watchChan:
			if watchResp.Err() != nil {
				r.logger.Error("监听etcd出错", zap.Error(watchResp.Err()))
				continue
			}

			for _, event := range watchResp.Events {
				// 只处理PUT事件（新增或更新）
				if event.Type == clientv3.EventTypePut {
					r.handleEtcdEvent(event)
				}
			}
		}
	}
}

// handleEtcdEvent 处理etcd事件
func (r *EtcdReceiver) handleEtcdEvent(event *clientv3.Event) {
	key := string(event.Kv.Key)
	value := event.Kv.Value

	r.logger.Debug("接收到etcd事件",
		zap.String("key", key),
		zap.Int("value_size", len(value)),
	)

	// 解析告警事件
	var alert models.AlertEvent
	if err := json.Unmarshal(value, &alert); err != nil {
		r.logger.Error("解析告警事件失败",
			zap.String("key", key),
			zap.Error(err),
		)
		return
	}

	// 调用处理函数
	if r.alertHandler != nil {
		r.alertHandler(&alert)
	}
}

// GetAlert 获取指定key的告警（用于测试）
func (r *EtcdReceiver) GetAlert(key string) (*models.AlertEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := r.etcdClient.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("告警不存在: %s", key)
	}

	var alert models.AlertEvent
	if err := json.Unmarshal(resp.Kvs[0].Value, &alert); err != nil {
		return nil, err
	}

	return &alert, nil
}
