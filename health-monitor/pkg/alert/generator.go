package alert

import (
	"context"
	"fmt"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

// Generator 告警生成器
type Generator struct {
	stateManager *state.StateManager // 状态管理器（用于阈值触发/恢复）
	alertAdapter *AlertAdapter       // 告警适配器（可选，用于直接发送到故障诊断）
}

// NewGenerator 创建新的告警生成器
// sm 可为 nil：nil 时按无状态阈值检查，仅触发告警，不生成恢复告警。
func NewGenerator(sm *state.StateManager) *Generator {
	return &Generator{
		stateManager: sm,
	}
}

// SetDiagnosisReceiver 设置故障诊断接收器（运行时配置）
func (g *Generator) SetDiagnosisReceiver(receiver DiagnosisReceiver) {
	g.alertAdapter = NewAlertAdapter(receiver)
}

// ProcessBusinessMetrics 处理业务层指标，生成告警事件
func (g *Generator) ProcessBusinessMetrics(ctx context.Context, bm *model.BusinessMetrics) {
	var alerts []*model.AlertEvent

	sm := g.stateManager

	// 根据具体数据类型调用对应的阈值检查函数。发布订阅采集不再依赖旧报文组件号。
	switch data := bm.Data.(type) {
	case *model.PowerMetrics:
		if sm != nil {
			// 使用有状态的检查（支持恢复告警）
			alerts = CheckPowerThresholdsWithState(data, sm)
		} else {
			// 使用无状态的检查（仅触发告警）
			alerts = CheckPowerThresholds(data)
		}

	case *model.ThermalMetrics:
		if sm != nil {
			alerts = CheckThermalThresholdsWithState(data, sm)
		} else {
			alerts = CheckThermalThresholds(data)
		}

	case *model.CommMetrics:
		if sm != nil {
			alerts = CheckCommThresholdsWithState(data, sm)
		} else {
			alerts = CheckCommThresholds(data)
		}

	case *model.ActuatorMetrics:
		if sm != nil {
			alerts = CheckActuatorThresholdsWithState(data, sm)
		} else {
			alerts = CheckActuatorThresholds(data)
		}

	case *model.AttitudeOrbitControlMetrics:
		if sm != nil {
			alerts = CheckAttitudeOrbitControlThresholdsWithState(data, sm)
		} else {
			alerts = CheckAttitudeOrbitControlThresholds(data)
		}
	}

	// 如果有告警，进行处理和输出
	if len(alerts) > 0 {
		g.outputAlerts(alerts)
	}
}

// outputAlerts 输出告警事件
func (g *Generator) outputAlerts(alerts []*model.AlertEvent) {
	// 过滤掉恢复告警（resolved状态），只输出 firing 告警
	var firingAlerts []*model.AlertEvent
	for _, alert := range alerts {
		if alert.Status == model.AlertStatusFiring {
			firingAlerts = append(firingAlerts, alert)
		}
	}

	// 如果有 firing 告警，统一输出
	if len(firingAlerts) > 0 {
		fmt.Println("\n========== 告警事件 ==========")
		fmt.Printf("\n【触发告警】共 %d 个:\n", len(firingAlerts))
		for _, alert := range firingAlerts {
			g.printAlert(alert)
		}
		fmt.Println("==============================")
		fmt.Println()
	}

	// 发送告警到故障诊断模块（如果已配置）
	if g.alertAdapter != nil {
		if err := g.alertAdapter.SendAlerts(alerts); err != nil {
			fmt.Printf("发送告警到故障诊断模块失败: %v\n", err)
		} else {
			fmt.Printf("已发送 %d 个告警到故障诊断模块\n", len(alerts))
		}
	}

	// TODO: 这里还可以将告警发送到：
	// 1. 消息队列 (MQ / etcd)
	// 2. 数据库
	// 3. 可视化平台
	// 4. 告警通知系统（邮件、短信等）
}

// printAlert 打印单个告警
func (g *Generator) printAlert(alert *model.AlertEvent) {
	serviceName := ""
	if alert.Metadata != nil {
		if v, ok := alert.Metadata["serviceName"].(string); ok {
			serviceName = v
		}
	}
	fmt.Printf("  [%s] %s\n", alert.AlertID, alert.Type)
	fmt.Printf("    故障码: %s\n", alert.FaultCode)
	fmt.Printf("    来源: %s\n", alert.Source)
	if serviceName != "" {
		fmt.Printf("    服务名: %s\n", serviceName)
	}
	fmt.Printf("    消息: %s\n", alert.Message)
	fmt.Printf("    指标值: %.2f\n", alert.MetricValue)
	fmt.Printf("    时间戳: %d\n\n", alert.Timestamp)
}
