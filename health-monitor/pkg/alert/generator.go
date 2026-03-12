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
	
	// 根据组件类型调用对应的阈值检查函数
	switch bm.ComponentType {
	case 0x03: // CompPower - 供电服务
		if powerData, ok := bm.Data.(*model.PowerMetrics); ok {
			if sm != nil {
				// 使用有状态的检查（支持恢复告警）
				alerts = CheckPowerThresholdsWithState(powerData, sm)
			} else {
				// 使用无状态的检查（仅触发告警）
				alerts = CheckPowerThresholds(powerData)
			}
		}
		
	case 0x06: // CompThermal - 热控服务
		if thermalData, ok := bm.Data.(*model.ThermalMetrics); ok {
			alerts = CheckThermalThresholds(thermalData)
		}
		
	case 0x02: // CompComm - 通信服务
		if commData, ok := bm.Data.(*model.CommMetrics); ok {
			alerts = CheckCommThresholds(commData)
		}
		
	case 0x0B: // CompActuator - 姿态控制机构
		if actuatorData, ok := bm.Data.(*model.ActuatorMetrics); ok {
			alerts = CheckActuatorThresholds(actuatorData)
		}
		
	// 可以继续添加其他组件类型的处理
	}
	
	// 如果有告警，进行处理和输出
	if len(alerts) > 0 {
		g.outputAlerts(alerts)
	}
}

// ProcessMicroserviceMetrics 处理微服务层指标，生成告警事件
func (g *Generator) ProcessMicroserviceMetrics(ctx context.Context, ms *model.MicroServiceMetricsSet) {
	var alerts []*model.AlertEvent

	sm := g.stateManager
	
	// 1. 阈值告警检查（已经发生的故障）
	// 处理节点指标
	for _, nodeMetrics := range ms.NodeMetrics {
		var nodeAlerts []*model.AlertEvent
		if sm != nil {
			nodeAlerts = CheckNodeThresholdsWithState(&nodeMetrics, sm)
		} else {
			nodeAlerts = CheckNodeThresholds(&nodeMetrics)
		}
		alerts = append(alerts, nodeAlerts...)
	}
	
	// 处理容器指标
	for _, containerMetrics := range ms.ContainerMetrics {
		var containerAlerts []*model.AlertEvent
		if sm != nil {
			containerAlerts = CheckContainerThresholdsWithState(&containerMetrics, sm)
		} else {
			containerAlerts = CheckContainerThresholds(&containerMetrics)
		}
		alerts = append(alerts, containerAlerts...)
	}
	
	// 处理服务指标
	for _, serviceMetrics := range ms.ServiceMetrics {
		var serviceAlerts []*model.AlertEvent
		if sm != nil {
			serviceAlerts = CheckServiceThresholdsWithState(&serviceMetrics, sm)
		} else {
			serviceAlerts = CheckServiceThresholds(&serviceMetrics)
		}
		alerts = append(alerts, serviceAlerts...)
	}
	
	// 2. 如果有告警，进行处理和输出
	if len(alerts) > 0 {
		g.outputAlerts(alerts)
	}
}

// outputAlerts 输出告警事件
func (g *Generator) outputAlerts(alerts []*model.AlertEvent) {
	// 告警压缩：去重和合并
	alerts = g.deduplicateAlerts(alerts)
	
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

// deduplicateAlerts 告警去重
func (g *Generator) deduplicateAlerts(alerts []*model.AlertEvent) []*model.AlertEvent {
	// 简单去重：基于 Source + Type + FaultCode
	seen := make(map[string]bool)
	var result []*model.AlertEvent
	
	for _, alert := range alerts {
		key := fmt.Sprintf("%s-%s-%s", alert.Source, alert.Type, alert.FaultCode)
		if !seen[key] {
			seen[key] = true
			result = append(result, alert)
		}
	}
	
	return result
}