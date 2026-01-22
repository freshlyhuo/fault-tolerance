package recovery

import (
	"errors"
	"log"
	"sync"
)

const (
	StateRecovering = "RECOVERING"
	StateHealthy    = "HEALTHY"
	StateFailed     = "FAILED"
)

// StateManager 与状态管理器交互的最小接口
type StateManager interface {
	LockRecovering(targetID string) (bool, error)
	UpdateState(targetID string, state string) error
	ReportResult(result RecoveryResult) error
}

// InMemoryStateManager 用于开发/演示的内存实现
// 生产可替换为与状态管理器通信的实现
type InMemoryStateManager struct {
	mu        sync.Mutex
	states    map[string]string
	recovering map[string]bool
	lastResult map[string]RecoveryResult
}

func NewInMemoryStateManager() *InMemoryStateManager {
	return &InMemoryStateManager{
		states:     make(map[string]string),
		recovering: make(map[string]bool),
		lastResult: make(map[string]RecoveryResult),
	}
}

func (sm *InMemoryStateManager) LockRecovering(targetID string) (bool, error) {
	if targetID == "" {
		return false, errors.New("empty targetID")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.recovering[targetID] {
		return false, nil
	}

	sm.recovering[targetID] = true
	sm.states[targetID] = StateRecovering
	return true, nil
}

func (sm *InMemoryStateManager) UpdateState(targetID string, state string) error {
	if targetID == "" {
		return errors.New("empty targetID")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.states[targetID] = state
	if state != StateRecovering {
		delete(sm.recovering, targetID)
	}
	return nil
}

func (sm *InMemoryStateManager) ReportResult(result RecoveryResult) error {
	sm.mu.Lock()
	sm.lastResult[result.TargetID] = result
	sm.mu.Unlock()

	log.Printf("recovery result: target=%s fault=%s action=%s status=%s msg=%s err=%s",
		result.TargetID, result.FaultCode, result.Action, result.Status, result.Message, result.Error)

	switch result.Status {
	case ResultSuccess:
		return sm.UpdateState(result.TargetID, StateHealthy)
	case ResultFailed, ResultTimeout:
		return sm.UpdateState(result.TargetID, StateFailed)
	default:
		return nil
	}
}
