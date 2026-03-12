package main

import (
	"fmt"
	"time"

	// 健康监测模块
	healthAlert "health-monitor/pkg/alert"
	healthModel "health-monitor/pkg/models"

	// 故障诊断模块
	diagnosisConfig "fault-diagnosis/pkg/config"
	diagnosisEngine "fault-diagnosis/pkg/engine"
	diagnosisModels "fault-diagnosis/pkg/models"
	diagnosisReceiver "fault-diagnosis/pkg/receiver"

	"go.uber.org/zap"
)

// 演示健康监测和故障诊断通过内存直接集成（无需 etcd）
func main() {
	// 1. 创建日志
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	fmt.Println("========== 健康监测 + 故障诊断 内存集成示例 ==========")
	fmt.Println()

	// 2. 创建故障诊断模块
	fmt.Println("1. 初始化故障诊断模块...")
	
	// 加载故障树配置
	loader := diagnosisConfig.NewLoader("./fault-diagnosis/configs/fault_tree_microservice.json")
	faultTree, err := loader.LoadFaultTree()
	if err != nil {
		logger.Fatal("加载故障树失败", zap.Error(err))
	}

	// 创建诊断引擎
	engine, err := diagnosisEngine.NewDiagnosisEngine(faultTree, logger)
	if err != nil {
		logger.Fatal("创建诊断引擎失败", zap.Error(err))
	}
	engine.SetCallback(func(result *diagnosisModels.DiagnosisResult) {
		fmt.Printf("  [诊断结果] 故障码: %s, 顶层事件: %s\n", result.FaultCode, result.TopEventName)
	})

	// 创建告警接收器
	receiver := diagnosisReceiver.NewChannelReceiver(500, logger)
	
	// 设置告警处理函数
	receiver.SetHandler(func(alert *diagnosisModels.AlertEvent) {
		fmt.Printf("  [诊断模块] 收到告警: %s\n", alert.AlertID)
		engine.ProcessAlert(alert)
	})
	
	// 启动接收器
	if err := receiver.Start(); err != nil {
		logger.Fatal("启动接收器失败", zap.Error(err))
	}
	defer receiver.Stop()

	// 3. 创建接收器包装器（适配健康监测的接口）
	receiverWrapper := diagnosisReceiver.NewReceiverWrapper(receiver)

	// 4. 模拟健康监测产生告警
	fmt.Println("2. 模拟健康监测产生告警...")
	fmt.Println()
	
	// 模拟微服务层告警
	alerts := []*healthModel.AlertEvent{
		{
			AlertID:     "SERVICE_P99_LATENCY_HIGH",
			Type:        "latency_high",
			Source:      "user-service",
			Message:     "用户服务P99延迟过高: 850ms",
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-001",
			MetricValue: 850.0,
			Metadata: map[string]interface{}{
				"threshold": "500ms",
				"service":   "user-service",
			},
		},
		{
			AlertID:     "CONTAINER_CPU_HIGH",
			Type:        "cpu_high",
			Source:      "user-service-container-1",
			Message:     "容器CPU使用率过高: 95%",
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-003",
			MetricValue: 95.0,
			Metadata: map[string]interface{}{
				"threshold":  "80%",
				"container":  "user-service-container-1",
				"pod":        "user-service-pod-1",
			},
		},
		{
			AlertID:     "SERVICE_ERROR_RATE_HIGH",
			Type:        "error_rate_high",
			Source:      "order-service",
			Message:     "订单服务错误率过高: 15%",
			Timestamp:   time.Now().Unix(),
			FaultCode:   "MS-002",
			MetricValue: 15.0,
			Metadata: map[string]interface{}{
				"threshold": "5%",
				"service":   "order-service",
			},
		},
	}

	// 发送告警
	fmt.Println("3. 健康监测发送告警到故障诊断...")
	fmt.Println()
	for _, alert := range alerts {
		if err := receiverWrapper.SendAlert(healthAlert.ConvertToDiagnosisAlertDirect(alert)); err != nil {
			fmt.Printf("发送告警失败: %v\n", err)
		}
	}

	// 等待处理完成
	time.Sleep(2 * time.Second)

	// 6. 查看接收器状态
	fmt.Printf("\n4. 接收器状态:\n")
	fmt.Printf("   队列长度: %d / %d\n", receiver.GetQueueLength(), receiver.GetQueueCapacity())

	fmt.Println("\n========== 集成演示完成 ==========")
	fmt.Println("\n优势:")
	fmt.Println("  ✓ 无需 etcd 依赖")
	fmt.Println("  ✓ 低延迟（内存通信）")
	fmt.Println("  ✓ 适合单机部署")
	fmt.Println("  ✓ 资源消耗低")
	fmt.Println("\n适用场景:")
	fmt.Println("  • 嵌入式系统")
	fmt.Println("  • 资源受限环境")
	fmt.Println("  • 单机故障诊断")
	fmt.Println("  • 开发测试环境")
}
