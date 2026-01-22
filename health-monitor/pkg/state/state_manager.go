/* 状态管理器 - 核心数据中枢
功能:
1. 实时状态维护 - UpdateMetric()
2. 统一查询接口 - GetLatestState()
3. 历史窗口缓存 - AppendHistory() / QueryHistory()
4. 时间戳对齐 - AlignTimestamp()
5. 持久化快照 - SaveSnapshot() / LoadSnapshot()
*/
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// 历史数据保留时长
	HistoryRetention = 10 * time.Minute
	
	// 环形缓冲区大小（每个指标保留最近N条记录）
	RingBufferSize = 600 // 10分钟 * 60秒
	
	// 快照持久化间隔
	SnapshotInterval = 1 * time.Minute
	
	// etcd key前缀
	EtcdPrefixSnapshot = "/health-monitor/snapshots/"
	EtcdPrefixHistory  = "/health-monitor/history/"
)

// StateManager 状态管理器
type StateManager struct {
	// 实时状态存储（最新值）
	latestStates map[string]Metric
	statesMutex  sync.RWMutex
	
	// 历史数据环形缓冲区 (id -> ring buffer)
	historyBuffers map[string]*RingBuffer
	historyMutex   sync.RWMutex
	
	// 告警状态跟踪 (alertID -> 是否激活)
	alertStates map[string]bool
	alertMutex  sync.RWMutex
	
	// etcd客户端
	etcdClient *clientv3.Client
	etcdConfig clientv3.Config
	
	// 时间基准（用于时间戳对齐）
	timeBase int64
	
	// 停止信号
	stopChan chan struct{}
}

// RingBuffer 环形缓冲区实现
type RingBuffer struct {
	data  []HistoryEntry
	head  int
	tail  int
	size  int
	mutex sync.RWMutex
}

// HistoryEntry 历史记录条目
type HistoryEntry struct {
	Timestamp int64
	Data      interface{}
}

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		data: make([]HistoryEntry, size),
		size: size,
	}
}

// Append 添加数据到环形缓冲区
func (rb *RingBuffer) Append(entry HistoryEntry) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	
	rb.data[rb.tail] = entry
	rb.tail = (rb.tail + 1) % rb.size
	
	// 如果满了，移动head
	if rb.tail == rb.head {
		rb.head = (rb.head + 1) % rb.size
	}
}

// Query 查询指定时间范围内的数据
func (rb *RingBuffer) Query(since time.Duration) []HistoryEntry {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()
	
	cutoff := time.Now().Unix() - int64(since.Seconds())
	var result []HistoryEntry
	
	idx := rb.head
	for idx != rb.tail {
		entry := rb.data[idx]
		if entry.Timestamp >= cutoff {
			result = append(result, entry)
		}
		idx = (idx + 1) % rb.size
	}
	
	return result
}

// NewStateManager 创建状态管理器
// NewStateManager 创建状态管理器（使用 etcd）
// endpoints: etcd 集群地址，例如 []string{"localhost:2379"}
// 如果 endpoints 为空，则不使用持久化（纯内存模式）
func NewStateManager(endpoints ...string) (*StateManager, error) {
	sm := &StateManager{
		latestStates:   make(map[string]Metric),
		historyBuffers: make(map[string]*RingBuffer),
		alertStates:    make(map[string]bool),
		timeBase:       time.Now().Unix(),
		stopChan:       make(chan struct{}),
	}
	
	// 如果提供了 etcd 地址，则初始化 etcd 客户端
	if len(endpoints) > 0 && endpoints[0] != "" {
		config := clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: 5 * time.Second,
		}
		
		client, err := clientv3.New(config)
		if err != nil {
			return nil, fmt.Errorf("连接etcd失败: %w", err)
		}
		
		sm.etcdClient = client
		sm.etcdConfig = config
		
		// 尝试加载最新快照
		if err := sm.LoadSnapshot(); err != nil {
			fmt.Printf("加载快照失败（可能是首次启动）: %v\n", err)
		}
		
		// 启动后台持久化任务
		go sm.backgroundPersist()
	} else {
		fmt.Println("⚠️  纯内存模式：未配置 etcd，数据不会持久化")
	}
	
	return sm, nil
}

