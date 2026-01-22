/* 静态阈值判断，例如：

CPU > 85%

电压 < 3.1V

业务延迟 > 5000ms

返回 AlertEvent(nil 或对象)。 */
package alert

import (
	"fmt"
	"health-monitor/pkg/models"
	"time"
)

// CheckPowerThresholds 检查供电服务阈值
func CheckPowerThresholds(metrics *model.PowerMetrics) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// 12V功率模块电压检查 (正常约13V)
	if metrics.PowerModule12V < 12.5 || metrics.PowerModule12V > 13.5 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("PWR-12V-%d", time.Now().Unix()),
			Type:        "VoltageAbnormal",
			Severity:    model.SeverityWarning,
			Source:      "PowerModule12V",
			Message:     fmt.Sprintf("12V功率模块电压异常: %.2fV (正常约13V)", metrics.PowerModule12V),
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-RG-ZD-1",
			MetricValue: metrics.PowerModule12V,
		})
	}
	
	// 蓄电池电压检查 (正常[21, 29.4]V)
	if metrics.BatteryVoltage < 21.0 || metrics.BatteryVoltage > 29.4 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("PWR-BAT-%d", time.Now().Unix()),
			Type:        "VoltageAbnormal",
			Severity:    model.SeverityCritical,
			Source:      "BatteryVoltage",
			Message:     fmt.Sprintf("蓄电池电压异常: %.2fV (正常[21,29.4]V)", metrics.BatteryVoltage),
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-RG-ZD-3",
			MetricValue: metrics.BatteryVoltage,
		})
	}
	
	// CPU板电压检查 (正常[3.1, 3.5]V)
	if metrics.CPUVoltage < 3.1 || metrics.CPUVoltage > 3.5 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("PWR-CPU-%d", time.Now().Unix()),
			Type:        "VoltageAbnormal",
			Severity:    model.SeverityCritical,
			Source:      "CPUVoltage",
			Message:     fmt.Sprintf("CPU板电压异常: %.2fV (正常[3.1,3.5]V)", metrics.CPUVoltage),
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-RG-ZD-3",
			MetricValue: metrics.CPUVoltage,
		})
	}
	
	// 负载电流检查 (正常[0.5, 5]A)
	if metrics.LoadCurrent < 0.5 || metrics.LoadCurrent > 5.0 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("PWR-LOAD-%d", time.Now().Unix()),
			Type:        "CurrentAbnormal",
			Severity:    model.SeverityWarning,
			Source:      "LoadCurrent",
			Message:     fmt.Sprintf("负载电流异常: %.2fA (正常[0.5,5]A)", metrics.LoadCurrent),
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-O2-CS-1",
			MetricValue: metrics.LoadCurrent,
		})
	}
	
	return alerts
}

// CheckThermalThresholds 检查热控服务阈值
func CheckThermalThresholds(metrics *model.ThermalMetrics) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// 检查10个热控温度点 (假设正常范围 [-20, 50]℃)
	for i, temp := range metrics.ThermalTemps {
		if temp < -20.0 || temp > 50.0 {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("THERM-TEMP%d-%d", i+1, time.Now().Unix()),
				Type:        "TemperatureAbnormal",
				Severity:    model.SeverityWarning,
				Source:      fmt.Sprintf("ThermalTemp%d", i+1),
				Message:     fmt.Sprintf("热控温度%d异常: %.1f℃", i+1, temp),
				Timestamp:   metrics.Timestamp,
				FaultCode:   "CJB-RG-ZD-4",
				MetricValue: temp,
			})
		}
	}
	
	// 蓄电池温度检查 (假设正常范围 [0, 45]℃)
	if metrics.BatteryTemp1 < 0.0 || metrics.BatteryTemp1 > 45.0 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("THERM-BAT1-%d", time.Now().Unix()),
			Type:        "TemperatureAbnormal",
			Severity:    model.SeverityWarning,
			Source:      "BatteryTemp1",
			Message:     fmt.Sprintf("蓄电池温度1异常: %.1f℃", metrics.BatteryTemp1),
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-RG-ZD-4",
			MetricValue: metrics.BatteryTemp1,
		})
	}
	
	return alerts
}

// CheckCommThresholds 检查通信服务阈值
func CheckCommThresholds(metrics *model.CommMetrics) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// CAN通信状态检查
	if metrics.CANStatus == 0 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("COMM-CAN-%d", time.Now().Unix()),
			Type:        "CommunicationFailure",
			Severity:    model.SeverityCritical,
			Source:      "CANStatus",
			Message:     "CAN通信无应答",
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-RG-ZD-2",
			MetricValue: float64(metrics.CANStatus),
		})
	}
	
	// 串口通信状态检查
	if metrics.SerialStatus == 0 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("COMM-SERIAL-%d", time.Now().Unix()),
			Type:        "CommunicationFailure",
			Severity:    model.SeverityWarning,
			Source:      "SerialStatus",
			Message:     "串口通信无遥测",
			Timestamp:   metrics.Timestamp,
			FaultCode:   "CJB-O2-CS-1",
			MetricValue: float64(metrics.SerialStatus),
		})
	}
	
	return alerts
}

