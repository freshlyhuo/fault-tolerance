package main

import (
	"fmt"
	"strings"
	"time"

	"fault-diagnosis/pkg/config"
	"fault-diagnosis/pkg/engine"
	"fault-diagnosis/pkg/models"
	"fault-diagnosis/pkg/utils"
)

func main() {
	// 创建日志
	logger, _ := utils.NewLogger("info")
	defer logger.Sync()

	fmt.Println("========================================")
	fmt.Println("故障诊断模块 - 业务层示例演示")
	fmt.Println("========================================\n")

	// 加载业务层故障树
	loader := config.NewLoader("./configs/fault_tree_business.json")
	faultTree, err := loader.LoadFaultTree()
	if err != nil {
		fmt.Printf("加载故障树失败: %v\n", err)
		return
	}

	fmt.Printf("已加载故障树: %s\n", faultTree.Description)
	fmt.Printf("顶层事件数量: %d\n", len(faultTree.TopEvents))
	fmt.Printf("基本事件数量: %d\n\n", len(faultTree.BasicEvents))

	// 创建诊断引擎
	diagnosisEngine, err := engine.NewDiagnosisEngine(faultTree, logger)
	if err != nil {
		fmt.Printf("创建诊断引擎失败: %v\n", err)
		return
	}

	// 设置诊断回调
	diagnosisEngine.SetCallback(func(diagnosis *models.DiagnosisResult) {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("🚨 检测到系统级故障!")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("诊断ID:     %s\n", diagnosis.DiagnosisID)
		fmt.Printf("故障码:     %s\n", diagnosis.FaultCode)
		fmt.Printf("顶层事件:   %s\n", diagnosis.TopEventName)
		fmt.Printf("故障原因:   %s\n", diagnosis.FaultReason)
		fmt.Printf("诊断时间:   %s\n", diagnosis.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("触发路径:   %v\n", diagnosis.TriggerPath)
		fmt.Printf("基本事件:   %v\n", diagnosis.BasicEvents)
		fmt.Println(strings.Repeat("=", 60))
	})

	// 场景1: 仅触发蓄电池电压异常 (不应触发顶层事件)
	fmt.Println("📌 场景1: 仅触发蓄电池电压异常")
	fmt.Println("   模拟: 蓄电池电压超出正常范围")
	alert1 := &models.AlertEvent{
		AlertID:   "BATTERY_VOLTAGE_ALERT",
		Type:      "voltage_abnormal",
		Severity:  "warning",
		Source:    "battery_monitor",
		Message:   "蓄电池电压异常",
		Timestamp: time.Now().Unix(),
	}
	diagnosisEngine.ProcessAlert(alert1)
	time.Sleep(100 * time.Millisecond)
	fmt.Println("   结果: 未触发顶层故障（需要更多证据）\n")

	// 场景2: 触发蓄电池和母线电压异常 + CPU板电压正常 (应触发蓄电池异常)
	fmt.Println("📌 场景2: 触发蓄电池和母线电压异常，CPU板电压正常")
	fmt.Println("   模拟: 母线电压异常 + CPU板电压正常")
	alert2 := &models.AlertEvent{
		AlertID:   "BUS_VOLTAGE_ALERT",
		Type:      "voltage_abnormal",
		Severity:  "warning",
		Source:    "bus_monitor",
		Message:   "母线电压异常",
		Timestamp: time.Now().Unix(),
	}
	diagnosisEngine.ProcessAlert(alert2)
	time.Sleep(500 * time.Millisecond)

	// 重置状态，演示另一个场景
	fmt.Println("\n重置所有事件状态...\n")
	diagnosisEngine.ResetAll()
	time.Sleep(500 * time.Millisecond)

	// 场景3: 仅触发CPU板电压异常 (应触发AD模块异常)
	fmt.Println("📌 场景3: 仅触发CPU板电压异常")
	fmt.Println("   模拟: CPU板电压不在正常区间")
	alert3 := &models.AlertEvent{
		AlertID:   "CPU_VOLTAGE_ALERT",
		Type:      "voltage_abnormal",
		Severity:  "critical",
		Source:    "cpu_board_monitor",
		Message:   "CPU板电压异常，TMEZD01011不在[3.1, 3.5]V区间",
		Timestamp: time.Now().Unix(),
	}
	diagnosisEngine.ProcessAlert(alert3)
	time.Sleep(500 * time.Millisecond)

	fmt.Println("\n========================================")
	fmt.Println("演示完成")
	fmt.Println("========================================")
}
