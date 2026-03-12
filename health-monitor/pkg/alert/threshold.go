/*
阈值检查统一实现：
1. 传入 StateManager 时，按状态变化输出 firing/resolved。
2. 不传 StateManager（启动初期/无状态模式）时，只输出 firing。
*/
package alert

import (
	"fmt"
	"time"

	"health-monitor/pkg/config"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

func valueOrDefault(v *float64, def float64) float64 {
	if v == nil {
		return def
	}
	return *v
}

func intOrDefault(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

func rangeOrDefault(t config.MetricThreshold, defMin, defMax float64) (float64, float64) {
	return valueOrDefault(t.Min, defMin), valueOrDefault(t.Max, defMax)
}

func shouldSendByState(sm *state.StateManager, alertID, source string, isFiring, bySource bool) (bool, bool) {
	if sm == nil {
		if isFiring {
			return true, true
		}
		return false, false
	}

	if bySource {
		return sm.CheckAndUpdateAlertStateWithSource(alertID, source, isFiring)
	}
	return sm.CheckAndUpdateAlertState(alertID, isFiring)
}

func appendAlert(
	alerts []*model.AlertEvent,
	sm *state.StateManager,
	alertID, source string,
	isFiring, bySource bool,
	alertType, faultCode, firingMsg, resolvedMsg string,
	metricValue float64,
	metadata map[string]interface{},
	timestamp int64,
) []*model.AlertEvent {
	shouldSend, firing := shouldSendByState(sm, alertID, source, isFiring, bySource)
	if !shouldSend {
		return alerts
	}

	status := model.AlertStatusResolved
	message := resolvedMsg
	if firing {
		status = model.AlertStatusFiring
		message = firingMsg
	}

	return append(alerts, &model.AlertEvent{
		AlertID:     alertID,
		Type:        alertType,
		Status:      status,
		Source:      source,
		Message:     message,
		Timestamp:   timestamp,
		FaultCode:   faultCode,
		MetricValue: metricValue,
		Metadata:    metadata,
	})
}

func nodeCPUUsage(metrics *model.NodeMetrics) (float64, bool) {
	switch v := metrics.CPUUsage.(type) {
	case float64:
		return v, true
	case model.CPUUsage:
		return v.Total, true
	default:
		return 0, false
	}
}

func containerMetadata(metrics *model.ContainerMetrics) map[string]interface{} {
	if metrics.ServiceName == "" && metrics.ServiceID == "" {
		return nil
	}
	md := map[string]interface{}{}
	if metrics.ServiceName != "" {
		md["serviceName"] = metrics.ServiceName
	}
	if metrics.ServiceID != "" {
		md["serviceId"] = metrics.ServiceID
	}
	return md
}

// CheckPowerThresholds 检查供电服务阈值（无状态入口）
func CheckPowerThresholds(metrics *model.PowerMetrics) []*model.AlertEvent {
	return CheckPowerThresholdsWithState(metrics, nil)
}

// CheckPowerThresholdsWithState 检查供电服务阈值（支持恢复告警）
func CheckPowerThresholdsWithState(metrics *model.PowerMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}

	tc := config.GetThresholdConfig()
	now := time.Now().Unix()
	var alerts []*model.AlertEvent
	p12Min, p12Max := rangeOrDefault(tc.Power.PowerModule12V, 12.5, 13.5)
	batMin, batMax := rangeOrDefault(tc.Power.BatteryVoltage, 21.0, 29.4)
	cpuMin, cpuMax := rangeOrDefault(tc.Power.CPUVoltage, 3.1, 3.5)
	loadMin, loadMax := rangeOrDefault(tc.Power.LoadCurrent, 0.5, 5.0)

	isFiring := metrics.PowerModule12V < p12Min || metrics.PowerModule12V > p12Max
	alerts = appendAlert(alerts, sm,
		"POWER_12V_ALERT", "PowerModule12V", isFiring, false,
		"voltage_abnormal", "CJB-RG-ZD-1",
		fmt.Sprintf("12V功率模块电压异常: %.2fV (正常[%.2f,%.2f]V)", metrics.PowerModule12V, p12Min, p12Max),
		fmt.Sprintf("12V功率模块电压已恢复正常: %.2fV", metrics.PowerModule12V),
		metrics.PowerModule12V, nil, now)

	isFiring = metrics.BatteryVoltage < batMin || metrics.BatteryVoltage > batMax
	alerts = appendAlert(alerts, sm,
		"BATTERY_VOLTAGE_ALERT", "BatteryVoltage", isFiring, false,
		"voltage_abnormal", "CJB-RG-ZD-3",
		fmt.Sprintf("蓄电池电压异常: %.2fV (正常[%.2f,%.2f]V)", metrics.BatteryVoltage, batMin, batMax),
		fmt.Sprintf("蓄电池电压已恢复正常: %.2fV", metrics.BatteryVoltage),
		metrics.BatteryVoltage, nil, now)

	isFiring = metrics.CPUVoltage < cpuMin || metrics.CPUVoltage > cpuMax
	alerts = appendAlert(alerts, sm,
		"CPU_VOLTAGE_ALERT", "CPUVoltage", isFiring, false,
		"voltage_abnormal", "CJB-RG-ZD-3",
		fmt.Sprintf("CPU板电压异常: %.2fV (正常[%.2f,%.2f]V)", metrics.CPUVoltage, cpuMin, cpuMax),
		fmt.Sprintf("CPU板电压已恢复正常: %.2fV", metrics.CPUVoltage),
		metrics.CPUVoltage, nil, now)

	isFiring = metrics.LoadCurrent < loadMin || metrics.LoadCurrent > loadMax
	alerts = appendAlert(alerts, sm,
		"LOAD_CURRENT_ALERT", "LoadCurrent", isFiring, false,
		"current_abnormal", "CJB-O2-CS-1",
		fmt.Sprintf("负载电流异常: %.2fA (正常[%.2f,%.2f]A)", metrics.LoadCurrent, loadMin, loadMax),
		fmt.Sprintf("负载电流已恢复正常: %.2fA", metrics.LoadCurrent),
		metrics.LoadCurrent, nil, now)

	return alerts
}

// CheckThermalThresholds 检查热控服务阈值（当前仅触发告警）
func CheckThermalThresholds(metrics *model.ThermalMetrics) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}
	tc := config.GetThresholdConfig()
	var alerts []*model.AlertEvent
	thermMin, thermMax := rangeOrDefault(config.MetricThreshold{}, -20.0, 50.0)
	if len(tc.Thermal.ThermalTemps) > 0 {
		thermMin, thermMax = rangeOrDefault(config.MetricThreshold{Min: tc.Thermal.ThermalTemps[0].Min, Max: tc.Thermal.ThermalTemps[0].Max}, -20.0, 50.0)
	}
	bat1Min, bat1Max := rangeOrDefault(tc.Thermal.BatteryTemp1, 0.0, 45.0)

	for i, temp := range metrics.ThermalTemps {
		if temp < thermMin || temp > thermMax {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("THERM-TEMP%d-%d", i+1, time.Now().Unix()),
				Type:        "TemperatureAbnormal",
				Status:      model.AlertStatusFiring,
				Source:      fmt.Sprintf("ThermalTemp%d", i+1),
				Message:     fmt.Sprintf("热控温度%d异常: %.1f℃", i+1, temp),
				Timestamp:   metrics.Timestamp,
				FaultCode:   "CJB-RG-ZD-4",
				MetricValue: temp,
			})
		}
	}

	if metrics.BatteryTemp1 < bat1Min || metrics.BatteryTemp1 > bat1Max {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("THERM-BAT1-%d", time.Now().Unix()),
			Type:        "TemperatureAbnormal",
			Status:      model.AlertStatusFiring,
			Source:      "BatteryTemp1",
			Message:     fmt.Sprintf("蓄电池温度1异常: %.1f℃", metrics.BatteryTemp1),
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-RG-ZD-4",
			MetricValue: metrics.BatteryTemp1,
		})
	}

	return alerts
}

