package engine

import (
	"fmt"
	"sync"

	"fault-diagnosis/pkg/models"
	"go.uber.org/zap"
)

// MultiDiagnosisEngine 在多个单树诊断引擎之上做路由和并发分发。
type MultiDiagnosisEngine struct {
	logger         *zap.Logger
	engines        []*DiagnosisEngine
	alertToEngines map[string][]*DiagnosisEngine

	mu       sync.RWMutex
	callback DiagnosisCallback
}

// NewMultiDiagnosisEngine 创建多故障树诊断引擎。
func NewMultiDiagnosisEngine(faultTrees []*models.FaultTree, logger *zap.Logger) (*MultiDiagnosisEngine, error) {
	if len(faultTrees) == 0 {
		return nil, fmt.Errorf("故障树配置不能为空")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	m := &MultiDiagnosisEngine{
		logger:         logger,
		engines:        make([]*DiagnosisEngine, 0, len(faultTrees)),
		alertToEngines: make(map[string][]*DiagnosisEngine),
	}

	for i, faultTree := range faultTrees {
		if faultTree == nil {
			return nil, fmt.Errorf("第 %d 棵故障树为空", i+1)
		}

		eng, err := NewDiagnosisEngine(faultTree, logger.Named(faultTree.FaultTreeID))
		if err != nil {
			return nil, fmt.Errorf("初始化第 %d 棵故障树失败: %w", i+1, err)
		}

		m.engines = append(m.engines, eng)

		seenAlerts := make(map[string]struct{})
		for _, basicEvent := range faultTree.BasicEvents {
			if _, exists := seenAlerts[basicEvent.AlertID]; exists {
				continue
			}
			seenAlerts[basicEvent.AlertID] = struct{}{}
			m.alertToEngines[basicEvent.AlertID] = append(m.alertToEngines[basicEvent.AlertID], eng)
		}
	}

	m.logger.Info("多故障树诊断引擎初始化成功",
		zap.Int("fault_trees", len(m.engines)),
		zap.Int("route_alert_ids", len(m.alertToEngines)),
	)

	return m, nil
}

// SetCallback 设置统一诊断回调，并转发每棵树引擎的输出。
func (m *MultiDiagnosisEngine) SetCallback(callback DiagnosisCallback) {
	m.mu.Lock()
	m.callback = callback
	m.mu.Unlock()

	forward := func(d *models.DiagnosisResult) {
		m.mu.RLock()
		cb := m.callback
		m.mu.RUnlock()
		if cb != nil {
			cb(d)
		}
	}

	for _, eng := range m.engines {
		eng.SetCallback(forward)
	}
}

// ProcessAlert 根据 alert_id 路由到所有匹配的故障树并发评估。
func (m *MultiDiagnosisEngine) ProcessAlert(alert *models.AlertEvent) {
	if alert == nil {
		return
	}

	targets := m.alertToEngines[alert.AlertID]
	if len(targets) == 0 {
		m.logger.Debug("告警未命中任何故障树",
			zap.String("alert_id", alert.AlertID),
			zap.String("source", alert.Source),
		)
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(targets))

	for _, target := range targets {
		eng := target
		faultTreeID := ""
		if eng != nil && eng.faultTree != nil {
			faultTreeID = eng.faultTree.FaultTreeID
		}

		go func(faultTreeID string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error("并行评估异常",
						zap.Any("panic", r),
						zap.String("alert_id", alert.AlertID),
						zap.String("fault_tree_id", faultTreeID),
					)
				}
			}()

			eng.ProcessAlert(alert)
		}(faultTreeID)
	}

	wg.Wait()
}