// CheckActuatorThresholds 检查姿态控制机构阈值
func CheckActuatorThresholds(metrics *model.ActuatorMetrics) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// 动量轮转速检查 (期望约100转，允许误差±10)
	expectedSpeed := int16(100)
	tolerance := int16(10)
	
	checkWheel := func(speed int16, axis string) {
		if speed < expectedSpeed-tolerance || speed > expectedSpeed+tolerance {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("ACTUATOR-%s-%d", axis, time.Now().Unix()),
				Type:        "ActuatorAbnormal",
				Severity:    model.SeverityWarning,
				Source:      fmt.Sprintf("WheelSpeed%s", axis),
				Message:     fmt.Sprintf("%s轴动量轮转速异常: %d (期望约100转)", axis, speed),
				Timestamp:   metrics.Timestamp,
				FaultCode:   "CJB-O2-CS-16",
				MetricValue: float64(speed),
			})
		}
	}
	
	checkWheel(metrics.WheelSpeedX, "X")
	checkWheel(metrics.WheelSpeedY, "Y")
	checkWheel(metrics.WheelSpeedZ, "Z")
	
	return alerts
}

// ========== 微服务层阈值检查函数 ==========

// CheckNodeThresholds 检查节点指标阈值
func CheckNodeThresholds(metrics *model.NodeMetrics) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// 节点在线状态检查
	if metrics.Status != "online" {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("NODE-%s-OFFLINE-%d", metrics.ID, time.Now().Unix()),
			Type:        "NodeOffline",
			Severity:    model.SeverityCritical,
			Source:      metrics.ID,
			Message:     fmt.Sprintf("节点 %s 离线", metrics.ID),
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-NO-FL-1",
			MetricValue: 0,
		})
	}
	
	// CPU使用率检查 (> 85%)
	if cpuUsage, ok := metrics.CPUUsage.(float64); ok {
		if cpuUsage > 85.0 {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("NODE-%s-CPU-%d", metrics.ID, time.Now().Unix()),
				Type:        "HighCPUUsage",
				Severity:    model.SeverityCritical,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("节点 %s CPU使用率过高: %.1f%%", metrics.ID, cpuUsage),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "MS-NO-FL-2",
				MetricValue: cpuUsage,
			})
		}
	}
	
	// 内存使用率检查 (> 90%)
	if metrics.MemoryTotal > 0 {
		memoryPercent := float64(metrics.MemoryTotal-metrics.MemoryFree) / float64(metrics.MemoryTotal) * 100
		if memoryPercent > 90.0 {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("NODE-%s-MEM-%d", metrics.ID, time.Now().Unix()),
				Type:        "HighMemoryUsage",
				Severity:    model.SeverityCritical,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("节点 %s 内存使用率过高: %.1f%%", metrics.ID, memoryPercent),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "MS-NO-FL-3",
				MetricValue: memoryPercent,
			})
		}
	}
	
	// 磁盘使用率检查 (> 90%)
	if metrics.DiskTotal > 0 {
		diskPercent := (metrics.DiskTotal - metrics.DiskFree) / metrics.DiskTotal * 100
		if diskPercent > 90.0 {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("NODE-%s-DISK-%d", metrics.ID, time.Now().Unix()),
				Type:        "HighDiskUsage",
				Severity:    model.SeverityCritical,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("节点 %s 磁盘使用率过高: %.1f%%", metrics.ID, diskPercent),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "MS-NO-FL-4",
				MetricValue: diskPercent,
			})
		}
	}
	
	// 容器运行比例检查 (< 0.8)
	/* if metrics.ContainerTotal > 0 {
		runningRatio := float64(metrics.ContainerRunning) / float64(metrics.ContainerTotal)
		if runningRatio < 0.8 {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("NODE-%s-CNTR-%d", metrics.ID, time.Now().Unix()),
				Type:        "LowContainerRunningRatio",
				Severity:    model.SeverityWarning,
				Source:      fmt.Sprintf("Node-%s-Containers", metrics.ID),
				Message:     fmt.Sprintf("节点 %s 容器运行比例过低: %.1f%% (%d/%d)", metrics.ID, runningRatio*100, metrics.ContainerRunning, metrics.ContainerTotal),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "MS-NO-FL-6",
				MetricValue: runningRatio * 100,
			})
		}
	} */
	
	return alerts
}

