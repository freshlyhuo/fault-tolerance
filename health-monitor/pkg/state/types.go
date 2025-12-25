/* 节点、容器、业务、服务状态结构
ID 命名规范（node-id / container-id / service-id / business-component-type） */
package state

import (
	"health-monitor/pkg/models"
)

// MetricType 指标类型
type MetricType string

const (
	MetricTypeNode      MetricType = "node"
	MetricTypeContainer MetricType = "container"
	MetricTypeService   MetricType = "service"
	MetricTypeBusiness  MetricType = "business"
)

// Metric 统一的指标接口
type Metric interface {
	GetID() string        // 获取唯一标识
	GetType() MetricType  // 获取指标类型
	GetTimestamp() int64  // 获取时间戳
	GetData() interface{} // 获取具体数据
}

// NodeMetric 节点指标包装
type NodeMetric struct {
	Data      *model.NodeMetrics
	Timestamp int64
}

func (m *NodeMetric) GetID() string        { return m.Data.ID }
func (m *NodeMetric) GetType() MetricType  { return MetricTypeNode }
func (m *NodeMetric) GetTimestamp() int64  { return m.Timestamp }
func (m *NodeMetric) GetData() interface{} { return m.Data }

// ContainerMetric 容器指标包装
type ContainerMetric struct {
	Data      *model.ContainerMetrics
	Timestamp int64
}

func (m *ContainerMetric) GetID() string        { return m.Data.ID }
func (m *ContainerMetric) GetType() MetricType  { return MetricTypeContainer }
func (m *ContainerMetric) GetTimestamp() int64  { return m.Timestamp }
func (m *ContainerMetric) GetData() interface{} { return m.Data }

// ServiceMetric 服务指标包装
type ServiceMetric struct {
	Data      *model.ServiceMetrics
	Timestamp int64
}

func (m *ServiceMetric) GetID() string        { return m.Data.ID }
func (m *ServiceMetric) GetType() MetricType  { return MetricTypeService }
func (m *ServiceMetric) GetTimestamp() int64  { return m.Timestamp }
func (m *ServiceMetric) GetData() interface{} { return m.Data }

// BusinessMetric 业务层指标包装
type BusinessMetric struct {
	Data      *model.BusinessMetrics
	Timestamp int64
}

func (m *BusinessMetric) GetID() string {
	// 使用组件类型作为ID
	return string(rune(m.Data.ComponentType))
}
func (m *BusinessMetric) GetType() MetricType  { return MetricTypeBusiness }
func (m *BusinessMetric) GetTimestamp() int64  { return m.Timestamp }
func (m *BusinessMetric) GetData() interface{} { return m.Data }

// StateSnapshot 状态快照
type StateSnapshot struct {
	Timestamp int64                  `json:"timestamp"`
	Nodes     []model.NodeMetrics    `json:"nodes"`
	Containers []model.ContainerMetrics `json:"containers"`
	Services  []model.ServiceMetrics `json:"services"`
	Business  []model.BusinessMetrics `json:"business"`
}