// CheckCommThresholds 检查通信服务阈值（当前仅触发告警）
func CheckCommThresholds(metrics *model.CommMetrics) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}
	tc := config.GetThresholdConfig()
	var alerts []*model.AlertEvent
	canExpected := intOrDefault(tc.Comm.CANStatus.Value, 1)
	serialExpected := intOrDefault(tc.Comm.SerialStatus.Value, 1)

	if int(metrics.CANStatus) != canExpected {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("COMM-CAN-%d", time.Now().Unix()),
			Type:        "CommunicationFailure",
			Status:      model.AlertStatusFiring,
			Source:      "CANStatus",
			Message:     "CAN通信无应答",
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-RG-ZD-2",
			MetricValue: float64(metrics.CANStatus),
		})
	}

	if int(metrics.SerialStatus) != serialExpected {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("COMM-SERIAL-%d", time.Now().Unix()),
			Type:        "CommunicationFailure",
			Status:      model.AlertStatusFiring,
			Source:      "SerialStatus",
			Message:     "串口通信无遥测",
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-O2-CS-1",
			MetricValue: float64(metrics.SerialStatus),
		})
	}

	return alerts
}

// CheckActuatorThresholds 检查姿态控制机构阈值（当前仅触发告警）
func CheckActuatorThresholds(metrics *model.ActuatorMetrics) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}
	tc := config.GetThresholdConfig()
	var alerts []*model.AlertEvent

	expectedX := valueOrDefault(tc.Actuator.WheelSpeed.X.Value, 100)
	expectedY := valueOrDefault(tc.Actuator.WheelSpeed.Y.Value, 100)
	expectedZ := valueOrDefault(tc.Actuator.WheelSpeed.Z.Value, 100)
	tolerance := valueOrDefault(tc.Actuator.WheelSpeedTolerance, 10)

	checkWheel := func(speed int16, axis string, expected float64) {
		s := float64(speed)
		if s < expected-tolerance || s > expected+tolerance {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("ACTUATOR-%s-%d", axis, time.Now().Unix()),
				Type:        "ActuatorAbnormal",
				Status:      model.AlertStatusFiring,
				Source:      fmt.Sprintf("WheelSpeed%s", axis),
				Message:     fmt.Sprintf("%s轴动量轮转速异常: %d (期望约100转)", axis, speed),
				Timestamp:   metrics.Timestamp,
				FaultCode:   "CJB-O2-CS-16",
				MetricValue: float64(speed),
			})
		}
	}

	checkWheel(metrics.WheelSpeedX, "X", expectedX)
	checkWheel(metrics.WheelSpeedY, "Y", expectedY)
	checkWheel(metrics.WheelSpeedZ, "Z", expectedZ)

	return alerts
}

