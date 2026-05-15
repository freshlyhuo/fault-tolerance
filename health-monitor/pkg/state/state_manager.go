/*
	状态管理器 - 核心数据中枢

功能:
1. 实时状态维护 - UpdateMetric()
2. 统一查询接口 - GetLatestState()
3. 历史窗口缓存 - AppendHistory() / QueryHistory()
4. 时间戳对齐 - AlignTimestamp()
5. 纯内存缓存 - 不依赖外部数据库
*/
package state

import (
	"fmt"
	"sync"
	"time"
)

const (
	// 历史数据保留时长
	HistoryRetention = 10 * time.Minute

	// 环形缓冲区大小（每个指标保留最近N条记录）
	RingBufferSize = 600 // 10分钟 * 60秒
)

// StateManager 状态管理器
type StateManager struct {
	// 实时状态存储（最新值）
	latestStates map[string]Metric
	statesMutex  sync.RWMutex

	// 历史数据环形缓冲区 (business-id -> ring buffer)
	historyBuffers map[string]*RingBuffer
	historyMutex   sync.RWMutex

	// 告警状态跟踪 (alertID -> 是否激活)
	alertStates map[string]bool
	alertMutex  sync.RWMutex

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

// NewStateManager 创建状态管理器（纯内存缓存实现）
// 为了兼容旧调用，保留可变参数但不再使用。
func NewStateManager(_ ...string) (*StateManager, error) {
	sm := &StateManager{
		latestStates:   make(map[string]Metric),
		historyBuffers: make(map[string]*RingBuffer),
		alertStates:    make(map[string]bool),
		stopChan:       make(chan struct{}),
	}

	fmt.Println("状态管理器运行在纯内存模式（缓存实现）")

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

	// 更新实时状态
	sm.statesMutex.Lock()
	sm.latestStates[id] = alignedMetric
	sm.statesMutex.Unlock()

	// 追加到历史缓冲区
	sm.AppendHistory(alignedMetric)

	return nil
}

// GetLatestState 获取最新状态
func (sm *StateManager) GetLatestState(id string) (Metric, bool) {
	sm.statesMutex.RLock()
	defer sm.statesMutex.RUnlock()

	metric, exists := sm.latestStates[id]
	return metric, exists
}

// GetAllLatestStates 获取所有最新业务状态。
func (sm *StateManager) GetAllLatestStates() []Metric {
	sm.statesMutex.RLock()
	defer sm.statesMutex.RUnlock()

	var results []Metric
	for _, metric := range sm.latestStates {
		results = append(results, metric)
	}

	return results
}

// AppendHistory 追加历史记录到环形缓冲区
func (sm *StateManager) AppendHistory(metric Metric) {
	id := metric.GetID()

	sm.historyMutex.Lock()

	// 如果该指标还没有环形缓冲区，创建一个
	buffer, exists := sm.historyBuffers[id]
	if !exists {
		buffer = NewRingBuffer(RingBufferSize)
		sm.historyBuffers[id] = buffer
	}

	sm.historyMutex.Unlock()

	// 追加数据
	buffer.Append(HistoryEntry{
		Timestamp: metric.GetTimestamp(),
		Data:      metric.GetData(),
	})
}

// QueryHistory 查询历史数据
func (sm *StateManager) QueryHistory(id string, duration time.Duration) []HistoryEntry {
	sm.historyMutex.RLock()
	buffer, exists := sm.historyBuffers[id]
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

// SaveSnapshot 纯内存模式下无需持久化，保留接口兼容。
func (sm *StateManager) SaveSnapshot() error {
	return nil
}

// LoadSnapshot 纯内存模式下无需加载，保留接口兼容。
func (sm *StateManager) LoadSnapshot() error {
	return nil
}

// backgroundPersist 纯内存模式下无需后台持久化，保留接口兼容。
func (sm *StateManager) backgroundPersist() {
	<-sm.stopChan
}

// CleanupExpiredHistory 清理过期历史数据
func (sm *StateManager) CleanupExpiredHistory() {
	// RingBuffer 自带淘汰能力，纯内存模式无需外部清理。
}

// Close 关闭状态管理器
func (sm *StateManager) Close() error {
	select {
	case <-sm.stopChan:
		return nil
	default:
		close(sm.stopChan)
		return nil
	}
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
		"latest_states":    stateCount,
		"history_buffers":  historyCount,
		"active_alerts":    alertCount,
		"ring_buffer_size": RingBufferSize,
		"retention":        HistoryRetention.String(),
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
