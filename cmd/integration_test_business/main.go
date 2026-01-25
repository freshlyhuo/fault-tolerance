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


	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()


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

	// 创建故障修复引擎
	recoveryState := recovery.NewInMemoryStateManager()
	recoveryEngine := recovery.NewEngine(recoveryState, recovery.NewEngineConfig{
		QueueSize: 200,
		Timeout:   20 * time.Second,
	})
	recoveryStore := recovery.NewRuntimeStore()
	// 业务层故障码统一走创建服务
	recoveryEngine.RegisterAction("CJB-RG-ZD-3", recovery.NewStartContainerAction(recoveryStore))
	recoveryEngine.RegisterPrefixAction("YW", recovery.NewStartContainerAction(recoveryStore))
	recoveryEngine.Start(ctx)

	// 设置诊断回调
	businessEngine.SetCallback(func(diagnosis *diagnosisModels.DiagnosisResult) {
		status := ""
		if diagnosis.Metadata != nil {
			if v, ok := diagnosis.Metadata["status"].(string); ok {
				status = v
			}
		}
		if status != "RESOLVED" {
			fmt.Println("\n" + strings.Repeat("═", 70))
			fmt.Println("[业务层] 检测到故障!")
			fmt.Println(strings.Repeat("═", 70))
			printDiagnosis(diagnosis)
		}
		_ = recoveryEngine.Submit(convertToRecoveryDiagnosis(diagnosis))
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

	// 创建状态管理器
	stateManager, err := healthState.NewStateManager()
	if err != nil {
		logger.Fatal("创建状态管理器失败", zap.Error(err))
	}
	defer stateManager.Close()
	fmt.Println("  ✓ 状态管理器已创建")

	// 创建告警接收器包装器（集成故障诊断）
	businessWrapper := diagnosisReceiver.NewReceiverWrapper(businessReceiver)

	// 创建业务层调度器和接收器
	businessDispatcher := healthBusiness.NewDispatcher(stateManager)
	businessDispatcher.SetDiagnosisReceiver(businessWrapper)
	businessRecv := healthBusiness.NewReceiver(businessDispatcher)
	go businessRecv.Start(ctx)
	fmt.Println("  ✓ 业务层调度器已创建")

	// ========== 场景 1：蓄电池和母线电压异常 ==========
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("1. 业务层故障模拟测试（场景1）")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	runBusinessScenario3(ctx, businessDispatcher)

	time.Sleep(5 * time.Second)
	fmt.Printf("恢复成功\n")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

}

// runBusinessScenario3 仅执行场景 3：蓄电池和母线电压异常
func runBusinessScenario3(ctx context.Context, dispatcher *healthBusiness.Dispatcher) {
	fmt.Println("\n[业务层] 场景 1: 蓄电池和母线电压异常 (应触发故障)\t")
	dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
		ComponentType: 0x03,
		Timestamp:     time.Now().Unix(),
		Data: &healthModel.PowerMetrics{
			BatteryVoltage: 19.0,
			BusVoltage:     19.0,
			CPUVoltage:     3.3,
			Timestamp:      time.Now().Unix(),
		},
	})

	time.Sleep(1 * time.Second)
	fmt.Println("\n[业务层] 场景 1: 1s 后恢复正常数据")
	dispatcher.HandleBusinessMetrics(ctx, &healthModel.BusinessMetrics{
		ComponentType: 0x03,
		Timestamp:     time.Now().Unix(),
		Data: &healthModel.PowerMetrics{
			BatteryVoltage: 26.0,
			BusVoltage:     26.0,
			CPUVoltage:     3.3,
			Timestamp:      time.Now().Unix(),
		},
	})
}

// runBusinessScenarioNoRecovery 故障报文不恢复的场景
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