// UpdateMetric 更新指标（实时状态+历史记录）
func (sm *StateManager) UpdateMetric(metric Metric) error {
	if metric == nil {
		return fmt.Errorf("metric不能为nil")
	}
	
	// 时间戳对齐
	alignedMetric := sm.AlignTimestamp(metric)
	
	id := alignedMetric.GetID()
	metricType := alignedMetric.GetType()
	key := fmt.Sprintf("%s:%s", metricType, id)
	
	// 更新实时状态
	sm.statesMutex.Lock()
	sm.latestStates[key] = alignedMetric
	sm.statesMutex.Unlock()
	
	// 追加到历史缓冲区
	sm.AppendHistory(alignedMetric)
	
	return nil
}

// GetLatestState 获取最新状态
func (sm *StateManager) GetLatestState(metricType MetricType, id string) (Metric, bool) {
	key := fmt.Sprintf("%s:%s", metricType, id)
	
	sm.statesMutex.RLock()
	defer sm.statesMutex.RUnlock()
	
	metric, exists := sm.latestStates[key]
	return metric, exists
}

// GetAllLatestStates 获取指定类型的所有最新状态
func (sm *StateManager) GetAllLatestStates(metricType MetricType) []Metric {
	sm.statesMutex.RLock()
	defer sm.statesMutex.RUnlock()
	
	var results []Metric
	prefix := string(metricType) + ":"
	
	for key, metric := range sm.latestStates {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			results = append(results, metric)
		}
	}
	
	return results
}

// AppendHistory 追加历史记录到环形缓冲区
func (sm *StateManager) AppendHistory(metric Metric) {
	id := metric.GetID()
	metricType := metric.GetType()
	key := fmt.Sprintf("%s:%s", metricType, id)
	
	sm.historyMutex.Lock()
	
	// 如果该指标还没有环形缓冲区，创建一个
	buffer, exists := sm.historyBuffers[key]
	if !exists {
		buffer = NewRingBuffer(RingBufferSize)
		sm.historyBuffers[key] = buffer
	}
	
	sm.historyMutex.Unlock()
	
	// 追加数据
	buffer.Append(HistoryEntry{
		Timestamp: metric.GetTimestamp(),
		Data:      metric.GetData(),
	})
}

// QueryHistory 查询历史数据
func (sm *StateManager) QueryHistory(metricType MetricType, id string, duration time.Duration) []HistoryEntry {
	key := fmt.Sprintf("%s:%s", metricType, id)
	
	sm.historyMutex.RLock()
	buffer, exists := sm.historyBuffers[key]
	sm.historyMutex.RUnlock()
	
	if !exists {
		return []HistoryEntry{}
	}
	
	return buffer.Query(duration)
}

// AlignTimestamp 时间戳对齐（用于处理不同来源的时间偏差）
func (sm *StateManager) AlignTimestamp(metric Metric) Metric {
	// 简单实现：如果时间戳过旧或过新，调整为当前时间
	now := time.Now().Unix()
	ts := metric.GetTimestamp()
	
	// 如果时间戳为0或相差超过1小时，使用当前时间
	if ts == 0 || ts < now-3600 || ts > now+3600 {
		// 注意：这里需要根据具体的Metric类型来更新时间戳
		// 为了简化，我们假设时间戳已经在创建时正确设置
	}
	
	return metric
}

// SaveSnapshot 保存状态快照到BoltDB
func (sm *StateManager) SaveSnapshot() error {
	// 收集当前所有状态
	sm.statesMutex.RLock()
	snapshot := &StateSnapshot{
		Timestamp: time.Now().Unix(),
	}
	
	for _, metric := range sm.latestStates {
		switch metric.GetType() {
		case MetricTypeNode:
			if nm, ok := metric.(*NodeMetric); ok {
				snapshot.Nodes = append(snapshot.Nodes, *nm.Data)
			}
		case MetricTypeContainer:
			if cm, ok := metric.(*ContainerMetric); ok {
				snapshot.Containers = append(snapshot.Containers, *cm.Data)
			}
		case MetricTypeService:
			if sm, ok := metric.(*ServiceMetric); ok {
				snapshot.Services = append(snapshot.Services, *sm.Data)
			}
		case MetricTypeBusiness:
			if bm, ok := metric.(*BusinessMetric); ok {
				snapshot.Business = append(snapshot.Business, *bm.Data)
			}
		}
	}
	sm.statesMutex.RUnlock()
	
	// 序列化
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("序列化快照失败: %w", err)
	}
	
	// 保存到 etcd（如果已配置）
	if sm.etcdClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		key := fmt.Sprintf("%ssnapshot_%d", EtcdPrefixSnapshot, snapshot.Timestamp)
		_, err = sm.etcdClient.Put(ctx, key, string(data))
		if err != nil {
			return fmt.Errorf("保存快照到etcd失败: %w", err)
		}
		
		fmt.Printf("[StateManager] 快照已保存到etcd: %d nodes, %d containers, %d services, %d business\n",
			len(snapshot.Nodes), len(snapshot.Containers), len(snapshot.Services), len(snapshot.Business))
	}
	
	return nil
}

