package engine

import (
	"fmt"
	"sync"

	"fault-diagnosis/pkg/models"
	"go.uber.org/zap"
)

// DiagnosisEngine 故障诊断引擎
type DiagnosisEngine struct {
	faultTree    *models.FaultTree       // 故障树配置
	topEvents    []*models.EventNode     // 顶层事件节点
	eventNodes   map[string]*models.EventNode // 事件ID -> 节点
	alertToEvent map[string]string       // 告警ID -> 基本事件ID
	stateManager *StateManager           // 状态管理器
	evaluator    *Evaluator              // 求值器
	logger       *zap.Logger             // 日志
	mu           sync.RWMutex            // 读写锁
	callback     DiagnosisCallback       // 诊断回调函数
}

// DiagnosisCallback 诊断回调函数类型
type DiagnosisCallback func(*models.DiagnosisResult)

// NewDiagnosisEngine 创建诊断引擎
func NewDiagnosisEngine(faultTree *models.FaultTree, logger *zap.Logger) (*DiagnosisEngine, error) {
	if faultTree == nil {
		return nil, fmt.Errorf("故障树配置不能为空")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	engine := &DiagnosisEngine{
		faultTree:    faultTree,
		eventNodes:   make(map[string]*models.EventNode),
		alertToEvent: make(map[string]string),
		stateManager: NewStateManager(),
		logger:       logger,
	}

	// 构建故障树运行时结构
	if err := engine.buildTree(); err != nil {
		return nil, fmt.Errorf("构建故障树失败: %w", err)
	}

	// 创建求值器
	engine.evaluator = NewEvaluator(engine.stateManager)

	logger.Info("故障诊断引擎初始化成功",
		zap.String("fault_tree_id", faultTree.FaultTreeID),
		zap.Int("top_events", len(faultTree.TopEvents)),
		zap.Int("basic_events", len(faultTree.BasicEvents)),
	)

	return engine, nil
}

// buildTree 构建故障树运行时结构
func (e *DiagnosisEngine) buildTree() error {
	// 1. 创建所有基本事件节点
	for _, basicEvent := range e.faultTree.BasicEvents {
		node := &models.EventNode{
			EventID:     basicEvent.EventID,
			Name:        basicEvent.Name,
			Description: basicEvent.Description,
			IsBasic:     true,
			AlertID:     basicEvent.AlertID,
			State:       models.StateFalse,
			Children:    make([]*models.EventNode, 0),
		}
		e.eventNodes[basicEvent.EventID] = node
		e.alertToEvent[basicEvent.AlertID] = basicEvent.EventID
		
		// 初始化状态为假
		e.stateManager.SetState(basicEvent.EventID, models.StateFalse)
	}

	// 2. 创建所有中间事件节点
	for _, intermediateEvent := range e.faultTree.IntermediateEvents {
		node := &models.EventNode{
			EventID:     intermediateEvent.EventID,
			Name:        intermediateEvent.Name,
			Description: intermediateEvent.Description,
			GateType:    intermediateEvent.GateType,
			IsBasic:     false,
			State:       models.StateFalse,
			Children:    make([]*models.EventNode, 0),
		}
		e.eventNodes[intermediateEvent.EventID] = node
	}

	// 3. 创建所有顶层事件节点
	e.topEvents = make([]*models.EventNode, 0, len(e.faultTree.TopEvents))
	for _, topEvent := range e.faultTree.TopEvents {
		node := &models.EventNode{
			EventID:     topEvent.EventID,
			Name:        topEvent.Name,
			Description: topEvent.Description,
			FaultCode:   topEvent.FaultCode,
			GateType:    topEvent.GateType,
			IsBasic:     false,
			State:       models.StateFalse,
			Children:    make([]*models.EventNode, 0),
		}
		e.eventNodes[topEvent.EventID] = node
		e.topEvents = append(e.topEvents, node)
	}

	// 4. 建立父子关系（处理中间事件）
	for _, intermediateEvent := range e.faultTree.IntermediateEvents {
		parentNode := e.eventNodes[intermediateEvent.EventID]
		for _, childID := range intermediateEvent.Children {
			// 处理 NOT 前缀
			actualChildID := childID
			isNOT := false
			if len(childID) > 4 && childID[:4] == "NOT-" {
				isNOT = true
				actualChildID = childID[4:]
			}

			childNode, ok := e.eventNodes[actualChildID]
			if !ok {
				return fmt.Errorf("中间事件 %s 的子事件 %s 不存在", intermediateEvent.EventID, actualChildID)
			}

			if isNOT {
				// 创建一个NOT门节点
				notNode := &models.EventNode{
					EventID:  "NOT-" + actualChildID,
					Name:     "NOT " + childNode.Name,
					GateType: models.GateNOT,
					IsBasic:  false,
					State:    models.StateFalse,
					Children: []*models.EventNode{childNode},
				}
				parentNode.AddChild(notNode)
			} else {
				parentNode.AddChild(childNode)
			}
		}
	}

	// 5. 建立父子关系（处理顶层事件）
	for _, topEvent := range e.faultTree.TopEvents {
		parentNode := e.eventNodes[topEvent.EventID]
		for _, childID := range topEvent.Children {
			// 处理 NOT 前缀
			actualChildID := childID
			isNOT := false
			if len(childID) > 4 && childID[:4] == "NOT-" {
				isNOT = true
				actualChildID = childID[4:]
			}

			childNode, ok := e.eventNodes[actualChildID]
			if !ok {
				return fmt.Errorf("顶层事件 %s 的子事件 %s 不存在", topEvent.EventID, actualChildID)
			}

			if isNOT {
				// 创建一个NOT门节点
				notNode := &models.EventNode{
					EventID:  "NOT-" + actualChildID,
					Name:     "NOT " + childNode.Name,
					GateType: models.GateNOT,
					IsBasic:  false,
					State:    models.StateFalse,
					Children: []*models.EventNode{childNode},
				}
				parentNode.AddChild(notNode)
			} else {
				parentNode.AddChild(childNode)
			}
		}
	}

	return nil
}

// SetCallback 设置诊断回调函数
func (e *DiagnosisEngine) SetCallback(callback DiagnosisCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callback = callback
}

// ProcessAlert 处理告警事件（支持恢复告警）
func (e *DiagnosisEngine) ProcessAlert(alert *models.AlertEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 判断是恢复告警还是触发告警
	isResolved := alert.IsResolved()
	if !isResolved{
		e.logger.Info("接收到告警事件",
		zap.String("alert_id", alert.AlertID),
		zap.String("type", alert.Type),
		zap.String("status", string(alert.Status)),
		zap.String("severity", alert.Severity),
		zap.Bool("is_resolved", isResolved))
	}

	// 将告警映射到基本事件
	eventID, ok := e.alertToEvent[alert.AlertID]
	if !ok {
		e.logger.Warn("告警ID未映射到任何基本事件",
			zap.String("alert_id", alert.AlertID),
		)
		return
	}

	// 根据告警状态更新基本事件
	if isResolved {
		// 恢复告警：将基本事件置为假
		e.stateManager.SetState(eventID, models.StateFalse)
	} else {
		// 触发告警：将基本事件置为真
		e.stateManager.SetState(eventID, models.StateTrue)
		e.logger.Info("基本事件状态已更新",
			zap.String("event_id", eventID),
			zap.String("state", "TRUE"),
		)
		
		// 触发诊断求值
		e.diagnose()
	}
}

// diagnose 执行诊断求值
func (e *DiagnosisEngine) diagnose() {
	// 对所有顶层事件进行求值
	triggeredTopEvents := e.evaluator.EvaluateTree(e.topEvents)

	if len(triggeredTopEvents) == 0 {
		e.logger.Debug("未触发任何顶层故障事件")
		return
	}

	// 生成诊断结果
	for _, topEvent := range triggeredTopEvents {
		diagnosis := e.generateDiagnosisResult(topEvent)
		
		e.logger.Info("检测到故障",
			zap.String("diagnosis_id", diagnosis.DiagnosisID),
			zap.String("fault_code", diagnosis.FaultCode),
			zap.String("top_event", diagnosis.TopEventName),
			zap.Strings("trigger_path", diagnosis.TriggerPath),
		)

		// 调用回调函数
		if e.callback != nil {
			e.callback(diagnosis)
		}
	}
}

// generateDiagnosisResult 生成诊断结果
func (e *DiagnosisEngine) generateDiagnosisResult(topEvent *models.EventNode) *models.DiagnosisResult {
	diagnosis := models.NewDiagnosisResult(
		e.faultTree.FaultTreeID,
		topEvent.EventID,
		topEvent.Name,
		topEvent.FaultCode,
		topEvent.Description,
	)

	// 收集触发路径
	triggerPath := make([]string, 0)
	e.evaluator.CollectTriggerPath(topEvent, &triggerPath)
	diagnosis.TriggerPath = triggerPath

	// 收集触发的基本事件
	basicEvents := make([]string, 0)
	e.evaluator.CollectTriggeredBasicEvents(topEvent, &basicEvents)
	diagnosis.BasicEvents = basicEvents

	return diagnosis
}

// ResetEvent 重置事件状态
func (e *DiagnosisEngine) ResetEvent(eventID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.stateManager.ResetState(eventID)
	e.logger.Info("事件状态已重置",
		zap.String("event_id", eventID),
	)
}

// ResetAll 重置所有事件状态
func (e *DiagnosisEngine) ResetAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.stateManager.ResetAll()
	e.logger.Info("所有事件状态已重置")
}

// GetStateManager 获取状态管理器（用于测试）
func (e *DiagnosisEngine) GetStateManager() *StateManager {
	return e.stateManager
}
