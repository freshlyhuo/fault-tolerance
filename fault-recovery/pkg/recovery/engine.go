package recovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// Engine 故障修复执行引擎（异步/非阻塞）
type Engine struct {
	sm        StateManager
	actions   map[string]Action
	prefixActions []prefixAction
	queue     chan DiagnosisResult
	timeout   time.Duration
}

type prefixAction struct {
	prefix string
	action Action
}

// NewEngine 创建修复引擎
type NewEngineConfig struct {
	QueueSize int
	Timeout   time.Duration
}

func NewEngine(sm StateManager, cfg NewEngineConfig) *Engine {
	qsize := cfg.QueueSize
	if qsize <= 0 {
		qsize = 100
	}
	tm := cfg.Timeout
	if tm <= 0 {
		tm = 10 * time.Second
	}

	return &Engine{
		sm:      sm,
		actions: make(map[string]Action),
		prefixActions: nil,
		queue:   make(chan DiagnosisResult, qsize),
		timeout: tm,
	}
}

func (e *Engine) RegisterAction(faultCode string, action Action) {
	if faultCode == "" || action == nil {
		return
	}
	e.actions[faultCode] = action
}

// RegisterPrefixAction 注册按故障码前缀匹配的动作（按注册顺序匹配）
func (e *Engine) RegisterPrefixAction(prefix string, action Action) {
	if prefix == "" || action == nil {
		return
	}
	e.prefixActions = append(e.prefixActions, prefixAction{prefix: prefix, action: action})
}

// Start 启动执行循环
func (e *Engine) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-e.queue:
				go e.handleEvent(event)
			}
		}
	}()
}

// Submit 提交诊断事件
func (e *Engine) Submit(event DiagnosisResult) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	select {
	case e.queue <- event:
		return nil
	default:
		return errors.New("recovery queue full")
	}
}

func (e *Engine) handleEvent(event DiagnosisResult) {
	targetID := DiagnosisTargetID(event)
	action := e.actions[event.FaultCode]
	if action == nil {
		for _, pa := range e.prefixActions {
			if strings.HasPrefix(event.FaultCode, pa.prefix) {
				action = pa.action
				break
			}
		}
	}
	status := DiagnosisStatus(event)
	if action == nil {
		_ = e.sm.ReportResult(RecoveryResult{
			TargetID:   targetID,
			FaultCode:  event.FaultCode,
			Action:     "",
			Status:     ResultNoAction,
			Message:    "no action registered",
			StartedAt:  nowUnix(),
			FinishedAt: nowUnix(),
		})
		return
	}

	if targetID == "" {
		_ = e.sm.ReportResult(RecoveryResult{
			TargetID:   "",
			FaultCode:  event.FaultCode,
			Action:     action.Name(),
			Status:     ResultFailed,
			Message:    "empty target id",
			StartedAt:  nowUnix(),
			FinishedAt: nowUnix(),
		})
		return
	}

	// 解决事件允许直接执行，避免被“正在恢复”锁拒绝
	if status == EventStatusResolved {
		started := nowUnix()
		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		defer cancel()

		err := e.executeAction(ctx, action, event)
		result := RecoveryResult{
			TargetID:   targetID,
			FaultCode:  event.FaultCode,
			Action:     action.Name(),
			StartedAt:  started,
			FinishedAt: nowUnix(),
		}

		switch {
		case errors.Is(err, context.DeadlineExceeded):
			result.Status = ResultTimeout
			result.Message = "action timeout"
			result.Error = err.Error()
		case err != nil:
			result.Status = ResultFailed
			result.Message = "action failed"
			result.Error = err.Error()
		default:
			result.Status = ResultSuccess
			result.Message = "action success"
		}

		if reportErr := e.sm.ReportResult(result); reportErr != nil {
			log.Printf("report result failed: %v", reportErr)
		}
		return
	}

	locked, err := e.sm.LockRecovering(targetID)
	if err != nil {
		_ = e.sm.ReportResult(RecoveryResult{
			TargetID:   targetID,
			FaultCode:  event.FaultCode,
			Action:     action.Name(),
			Status:     ResultFailed,
			Message:    "lock recovering failed",
			Error:      err.Error(),
			StartedAt:  nowUnix(),
			FinishedAt: nowUnix(),
		})
		return
	}
	if !locked {
		_ = e.sm.ReportResult(RecoveryResult{
			TargetID:   targetID,
			FaultCode:  event.FaultCode,
			Action:     action.Name(),
			Status:     ResultRejected,
			Message:    "target already recovering",
			StartedAt:  nowUnix(),
			FinishedAt: nowUnix(),
		})
		return
	}

	started := nowUnix()
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	err = e.executeAction(ctx, action, event)
	result := RecoveryResult{
		TargetID:   targetID,
		FaultCode:  event.FaultCode,
		Action:     action.Name(),
		StartedAt:  started,
		FinishedAt: nowUnix(),
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		result.Status = ResultTimeout
		result.Message = "action timeout"
		result.Error = err.Error()
	case err != nil:
		result.Status = ResultFailed
		result.Message = "action failed"
		result.Error = err.Error()
	default:
		result.Status = ResultSuccess
		result.Message = "action success"
	}

	if reportErr := e.sm.ReportResult(result); reportErr != nil {
		log.Printf("report result failed: %v", reportErr)
	}
}

func (e *Engine) executeAction(ctx context.Context, action Action, event DiagnosisResult) error {
	if DiagnosisStatus(event) == EventStatusResolved {
		if resolver, ok := action.(Resolver); ok {
			if err := resolver.Resolve(ctx, event); err != nil {
				return err
			}
			return action.Verify(ctx, event)
		}
		return fmt.Errorf("action %s does not support resolve", action.Name())
	}

	if err := action.Execute(ctx, event); err != nil {
		return err
	}
	return action.Verify(ctx, event)
}
