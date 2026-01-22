// UNUSED: 示例程序，当前未被 health-monitor 模块的可执行程序依赖（截至 2026-01-22）。
package main

import (
	"fmt"
	"model"
	"state"
	"time"
)

func main() {
	fmt.Println("========== 状态管理器示例程序 ==========\n")
	
	// 1. 创建状态管理器
	fmt.Println("1. 创建状态管理器...")
	sm, err := state.NewStateManager("/tmp/demo_state.db")
	if err != nil {
		panic(err)
	}
	defer sm.Close()
	
	// 2. 模拟数据写入
	fmt.Println("\n2. 模拟10秒钟的指标采集...")
	for i := 0; i < 10; i++ {
		// 模拟节点指标变化
		nodeMetric := &state.NodeMetric{
			Data: &model.NodeMetrics{
				ID:               "node-demo",
				Status:           "online",
				CPUUsage:         50.0 + float64(i*3), // CPU逐渐上升
				MemoryTotal:      16000000000,
				MemoryFree:       8000000000 - int64(i*100000000), // 内存逐渐减少
				ContainerTotal:   10,
				ContainerRunning: 10 - i/3, // 部分容器失败
			},
			Timestamp: time.Now().Unix(),
		}
		sm.UpdateMetric(nodeMetric)
		
		// 模拟容器指标
		containerMetric := &state.ContainerMetric{
			Data: &model.ContainerMetrics{
				ID:           "container-demo",
				Status:       "running",
				DeployStatus: "success",
				Uptime:       100 + i*10,
				CPUUsage: model.CPUUsage{
					Total: 60.0 + float64(i*4),
				},
				MemoryUsage: 500000000 + int64(i*50000000),
				MemoryLimit: 1000000000,
			},
			Timestamp: time.Now().Unix(),
		}
		sm.UpdateMetric(containerMetric)
		
		fmt.Printf("  第%d秒: CPU=%.1f%%, 运行容器=%d/%d\n", 
			i+1, nodeMetric.Data.CPUUsage, nodeMetric.Data.ContainerRunning, nodeMetric.Data.ContainerTotal)
		
		time.Sleep(1 * time.Second)
	}
	
	// 3. 查询最新状态
	fmt.Println("\n3. 查询最新状态...")
	if metric, exists := sm.GetLatestState(state.MetricTypeNode, "node-demo"); exists {
		nm := metric.(*state.NodeMetric)
		fmt.Printf("  节点ID: %s\n", nm.Data.ID)
		fmt.Printf("  状态: %s\n", nm.Data.Status)
		fmt.Printf("  CPU使用率: %.1f%%\n", nm.Data.CPUUsage)
		fmt.Printf("  容器运行: %d/%d\n", nm.Data.ContainerRunning, nm.Data.ContainerTotal)
	}
	
	// 4. 查询历史趋势
	fmt.Println("\n4. 查询最近10秒的历史数据...")
	history := sm.QueryHistory(state.MetricTypeNode, "node-demo", 10*time.Second)
	fmt.Printf("  找到 %d 条历史记录\n", len(history))
	
	// 分析CPU趋势
	if len(history) >= 2 {
		firstCPU := history[0].Data.(*model.NodeMetrics).CPUUsage.(float64)
		lastCPU := history[len(history)-1].Data.(*model.NodeMetrics).CPUUsage.(float64)
		trend := lastCPU - firstCPU
		
		fmt.Printf("  CPU变化: %.1f%% → %.1f%% (增长 %.1f%%)\n", firstCPU, lastCPU, trend)
		
		if trend > 10 {
			fmt.Println("  ⚠️ 检测到CPU持续上升趋势!")
		}
	}
	
	// 5. 容器历史分析
	fmt.Println("\n5. 分析容器资源使用趋势...")
	containerHistory := sm.QueryHistory(state.MetricTypeContainer, "container-demo", 10*time.Second)
	
	if len(containerHistory) >= 2 {
		firstMem := containerHistory[0].Data.(*model.ContainerMetrics).MemoryUsage
		lastMem := containerHistory[len(containerHistory)-1].Data.(*model.ContainerMetrics).MemoryUsage
		memLimit := containerHistory[0].Data.(*model.ContainerMetrics).MemoryLimit
		
		firstPercent := float64(firstMem) / float64(memLimit) * 100
		lastPercent := float64(lastMem) / float64(memLimit) * 100
		
		fmt.Printf("  内存使用: %.1f%% → %.1f%%\n", firstPercent, lastPercent)
		
		if lastPercent > 80 {
			fmt.Println("  ⚠️ 容器内存使用率过高!")
		}
	}
	
	// 6. 保存快照
	fmt.Println("\n6. 保存状态快照到BoltDB...")
	if err := sm.SaveSnapshot(); err != nil {
		fmt.Printf("  保存失败: %v\n", err)
	} else {
		fmt.Println("  ✅ 快照保存成功")
	}
	
	// 7. 统计信息
	fmt.Println("\n7. 状态管理器统计信息:")
	stats := sm.GetStats()
	for k, v := range stats {
		fmt.Printf("  %s: %v\n", k, v)
	}
	
	// 8. 模拟程序重启恢复
	fmt.Println("\n8. 模拟程序重启恢复...")
	sm.Close()
	
	fmt.Println("  重新启动状态管理器...")
	sm2, err := state.NewStateManager("/tmp/demo_state.db")
	if err != nil {
		panic(err)
	}
	defer sm2.Close()
	
	// 验证数据已恢复
	if metric, exists := sm2.GetLatestState(state.MetricTypeNode, "node-demo"); exists {
		nm := metric.(*state.NodeMetric)
		fmt.Printf("  ✅ 快照恢复成功! 节点 %s 状态: %s\n", nm.Data.ID, nm.Data.Status)
	}
	
	fmt.Println("\n========== 演示完成 ==========")
	fmt.Println("\n核心优势:")
	fmt.Println("  ✅ Ring Buffer: 纳秒级查询，支持实时趋势分析")
	fmt.Println("  ✅ BoltDB: 持久化快照，程序重启数据不丢失")
	fmt.Println("  ✅ 自动清理: 过期数据自动淘汰，内存可控")
	fmt.Println("  ✅ 并发安全: 读写锁保护，支持高并发访问")
}
