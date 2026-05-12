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
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ========== 1. 初始化故障诊断模块 ==========
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("1. 初始化故障诊断模块")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// 加载业务层故障树
	businessLoader := diagnosisConfig.NewLoader("./fault-diagnosis/configs/fault_tree_business.json")
	businessTree, err := businessLoader.LoadFaultTree()
	if err != nil {
		logger.Fatal("加载业务层故障树失败", zap.Error(err))
	}
	fmt.Printf("  ✓ 业务层故障树: %s\n", businessTree.Description)

	// 创建诊断日志
	diagnosisLogger, _ := diagnosisUtils.NewLogger("info")

	// 创建业务层诊断引擎
	businessEngine, err := diagnosisEngine.NewDiagnosisEngine(businessTree, diagnosisLogger)
	if err != nil {
		logger.Fatal("创建业务层诊断引擎失败", zap.Error(err))
	}

	// 创建故障修复接收层（内部对象输入，不走 HTTP）
	recoveryReceive := recovery.NewReceiveService(recovery.ReceiveConfig{
		QueueSize: 200,
	})
	recoveryReceive.Start(ctx)

	// 设置诊断回调
	businessEngine.SetCallback(func(diagnosis *diagnosisModels.DiagnosisResult) {
		fmt.Println("\n" + strings.Repeat("═", 70))
		fmt.Println("[业务层] 检测到故障!")
		fmt.Println(strings.Repeat("═", 70))
		printDiagnosis(diagnosis)
		_ = recoveryReceive.Submit(convertToRecoveryDiagnosis(diagnosis))
	})

	// 创建告警接收器
	businessReceiver := diagnosisReceiver.NewChannelReceiver(500, diagnosisLogger)

	businessReceiver.SetHandler(func(alert *diagnosisModels.AlertEvent) {
		if alert.Status == "firing" {
			fmt.Printf("  [业务层诊断] 收到告警: %s (status=%s)\n", alert.AlertID, alert.Status)
		}
		businessEngine.ProcessAlert(alert)
	})

	if err := businessReceiver.Start(); err != nil {
		logger.Fatal("启动业务层接收器失败", zap.Error(err))
	}
	defer businessReceiver.Stop()

	// ========== 2. 初始化健康监测模块 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("2. 初始化健康监测模块")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// 创建状态管理器
	stateManager, err := healthState.NewStateManager()
	if err != nil {
		logger.Fatal("创建状态管理器失败", zap.Error(err))
	}
	defer stateManager.Close()
	fmt.Println("  ✓ 状态管理器已创建")

	// 创建告警接收器包装器（集成故障诊断）
	businessWrapper := diagnosisReceiver.NewReceiverWrapper(businessReceiver)

	// 创建业务层调度器
	businessDispatcher := healthBusiness.NewDispatcher(stateManager)
	businessDispatcher.SetDiagnosisReceiver(businessWrapper) // 配置告警接收器
	fmt.Println("  ✓ 业务层调度器已创建")

	// ========== 3. 启动业务层模拟测试 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("3. 业务层故障模拟测试")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// 启动业务层模拟协程
	go runBusinessSimulation(ctx, businessDispatcher, businessWrapper)

	// ========== 4. 等待信号 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("4. 集成测试运行中... (Ctrl+C 停止)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("集成测试结束")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// runBusinessSimulation 运行业务层模拟
func runBusinessSimulation(ctx context.Context, dispatcher *healthBusiness.Dispatcher, diagnosisWrapper *diagnosisReceiver.ReceiverWrapper) {
	fmt.Println("  [业务层] 开始模拟测试...")
	fmt.Println()

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
					Timestamp: time.Now().Unix(),
					Data: &healthModel.PowerMetrics{
						BatteryVoltage: 25.0,
						BusVoltage:     25.0,
						CPUVoltage:     3.3,
						Timestamp:      time.Now().Unix(),
					},
				})

			case 1:
				// 场景2: 蓄电池电压异常
				fmt.Println("\n[业务层] 场景 2: 蓄电池电压异常")
				dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
					Timestamp: time.Now().Unix(),
					Data: &healthModel.PowerMetrics{
						BatteryVoltage: 19.5, // 低于21V
						BusVoltage:     25.0,
						CPUVoltage:     3.3,
						Timestamp:      time.Now().Unix(),
					},
				})

			case 2:
				// 场景3: 蓄电池+母线电压异常
				fmt.Println("\n[业务层] 场景 3: 蓄电池和母线电压异常 (应触发故障)")
				dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
					Timestamp: time.Now().Unix(),
					Data: &healthModel.PowerMetrics{
						BatteryVoltage: 19.0, // 低于21V (母线异常)
						BusVoltage:     19.0,
						CPUVoltage:     3.3,
						Timestamp:      time.Now().Unix(),
					},
				})

			case 3:
				// 场景4: 恢复正常
				fmt.Println("\n[业务层] 场景 4: AD模块异常 ")
				dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
					Timestamp: time.Now().Unix(),
					Data: &healthModel.PowerMetrics{
						BatteryVoltage: 26.0,
						BusVoltage:     26.0,
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
