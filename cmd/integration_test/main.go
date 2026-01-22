package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	// 健康监测模块
	healthBusiness "health-monitor/pkg/business"
	healthMicroservice "health-monitor/pkg/microservice"
	healthModel "health-monitor/pkg/models"
	healthState "health-monitor/pkg/state"

	// 故障诊断模块
	diagnosisConfig "fault-diagnosis/pkg/config"
	diagnosisEngine "fault-diagnosis/pkg/engine"
	diagnosisModels "fault-diagnosis/pkg/models"
	diagnosisReceiver "fault-diagnosis/pkg/receiver"
	diagnosisUtils "fault-diagnosis/pkg/utils"

	// 故障修复模块
	recovery "fault-tolerance/fault-recovery/pkg/recovery"

	"go.uber.org/zap"
)

func main() {
	// 创建日志
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║     健康监测 + 故障诊断 集成测试                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ========== 1. 初始化故障诊断模块 ==========
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("1. 初始化故障诊断模块")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 加载业务层故障树
	businessLoader := diagnosisConfig.NewLoader("./fault-diagnosis/configs/fault_tree_business.json")
	businessTree, err := businessLoader.LoadFaultTree()
	if err != nil {
		logger.Fatal("加载业务层故障树失败", zap.Error(err))
	}
	fmt.Printf("  ✓ 业务层故障树: %s\n", businessTree.Description)

	// 加载微服务层故障树
	microserviceLoader := diagnosisConfig.NewLoader("./fault-diagnosis/configs/fault_tree_microservice.json")
	microserviceTree, err := microserviceLoader.LoadFaultTree()
	if err != nil {
		logger.Fatal("加载微服务层故障树失败", zap.Error(err))
	}
	fmt.Printf("  ✓ 微服务层故障树: %s\n", microserviceTree.Description)

	// 创建诊断日志
	diagnosisLogger, _ := diagnosisUtils.NewLogger("info")

	// 创建业务层诊断引擎
	businessEngine, err := diagnosisEngine.NewDiagnosisEngine(businessTree, diagnosisLogger)
	if err != nil {
		logger.Fatal("创建业务层诊断引擎失败", zap.Error(err))
	}

	// 创建微服务层诊断引擎
	microserviceEngine, err := diagnosisEngine.NewDiagnosisEngine(microserviceTree, diagnosisLogger)
	if err != nil {
		logger.Fatal("创建微服务层诊断引擎失败", zap.Error(err))
	}

	// 创建故障修复引擎
	recoveryState := recovery.NewInMemoryStateManager()
	recoveryEngine := recovery.NewEngine(recoveryState, recovery.NewEngineConfig{
		QueueSize: 200,
		Timeout:   10 * time.Second,
	})
	recoveryStore := recovery.NewRuntimeStore()
	recoveryEngine.RegisterAction("CONTAINER-RESOURCE-001", recovery.NewCircuitBreakerAction(recoveryStore))
	recoveryEngine.RegisterAction("BUSINESS-IMAGE-START", recovery.NewStartContainerAction(recoveryStore))
	recoveryEngine.Start(ctx)

	// 设置诊断回调
	businessEngine.SetCallback(func(diagnosis *diagnosisModels.DiagnosisResult) {
		fmt.Println("\n" + strings.Repeat("═", 70))
		fmt.Println("[业务层] 检测到故障!")
		fmt.Println(strings.Repeat("═", 70))
		printDiagnosis(diagnosis)
		_ = recoveryEngine.Submit(convertToRecoveryDiagnosis(diagnosis))
	})

	microserviceEngine.SetCallback(func(diagnosis *diagnosisModels.DiagnosisResult) {
		fmt.Println("\n" + strings.Repeat("═", 70))
		fmt.Println("[微服务层] 检测到故障!")
		fmt.Println(strings.Repeat("═", 70))
		printDiagnosis(diagnosis)
		_ = recoveryEngine.Submit(convertToRecoveryDiagnosis(diagnosis))
	})

	// 创建告警接收器
	businessReceiver := diagnosisReceiver.NewChannelReceiver(500, diagnosisLogger)
	microserviceReceiver := diagnosisReceiver.NewChannelReceiver(500, diagnosisLogger)

	businessReceiver.SetHandler(func(alert *diagnosisModels.AlertEvent) {
		if alert.Status == "firing" {
			fmt.Printf("  [业务层诊断] 收到告警: %s (status=%s)\n", alert.AlertID, alert.Status)
		}
		businessEngine.ProcessAlert(alert)
	})

	microserviceReceiver.SetHandler(func(alert *diagnosisModels.AlertEvent) {
		if alert.Status == "firing" {
			fmt.Printf("  [微服务层诊断] 收到告警: %s (status=%s)\n", alert.AlertID, alert.Status)
		}
		microserviceEngine.ProcessAlert(alert)
	})

	if err := businessReceiver.Start(); err != nil {
		logger.Fatal("启动业务层接收器失败", zap.Error(err))
	}
	defer businessReceiver.Stop()

	if err := microserviceReceiver.Start(); err != nil {
		logger.Fatal("启动微服务层接收器失败", zap.Error(err))
	}
	defer microserviceReceiver.Stop()

	// ========== 2. 初始化健康监测模块 ==========
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

	// 创建告警接收器包装器（集成故障诊断）
	businessWrapper := diagnosisReceiver.NewReceiverWrapper(businessReceiver)
	microserviceWrapper := diagnosisReceiver.NewReceiverWrapper(microserviceReceiver)

	// 创建业务层调度器和接收器
	businessDispatcher := healthBusiness.NewDispatcher(stateManager)
	businessDispatcher.SetDiagnosisReceiver(businessWrapper) // 配置告警接收器
	businessRecv := healthBusiness.NewReceiver(businessDispatcher)
	go businessRecv.Start(ctx) // 启动业务层接收器
	fmt.Println("  ✓ 业务层调度器已创建")

	// 创建微服务层获取器和调度器
	microserviceFetcher := healthMicroservice.NewFetcher("http://192.168.31.127:3001") // ECSM地址
	microserviceDispatcher := healthMicroservice.NewDispatcher(microserviceFetcher, stateManager)
	microserviceDispatcher.SetDiagnosisReceiver(microserviceWrapper) // 配置告警接收器
	fmt.Println("  ✓ 微服务层调度器已创建")

	// ========== 3. 启动业务层模拟测试 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("3. 业务层故障模拟测试")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 启动业务层模拟协程
	go runBusinessSimulation(ctx, businessDispatcher, businessWrapper)

	// ========== 4. 启动微服务层监测 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("4. 微服务层 ECSM 监测")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 启动微服务层监测协程
	go runMicroserviceMonitoring(ctx, microserviceDispatcher, microserviceWrapper)

	// ========== 5. 等待信号 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("5. 集成测试运行中... (Ctrl+C 停止)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("集成测试结束")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// runBusinessSimulation 运行业务层模拟
func runBusinessSimulation(ctx context.Context, dispatcher *healthBusiness.Dispatcher, diagnosisWrapper *diagnosisReceiver.ReceiverWrapper) {
	fmt.Println("  [业务层] 开始模拟测试...\n")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	scenario := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			switch scenario {
			case 0:
				// 场景1: 正常状态
				fmt.Println("\n[业务层] 场景 1: 所有指标正常")
				dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
					ComponentType: 0x03,
					Timestamp:     time.Now().Unix(),
					Data: &healthModel.PowerMetrics{
						BatteryVoltage: 25.0,
						BusVoltage:	    25.0,
						CPUVoltage:     3.3,
						Timestamp:      time.Now().Unix(),
					},
				})

			case 1:
				// 场景2: 蓄电池电压异常
				fmt.Println("\n[业务层] 场景 2: 蓄电池电压异常")
				dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
					ComponentType: 0x03,
					Timestamp:     time.Now().Unix(),
					Data: &healthModel.PowerMetrics{
						BatteryVoltage: 19.5, // 低于21V
						BusVoltage:	    25.0,
						CPUVoltage:     3.3,
						Timestamp:      time.Now().Unix(),
					},
				})

			case 2:
				// 场景3: 蓄电池+母线电压异常
				fmt.Println("\n[业务层] 场景 3: 蓄电池和母线电压异常 (应触发故障)")
				dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
					ComponentType: 0x03,
					Timestamp:     time.Now().Unix(),
					Data: &healthModel.PowerMetrics{
						BatteryVoltage: 19.0, // 低于21V (母线异常)
						BusVoltage:	    19.0,
						CPUVoltage:     3.3,
						Timestamp:      time.Now().Unix(),
					},
				})

			case 3:
				// 场景4: 恢复正常
				fmt.Println("\n[业务层] 场景 4: AD模块异常 ")
				dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
					ComponentType: 0x03,
					Timestamp:     time.Now().Unix(),
					Data: &healthModel.PowerMetrics{
						BatteryVoltage: 26.0,
						BusVoltage:	    26.0,
						CPUVoltage:     2.3,
						Timestamp:      time.Now().Unix(),
					},
				})
				return
			}
			scenario = (scenario + 1) % 4
		}

	}
}

