package main

import (
	"context"
	"fmt"
	"time"

	"health-monitor/pkg/alert"
	"health-monitor/pkg/microservice"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

func main() {
	fmt.Println("========== 趋势分析演示 ==========\n")

	ctx := context.Background()

	// 1. 创建状态管理器（纯内存模式）
	fmt.Println("1. 初始化状态管理器（纯内存模式）...")
	sm, err := state.NewStateManager() // 不传参数 = 纯内存模式
	if err != nil {
		panic(err)
	}
	defer sm.Close()

	// 2. 创建告警生成器（带趋势分析）
	fmt.Println("2. 初始化告警生成器（含趋势分析）...")
	generator := alert.NewGeneratorWithStateManager(sm)

	// 3. 创建微服务层组件
	fmt.Println("3. 初始化微服务层监控...\n")

	// ========== 场景1: CPU持续上升趋势 ==========
	fmt.Println("========== 场景1: CPU持续上升趋势 ==========")
	simulateCPUTrend(ctx, sm, generator)

	time.Sleep(2 * time.Second)

	// ========== 场景2: 内存持续增长趋势 ==========
	fmt.Println("\n========== 场景2: 内存持续增长趋势 ==========")
	simulateMemoryTrend(ctx, sm, generator)

	time.Sleep(2 * time.Second)

	// ========== 场景3: 容器频繁重启趋势 ==========
	fmt.Println("\n========== 场景3: 容器频繁重启趋势 ==========")
	simulateRestartTrend(ctx, sm, generator)

	time.Sleep(2 * time.Second)

	// ========== 场景4: 业务校验失败率上升 ==========
	fmt.Println("\n========== 场景4: 业务校验失败率上升 ==========")
	simulateValidationTrend(ctx, sm, generator)

	fmt.Println("\n========== 演示完成 ==========")
	fmt.Println("\n趋势分析说明:")
	fmt.Println("  1. 连续上升: 指标值在连续多个采样点持续增长")
	fmt.Println("  2. 连续下降: 指标值在连续多个采样点持续降低")
	fmt.Println("  3. 变化率: 计算相邻点的平均变化率")
	fmt.Println("  4. 预测: 根据当前趋势预测未来可能发生的问题")
	fmt.Println("\n告警级别:")
	fmt.Println("  - Critical: 已经发生故障，需要立即干预")
	fmt.Println("  - Warning: 有指标趋势异常，可能即将发生故障")
	fmt.Println("  - Info: 信息提示")
}

// simulateCPUTrend 模拟CPU持续上升趋势
func simulateCPUTrend(ctx context.Context, sm *state.StateManager, generator *alert.Generator) {
	nodeID := "node-cpu-trend"
	fmt.Printf("模拟节点 %s 的CPU使用率持续上升...\n", nodeID)

	// 插入12个数据点，CPU从60%上升到85%
	for i := 0; i < 12; i++ {
		cpuUsage := 60.0 + float64(i)*2.5 // 每次增加2.5%
		nodeMetric := &state.NodeMetric{
			Data: &model.NodeMetrics{
				ID:          nodeID,
				Status:      "online",
				CPUUsage:    cpuUsage,
				MemoryTotal: 16000000000,
				MemoryFree:  8000000000,
			},
			Timestamp: time.Now().Unix(),
		}
		sm.UpdateMetric(nodeMetric)
		fmt.Printf("  [%02d] CPU: %.1f%%\n", i+1, cpuUsage)
		time.Sleep(100 * time.Millisecond)
	}

	// 创建微服务指标集并触发分析
	time.Sleep(500 * time.Millisecond)

	// 获取最新指标
	if metric, exists := sm.GetLatestState(state.MetricTypeNode, nodeID); exists {
		nm := metric.(*state.NodeMetric)
		metrics := &model.MicroServiceMetricsSet{
			NodeMetrics: []model.NodeMetrics{*nm.Data},
		}

		fmt.Println("\n执行趋势分析...")
		generator.ProcessMicroserviceMetrics(ctx, metrics)
	}
}

// simulateMemoryTrend 模拟内存持续增长趋势
func simulateMemoryTrend(ctx context.Context, sm *state.StateManager, generator *alert.Generator) {
	nodeID := "node-mem-trend"
	fmt.Printf("模拟节点 %s 的内存使用率持续上升...\n", nodeID)

	memTotal := int64(16000000000) // 16GB

	// 插入12个数据点，内存从50%上升到88%
	for i := 0; i < 12; i++ {
		usagePercent := 50.0 + float64(i)*3.5 // 每次增加3.5%
		memFree := int64(float64(memTotal) * (100.0 - usagePercent) / 100.0)

		nodeMetric := &state.NodeMetric{
			Data: &model.NodeMetrics{
				ID:          nodeID,
				Status:      "online",
				CPUUsage:    50.0,
				MemoryTotal: memTotal,
				MemoryFree:  memFree,
			},
			Timestamp: time.Now().Unix(),
		}
		sm.UpdateMetric(nodeMetric)
		fmt.Printf("  [%02d] 内存使用: %.1f%%\n", i+1, usagePercent)
		time.Sleep(100 * time.Millisecond)
	}

	// 执行趋势分析
	time.Sleep(500 * time.Millisecond)

	if metric, exists := sm.GetLatestState(state.MetricTypeNode, nodeID); exists {
		nm := metric.(*state.NodeMetric)
		metrics := &model.MicroServiceMetricsSet{
			NodeMetrics: []model.NodeMetrics{*nm.Data},
		}

		fmt.Println("\n执行趋势分析...")
		generator.ProcessMicroserviceMetrics(ctx, metrics)
	}
}

// simulateRestartTrend 模拟容器频繁重启
func simulateRestartTrend(ctx context.Context, sm *state.StateManager, generator *alert.Generator) {
	containerID := "container-restart-trend"
	fmt.Printf("模拟容器 %s 频繁重启...\n", containerID)

	// 插入12个数据点，模拟重启（Uptime重置）
	uptime := 3600 // 初始运行1小时
	for i := 0; i < 12; i++ {
		// 每3个点重启一次
		if i > 0 && i%3 == 0 {
			uptime = 60 // 重启，从60秒开始
			fmt.Printf("  [%02d] 容器重启! Uptime重置为 %d秒\n", i+1, uptime)
		} else {
			uptime += 300 // 正常运行，增加5分钟
			fmt.Printf("  [%02d] 运行正常, Uptime: %d秒\n", i+1, uptime)
		}

		containerMetric := &state.ContainerMetric{
			Data: &model.ContainerMetrics{
				ID:     containerID,
				Status: "running",
				Uptime: uptime,
			},
			Timestamp: time.Now().Unix(),
		}
		sm.UpdateMetric(containerMetric)
		time.Sleep(100 * time.Millisecond)
	}

	// 执行趋势分析
	time.Sleep(500 * time.Millisecond)

	if metric, exists := sm.GetLatestState(state.MetricTypeContainer, containerID); exists {
		cm := metric.(*state.ContainerMetric)
		metrics := &model.MicroServiceMetricsSet{
			ContainerMetrics: []model.ContainerMetrics{*cm.Data},
		}

		fmt.Println("\n执行趋势分析...")
		generator.ProcessMicroserviceMetrics(ctx, metrics)
	}
}

// simulateValidationTrend 模拟业务校验失败率上升
func simulateValidationTrend(ctx context.Context, sm *state.StateManager, generator *alert.Generator) {
	serviceID := "service-validation-trend"
	fmt.Printf("模拟服务 %s 业务校验失败率上升...\n", serviceID)

	// 插入12个数据点，失败率从1%上升到15%
	for i := 0; i < 12; i++ {
		totalChecks := 1000
		failurePercent := 1.0 + float64(i)*1.2 // 每次增加1.2%
		failures := int(float64(totalChecks) * failurePercent / 100.0)
		successes := totalChecks - failures

		serviceMetric := &state.ServiceMetric{
			Data: &model.ServiceMetrics{
				ID:                   serviceID,
				Status:               "running",
				BusinessCheckSuccess: successes,
				BusinessCheckFail:    failures,
			},
			Timestamp: time.Now().Unix(),
		}
		sm.UpdateMetric(serviceMetric)
		fmt.Printf("  [%02d] 业务校验: 成功=%d, 失败=%d (失败率: %.1f%%)\n",
			i+1, successes, failures, failurePercent)
		time.Sleep(100 * time.Millisecond)
	}

	// 执行趋势分析
	time.Sleep(500 * time.Millisecond)

	if metric, exists := sm.GetLatestState(state.MetricTypeService, serviceID); exists {
		sm := metric.(*state.ServiceMetric)
		metrics := &model.MicroServiceMetricsSet{
			ServiceMetrics: []model.ServiceMetrics{*sm.Data},
		}

		fmt.Println("\n执行趋势分析...")
		generator.ProcessMicroserviceMetrics(ctx, metrics)
	}
}
