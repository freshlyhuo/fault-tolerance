package engine

import (
	"fmt"

	"fault-diagnosis/pkg/models"
)

// Evaluator 故障树求值器
type Evaluator struct {
	stateManager *StateManager
}

// NewEvaluator 创建求值器
func NewEvaluator(stateManager *StateManager) *Evaluator {
	return &Evaluator{
		stateManager: stateManager,
	}
}

// EvaluateNode 求值单个节点（递归）
// 返回节点的当前状态
func (e *Evaluator) EvaluateNode(node *models.EventNode) models.EventState {
	// 基本事件：直接从状态管理器获取
	if node.IsBasic {
		state := e.stateManager.GetState(node.EventID)
		node.SetState(state)
		return state
	}

	// 中间事件或顶层事件：根据逻辑门和子节点状态计算
	return e.evaluateGate(node)
}

// evaluateGate 根据逻辑门类型求值
func (e *Evaluator) evaluateGate(node *models.EventNode) models.EventState {
	if len(node.Children) == 0 {
		// 没有子节点，状态为假
		node.SetState(models.StateFalse)
		return models.StateFalse
	}

	switch node.GateType {
	case models.GateAND:
		return e.evaluateAND(node)
	case models.GateOR:
		return e.evaluateOR(node)
	case models.GateNOT:
		return e.evaluateNOT(node)
	case models.GateBASIC:
		// BASIC门类型通常用于标记中间事件直接关联一个基本事件
		if len(node.Children) > 0 {
			return e.EvaluateNode(node.Children[0])
		}
		return models.StateFalse
	default:
		fmt.Printf("警告：未知的逻辑门类型 %s，默认为FALSE\n", node.GateType)
		node.SetState(models.StateFalse)
		return models.StateFalse
	}
}

// evaluateAND 与门求值：所有子节点为真时才为真
func (e *Evaluator) evaluateAND(node *models.EventNode) models.EventState {
	for _, child := range node.Children {
		childState := e.EvaluateNode(child)
		if childState != models.StateTrue {
			// 任一子节点为假，整体为假
			node.SetState(models.StateFalse)
			return models.StateFalse
		}
	}
	// 所有子节点为真
	node.SetState(models.StateTrue)
	return models.StateTrue
}

// evaluateOR 或门求值：任一子节点为真时即为真
func (e *Evaluator) evaluateOR(node *models.EventNode) models.EventState {
	for _, child := range node.Children {
		childState := e.EvaluateNode(child)
		if childState == models.StateTrue {
			// 任一子节点为真，整体为真
			node.SetState(models.StateTrue)
			return models.StateTrue
		}
	}
	// 所有子节点为假
	node.SetState(models.StateFalse)
	return models.StateFalse
}

// evaluateNOT 非门求值：子节点为假时为真
func (e *Evaluator) evaluateNOT(node *models.EventNode) models.EventState {
	if len(node.Children) == 0 {
		node.SetState(models.StateFalse)
		return models.StateFalse
	}

	// 非门只考虑第一个子节点
	childState := e.EvaluateNode(node.Children[0])
	if childState == models.StateFalse {
		node.SetState(models.StateTrue)
		return models.StateTrue
	}
	node.SetState(models.StateFalse)
	return models.StateFalse
}

// EvaluateTree 求值整个故障树
// 返回所有被触发（状态为真）的顶层事件
func (e *Evaluator) EvaluateTree(topEvents []*models.EventNode) []*models.EventNode {
	triggeredTopEvents := make([]*models.EventNode, 0)

	for _, topEvent := range topEvents {
		state := e.EvaluateNode(topEvent)
		if state == models.StateTrue {
			triggeredTopEvents = append(triggeredTopEvents, topEvent)
		}
	}

	return triggeredTopEvents
}

// CollectTriggerPath 收集触发路径（从顶层事件到基本事件）
func (e *Evaluator) CollectTriggerPath(node *models.EventNode, path *[]string) {
	if node.GetState() != models.StateTrue {
		return
	}

	*path = append(*path, node.EventID)

	// 递归收集子节点
	for _, child := range node.Children {
		e.CollectTriggerPath(child, path)
	}
}

// CollectTriggeredBasicEvents 收集所有触发的基本事件
func (e *Evaluator) CollectTriggeredBasicEvents(node *models.EventNode, basicEvents *[]string) {
	if node.GetState() != models.StateTrue {
		return
	}

	if node.IsBasic {
		*basicEvents = append(*basicEvents, node.EventID)
		return
	}

	// 递归收集子节点
	for _, child := range node.Children {
		e.CollectTriggeredBasicEvents(child, basicEvents)
	}
}
