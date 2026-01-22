package state

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

// RecoveryResult 修复执行结果
// status: SUCCESS | FAILED | TIMEOUT | REJECTED | NO_ACTION
type RecoveryResult struct {
	TargetID   string `json:"target_id"`
	FaultCode  string `json:"fault_code"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	Error      string `json:"error,omitempty"`
}

const (
	ResultSuccess  = "SUCCESS"
	ResultFailed   = "FAILED"
	ResultTimeout  = "TIMEOUT"
	ResultRejected = "REJECTED"
	ResultNoAction = "NO_ACTION"
)

// RecoveryStateManager 与状态管理器交互的最小接口
type RecoveryStateManager interface {
	LockRecovering(targetID string) (bool, error)
	UpdateState(targetID string, state string) error
	ReportResult(result RecoveryResult) error
}

// InMemoryRecoveryStateManager 用于开发/演示的内存实现
// 生产可替换为与状态管理器通信的实现
type InMemoryRecoveryStateManager struct {
	mu         sync.Mutex
	states     map[string]string
	recovering map[string]bool
	lastResult map[string]RecoveryResult
}

func NewInMemoryRecoveryStateManager() *InMemoryRecoveryStateManager {
	return &InMemoryRecoveryStateManager{
		states:     make(map[string]string),
		recovering: make(map[string]bool),
		lastResult: make(map[string]RecoveryResult),
	}
}

func (sm *InMemoryRecoveryStateManager) LockRecovering(targetID string) (bool, error) {
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

func (sm *InMemoryRecoveryStateManager) UpdateState(targetID string, state string) error {
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

func (sm *InMemoryRecoveryStateManager) ReportResult(result RecoveryResult) error {
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
