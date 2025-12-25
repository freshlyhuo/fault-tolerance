/* 功能：

整合所有判断（阈值 / 趋势 / 时空关联）

进行告警压缩：重复合并

进行优先级提升：关键服务优先

输出最终告警事件

最终输出：
可以分成——
业务层故障事件和微服务层故障事件
业务层故障分级按照故障表分级
微服务层故障分为两级
1.已经发生故障，需要立即干预
2.有指标趋势，如波动，持续增长

[]AlertEvent → 故障诊断模块。 */
package alert

import (
	"context"
	"fmt"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

// Generator 告警生成器
type Generator struct {
	trendAnalyzer *TrendAnalyzer // 趋势分析器
}

// NewGenerator 创建新的告警生成器
func NewGenerator() *Generator {
	return &Generator{}
}

// NewGeneratorWithStateManager 创建带状态管理的告警生成器
func NewGeneratorWithStateManager(sm *state.StateManager) *Generator {
	return &Generator{
		trendAnalyzer: NewTrendAnalyzer(sm),
	}
}

// ProcessBusinessMetrics 处理业务层指标，生成告警事件
func (g *Generator) ProcessBusinessMetrics(ctx context.Context, bm *model.BusinessMetrics) {
	var alerts []*model.AlertEvent
	
	// 根据组件类型调用对应的阈值检查函数
	switch bm.ComponentType {
	case 0x03: // CompPower - 供电服务
		if powerData, ok := bm.Data.(*model.PowerMetrics); ok {
			alerts = CheckPowerThresholds(powerData)
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
	
	// 1. 阈值告警检查（已经发生的故障）
	// 处理节点指标
	for _, nodeMetrics := range ms.NodeMetrics {
		nodeAlerts := CheckNodeThresholds(&nodeMetrics)
		alerts = append(alerts, nodeAlerts...)
	}
	
	// 处理容器指标
	for _, containerMetrics := range ms.ContainerMetrics {
		containerAlerts := CheckContainerThresholds(&containerMetrics)
		alerts = append(alerts, containerAlerts...)
	}
	
	// 处理服务指标
	for _, serviceMetrics := range ms.ServiceMetrics {
		serviceAlerts := CheckServiceThresholds(&serviceMetrics)
		alerts = append(alerts, serviceAlerts...)
	}
	
	// 2. 趋势告警检查（即将发生的故障）
	if g.trendAnalyzer != nil {
		// 分析节点趋势
		for _, nodeMetrics := range ms.NodeMetrics {
			trendAlerts := g.trendAnalyzer.AnalyzeNodeTrends(ctx, nodeMetrics.ID)
			alerts = append(alerts, trendAlerts...)
		}
		
		// 分析容器趋势
		for _, containerMetrics := range ms.ContainerMetrics {
			trendAlerts := g.trendAnalyzer.AnalyzeContainerTrends(ctx, containerMetrics.ID)
			alerts = append(alerts, trendAlerts...)
		}
		
		// 分析服务趋势
		for _, serviceMetrics := range ms.ServiceMetrics {
			trendAlerts := g.trendAnalyzer.AnalyzeServiceTrends(ctx, serviceMetrics.ID)
			alerts = append(alerts, trendAlerts...)
		}
	}
	
	// 3. 如果有告警，进行处理和输出
	if len(alerts) > 0 {
		g.outputAlerts(alerts)
	}
}

// outputAlerts 输出告警事件
func (g *Generator) outputAlerts(alerts []*model.AlertEvent) {
	// 告警压缩：去重和合并
	alerts = g.deduplicateAlerts(alerts)
	
	// 按严重程度分类
	var critical, warning, info []*model.AlertEvent
	for _, alert := range alerts {
		switch alert.Severity {
		case model.SeverityCritical:
			critical = append(critical, alert)
		case model.SeverityWarning:
			warning = append(warning, alert)
		case model.SeverityInfo:
			info = append(info, alert)
		}
	}
	
	// 输出告警
	fmt.Println("\n========== 告警事件 ==========")
	
	if len(critical) > 0 {
		fmt.Printf("\n【严重告警】共 %d 个:\n", len(critical))
		for _, alert := range critical {
			g.printAlert(alert)
		}
	}
	
	if len(warning) > 0 {
		fmt.Printf("\n【警告告警】共 %d 个:\n", len(warning))
		for _, alert := range warning {
			g.printAlert(alert)
		}
	}
	
	if len(info) > 0 {
		fmt.Printf("\n【信息告警】共 %d 个:\n", len(info))
		for _, alert := range info {
			g.printAlert(alert)
		}
	}
	
	fmt.Println("==============================\n")
	
	// TODO: 这里可以将告警发送到：
	// 1. 消息队列 (MQ)
	// 2. 数据库
	// 3. 可视化平台
	// 4. 故障诊断模块
	// 5. 告警通知系统（邮件、短信等）
}

// printAlert 打印单个告警
func (g *Generator) printAlert(alert *model.AlertEvent) {
	fmt.Printf("  [%s] %s\n", alert.AlertID, alert.Type)
	fmt.Printf("    故障码: %s\n", alert.FaultCode)
	fmt.Printf("    来源: %s\n", alert.Source)
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