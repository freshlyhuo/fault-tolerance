package business

import (
	"context"
	"encoding/binary"
	"testing"
	"time"
)

// TestDispatcherToGeneratorFlow 测试从Dispatcher到Generator的完整流程
func TestDispatcherToGeneratorFlow(t *testing.T) {
	// 1. 创建组件
	dispatcher := NewDispatcher()
	receiver := NewReceiver(dispatcher)
	
	ctx := context.Background()
	
	// 2. 构造供电服务报文 - 包含异常数据
	t.Log("=== 测试1: 供电服务异常 ===")
	packet1 := buildAbnormalPowerPacket()
	
	// 3. 解析报文
	metrics1, err := receiver.ParsePacket(packet1)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}
	
	// 4. Dispatcher处理指标 -> Generator检查阈值 -> 输出告警
	dispatcher.HandleBusinessMetrics(ctx, metrics1)
	
	time.Sleep(100 * time.Millisecond)
	
	// 5. 测试热控服务
	t.Log("\n=== 测试2: 热控服务异常 ===")
	packet2 := buildAbnormalThermalPacket()
	metrics2, err := receiver.ParsePacket(packet2)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}
	dispatcher.HandleBusinessMetrics(ctx, metrics2)
	
	time.Sleep(100 * time.Millisecond)
	
	// 6. 测试通信服务
	t.Log("\n=== 测试3: 通信服务异常 ===")
	packet3 := buildAbnormalCommPacket()
	metrics3, err := receiver.ParsePacket(packet3)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}
	dispatcher.HandleBusinessMetrics(ctx, metrics3)
	
	time.Sleep(100 * time.Millisecond)
	
	// 7. 测试姿态控制机构
	t.Log("\n=== 测试4: 姿态控制机构异常 ===")
	packet4 := buildAbnormalActuatorPacket()
	metrics4, err := receiver.ParsePacket(packet4)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}
	dispatcher.HandleBusinessMetrics(ctx, metrics4)
	
	time.Sleep(100 * time.Millisecond)
}

// TestNormalMetrics 测试正常数据（无告警）
func TestNormalMetrics(t *testing.T) {
	dispatcher := NewDispatcher()
	receiver := NewReceiver(dispatcher)
	ctx := context.Background()
	
	t.Log("=== 测试正常数据（预期无告警） ===")
	
	// 构造正常的供电服务报文
	packet := make([]byte, 3+14)
	packet[0] = 0x03 // CompPower
	binary.BigEndian.PutUint16(packet[1:3], 14)
	
	// 所有数据都在正常范围内
	binary.BigEndian.PutUint16(packet[3:5], 13000)   // 13.0V - 正常
	binary.BigEndian.PutUint16(packet[5:7], 25000)   // 25.0V - 正常
	binary.BigEndian.PutUint16(packet[7:9], 24500)   // 24.5V - 正常
	binary.BigEndian.PutUint16(packet[9:11], 3300)   // 3.3V - 正常
	binary.BigEndian.PutUint16(packet[11:13], 5000)  // 5.0V - 正常
	binary.BigEndian.PutUint16(packet[13:15], 1200)  // 1.2A - 正常
	binary.BigEndian.PutUint16(packet[15:17], 2000)  // 2.0A - 正常
	
	metrics, err := receiver.ParsePacket(packet)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}
	
	dispatcher.HandleBusinessMetrics(ctx, metrics)
	time.Sleep(100 * time.Millisecond)
}

// 辅助函数：构造异常的供电服务报文
func buildAbnormalPowerPacket() []byte {
	packet := make([]byte, 3+14)
	packet[0] = 0x03 // CompPower
	binary.BigEndian.PutUint16(packet[1:3], 14)
	
	// 填充异常数据
	binary.BigEndian.PutUint16(packet[3:5], 11000)   // 11.0V - 异常！(正常约13V)
	binary.BigEndian.PutUint16(packet[5:7], 19000)   // 19.0V - 异常！(正常[21,29.4]V)
	binary.BigEndian.PutUint16(packet[7:9], 24500)   // 24.5V - 正常
	binary.BigEndian.PutUint16(packet[9:11], 2800)   // 2.8V - 异常！(正常[3.1,3.5]V)
	binary.BigEndian.PutUint16(packet[11:13], 5000)  // 5.0V - 正常
	binary.BigEndian.PutUint16(packet[13:15], 1200)  // 1.2A - 正常
	binary.BigEndian.PutUint16(packet[15:17], 6000)  // 6.0A - 异常！(正常[0.5,5]A)
	
	return packet
}

// 辅助函数：构造异常的热控服务报文
func buildAbnormalThermalPacket() []byte {
	packet := make([]byte, 3+31)
	packet[0] = 0x06 // CompThermal
	binary.BigEndian.PutUint16(packet[1:3], 31)
	
	// 填充10个热控温度点 - 部分异常
	for i := 0; i < 10; i++ {
		temp := int16(230) // 23.0℃ (正常)
		if i == 2 {
			temp = int16(600) // 60.0℃ - 异常！
		}
		if i == 7 {
			temp = int16(-250) // -25.0℃ - 异常！
		}
		binary.BigEndian.PutUint16(packet[3+i*2:5+i*2], uint16(temp))
	}
	
	// 蓄电池温度
	binary.BigEndian.PutUint16(packet[23:25], uint16(int16(280)))  // 28.0℃ - 正常
	binary.BigEndian.PutUint16(packet[25:27], uint16(int16(500))) // 50.0℃ - 异常！
	
	// 其他温度
	binary.BigEndian.PutUint16(packet[27:29], uint16(int16(240)))
	binary.BigEndian.PutUint16(packet[29:31], uint16(int16(220)))
	binary.BigEndian.PutUint16(packet[31:33], uint16(int16(220)))
	
	// 开关状态
	packet[33] = 0x07
	
	return packet
}

// 辅助函数：构造异常的通信服务报文
func buildAbnormalCommPacket() []byte {
	packet := make([]byte, 3+6)
	packet[0] = 0x02 // CompComm
	binary.BigEndian.PutUint16(packet[1:3], 6)
	
	packet[3] = 15 // SNR
	binary.BigEndian.PutUint16(packet[4:6], 9600) // rate
	packet[6] = 0  // CAN状态: 无应答 - 异常！
	packet[7] = 0  // 串口状态: 无遥测 - 异常！
	packet[8] = 1  // 空空通信状态: 正常
	
	return packet
}

// 辅助函数：构造异常的姿态控制机构报文
func buildAbnormalActuatorPacket() []byte {
	packet := make([]byte, 3+6)
	packet[0] = 0x0B // CompActuator
	binary.BigEndian.PutUint16(packet[1:3], 6)
	
	binary.BigEndian.PutUint16(packet[3:5], 98)   // X轴: 98转 - 正常
	binary.BigEndian.PutUint16(packet[5:7], 150)  // Y轴: 150转 - 异常！
	binary.BigEndian.PutUint16(packet[7:9], 70)   // Z轴: 70转 - 异常！
	
	return packet
}
