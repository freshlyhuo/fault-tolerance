package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	//"health-monitor/pkg/alert"
	"health-monitor/pkg/business"
	"health-monitor/pkg/microservice"
	"health-monitor/pkg/state"
)

func main() {
	// 命令行参数
	ecsmURL := flag.String("ecsm-url", "http://192.168.31.129:3001", "容器平台 API 地址")
	interval := flag.Int("interval", 5, "监控采集间隔(秒)")
	testBusiness := flag.Bool("test-business", false, "测试模式：模拟业务层报文")
	testInterval := flag.Int("test-interval", 5, "测试模式下报文发送间隔(秒)")
	flag.Parse()

	fmt.Printf("========== 健康监控系统启动 ==========\n")
	fmt.Printf("容器平台地址: %s\n", *ecsmURL)
	fmt.Println("存储模式: 纯内存缓存")
	fmt.Printf("微服务层采集间隔: %d秒\n", *interval)
	if *testBusiness {
		fmt.Printf("业务层测试模式: 已启用（报文间隔: %d秒）\n", *testInterval)
	}
	fmt.Println("======================================")
	fmt.Println()

	// 创建 context，用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. 初始化状态管理器
	fmt.Println("初始化状态管理器...")
	sm, err := state.NewStateManager()
	if err != nil {
		fmt.Printf("❌ 初始化状态管理器失败: %v\n", err)
		os.Exit(1)
	}
	defer sm.Close()

	// 2. 初始化业务层组件
	fmt.Println("初始化业务层监控...")
	businessDispatcher := business.NewDispatcher(sm)
	businessReceiver := business.NewReceiver(businessDispatcher)
	businessReceiver.Start(ctx)

	// 3. 如果启用测试模式，启动业务层报文模拟
	if *testBusiness {
		fmt.Println("启动业务层报文模拟...")
		go businessTestLoop(ctx, businessReceiver, time.Duration(*testInterval)*time.Second)
	}

	// 4. 初始化微服务层组件
	fmt.Println("初始化微服务层监控...")
	fetcher := microservice.NewFetcher(*ecsmURL)
	microDispatcher := microservice.NewDispatcher(fetcher, sm)

	// 5. 启动微服务层定期采集
	fmt.Println("启动微服务层定期采集...")
	fmt.Println()
	go microServiceMonitorLoop(ctx, microDispatcher, time.Duration(*interval)*time.Second)

	// 6. 监听系统信号，优雅退出
	fmt.Println("✅ 系统运行中，按 Ctrl+C 停止")
	fmt.Println()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n收到退出信号，正在关闭...")
	cancel()
	businessReceiver.Stop()
	time.Sleep(time.Second)
	fmt.Println("系统已停止")
}

// 微服务层监控循环
func microServiceMonitorLoop(ctx context.Context, dispatcher *microservice.Dispatcher, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即执行一次
	collectAndReport(ctx, dispatcher)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collectAndReport(ctx, dispatcher)
		}
	}
}

// 采集并报告
func collectAndReport(ctx context.Context, dispatcher *microservice.Dispatcher) {
	startTime := time.Now()
	_, err := dispatcher.RunOnce(ctx)
	if err != nil {
		fmt.Printf("⚠️  [%s] 微服务层采集失败: %v\n", time.Now().Format("15:04:05"), err)
	} else {
		duration := time.Since(startTime)
		fmt.Printf("✅ [%s] 微服务层采集成功 (耗时: %v)\n", time.Now().Format("15:04:05"), duration)
	}
}

// 业务层测试循环 - 模拟报文发送
func businessTestLoop(ctx context.Context, receiver *business.Receiver, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	packetCount := 0

	// 立即发送一次
	sendTestPackets(receiver, &packetCount)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendTestPackets(receiver, &packetCount)
		}
	}
}

// 发送测试报文
func sendTestPackets(receiver *business.Receiver, count *int) {
	*count++
	
	// 模拟供电服务报文（交替正常和异常）
	var powerPacket []byte
	if *count%2 == 1 {
		// 正常报文
		powerPacket = buildPowerPacket(12.5, 25.0, 3.3, 1.2)
		fmt.Printf("📤 [%s] 发送业务层报文 #%d: 供电服务(正常)\n", time.Now().Format("15:04:05"), *count)
	} else {
		// 异常报文
		powerPacket = buildPowerPacket(10.8, 19.0, 2.7, 6.8)
		fmt.Printf("📤 [%s] 发送业务层报文 #%d: 供电服务(异常)\n", time.Now().Format("15:04:05"), *count)
	}
	receiver.Submit(powerPacket)
	
	// 模拟热控服务报文
	temps := []float64{25, 26, 24, 27, 23, 25, 26, 24, 25, 26}
	if *count%3 == 0 {
		// 偶尔发送高温报文
		temps[0] = 85.0
		fmt.Printf("📤 [%s] 发送业务层报文 #%d: 热控服务(高温)\n", time.Now().Format("15:04:05"), *count)
	}
	thermalPacket := buildThermalPacket(temps)
	receiver.Submit(thermalPacket)
	
	// 模拟通信服务报文
	commPacket := buildCommPacket(0x01, 0x00) // 正常状态
	receiver.Submit(commPacket)
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

// buildCommPacket 构建通信服务报文
func buildCommPacket(status, errorCode byte) []byte {
	packet := make([]byte, 3+2)
	packet[0] = 0x07 // 通信服务
	packet[1] = 0x00
	packet[2] = 2 // 长度
	packet[3] = status
	packet[4] = errorCode
	
	return packet
}