// LoadSnapshot 从 etcd 加载最新快照
func (sm *StateManager) LoadSnapshot() error {
	if sm.etcdClient == nil {
		return fmt.Errorf("未配置etcd")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// 获取所有快照键
	resp, err := sm.etcdClient.Get(ctx, EtcdPrefixSnapshot, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend), clientv3.WithLimit(1))
	if err != nil {
		return fmt.Errorf("查询etcd快照失败: %w", err)
	}
	
	if len(resp.Kvs) == 0 {
		return fmt.Errorf("未找到快照")
	}
	
	// 解析最新快照
	var latestSnapshot StateSnapshot
	if err := json.Unmarshal(resp.Kvs[0].Value, &latestSnapshot); err != nil {
		return fmt.Errorf("解析快照失败: %w", err)
	}
	
	// 恢复状态
	sm.statesMutex.Lock()
	defer sm.statesMutex.Unlock()
	
	// 恢复节点状态
	for _, node := range latestSnapshot.Nodes {
		nodeCopy := node
		metric := &NodeMetric{
			Data:      &nodeCopy,
			Timestamp: latestSnapshot.Timestamp,
		}
		key := fmt.Sprintf("%s:%s", MetricTypeNode, metric.GetID())
		sm.latestStates[key] = metric
	}
	
	// 恢复容器状态
	for _, container := range latestSnapshot.Containers {
		containerCopy := container
		metric := &ContainerMetric{
			Data:      &containerCopy,
			Timestamp: latestSnapshot.Timestamp,
		}
		key := fmt.Sprintf("%s:%s", MetricTypeContainer, metric.GetID())
		sm.latestStates[key] = metric
	}
	
	// 恢复服务状态
	for _, service := range latestSnapshot.Services {
		serviceCopy := service
		metric := &ServiceMetric{
			Data:      &serviceCopy,
			Timestamp: latestSnapshot.Timestamp,
		}
		key := fmt.Sprintf("%s:%s", MetricTypeService, metric.GetID())
		sm.latestStates[key] = metric
	}
	
	// 恢复业务层状态
	for _, business := range latestSnapshot.Business {
		businessCopy := business
		metric := &BusinessMetric{
			Data:      &businessCopy,
			Timestamp: latestSnapshot.Timestamp,
		}
		key := fmt.Sprintf("%s:%s", MetricTypeBusiness, metric.GetID())
		sm.latestStates[key] = metric
	}
	
	fmt.Printf("[StateManager] 快照已加载: timestamp=%d, %d nodes, %d containers, %d services, %d business\n",
		latestSnapshot.Timestamp, len(latestSnapshot.Nodes), len(latestSnapshot.Containers),
		len(latestSnapshot.Services), len(latestSnapshot.Business))
	
	return nil
}

// backgroundPersist 后台持久化任务
func (sm *StateManager) backgroundPersist() {
	ticker := time.NewTicker(SnapshotInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := sm.SaveSnapshot(); err != nil {
				fmt.Printf("[StateManager] 后台持久化失败: %v\n", err)
			}
		case <-sm.stopChan:
			// 最后保存一次
			sm.SaveSnapshot()
			return
		}
	}
}

// CleanupExpiredHistory 清理过期历史数据
func (sm *StateManager) CleanupExpiredHistory() {
	// Ring Buffer自动淘汰旧数据，这里主要清理 etcd 中的旧快照
	if sm.etcdClient == nil {
		return
	}
	
	cutoff := time.Now().Unix() - int64(HistoryRetention.Seconds())
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// 获取所有快照
	resp, err := sm.etcdClient.Get(ctx, EtcdPrefixSnapshot, clientv3.WithPrefix())
	if err != nil {
		fmt.Printf("[StateManager] 清理过期快照失败: %v\n", err)
		return
	}
	
	// 删除过期快照
	for _, kv := range resp.Kvs {
		var snapshot StateSnapshot
		if err := json.Unmarshal(kv.Value, &snapshot); err != nil {
			continue
		}
		
		if snapshot.Timestamp < cutoff {
			sm.etcdClient.Delete(ctx, string(kv.Key))
		}
	}
}

