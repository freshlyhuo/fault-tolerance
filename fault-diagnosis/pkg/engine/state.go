package engine

import (
	"fmt"
	"sync"

	"fault-diagnosis/pkg/models"
)

// StateManager 事件状态管理器
type StateManager struct {
	mu     sync.RWMutex
	states map[string]models.EventState // eventID -> state
}

// NewStateManager 创建状态管理器
func NewStateManager() *StateManager {
	return &StateManager{
		states: make(map[string]models.EventState),
	}
}

// SetState 设置事件状态
func (sm *StateManager) SetState(eventID string, state models.EventState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states[eventID] = state
}

// GetState 获取事件状态
func (sm *StateManager) GetState(eventID string) models.EventState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if state, ok := sm.states[eventID]; ok {
		return state
	}
	return models.StateFalse
}

// ResetState 重置事件状态
func (sm *StateManager) ResetState(eventID string) {
	sm.SetState(eventID, models.StateFalse)
}

// ResetAll 重置所有状态
func (sm *StateManager) ResetAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states = make(map[string]models.EventState)
}

// GetAllStates 获取所有状态
func (sm *StateManager) GetAllStates() map[string]models.EventState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	result := make(map[string]models.EventState)
	for k, v := range sm.states {
		result[k] = v
	}
	return result
}

// GetTrueEvents 获取所有状态为真的事件ID
func (sm *StateManager) GetTrueEvents() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	trueEvents := make([]string, 0)
	for eventID, state := range sm.states {
		if state == models.StateTrue {
			trueEvents = append(trueEvents, eventID)
		}
	}
	return trueEvents
}

// String 返回状态管理器的字符串表示
func (sm *StateManager) String() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	result := "StateManager{\n"
	for eventID, state := range sm.states {
		result += fmt.Sprintf("  %s: %s\n", eventID, state.String())
	}
	result += "}"
	return result
}
