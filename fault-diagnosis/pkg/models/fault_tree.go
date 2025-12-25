package models

// GateType 逻辑门类型
type GateType string

const (
	GateAND   GateType = "AND"   // 与门：所有输入为真时输出为真
	GateOR    GateType = "OR"    // 或门：任一输入为真时输出为真
	GateNOT   GateType = "NOT"   // 非门：输入为假时输出为真
	GateBASIC GateType = "BASIC" // 基本事件，无逻辑门
)

// FaultTree 故障树配置
type FaultTree struct {
	FaultTreeID        string               `json:"fault_tree_id"`        // 故障树唯一标识
	Description        string               `json:"description"`          // 描述
	TopEvents          []Event              `json:"top_events"`           // 顶层事件列表
	IntermediateEvents []Event              `json:"intermediate_events"`  // 中间事件列表
	BasicEvents        []BasicEvent         `json:"basic_events"`         // 基本事件列表
}

// Event 事件节点（顶层事件或中间事件）
type Event struct {
	EventID     string   `json:"event_id"`     // 事件唯一标识
	Name        string   `json:"name"`         // 事件名称
	Description string   `json:"description"`  // 事件描述
	FaultCode   string   `json:"fault_code"`   // 故障码（仅顶层事件）
	GateType    GateType `json:"gate_type"`    // 逻辑门类型
	Children    []string `json:"children"`     // 子事件ID列表
}

// BasicEvent 基本事件（叶子节点）
type BasicEvent struct {
	EventID     string `json:"event_id"`     // 事件唯一标识
	Name        string `json:"name"`         // 事件名称
	Description string `json:"description"`  // 事件描述
	AlertID     string `json:"alert_id"`     // 对应的告警ID（用于映射）
}

// EventNode 事件节点运行时结构（用于求值）
type EventNode struct {
	EventID     string      // 事件ID
	Name        string      // 事件名称
	Description string      // 描述
	FaultCode   string      // 故障码（仅顶层事件）
	GateType    GateType    // 逻辑门类型
	Children    []*EventNode // 子节点
	State       EventState  // 当前状态
	IsBasic     bool        // 是否为基本事件
	AlertID     string      // 对应的告警ID（基本事件）
}

// NewEventNode 创建事件节点
func NewEventNode(eventID, name string, gateType GateType) *EventNode {
	return &EventNode{
		EventID:  eventID,
		Name:     name,
		GateType: gateType,
		State:    StateFalse,
		Children: make([]*EventNode, 0),
	}
}

// AddChild 添加子节点
func (n *EventNode) AddChild(child *EventNode) {
	n.Children = append(n.Children, child)
}

// SetState 设置状态
func (n *EventNode) SetState(state EventState) {
	n.State = state
}

// GetState 获取状态
func (n *EventNode) GetState() EventState {
	return n.State
}
