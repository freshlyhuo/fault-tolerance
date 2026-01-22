package alert

import (
	"fmt"
	"time"

	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

// CheckPowerThresholdsWithState 检查供电服务阈值（支持恢复告警）
func CheckPowerThresholdsWithState(metrics *model.PowerMetrics, sm *state.StateManager) []*model.AlertEvent {
	var alerts []*model.AlertEvent

	// 蓄电池电压检查 (正常[21, 29.4]V)
	isFiring := metrics.BatteryVoltage < 21.0 || metrics.BatteryVoltage > 29.4
	shouldSend, firing := sm.CheckAndUpdateAlertState("BATTERY_VOLTAGE_ALERT", isFiring)
	
	if shouldSend {
		severity := model.SeverityCritical
		if metrics.BatteryVoltage >= 20.0 && metrics.BatteryVoltage < 21.0 {
			severity = model.SeverityWarning
		}
		
		var alert *model.AlertEvent
		if firing {
			// 触发告警
			alert = &model.AlertEvent{
				AlertID:     "BATTERY_VOLTAGE_ALERT",
				Type:        "voltage_abnormal",
				Status:      model.AlertStatusFiring,
				Severity:    severity,
				Source:      "battery_monitor",
				Message:     fmt.Sprintf("蓄电池电压异常: %.2fV (正常[21,29.4]V)", metrics.BatteryVoltage),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "CJB-RG-ZD-3",
				MetricValue: metrics.BatteryVoltage,
			}
		} else {
			// 恢复告警
			alert = &model.AlertEvent{
				AlertID:     "BATTERY_VOLTAGE_ALERT",
				Type:        "voltage_abnormal",
				Status:      model.AlertStatusResolved,
				Severity:    model.SeverityInfo,
				Source:      "battery_monitor",
				Message:     fmt.Sprintf("蓄电池电压已恢复正常: %.2fV", metrics.BatteryVoltage),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "CJB-RG-ZD-3",
				MetricValue: metrics.BatteryVoltage,
			}
		}
		alerts = append(alerts, alert)
	}

	// CPU板电压检查 (正常[3.1, 3.5]V)
	isFiring = metrics.CPUVoltage < 3.1 || metrics.CPUVoltage > 3.5
	shouldSend, firing = sm.CheckAndUpdateAlertState("CPU_VOLTAGE_ALERT", isFiring)
	
	if shouldSend {
		var alert *model.AlertEvent
		if firing {
			alert = &model.AlertEvent{
				AlertID:     "CPU_VOLTAGE_ALERT",
				Type:        "voltage_abnormal",
				Status:      model.AlertStatusFiring,
				Severity:    model.SeverityCritical,
				Source:      "cpu_board_monitor",
				Message:     fmt.Sprintf("CPU板电压异常: %.2fV (正常[3.1,3.5]V)", metrics.CPUVoltage),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "CJB-RG-ZD-3",
				MetricValue: metrics.CPUVoltage,
			}
		} else {
			alert = &model.AlertEvent{
				AlertID:     "CPU_VOLTAGE_ALERT",
				Type:        "voltage_abnormal",
				Status:      model.AlertStatusResolved,
				Severity:    model.SeverityInfo,
				Source:      "cpu_board_monitor",
				Message:     fmt.Sprintf("CPU板电压已恢复正常: %.2fV", metrics.CPUVoltage),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "CJB-RG-ZD-3",
				MetricValue: metrics.CPUVoltage,
			}
		}
		alerts = append(alerts, alert)
	}

	// 母线电压检查 (正常[24, 28]V)
	busVoltage := metrics.BusVoltage
	isFiring = busVoltage < 24.0 || busVoltage > 28.0
	shouldSend, firing = sm.CheckAndUpdateAlertState("BUS_VOLTAGE_ALERT", isFiring)
	
	if shouldSend {
		var alert *model.AlertEvent
		if firing {
			alert = &model.AlertEvent{
				AlertID:     "BUS_VOLTAGE_ALERT",
				Type:        "voltage_abnormal",
				Status:      model.AlertStatusFiring,
				Severity:    model.SeverityCritical,
				Source:      "bus_monitor",
				Message:     fmt.Sprintf("母线电压异常: %.2fV (正常[24,28]V)", busVoltage),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "CJB-RG-ZD-3",
				MetricValue: busVoltage,
			}
		} else {
			alert = &model.AlertEvent{
				AlertID:     "BUS_VOLTAGE_ALERT",
				Type:        "voltage_abnormal",
				Status:      model.AlertStatusResolved,
				Severity:    model.SeverityInfo,
				Source:      "bus_monitor",
				Message:     fmt.Sprintf("母线电压已恢复正常: %.2fV", busVoltage),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "CJB-RG-ZD-3",
				MetricValue: busVoltage,
			}
		}
		alerts = append(alerts, alert)
	}

	return alerts
}

// CheckNodeThresholdsWithState 检查节点指标（支持恢复告警）
func CheckNodeThresholdsWithState(metrics *model.NodeMetrics, sm *state.StateManager) []*model.AlertEvent {
	var alerts []*model.AlertEvent

	// // CPU使用率检查（NodeMetrics.CPUUsage 是 interface{}）
	// var cpuUsage float64
	// if cpu, ok := metrics.CPUUsage.(float64); ok {
	// 	cpuUsage = cpu
	// } else if cpuStruct, ok := metrics.CPUUsage.(model.CPUUsage); ok {
	// 	cpuUsage = cpuStruct.Total
	// }
	
	// alertID := fmt.Sprintf("NODE_CPU_HIGH")
	// isFiring := cpuUsage > 60.0
	// shouldSend, firing := sm.CheckAndUpdateAlertState(alertID, isFiring)
	
	// if shouldSend {
	// 	var alert *model.AlertEvent
	// 	if firing {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     alertID,
	// 			Type:        "cpu_high",
	// 			Status:      model.AlertStatusFiring,
	// 			Severity:    model.SeverityCritical,
	// 			Source:      metrics.ID,
	// 			Message:     fmt.Sprintf("节点CPU使用率过高: %.2f%%", cpuUsage),
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: cpuUsage,
	// 		}
	// 	} else {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     alertID,
	// 			Type:        "cpu_high",
	// 			Status:      model.AlertStatusResolved,
	// 			Severity:    model.SeverityInfo,
	// 			Source:      metrics.ID,
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: cpuUsage,
	// 		}
	// 	}
	// 	alerts = append(alerts, alert)
	// }

	// // 内存使用率检查（使用 MemoryFree/MemoryTotal 计算）
	// var memoryUsage float64
	// if metrics.MemoryTotal > 0 {
	// 	memoryUsage = float64(metrics.MemoryTotal-metrics.MemoryFree) / float64(metrics.MemoryTotal) * 100.0
	// }
	
	// alertID = fmt.Sprintf("NODE_MEMORY_HIGH")
	// isFiring = memoryUsage > 90.0
	// shouldSend, firing = sm.CheckAndUpdateAlertState(alertID, isFiring)
	
	// if shouldSend {
	// 	var alert *model.AlertEvent
	// 	if firing {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     alertID,
	// 			Type:        "memory_high",
	// 			Status:      model.AlertStatusFiring,
	// 			Severity:    model.SeverityCritical,
	// 			Source:      metrics.ID,
	// 			Message:     fmt.Sprintf("节点内存使用率过高: %.2f%%", memoryUsage),
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: memoryUsage,
	// 		}
	// 	} else {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     alertID,
	// 			Type:        "memory_high",
	// 			Status:      model.AlertStatusResolved,
	// 			Severity:    model.SeverityInfo,
	// 			Source:      metrics.ID,
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: memoryUsage,
	// 		}
	// 	}
	// 	alerts = append(alerts, alert)
	// }

	return alerts
}

// CheckContainerThresholdsWithState 检查容器指标（支持恢复告警）
func CheckContainerThresholdsWithState(metrics *model.ContainerMetrics, sm *state.StateManager) []*model.AlertEvent {
	var alerts []*model.AlertEvent

	// CPU使用率检查（ContainerMetrics.CPUUsage 是结构体）
	cpuUsage := metrics.CPUUsage.Total
	alertID := fmt.Sprintf("CONTAINER_CPU_HIGH")
	isFiring := cpuUsage > 60.0
	shouldSend, firing := sm.CheckAndUpdateAlertStateWithSource(alertID, metrics.ID, isFiring)
	var containerMetadata map[string]interface{}
	if metrics.ServiceName != "" || metrics.ServiceID != "" {
		containerMetadata = map[string]interface{}{}
		if metrics.ServiceName != "" {
			containerMetadata["serviceName"] = metrics.ServiceName
		}
		if metrics.ServiceID != "" {
			containerMetadata["serviceId"] = metrics.ServiceID
		}
	}
	
	if shouldSend {
		var alert *model.AlertEvent
		if firing {
			alert = &model.AlertEvent{
				AlertID:     alertID,
				Type:        "cpu_high",
				Status:      model.AlertStatusFiring,
				Severity:    model.SeverityCritical,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("容器CPU使用率过高: %.2f%%", cpuUsage),
				Timestamp:   time.Now().Unix(),
				MetricValue: cpuUsage,
				Metadata:    containerMetadata,
			}
		} else {
			alert = &model.AlertEvent{
				AlertID:     alertID,
				Type:        "cpu_high",
				Status:      model.AlertStatusResolved,
				Severity:    model.SeverityInfo,
				Source:      metrics.ID,
				Timestamp:   time.Now().Unix(),
				MetricValue: cpuUsage,
				Metadata:    containerMetadata,
			}
		}
		alerts = append(alerts, alert)
	}

	// 内存使用率检查（ContainerMetrics.MemoryUsage 是 int64）
	var memoryUsage float64
	if metrics.MemoryLimit > 0 {
		memoryUsage = float64(metrics.MemoryUsage) / float64(metrics.MemoryLimit) * 100.0
	}

	alertID = fmt.Sprintf("CONTAINER_MEMORY_HIGH")
	isFiring = memoryUsage > 90.0
	shouldSend, firing = sm.CheckAndUpdateAlertStateWithSource(alertID, metrics.ID, isFiring)

	if shouldSend {
		var alert *model.AlertEvent
		if firing {
			alert = &model.AlertEvent{
				AlertID:     alertID,
				Type:        "memory_high",
				Status:      model.AlertStatusFiring,
				Severity:    model.SeverityCritical,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("容器内存使用率过高: %.2f%%", memoryUsage),
				Timestamp:   time.Now().Unix(),
				MetricValue: memoryUsage,
				Metadata:    containerMetadata,
			}
		} else {
			alert = &model.AlertEvent{
				AlertID:     alertID,
				Type:        "memory_high",
				Status:      model.AlertStatusResolved,
				Severity:    model.SeverityInfo,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("容器内存使用率已恢复正常: %.2f%%", memoryUsage),
				Timestamp:   time.Now().Unix(),
				MetricValue: memoryUsage,
				Metadata:    containerMetadata,
			}
		}
		alerts = append(alerts, alert)
	}

	// 磁盘使用率检查（ContainerMetrics.SizeUsage 是 int64）
	var diskUsage float64
	if metrics.SizeLimit > 0 {
		diskUsage = float64(metrics.SizeUsage) / float64(metrics.SizeLimit) * 100.0
	}

	alertID = fmt.Sprintf("CONTAINER_DISK_HIGH")
	isFiring = diskUsage > 90.0
	shouldSend, firing = sm.CheckAndUpdateAlertStateWithSource(alertID, metrics.ID, isFiring)

	if shouldSend {
		var alert *model.AlertEvent
		if firing {
			alert = &model.AlertEvent{
				AlertID:     alertID,
				Type:        "disk_high",
				Status:      model.AlertStatusFiring,
				Severity:    model.SeverityCritical,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("容器磁盘使用率过高: %.2f%%", diskUsage),
				Timestamp:   time.Now().Unix(),
				MetricValue: diskUsage,
				Metadata:    containerMetadata,
			}
		} else {
			alert = &model.AlertEvent{
				AlertID:     alertID,
				Type:        "disk_high",
				Status:      model.AlertStatusResolved,
				Severity:    model.SeverityInfo,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("容器磁盘使用率已恢复正常: %.2f%%", diskUsage),
				Timestamp:   time.Now().Unix(),
				MetricValue: diskUsage,
				Metadata:    containerMetadata,
			}
		}
		alerts = append(alerts, alert)
	}

	// // CPU波动检查
	// isFiring = metrics.CPUUsage > 60.0 && metrics.CPUUsage < 90.0
	// shouldSend, firing = sm.CheckAndUpdateAlertState("CONTAINER_CPU_FLUCTUATION", isFiring)
	
	// if shouldSend {
	// 	var alert *model.AlertEvent
	// 	if firing {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     "CONTAINER_CPU_FLUCTUATION",
	// 			Type:        "cpu_fluctuation",
	// 			Status:      model.AlertStatusFiring,
	// 			Severity:    model.SeverityWarning,
	// 			Source:      metrics.ID,
	// 			Message:     fmt.Sprintf("容器CPU使用率波动: %.2f%%", metrics.CPUUsage),
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: metrics.CPUUsage,
	// 		}
	// 	} else {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     "CONTAINER_CPU_FLUCTUATION",
	// 			Type:        "cpu_fluctuation",
	// 			Status:      model.AlertStatusResolved,
	// 			Severity:    model.SeverityInfo,
	// 			Source:      metrics.ID,
	// 			Message:     fmt.Sprintf("容器CPU使用率波动已消除: %.2f%%", metrics.CPUUsage),
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: metrics.CPUUsage,
	// 		}
	// 	}
	// 	alerts = append(alerts, alert)
	// }

	return alerts
}

// CheckServiceThresholdsWithState 检查服务指标（支持恢复告警）
func CheckServiceThresholdsWithState(metrics *model.ServiceMetrics, sm *state.StateManager) []*model.AlertEvent {
	var alerts []*model.AlertEvent

	// // P99延迟检查
	// isFiring := metrics.P99Latency > 500.0
	// shouldSend, firing := sm.CheckAndUpdateAlertState("SERVICE_P99_LATENCY_HIGH", isFiring)
	
	// if shouldSend {
	// 	var alert *model.AlertEvent
	// 	if firing {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     "SERVICE_P99_LATENCY_HIGH",
	// 			Type:        "latency_high",
	// 			Status:      model.AlertStatusFiring,
	// 			Severity:    model.SeverityWarning,
	// 			Source:      metrics.ID,
	// 			Message:     fmt.Sprintf("服务P99延迟过高: %.2fms", metrics.P99Latency),
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: metrics.P99Latency,
	// 		}
	// 	} else {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     "SERVICE_P99_LATENCY_HIGH",
	// 			Type:        "latency_high",
	// 			Status:      model.AlertStatusResolved,
	// 			Severity:    model.SeverityInfo,
	// 			Source:      metrics.ID,
	// 			Message:     fmt.Sprintf("服务P99延迟已恢复: %.2fms", metrics.P99Latency),
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: metrics.P99Latency,
	// 		}
	// 	}
	// 	alerts = append(alerts, alert)
	// }

	// // 错误率检查
	// isFiring = metrics.ErrorRate > 5.0
	// shouldSend, firing = sm.CheckAndUpdateAlertState("SERVICE_ERROR_RATE_HIGH", isFiring)
	
	// if shouldSend {
	// 	var alert *model.AlertEvent
	// 	if firing {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     "SERVICE_ERROR_RATE_HIGH",
	// 			Type:        "error_rate_high",
	// 			Status:      model.AlertStatusFiring,
	// 			Severity:    model.SeverityCritical,
	// 			Source:      metrics.ID,
	// 			Message:     fmt.Sprintf("服务错误率过高: %.2f%%", metrics.ErrorRate),
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: metrics.ErrorRate,
	// 		}
	// 	} else {
	// 		alert = &model.AlertEvent{
	// 			AlertID:     "SERVICE_ERROR_RATE_HIGH",
	// 			Type:        "error_rate_high",
	// 			Status:      model.AlertStatusResolved,
	// 			Severity:    model.SeverityInfo,
	// 			Source:      metrics.ID,
	// 			Message:     fmt.Sprintf("服务错误率已恢复: %.2f%%", metrics.ErrorRate),
	// 			Timestamp:   time.Now().Unix(),
	// 			MetricValue: metrics.ErrorRate,
	// 		}
	// 	}
	// 	alerts = append(alerts, alert)
	// }

	return alerts
}
