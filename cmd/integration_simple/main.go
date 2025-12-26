package main

import (
	"context"
	"fmt"
	"time"

	// 健康监测模块
	healthAlert "health-monitor/pkg/alert"
	healthModel "health-monitor/pkg/models"
	healthState "health-monitor/pkg/state"

	// 故障诊断模块
	diagnosisConfig "fault-diagnosis/pkg/config"
	diagnosisEngine "fault-diagnosis/pkg/engine"
	diagnosisModels "fault-diagnosis/pkg/models"
	diagnosisReceiver "fault-diagnosis/pkg/receiver"
	diagnosisUtils "fault-diagnosis/pkg/utils"

	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║     健康监测 + 故障诊断 集成测试 (简化版)                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝\n")

	ctx := context.Background()

	// ========== 1. 初始化故障诊断 ==========
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("1. 初始化故障诊断模块")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 加载故障树
	loader := diagnosisConfig.NewLoader("../fault-diagnosis/configs/fault_tree_business.json")
	faultTree, err := loader.LoadFaultTree()
	if err != nil {
		logger.Fatal("加载故障树失败", zap.Error(err))
	}
	fmt.Printf("  ✓ 故障树: %s\n", faultTree.Description)

	// 创建诊断引擎
	diagLogger, _ := diagnosisUtils.NewLogger("info")
	engine, err := diagnosisEngine.NewDiagnosisEngine(faultTree, diagLogger)
	if err != nil {
		logger.Fatal("创建诊断引擎失败", zap.Error(err))
	}

	// 设置诊断回调
	engine.SetCallback(func(diagnosis *diagnosisModels.DiagnosisResult) {
		fmt.Println("\n" + "═"*70)
		fmt.Println("🚨 检测到系统级故障!")
		fmt.Println("═"*70)
		fmt.Printf("  📋 诊断ID:     %s\n", diagnosis.DiagnosisID)
		fmt.Printf("  ⚠️  故障码:     %s\n", diagnosis.FaultCode)
		fmt.Printf("  📊 顶层事件:   %s\n", diagnosis.TopEventName)
		fmt.Printf("  📝 故障原因:   %s\n", diagnosis.FaultReason)
		fmt.Printf("  ⏰ 诊断时间:   %s\n", diagnosis.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("  🔍 触发路径:   %v\n", diagnosis.TriggerPath)
		fmt.Printf("  🎯 基本事件:   %v\n", diagnosis.BasicEvents)
		fmt.Println("═"*70 + "\n")
	})

	// 创建接收器
	receiver := diagnosisReceiver.NewChannelReceiver(500, diagLogger)
	receiver.SetHandler(func(alert *diagnosisModels.AlertEvent) {
		fmt.Printf("  [诊断] 收到告警: %s (status=%s, severity=%s)\n",
			alert.AlertID, alert.Status, alert.Severity)
		engine.ProcessAlert(alert)
	})

	if err := receiver.Start(); err != nil {
		logger.Fatal("启动接收器失败", zap.Error(err))
	}
	defer receiver.Stop()

	// ========== 2. 初始化健康监测 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("2. 初始化健康监测模块")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 创建状态管理器
	stateManager, err := healthState.NewStateManager()
	if err != nil {
		logger.Fatal("创建状态管理器失败", zap.Error(err))
	}
	defer stateManager.Close()
	fmt.Println("  ✓ 状态管理器已创建")

	// ========== 3. 运行测试场景 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("3. 运行测试场景")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 场景1: 正常状态
	fmt.Println("📌 场景 1: 所有指标正常")
	powerMetrics1 := &healthModel.PowerMetrics{
		BatteryVoltage: 25.0, // 正常
		CPUVoltage:     3.3,  // 正常
		Timestamp:      time.Now().Unix(),
	}
	alerts := healthAlert.CheckPowerThresholdsWithState(powerMetrics1, stateManager)
	sendAlerts(alerts, receiver)
	time.Sleep(2 * time.Second)

	// 场景2: 蓄电池电压异常
	fmt.Println("\n📌 场景 2: 蓄电池电压异常 (触发告警)")
	powerMetrics2 := &healthModel.PowerMetrics{
		BatteryVoltage: 19.5, // 异常：低于21V
		CPUVoltage:     3.3,
		Timestamp:      time.Now().Unix(),
	}
	alerts = healthAlert.CheckPowerThresholdsWithState(powerMetrics2, stateManager)
	sendAlerts(alerts, receiver)
	time.Sleep(2 * time.Second)

	// 场景3: 蓄电池+母线电压异常（应触发故障）
	fmt.Println("\n📌 场景 3: 蓄电池和母线电压同时异常 (应触发故障诊断)")
	powerMetrics3 := &healthModel.PowerMetrics{
		BatteryVoltage: 23.0, // 异常：低于24V（母线）
		CPUVoltage:     3.3,
		Timestamp:      time.Now().Unix(),
	}
	alerts = healthAlert.CheckPowerThresholdsWithState(powerMetrics3, stateManager)
	sendAlerts(alerts, receiver)
	time.Sleep(3 * time.Second)

	// 场景4: CPU板电压异常
	fmt.Println("\n📌 场景 4: CPU板电压异常 (应触发AD模块故障)")
	powerMetrics4 := &healthModel.PowerMetrics{
		BatteryVoltage: 25.0,
		CPUVoltage:     3.8, // 异常：超过3.5V
		Timestamp:      time.Now().Unix(),
	}
	alerts = healthAlert.CheckPowerThresholdsWithState(powerMetrics4, stateManager)
	sendAlerts(alerts, receiver)
	time.Sleep(3 * time.Second)

	// 场景5: 恢复正常
	fmt.Println("\n📌 场景 5: 所有指标恢复正常 (恢复告警)")
	powerMetrics5 := &healthModel.PowerMetrics{
		BatteryVoltage: 26.0, // 恢复正常
		CPUVoltage:     3.3,  // 恢复正常
		Timestamp:      time.Now().Unix(),
	}
	alerts = healthAlert.CheckPowerThresholdsWithState(powerMetrics5, stateManager)
	sendAlerts(alerts, receiver)
	time.Sleep(2 * time.Second)

	// ========== 4. 微服务层测试 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("4. 微服务层故障测试")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 加载微服务层故障树
	msLoader := diagnosisConfig.NewLoader("../fault-diagnosis/configs/fault_tree_microservice.json")
	msFaultTree, err := msLoader.LoadFaultTree()
	if err != nil {
		logger.Fatal("加载微服务故障树失败", zap.Error(err))
	}

	msEngine, err := diagnosisEngine.NewDiagnosisEngine(msFaultTree, diagLogger)
	if err != nil {
		logger.Fatal("创建微服务诊断引擎失败", zap.Error(err))
	}

	msEngine.SetCallback(func(diagnosis *diagnosisModels.DiagnosisResult) {
		fmt.Println("\n" + "═"*70)
		fmt.Println("🚨 [微服务层] 检测到故障!")
		fmt.Println("═"*70)
		fmt.Printf("  📋 诊断ID:     %s\n", diagnosis.DiagnosisID)
		fmt.Printf("  ⚠️  故障码:     %s\n", diagnosis.FaultCode)
		fmt.Printf("  📊 顶层事件:   %s\n", diagnosis.TopEventName)
		fmt.Printf("  🔍 触发路径:   %v\n", diagnosis.TriggerPath)
		fmt.Println("═"*70 + "\n")
	})

	msReceiver := diagnosisReceiver.NewChannelReceiver(500, diagLogger)
	msReceiver.SetHandler(func(alert *diagnosisModels.AlertEvent) {
		fmt.Printf("  [微服务诊断] 收到告警: %s (status=%s)\n", alert.AlertID, alert.Status)
		msEngine.ProcessAlert(alert)
	})
	msReceiver.Start()
	defer msReceiver.Stop()

	// 场景6: 容器CPU使用率过高
	fmt.Println("📌 场景 6: 容器CPU使用率过高")
	containerMetrics1 := &healthModel.ContainerMetrics{
		ID:          "container-1",
		CPUUsage:    95.0,
		MemoryUsage: 50.0,
	}
	alerts = healthAlert.CheckContainerThresholdsWithState(containerMetrics1, stateManager)
	sendAlerts(alerts, msReceiver)
	time.Sleep(2 * time.Second)

	// 场景7: 容器内存也过高（级联故障）
	fmt.Println("\n📌 场景 7: 容器CPU和内存同时过高 (级联故障)")
	containerMetrics2 := &healthModel.ContainerMetrics{
		ID:          "container-1",
		CPUUsage:    95.0,
		MemoryUsage: 92.0,
	}
	alerts = healthAlert.CheckContainerThresholdsWithState(containerMetrics2, stateManager)
	sendAlerts(alerts, msReceiver)
	time.Sleep(3 * time.Second)

	// 场景8: 容器恢复正常
	fmt.Println("\n📌 场景 8: 容器指标恢复正常")
	containerMetrics3 := &healthModel.ContainerMetrics{
		ID:          "container-1",
		CPUUsage:    45.0,
		MemoryUsage: 50.0,
	}
	alerts = healthAlert.CheckContainerThresholdsWithState(containerMetrics3, stateManager)
	sendAlerts(alerts, msReceiver)
	time.Sleep(2 * time.Second)

	// ========== 5. 结束 ==========
	fmt.Println("\n╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║     集成测试完成                                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")

	// 显示统计信息
	fmt.Println("\n统计信息:")
	stats := stateManager.GetStats()
	fmt.Printf("  - 活跃告警数: %v\n", stats["active_alerts"])
	fmt.Printf("  - 状态记录数: %v\n", stats["latest_states"])
	fmt.Printf("  - 队列容量: %d / %d\n", receiver.GetQueueLength(), receiver.GetQueueCapacity())
}

// sendAlerts 发送告警到故障诊断
func sendAlerts(alerts []*healthModel.AlertEvent, receiver *diagnosisReceiver.ChannelReceiver) {
	for _, alert := range alerts {
		// 转换告警格式
		diagAlert := healthAlert.ConvertToDiagnosisAlertDirect(alert)
		if err := receiver.SendAlert(diagAlert.(*diagnosisModels.AlertEvent)); err != nil {
			fmt.Printf("  ❌ 发送告警失败: %v\n", err)
		} else {
			statusIcon := "🔴"
			if alert.Status == healthModel.AlertStatusResolved {
				statusIcon = "🟢"
			}
			fmt.Printf("  %s 发送告警: %s (%s)\n", statusIcon, alert.AlertID, alert.Status)
		}
	}
}
