package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"fault-diagnosis/pkg/config"
	"fault-diagnosis/pkg/engine"
	"fault-diagnosis/pkg/models"
	"fault-diagnosis/pkg/utils"

	"go.uber.org/zap"
)

func main() {
	// 创建日志
	logger, _ := utils.NewLogger("info")
	defer logger.Sync()

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║         故障诊断模块 - 综合测试演示                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝\n")

	// 选择测试场景
	fmt.Println("请选择测试场景:")
	fmt.Println("  1. 业务层故障诊断（电源系统）")
	fmt.Println("  2. 微服务层故障诊断（性能问题）")
	fmt.Println("  3. 全部测试")
	fmt.Print("\n请输入选项 (1-3): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(input)

	switch choice {
	case "1":
		testBusinessLayer(logger)
	case "2":
		testMicroserviceLayer(logger)
	case "3":
		testBusinessLayer(logger)
		fmt.Println("\n\n")
		testMicroserviceLayer(logger)
	default:
		fmt.Println("无效选项，运行全部测试...")
		testBusinessLayer(logger)
		fmt.Println("\n\n")
		testMicroserviceLayer(logger)
	}
}

// 测试业务层故障诊断
func testBusinessLayer(logger *zap.Logger) {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              业务层故障诊断 - 电源系统测试                  ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝\n")

	// 加载业务层故障树
	loader := config.NewLoader("./configs/fault_tree_business.json")
	faultTree, err := loader.LoadFaultTree()
	if err != nil {
		fmt.Printf("❌ 加载故障树失败: %v\n", err)
		return
	}

	fmt.Printf("✓ 已加载故障树: %s\n", faultTree.Description)
	fmt.Printf("✓ 顶层事件数量: %d\n", len(faultTree.TopEvents))
	fmt.Printf("✓ 基本事件数量: %d\n\n", len(faultTree.BasicEvents))

	// 创建诊断引擎
	diagnosisEngine, err := engine.NewDiagnosisEngine(faultTree, logger)
	if err != nil {
		fmt.Printf("❌ 创建诊断引擎失败: %v\n", err)
		return
	}

	// 设置诊断回调
	diagnosisEngine.SetCallback(printDiagnosisResult)

	// 场景1: 仅蓄电池电压异常（不触发顶层故障）
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📌 场景 1: 单一告警 - 蓄电池电压异常")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 说明: 仅触发蓄电池电压异常，不满足故障树逻辑")
	fmt.Println("🎯 预期: 不应触发顶层故障（AND门需要多个条件）\n")

	alert1 := &models.AlertEvent{
		AlertID:     "BATTERY_VOLTAGE_ALERT",
		Type:        "voltage_abnormal",
		Severity:    "warning",
		Source:      "battery_monitor",
		Message:     "蓄电池电压异常: 23.5V (正常范围: 24V-28V)",
		Timestamp:   time.Now().Unix(),
		FaultCode:   "",
		MetricValue: 23.5,
		Metadata: map[string]interface{}{
			"threshold": "24-28V",
			"actual":    "23.5V",
		},
	}
	diagnosisEngine.ProcessAlert(alert1)
	time.Sleep(300 * time.Millisecond)
	fmt.Println("✓ 场景 1 完成\n")

	// 场景2: 蓄电池异常（蓄电池 + 母线电压异常）
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📌 场景 2: 蓄电池故障 - 蓄电池和母线电压同时异常")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 说明: 满足蓄电池异常条件 (EVT-001 AND EVT-002 AND NOT EVT-003)")
	fmt.Println("🎯 预期: 触发顶层故障 CJB-RG-ZD-3，诊断为蓄电池异常\n")

	alert2 := &models.AlertEvent{
		AlertID:     "BUS_VOLTAGE_ALERT",
		Type:        "voltage_abnormal",
		Severity:    "warning",
		Source:      "bus_monitor",
		Message:     "母线电压异常: 26.2V (正常范围: 24V-28V)",
		Timestamp:   time.Now().Unix(),
		FaultCode:   "",
		MetricValue: 26.2,
		Metadata: map[string]interface{}{
			"threshold": "24-28V",
			"actual":    "26.2V",
		},
	}
	diagnosisEngine.ProcessAlert(alert2)
	time.Sleep(800 * time.Millisecond)
	fmt.Println("✓ 场景 2 完成\n")

	// 重置状态
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔄 重置所有事件状态...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	diagnosisEngine.ResetAll()
	time.Sleep(500 * time.Millisecond)

	// 场景3: AD模块异常（仅CPU板电压异常）
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📌 场景 3: AD 模块故障 - CPU板电压异常")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 说明: CPU板电压异常可能是AD模块采集错误")
	fmt.Println("🎯 预期: 触发顶层故障 CJB-RG-ZD-3，诊断为AD模块异常\n")

	alert3 := &models.AlertEvent{
		AlertID:     "CPU_VOLTAGE_ALERT",
		Type:        "voltage_abnormal",
		Severity:    "critical",
		Source:      "cpu_board_monitor",
		Message:     "CPU板电压异常: TMEZD01011 = 3.8V (正常范围: 3.1V-3.5V)",
		Timestamp:   time.Now().Unix(),
		FaultCode:   "",
		MetricValue: 3.8,
		Metadata: map[string]interface{}{
			"threshold": "3.1-3.5V",
			"actual":    "3.8V",
			"sensor":    "TMEZD01011",
		},
	}
	diagnosisEngine.ProcessAlert(alert3)
	time.Sleep(800 * time.Millisecond)
	fmt.Println("✓ 场景 3 完成\n")

	// 重置状态
	fmt.Println("🔄 重置状态...\n")
	diagnosisEngine.ResetAll()
	time.Sleep(300 * time.Millisecond)

	// 场景4: 所有告警同时触发（AD模块优先）
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📌 场景 4: 多重故障 - 所有电压异常同时发生")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 说明: 蓄电池、母线、CPU板电压全部异常")
	fmt.Println("🎯 预期: 优先诊断为AD模块异常（因为存在NOT逻辑）\n")

	diagnosisEngine.ProcessAlert(alert1)
	time.Sleep(100 * time.Millisecond)
	diagnosisEngine.ProcessAlert(alert2)
	time.Sleep(100 * time.Millisecond)
	diagnosisEngine.ProcessAlert(alert3)
	time.Sleep(800 * time.Millisecond)
	fmt.Println("✓ 场景 4 完成\n")

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           业务层故障诊断测试完成                            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
}