// runMicroserviceMonitoring 运行微服务层监测
func runMicroserviceMonitoring(ctx context.Context, dispatcher *healthMicroservice.Dispatcher, diagnosisWrapper *diagnosisReceiver.ReceiverWrapper) {
	fmt.Println("  [微服务层] 开始 ECSM 监测...\n")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 从 ECSM 获取容器指标
			metricsSet, err := dispatcher.RunOnce(ctx)
			if err != nil {
				fmt.Printf("  ⚠️  [微服务层] 获取指标失败: %v\n", err)
				continue
			}

			// 统计信息
			fmt.Printf("  [微服务层] 获取到 %d 个容器指标\n", len(metricsSet.ContainerMetrics))
		}
	}
}

// printDiagnosis 打印诊断结果
func printDiagnosis(diagnosis *diagnosisModels.DiagnosisResult) {
	fmt.Printf("诊断ID:     %s\n", diagnosis.DiagnosisID)
	fmt.Printf("故障码:     %s\n", diagnosis.FaultCode)
	fmt.Printf("顶层事件:   %s\n", diagnosis.TopEventName)
	fmt.Printf("故障原因:   %s\n", diagnosis.FaultReason)
	fmt.Printf("诊断源:     %s\n", diagnosis.Source)
	fmt.Printf("诊断时间:   %s\n", diagnosis.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("触发路径:   %v\n", diagnosis.TriggerPath)
	fmt.Printf("基本事件:   %v\n", diagnosis.BasicEvents)
	fmt.Println(strings.Repeat("═", 70) + "\n")
}

func convertToRecoveryDiagnosis(diagnosis *diagnosisModels.DiagnosisResult) recovery.DiagnosisResult {
	result := recovery.DiagnosisResult{
		DiagnosisID:  diagnosis.DiagnosisID,
		FaultTreeID:  diagnosis.FaultTreeID,
		TopEventID:   diagnosis.TopEventID,
		TopEventName: diagnosis.TopEventName,
		FaultCode:    diagnosis.FaultCode,
		FaultReason:  diagnosis.FaultReason,
		Source:       diagnosis.Source,
		Timestamp:    diagnosis.Timestamp,
		TriggerPath:  diagnosis.TriggerPath,
		BasicEvents:  diagnosis.BasicEvents,
		Metadata:     diagnosis.Metadata,
	}

	if result.Metadata == nil {
		result.Metadata = map[string]interface{}{}
	}

	if _, ok := result.Metadata["status"]; !ok {
		result.Metadata["status"] = "FIRING"
	}

	return result
}
