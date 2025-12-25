package main

import (
	"encoding/binary"
	"fmt"
	"time"
	
	"health-monitor/pkg/alert"
	"health-monitor/pkg/business"
	model "health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

func main() {
	fmt.Println("========== 完整系统集成演示 ==========\n")
	
	// 1. 创建状态管理器（纯内存模式）
	fmt.Println("1. 初始化状态管理器（纯内存模式）...")
	sm, err := state.NewStateManager() // 不传参数 = 纯内存模式
	if err != nil {
		panic(err)
	}
	defer sm.Close()
	
	// 2. 创建业务层组件
	fmt.Println("2. 初始化业务层监控...")
	businessDispatcher := business.NewDispatcher(sm)
	businessReceiver := business.NewReceiver(businessDispatcher)
	
	fmt.Println("\n========== 模拟数据采集 ==========\n")
	
	// 模拟业务层报文
	fmt.Println("3. 模拟业务层报文...")
	
	// 模拟供电服务报文
	powerPacket := buildPowerPacket(12.8, 25.5, 3.3, 1.1) // 电压正常
	businessReceiver.Submit(powerPacket)
	time.Sleep(500 * time.Millisecond)
	
	// 模拟异常供电报文
	powerPacketAbnormal := buildPowerPacket(11.0, 19.5, 2.8, 6.5) // 电压异常
	businessReceiver.Submit(powerPacketAbnormal)
	time.Sleep(500 * time.Millisecond)
	
	// 模拟热控服务报文
	thermalPacket := buildThermalPacket([]float64{25, 26, 24, 27, 23, 25, 26, 24, 25, 26})
	businessReceiver.Submit(thermalPacket)
	time.Sleep(500 * time.Millisecond)
	
	// 6. 模拟微服务层数据
	fmt.Println("\n6. 模拟微服务层指标...")
	
	// 模拟正常节点
	normalNode := &state.NodeMetric{
		Data: &model.NodeMetrics{
			ID:               "node-001",
			Status:           "online",
			CPUUsage:         70.0,
			MemoryTotal:      16000000000,
			MemoryFree:       6000000000,
			DiskTotal:        100.0,
			DiskFree:         40.0,
			ContainerTotal:   10,
			ContainerRunning: 9,
		},
		Timestamp: time.Now().Unix(),
	}
	sm.UpdateMetric(normalNode)
	fmt.Println("  ✅ 正常节点数据已保存")
	
	// 模拟异常节点（CPU高、内存低）
	abnormalNode := &state.NodeMetric{
		Data: &model.NodeMetrics{
			ID:               "node-002",
			Status:           "online",
			CPUUsage:         92.0, // 触发告警
			MemoryTotal:      16000000000,
			MemoryFree:       800000000, // 95%使用率，触发告警
			DiskTotal:        100.0,
			DiskFree:         8.0, // 92%使用率，触发告警
			ContainerTotal:   10,
			ContainerRunning: 6, // 60%运行，触发告警
		},
		Timestamp: time.Now().Unix(),
	}
	sm.UpdateMetric(abnormalNode)
	
	// 检查节点阈值
	alerts := alert.CheckNodeThresholds(abnormalNode.Data)
	if len(alerts) > 0 {
		fmt.Printf("  ⚠️  异常节点检测到 %d 个告警\n", len(alerts))
		for _, a := range alerts {
			fmt.Printf("    - %s: %s\n", a.FaultCode, a.Message)
		}
	}
	
	time.Sleep(500 * time.Millisecond)
	
	// 7. 查询状态管理器
	fmt.Println("\n========== 状态查询演示 ==========\n")
	
	fmt.Println("7. 查询最新状态...")
	
	// 查询节点状态
	if metric, exists := sm.GetLatestState(state.MetricTypeNode, "node-001"); exists {
		nm := metric.(*state.NodeMetric)
		fmt.Printf("  节点 %s:\n", nm.Data.ID)
		fmt.Printf("    状态: %s\n", nm.Data.Status)
		fmt.Printf("    CPU: %.1f%%\n", nm.Data.CPUUsage)
		fmt.Printf("    内存使用: %.1f%%\n", 
			float64(nm.Data.MemoryTotal-nm.Data.MemoryFree)/float64(nm.Data.MemoryTotal)*100)
		fmt.Printf("    容器运行: %d/%d\n", nm.Data.ContainerRunning, nm.Data.ContainerTotal)
	}
	
	// 查询业务层状态（供电服务）
	if metric, exists := sm.GetLatestState(state.MetricTypeBusiness, string(rune(0x03))); exists {
		bm := metric.(*state.BusinessMetric)
		if powerData, ok := bm.Data.Data.(*model.PowerMetrics); ok {
			fmt.Printf("\n  供电服务:\n")
			fmt.Printf("    12V电压: %.2fV\n", powerData.PowerModule12V)
			fmt.Printf("    蓄电池电压: %.2fV\n", powerData.BatteryVoltage)
			fmt.Printf("    CPU电压: %.2fV\n", powerData.CPUVoltage)
		}
	}
	
	// 8. 历史趋势分析
	fmt.Println("\n8. 历史趋势分析...")
	
	// 插入多条历史数据模拟趋势
	for i := 0; i < 5; i++ {
		trendNode := &state.NodeMetric{
			Data: &model.NodeMetrics{
				ID:       "node-trend",
				Status:   "online",
				CPUUsage: 60.0 + float64(i*5), // 逐步上升
			},
			Timestamp: time.Now().Unix(),
		}
		sm.UpdateMetric(trendNode)
		time.Sleep(200 * time.Millisecond)
	}
	
	history := sm.QueryHistory(state.MetricTypeNode, "node-trend", 5*time.Second)
	if len(history) >= 2 {
		firstCPU := history[0].Data.(*model.NodeMetrics).CPUUsage.(float64)
		lastCPU := history[len(history)-1].Data.(*model.NodeMetrics).CPUUsage.(float64)
		fmt.Printf("  节点 node-trend CPU趋势:\n")
		fmt.Printf("    起始: %.1f%%\n", firstCPU)
		fmt.Printf("    当前: %.1f%%\n", lastCPU)
		fmt.Printf("    变化: +%.1f%%\n", lastCPU-firstCPU)
		
		if lastCPU-firstCPU > 10 {
			fmt.Println("    ⚠️  检测到CPU持续上升趋势!")
		}
	}
	
	// 9. 统计信息
	fmt.Println("\n9. 系统统计信息:")
	stats := sm.GetStats()
	for k, v := range stats {
		fmt.Printf("  %s: %v\n", k, v)
	}
	
	// 10. 保存快照
	fmt.Println("\n10. 保存状态快照...")
	if err := sm.SaveSnapshot(); err != nil {
		fmt.Printf("  保存失败: %v\n", err)
	} else {
		fmt.Println("  ✅ 快照已保存到 /tmp/integration_demo.db")
	}
	
	fmt.Println("\n========== 演示完成 ==========")
	fmt.Println("\n数据流向:")
	fmt.Println("  业务层: 报文 → Receiver → Dispatcher → StateManager + Alert")
	fmt.Println("  微服务层: ECSM → Fetcher → Extractor → Dispatcher → StateManager + Alert")
	fmt.Println("  状态管理: Ring Buffer(实时) + BoltDB(持久化)")
	fmt.Println("  告警生成: Threshold检查 → Generator输出")
}

// buildPowerPacket 构建供电服务报文
func buildPowerPacket(v12, vBat, vCPU, current float64) []byte {
	packet := make([]byte, 3+14)
	packet[0] = 0x03 // 供电服务
	packet[1] = 0x00
	packet[2] = 14 // 长度
	
	// 12V电压
	binary.BigEndian.PutUint16(packet[3:5], uint16(v12*1000))
	// 蓄电池电压
	binary.BigEndian.PutUint16(packet[5:7], uint16(vBat*1000))
	// 母线电压
	binary.BigEndian.PutUint16(packet[7:9], uint16(vBat*1000))
	// CPU电压
	binary.BigEndian.PutUint16(packet[9:11], uint16(vCPU*1000))
	// 热敏基准电压
	binary.BigEndian.PutUint16(packet[11:13], uint16(5.0*1000))
	// 12V电流
	binary.BigEndian.PutUint16(packet[13:15], uint16(1.2*1000))
	// 负载电流
	binary.BigEndian.PutUint16(packet[15:17], uint16(current*1000))
	
	return packet
}

// buildThermalPacket 构建热控服务报文
func buildThermalPacket(temps []float64) []byte {
	packet := make([]byte, 3+31)
	packet[0] = 0x06 // 热控服务
	packet[1] = 0x00
	packet[2] = 31 // 长度
	
	// 10个温度点
	for i := 0; i < 10 && i < len(temps); i++ {
		binary.BigEndian.PutUint16(packet[3+i*2:5+i*2], uint16(temps[i]*10))
	}
	
	// 蓄电池温度
	binary.BigEndian.PutUint16(packet[23:25], uint16(25.0*10))
	binary.BigEndian.PutUint16(packet[25:27], uint16(26.0*10))
	
	// 其他温度
	binary.BigEndian.PutUint16(packet[27:29], uint16(30.0*10))
	binary.BigEndian.PutUint16(packet[29:31], uint16(28.0*10))
	binary.BigEndian.PutUint16(packet[31:33], uint16(25.0*10))
	
	// 开关状态
	packet[33] = 0x07 // 所有开关打开
	
	return packet
}