// 测试微服务层故障诊断
func testMicroserviceLayer(logger *zap.Logger) {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║            微服务层故障诊断 - 性能问题测试                  ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝\n")

	// 加载微服务层故障树
	loader := config.NewLoader("./configs/fault_tree_microservice.json")
	faultTree, err := loader.LoadFaultTree()
	if err != nil {
		fmt.Printf("❌ 加载故障树失败: %v\n", err)
		return
	}

	fmt.Printf("✓ 已加载故障树: %s\n", faultTree.Description)
	fmt.Printf("✓ 顶层事件数量: %d\n", len(faultTree.TopEvents))
	fmt.Printf("✓ 基本事件数量: %d\n\n", len(faultTree.BasicEvents))

	// 创建诊断引擎
	diagnosisEngine, err := engine.NewDiagnosisEngine(faultTree, logger)
	if err != nil {
		fmt.Printf("❌ 创建诊断引擎失败: %v\n", err)
		return
	}

	// 设置诊断回调
	diagnosisEngine.SetCallback(printDiagnosisResult)

	// 场景1: 容器CPU使用率过高
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📌 场景 1: 容器 CPU 过载")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 说明: 容器CPU使用率达到95%")
	fmt.Println("🎯 预期: 触发 CPU过载故障 (SVC-PERF-001) 和 容器资源耗尽 (CONTAINER-RESOURCE-001)\n")

	alertCPU := &models.AlertEvent{
		AlertID:     "CONTAINER_CPU_HIGH",
		Type:        "cpu_high",
		Severity:    "critical",
		Source:      "user-service-container-1",
		Message:     "容器CPU使用率过高: 95%",
		Timestamp:   time.Now().Unix(),
		FaultCode:   "",
		MetricValue: 95.0,
		Metadata: map[string]interface{}{
			"threshold": "90%",
			"container": "user-service-container-1",
			"pod":       "user-service-pod-abc123",
		},
	}
	diagnosisEngine.ProcessAlert(alertCPU)
	time.Sleep(800 * time.Millisecond)
	fmt.Println("✓ 场景 1 完成\n")

	// 场景2: CPU波动异常
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📌 场景 2: CPU 波动异常")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 说明: CPU使用率在短时间内剧烈波动")
	fmt.Println("🎯 预期: 触发 CPU过载或波动异常 (SVC-PERF-001)\n")

	alertFluctuation := &models.AlertEvent{
		AlertID:     "CONTAINER_CPU_FLUCTUATION",
		Type:        "cpu_fluctuation",
		Severity:    "warning",
		Source:      "order-service-container-1",
		Message:     "CPU使用率波动异常: 标准差 = 35%",
		Timestamp:   time.Now().Unix(),
		FaultCode:   "",
		MetricValue: 35.0,
		Metadata: map[string]interface{}{
			"threshold": "20%",
			"metric":    "cpu_usage_stddev",
			"container": "order-service-container-1",
		},
	}
	diagnosisEngine.ProcessAlert(alertFluctuation)
	time.Sleep(800 * time.Millisecond)
	fmt.Println("✓ 场景 2 完成\n")

	// 重置状态
	fmt.Println("🔄 重置状态...\n")
	diagnosisEngine.ResetAll()
	time.Sleep(300 * time.Millisecond)

	// 场景3: 容器内存使用率过高
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📌 场景 3: 容器内存耗尽")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 说明: 容器内存使用率达到92%")
	fmt.Println("🎯 预期: 触发 容器资源耗尽 (CONTAINER-RESOURCE-001)\n")

	alertMemory := &models.AlertEvent{
		AlertID:     "CONTAINER_MEMORY_HIGH",
		Type:        "memory_high",
		Severity:    "critical",
		Source:      "payment-service-container-1",
		Message:     "容器内存使用率过高: 92%",
		Timestamp:   time.Now().Unix(),
		FaultCode:   "",
		MetricValue: 92.0,
		Metadata: map[string]interface{}{
			"threshold": "90%",
			"container": "payment-service-container-1",
			"limit":     "2Gi",
			"used":      "1.84Gi",
		},
	}
	diagnosisEngine.ProcessAlert(alertMemory)
	time.Sleep(800 * time.Millisecond)
	fmt.Println("✓ 场景 3 完成\n")

	// 重置状态
	fmt.Println("🔄 重置状态...\n")
	diagnosisEngine.ResetAll()
	time.Sleep(300 * time.Millisecond)

	// 场景4: 服务级联故障（CPU + 内存同时过高）
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📌 场景 4: 服务级联故障")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 说明: CPU和内存同时达到阈值，触发级联故障")
	fmt.Println("🎯 预期: 同时触发多个顶层故障，包括服务级联故障 (SVC-CASCADE-001)\n")

	// 先触发CPU告警
	diagnosisEngine.ProcessAlert(alertCPU)
	time.Sleep(200 * time.Millisecond)

	// 再触发内存告警
	diagnosisEngine.ProcessAlert(alertMemory)
	time.Sleep(1000 * time.Millisecond)
	fmt.Println("✓ 场景 4 完成\n")

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          微服务层故障诊断测试完成                           ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
}

// 打印诊断结果
func printDiagnosisResult(diagnosis *models.DiagnosisResult) {
	fmt.Println("\n" + strings.Repeat("═", 70))
	fmt.Println("🚨 检测到系统级故障!")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("📋 诊断ID:     %s\n", diagnosis.DiagnosisID)
	fmt.Printf("⚠️  故障码:     %s\n", diagnosis.FaultCode)
	fmt.Printf("📊 顶层事件:   %s\n", diagnosis.TopEventName)
	fmt.Printf("📝 故障原因:   %s\n", diagnosis.FaultReason)
	fmt.Printf("⏰ 诊断时间:   %s\n", diagnosis.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("🔍 触发路径:   %v\n", diagnosis.TriggerPath)
	fmt.Printf("🎯 基本事件:   %v\n", diagnosis.BasicEvents)
	fmt.Println(strings.Repeat("═", 70) + "\n")
}
