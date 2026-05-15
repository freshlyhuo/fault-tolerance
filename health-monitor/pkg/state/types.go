package state

import (
	"health-monitor/pkg/models"
)

// Metric 业务指标接口。
type Metric interface {
	GetID() string        // 获取唯一标识
	GetTimestamp() int64  // 获取时间戳
	GetData() interface{} // 获取具体数据
}

// BusinessMetric 业务层指标包装
type BusinessMetric struct {
	Data      *model.BusinessMetrics
	Timestamp int64
}

func (m *BusinessMetric) GetID() string {
	switch m.Data.Data.(type) {
	case *model.PowerMetrics:
		return "power"
	case *model.ThermalMetrics:
		return "thermal"
	case *model.CommMetrics:
		return "comm"
	case *model.ActuatorMetrics:
		return "momentum_wheel"
	case *model.AttitudeOrbitControlMetrics:
		return "attitude_orbit_control"
	default:
		return "unknown"
	}
}
func (m *BusinessMetric) GetTimestamp() int64  { return m.Timestamp }
func (m *BusinessMetric) GetData() interface{} { return m.Data }