// CheckContainerThresholds 检查容器指标阈值
func CheckContainerThresholds(metrics *model.ContainerMetrics) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// 容器部署状态检查
	if metrics.DeployStatus != "success" {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("CNTR-%s-DEPLOY-%d", metrics.ID, time.Now().Unix()),
			Type:        "DeploymentFailure",
			Severity:    model.SeverityCritical,
			Source:      metrics.ID,
			Message:     fmt.Sprintf("容器 %s 部署失败: %s", metrics.ID, metrics.DeployStatus),
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-CN-FL-1",
			MetricValue: 0,
		})
	}
	
	// 容器启动状态检查
	/* if metrics.Status != "running" {
		severity := model.SeverityWarning
		if metrics.Status == "exited" {
			severity = model.SeverityCritical
		}
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("CNTR-%s-STATUS-%d", metrics.ID, time.Now().Unix()),
			Type:        "ContainerNotRunning",
			Severity:    severity,
			Source:      fmt.Sprintf("Container-%s", metrics.ID),
			Message:     fmt.Sprintf("容器 %s 状态异常: %s", metrics.ID, metrics.Status),
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-CN-FL-2",
			MetricValue: 0,
		})
	} */
	
	// 容器运行中断检查 (< 60s)
	/* if metrics.Uptime < 60 && metrics.Status == "running" {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("CNTR-%s-UPTIME-%d", metrics.ID, time.Now().Unix()),
			Type:        "ShortUptime",
			Severity:    model.SeverityWarning,
			Source:      fmt.Sprintf("Container-%s", metrics.ID),
			Message:     fmt.Sprintf("容器 %s 运行时间过短: %ds", metrics.ID, metrics.Uptime),
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-CN-FL-3",
			MetricValue: float64(metrics.Uptime),
		})
	} */
	
	// 容器CPU使用率检查 (> 75%)
	if metrics.CPUUsage.Total > 65.0 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("CNTR-%s-CPU-%d", metrics.ID, time.Now().Unix()),
			Type:        "HighCPUUsage",
			Severity:    model.SeverityCritical,
			Source:      metrics.ID,
			Message:     fmt.Sprintf("容器 %s CPU使用率过高: %.1f%%", metrics.ID, metrics.CPUUsage.Total),
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-CN-FL-5",
			MetricValue: metrics.CPUUsage.Total,
		})
	}
	
	// 容器内存使用率检查 (> 90%)
	if metrics.MemoryLimit > 0 {
		memoryPercent := float64(metrics.MemoryUsage) / float64(metrics.MemoryLimit) * 100
		if memoryPercent > 80.0 {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("CNTR-%s-MEM-%d", metrics.ID, time.Now().Unix()),
				Type:        "HighMemoryUsage",
				Severity:    model.SeverityCritical,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("容器 %s 内存使用率过高: %.1f%%", metrics.ID, memoryPercent),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "MS-CN-FL-5",
				MetricValue: memoryPercent,
			})
		}
	}
	
	// 容器磁盘占用率检查 (> 90%)
	if metrics.SizeLimit > 0 {
		diskPercent := float64(metrics.SizeUsage) / float64(metrics.SizeLimit) * 100
		if diskPercent > 65.0 {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("CNTR-%s-DISK-%d", metrics.ID, time.Now().Unix()),
				Type:        "HighDiskUsage",
				Severity:    model.SeverityCritical,
				Source:      metrics.ID,
				Message:     fmt.Sprintf("容器 %s 磁盘占用率过高: %.1f%%", metrics.ID, diskPercent),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "MS-CN-FL-6",
				MetricValue: diskPercent,
			})
		}
	}
	
	return alerts
}

// CheckServiceThresholds 检查服务指标阈值
func CheckServiceThresholds(metrics *model.ServiceMetrics) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// 服务健康状态检查
	if !metrics.Healthy {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("SVC-%s-HEALTH-%d", metrics.ID, time.Now().Unix()),
			Type:        "ServiceUnhealthy",
			Severity:    model.SeverityWarning,
			Source:      metrics.ID,
			Message:     fmt.Sprintf("服务 %s 健康检查失败", metrics.ID),
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-SV-FL-1",
			MetricValue: 0,
		})
	}
	
	// 服务节点数量检查 (= 0)
	if metrics.InstanceOnline == 0 {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     fmt.Sprintf("SVC-%s-NODES-%d", metrics.ID, time.Now().Unix()),
			Type:        "NoOnlineNodes",
			Severity:    model.SeverityWarning,
			Source:      metrics.ID,
			Message:     fmt.Sprintf("服务 %s 无在线节点", metrics.ID),
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-SV-FL-5",
			MetricValue: 0,
		})
	}
	
	// 容器运行比例检查
	/* if len(metrics.ContainerStatusGroup) > 0 {
		runningCount := 0
		for _, status := range metrics.ContainerStatusGroup {
			if status == "running" {
				runningCount++
			}
		}
		runningRatio := float64(runningCount) / float64(len(metrics.ContainerStatusGroup))
		if runningRatio < 0.8 {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("SVC-%s-CNTR-%d", metrics.ID, time.Now().Unix()),
				Type:        "LowContainerRunningRatio",
				Severity:    model.SeverityWarning,
				Source:      fmt.Sprintf("Service-%s-Containers", metrics.ID),
				Message:     fmt.Sprintf("服务 %s 容器运行比例过低: %.1f%% (%d/%d)", metrics.ID, runningRatio*100, runningCount, len(metrics.ContainerStatusGroup)),
				Timestamp:   time.Now().Unix(),
				FaultCode:   "MS-SV-FL-4",
				MetricValue: runningRatio * 100,
			})
		}
	} */
	
	return alerts
}