// CheckNodeThresholds 检查节点指标阈值（无状态入口）
func CheckNodeThresholds(metrics *model.NodeMetrics) []*model.AlertEvent {
	return CheckNodeThresholdsWithState(metrics, nil)
}

// CheckNodeThresholdsWithState 检查节点指标（支持恢复告警）
func CheckNodeThresholdsWithState(metrics *model.NodeMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}

	tc := config.GetThresholdConfig()
	now := time.Now().Unix()
	var alerts []*model.AlertEvent

	isFiring := metrics.Status != "online"
	alerts = appendAlert(alerts, sm,
		"NODE_OFFLINE", metrics.ID, isFiring, true,
		"node_offline", "MS-NO-FL-1",
		fmt.Sprintf("节点 %s 离线", metrics.ID),
		fmt.Sprintf("节点 %s 已恢复在线", metrics.ID),
		0, nil, now)

	if cpuUsage, ok := nodeCPUUsage(metrics); ok {
		isFiring = cpuUsage > tc.Node.CPUUsageMax
		alerts = appendAlert(alerts, sm,
			"NODE_CPU_HIGH", metrics.ID, isFiring, true,
			"cpu_high", "MS-NO-FL-2",
			fmt.Sprintf("节点 %s CPU使用率过高: %.1f%%", metrics.ID, cpuUsage),
			fmt.Sprintf("节点 %s CPU使用率已恢复: %.1f%%", metrics.ID, cpuUsage),
			cpuUsage, nil, now)
	}

	if metrics.MemoryTotal > 0 {
		memoryPercent := float64(metrics.MemoryTotal-metrics.MemoryFree) / float64(metrics.MemoryTotal) * 100
		isFiring = memoryPercent > tc.Node.MemoryUsageMax
		alerts = appendAlert(alerts, sm,
			"NODE_MEMORY_HIGH", metrics.ID, isFiring, true,
			"memory_high", "MS-NO-FL-3",
			fmt.Sprintf("节点 %s 内存使用率过高: %.1f%%", metrics.ID, memoryPercent),
			fmt.Sprintf("节点 %s 内存使用率已恢复: %.1f%%", metrics.ID, memoryPercent),
			memoryPercent, nil, now)
	}

	if metrics.DiskTotal > 0 {
		diskPercent := (metrics.DiskTotal - metrics.DiskFree) / metrics.DiskTotal * 100
		isFiring = diskPercent > tc.Node.DiskUsageMax
		alerts = appendAlert(alerts, sm,
			"NODE_DISK_HIGH", metrics.ID, isFiring, true,
			"disk_high", "MS-NO-FL-4",
			fmt.Sprintf("节点 %s 磁盘使用率过高: %.1f%%", metrics.ID, diskPercent),
			fmt.Sprintf("节点 %s 磁盘使用率已恢复: %.1f%%", metrics.ID, diskPercent),
			diskPercent, nil, now)
	}

	return alerts
}