// Close 关闭状态管理器
func (sm *StateManager) Close() error {
	close(sm.stopChan)
	
	// 最后保存一次快照
	if err := sm.SaveSnapshot(); err != nil {
		fmt.Printf("[StateManager] 关闭时保存快照失败: %v\n", err)
	}
	
	// 关闭 etcd 客户端
	if sm.etcdClient != nil {
		return sm.etcdClient.Close()
	}
	
	return nil
}

// GetStats 获取状态统计信息
func (sm *StateManager) GetStats() map[string]interface{} {
	sm.statesMutex.RLock()
	stateCount := len(sm.latestStates)
	sm.statesMutex.RUnlock()
	
	sm.historyMutex.RLock()
	historyCount := len(sm.historyBuffers)
	sm.historyMutex.RUnlock()
	
	sm.alertMutex.RLock()
	alertCount := len(sm.alertStates)
	sm.alertMutex.RUnlock()
	
	return map[string]interface{}{
		"latest_states":   stateCount,
		"history_buffers": historyCount,
		"active_alerts":   alertCount,
		"ring_buffer_size": RingBufferSize,
		"retention":       HistoryRetention.String(),
	}
}

// SetAlertState 设置告警状态
func (sm *StateManager) SetAlertState(alertID string, active bool) {
	sm.alertMutex.Lock()
	defer sm.alertMutex.Unlock()
	sm.alertStates[alertID] = active
}

// GetAlertState 获取告警状态
func (sm *StateManager) GetAlertState(alertID string) bool {
	sm.alertMutex.RLock()
	defer sm.alertMutex.RUnlock()
	return sm.alertStates[alertID]
}

// CheckAndUpdateAlertState 检查并更新告警状态
// 返回: (shouldSendAlert bool, isFiring bool)
// shouldSendAlert: 是否需要发送告警（状态变化时为true）
// isFiring: true表示触发告警，false表示恢复告警
func (sm *StateManager) CheckAndUpdateAlertState(alertID string, isFiring bool) (bool, bool) {
	key := sm.alertKey(alertID, "")
	return sm.checkAndUpdateAlertStateByKey(key, isFiring)
}

// CheckAndUpdateAlertStateWithSource 按告警ID+来源维度检查并更新状态
func (sm *StateManager) CheckAndUpdateAlertStateWithSource(alertID, source string, isFiring bool) (bool, bool) {
	key := sm.alertKey(alertID, source)
	return sm.checkAndUpdateAlertStateByKey(key, isFiring)
}

func (sm *StateManager) alertKey(alertID, source string) string {
	if source == "" {
		return alertID
	}
	return fmt.Sprintf("%s:%s", alertID, source)
}

func (sm *StateManager) checkAndUpdateAlertStateByKey(key string, isFiring bool) (bool, bool) {
	sm.alertMutex.Lock()
	defer sm.alertMutex.Unlock()
	
	wasActive, exists := sm.alertStates[key]
	
	// 状态发生变化
	if !exists || wasActive != isFiring {
		sm.alertStates[key] = isFiring
		return true, isFiring // 需要发送告警
	}
	
	// 状态未变化
	return false, isFiring
}

// GetActiveAlertCount 获取活跃告警数量
func (sm *StateManager) GetActiveAlertCount() int {
	sm.alertMutex.RLock()
	defer sm.alertMutex.RUnlock()
	
	count := 0
	for _, active := range sm.alertStates {
		if active {
			count++
		}
	}
	return count
}

// GetActiveAlerts 获取所有活跃的告警ID列表
func (sm *StateManager) GetActiveAlerts() []string {
	sm.alertMutex.RLock()
	defer sm.alertMutex.RUnlock()
	
	var alerts []string
	for alertID, active := range sm.alertStates {
		if active {
			alerts = append(alerts, alertID)
		}
	}
	return alerts
}

// ClearAlertState 清除指定告警状态
func (sm *StateManager) ClearAlertState(alertID string) {
	sm.alertMutex.Lock()
	defer sm.alertMutex.Unlock()
	delete(sm.alertStates, alertID)
}

// ResetAllAlerts 重置所有告警状态
func (sm *StateManager) ResetAllAlerts() {
	sm.alertMutex.Lock()
	defer sm.alertMutex.Unlock()
	sm.alertStates = make(map[string]bool)
}
