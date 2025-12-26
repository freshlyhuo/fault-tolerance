package engine

import (
	"fmt"
	"sync"
	"time"

	"fault-diagnosis/pkg/models"
)

// EventStateRecord 事件状态记录（带时间戳和TTL）
type EventStateRecord struct {
	State      models.EventState // 当前状态
	LastUpdate time.Time         // 最后更新时间
	TTL        time.Duration     // 生存时间（0表示永不过期）
}

// IsExpired 判断是否过期
func (r *EventStateRecord) IsExpired() bool {
	if r.TTL == 0 {
		return false // 永不过期
	}
	return time.Since(r.LastUpdate) > r.TTL
}

// StateManager 事件状态管理器（支持TTL和自动过期）
type StateManager struct {
	mu              sync.RWMutex
	states          map[string]*EventStateRecord // eventID -> stateRecord
	defaultTTL      time.Duration                // 默认TTL
	enableAutoClean bool                         // 是否启用自动清理
	stopClean       chan struct{}                // 停止清理信号
}

// NewStateManager 创建状态管理器
func NewStateManager() *StateManager {
	return NewStateManagerWithTTL(5 * time.Minute) // 默认5分钟TTL
}

// NewStateManagerWithTTL 创建带TTL的状态管理器
func NewStateManagerWithTTL(defaultTTL time.Duration) *StateManager {
	sm := &StateManager{
		states:          make(map[string]*EventStateRecord),
		defaultTTL:      defaultTTL,
		enableAutoClean: true,
		stopClean:       make(chan struct{}),
	}
	
	// 启动自动清理协程
	if sm.enableAutoClean {
		go sm.autoCleanExpired()
	}
	
	return sm
}

// Stop 停止状态管理器（停止自动清理）
func (sm *StateManager) Stop() {
	if sm.enableAutoClean {
		close(sm.stopClean)
	}
}

// SetState 设置事件状态
func (sm *StateManager) SetState(eventID string, state models.EventState) {
	sm.SetStateWithTTL(eventID, state, sm.defaultTTL)
}

// SetStateWithTTL 设置事件状态并指定TTL
func (sm *StateManager) SetStateWithTTL(eventID string, state models.EventState, ttl time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.states[eventID] = &EventStateRecord{
		State:      state,
		LastUpdate: time.Now(),
		TTL:        ttl,
	}
}

// SetStatePermanent 设置永久状态（不过期）
func (sm *StateManager) SetStatePermanent(eventID string, state models.EventState) {
	sm.SetStateWithTTL(eventID, state, 0)
}

// GetState 获取事件状态（自动检查过期）
func (sm *StateManager) GetState(eventID string) models.EventState {
	sm.mu.RLock()
	record, ok := sm.states[eventID]
	sm.mu.RUnlock()
	
	if !ok {
		return models.StateFalse
	}
	
	// 检查是否过期
	if record.IsExpired() {
		sm.ResetState(eventID)
		return models.StateFalse
	}
	
	return record.State
}

// GetStateWithTimestamp 获取事件状态和时间戳
func (sm *StateManager) GetStateWithTimestamp(eventID string) (models.EventState, time.Time) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if record, ok := sm.states[eventID]; ok {
		if !record.IsExpired() {
			return record.State, record.LastUpdate
		}
	}
	return models.StateFalse, time.Time{}
}

// ResetState 重置事件状态
func (sm *StateManager) ResetState(eventID string) {
	sm.SetState(eventID, models.StateFalse)
}

// ResetAll 重置所有状态
func (sm *StateManager) ResetAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states = make(map[string]*EventStateRecord)
}

// GetAllStates 获取所有状态
func (sm *StateManager) GetAllStates() map[string]models.EventState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	result := make(map[string]models.EventState)
	for k, v := range sm.states {
		if !v.IsExpired() {
			result[k] = v.State
		}
	}
	return result
}

// GetTrueEvents 获取所有状态为真的事件ID（自动过滤过期事件）
func (sm *StateManager) GetTrueEvents() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	trueEvents := make([]string, 0)
	for eventID, record := range sm.states {
		if !record.IsExpired() && record.State == models.StateTrue {
			trueEvents = append(trueEvents, eventID)
		}
	}
	return trueEvents
}

// autoCleanExpired 自动清理过期状态（后台协程）
func (sm *StateManager) autoCleanExpired() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒清理一次
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sm.cleanExpired()
		case <-sm.stopClean:
			return
		}
	}
}

// cleanExpired 清理过期状态
func (sm *StateManager) cleanExpired() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	expiredCount := 0
	for eventID, record := range sm.states {
		if record.IsExpired() {
			delete(sm.states, eventID)
			expiredCount++
		}
	}
	
	if expiredCount > 0 {
		fmt.Printf("[StateManager] 清理了 %d 个过期事件状态\n", expiredCount)
	}
}

// String 返回状态管理器的字符串表示
func (sm *StateManager) String() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	result := fmt.Sprintf("StateManager{DefaultTTL: %v, Events: %d\n", sm.defaultTTL, len(sm.states))
	for eventID, record := range sm.states {
		expired := ""
		if record.IsExpired() {
			expired = " [EXPIRED]"
		}
		result += fmt.Sprintf("  %s: %s (TTL: %v, Updated: %s)%s\n", 
			eventID, record.State.String(), record.TTL, 
			record.LastUpdate.Format("15:04:05"), expired)
	}
	result += "}"
	return result
}