// CheckContainerThresholds 检查容器指标阈值（无状态入口）
func CheckContainerThresholds(metrics *model.ContainerMetrics) []*model.AlertEvent {
	return CheckContainerThresholdsWithState(metrics, nil)
}

// CheckContainerThresholdsWithState 检查容器指标（支持恢复告警）
func CheckContainerThresholdsWithState(metrics *model.ContainerMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}

	tc := config.GetThresholdConfig()
	now := time.Now().Unix()
	md := containerMetadata(metrics)
	var alerts []*model.AlertEvent

	isFiring := metrics.DeployStatus != "success"
	alerts = appendAlert(alerts, sm,
		"CONTAINER_DEPLOY_FAIL", metrics.ID, isFiring, true,
		"deployment_failure", "MS-CN-FL-1",
		fmt.Sprintf("容器 %s 部署失败: %s", metrics.ID, metrics.DeployStatus),
		fmt.Sprintf("容器 %s 部署状态已恢复: %s", metrics.ID, metrics.DeployStatus),
		0, md, now)

	cpuUsage := metrics.CPUUsage.Total
	isFiring = cpuUsage > tc.Container.CPUUsageMax
	alerts = appendAlert(alerts, sm,
		"CONTAINER_CPU_HIGH", metrics.ID, isFiring, true,
		"cpu_high", "MS-CN-FL-5",
		fmt.Sprintf("容器 %s CPU使用率过高: %.1f%%", metrics.ID, cpuUsage),
		fmt.Sprintf("容器 %s CPU使用率已恢复: %.1f%%", metrics.ID, cpuUsage),
		cpuUsage, md, now)

	if metrics.MemoryLimit > 0 {
		memoryPercent := float64(metrics.MemoryUsage) / float64(metrics.MemoryLimit) * 100
		isFiring = memoryPercent > tc.Container.MemoryUsageMax
		alerts = appendAlert(alerts, sm,
			"CONTAINER_MEMORY_HIGH", metrics.ID, isFiring, true,
			"memory_high", "MS-CN-FL-5",
			fmt.Sprintf("容器 %s 内存使用率过高: %.1f%%", metrics.ID, memoryPercent),
			fmt.Sprintf("容器 %s 内存使用率已恢复: %.1f%%", metrics.ID, memoryPercent),
			memoryPercent, md, now)
	}

	if metrics.SizeLimit > 0 {
		diskPercent := float64(metrics.SizeUsage) / float64(metrics.SizeLimit) * 100
		isFiring = diskPercent > tc.Container.DiskUsageMax
		alerts = appendAlert(alerts, sm,
			"CONTAINER_DISK_HIGH", metrics.ID, isFiring, true,
			"disk_high", "MS-CN-FL-6",
			fmt.Sprintf("容器 %s 磁盘占用率过高: %.1f%%", metrics.ID, diskPercent),
			fmt.Sprintf("容器 %s 磁盘占用率已恢复: %.1f%%", metrics.ID, diskPercent),
			diskPercent, md, now)
	}

	return alerts
}

// CheckServiceThresholds 检查服务指标阈值（无状态入口）
func CheckServiceThresholds(metrics *model.ServiceMetrics) []*model.AlertEvent {
	return CheckServiceThresholdsWithState(metrics, nil)
}

// CheckServiceThresholdsWithState 检查服务指标（支持恢复告警）
func CheckServiceThresholdsWithState(metrics *model.ServiceMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}

	tc := config.GetThresholdConfig()
	now := time.Now().Unix()
	var alerts []*model.AlertEvent
	requireHealthy := tc.Service.RequireHealthy
	instanceOnlineMin := tc.Service.InstanceOnlineMin
	if instanceOnlineMin <= 0 {
		instanceOnlineMin = 1
	}

	isFiring := requireHealthy && !metrics.Healthy
	alerts = appendAlert(alerts, sm,
		"SERVICE_UNHEALTHY", metrics.ID, isFiring, true,
		"service_unhealthy", "MS-SV-FL-1",
		fmt.Sprintf("服务 %s 健康检查失败", metrics.ID),
		fmt.Sprintf("服务 %s 健康检查已恢复", metrics.ID),
		0, nil, now)

	isFiring = metrics.InstanceOnline < instanceOnlineMin
	alerts = appendAlert(alerts, sm,
		"SERVICE_NO_ONLINE_NODES", metrics.ID, isFiring, true,
		"no_online_nodes", "MS-SV-FL-5",
		fmt.Sprintf("服务 %s 无在线节点", metrics.ID),
		fmt.Sprintf("服务 %s 在线节点已恢复", metrics.ID),
		0, nil, now)

	return alerts
}
