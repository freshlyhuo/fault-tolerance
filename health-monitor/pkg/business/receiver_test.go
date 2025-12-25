package business

import (
	"encoding/binary"
	"testing"
)

// TestPowerPacketParsing 测试供电服务报文解析
func TestPowerPacketParsing(t *testing.T) {
	dispatcher := NewDispatcher()
	receiver := NewReceiver(dispatcher)

	// 构造供电服务报文
	packet := make([]byte, 3+18)
	packet[0] = CompPower // 类型: 供电服务
	binary.BigEndian.PutUint16(packet[1:3], 18) // 数据长度

	// 填充数据
	binary.BigEndian.PutUint16(packet[3:5], 24000)   // voltage = 24.0V
	binary.BigEndian.PutUint16(packet[5:7], 1500)    // current = 1.5A
	binary.BigEndian.PutUint16(packet[7:9], 13000)   // power_module_12v = 13.0V
	binary.BigEndian.PutUint16(packet[9:11], 25000)  // battery_voltage = 25.0V
	binary.BigEndian.PutUint16(packet[11:13], 24500) // bus_voltage = 24.5V
	binary.BigEndian.PutUint16(packet[13:15], 3300)  // cpu_voltage = 3.3V
	binary.BigEndian.PutUint16(packet[15:17], 5000)  // thermal_ref_voltage = 5.0V
	binary.BigEndian.PutUint16(packet[17:19], 1200)  // bracket_12v_current = 1.2A
	binary.BigEndian.PutUint16(packet[19:21], 2000)  // load_current = 2.0A

	// 解析报文
	metrics, err := receiver.ParsePacket(packet)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}

	// 验证基本字段
	if metrics.ComponentType != CompPower {
		t.Errorf("组件类型错误: got %d, want %d", metrics.ComponentType, CompPower)
	}
	if metrics.Voltage != 24.0 {
		t.Errorf("电压错误: got %f, want %f", metrics.Voltage, 24.0)
	}
	if metrics.Current != 1.5 {
		t.Errorf("电流错误: got %f, want %f", metrics.Current, 1.5)
	}

	// 验证扩展字段
	if v, ok := metrics.Payload["power_module_12v"].(float64); !ok || v != 13.0 {
		t.Errorf("12V功率模块电压错误: got %v, want 13.0", v)
	}
	if v, ok := metrics.Payload["battery_voltage"].(float64); !ok || v != 25.0 {
		t.Errorf("蓄电池电压错误: got %v, want 25.0", v)
	}
	if v, ok := metrics.Payload["cpu_voltage"].(float64); !ok || v != 3.3 {
		t.Errorf("CPU板电压错误: got %v, want 3.3", v)
	}
}

// TestThermalPacketParsing 测试热控服务报文解析
func TestThermalPacketParsing(t *testing.T) {
	dispatcher := NewDispatcher()
	receiver := NewReceiver(dispatcher)

	// 构造热控服务报文
	packet := make([]byte, 3+31)
	packet[0] = CompThermal // 类型: 热控服务
	binary.BigEndian.PutUint16(packet[1:3], 31) // 数据长度

	// 填充主温度
	binary.BigEndian.PutUint16(packet[3:5], 250) // temperature = 25.0℃

	// 10个热控温度点
	for i := 0; i < 10; i++ {
		binary.BigEndian.PutUint16(packet[5+i*2:7+i*2], 230) // 23.0℃
	}

	// 蓄电池温度
	binary.BigEndian.PutUint16(packet[25:27], 280) // battery_temp_1 = 28.0℃
	binary.BigEndian.PutUint16(packet[27:29], 275) // battery_temp_2 = 27.5℃

	// 其他温度
	binary.BigEndian.PutUint16(packet[29:31], 240) // platform_thermal_temp = 24.0℃
	binary.BigEndian.PutUint16(packet[31:33], 220) // tank_thermal_temp = 22.0℃

	// 开关状态: 全部打开
	packet[33] = 0x07 // bit0=1, bit1=1, bit2=1

	// 解析报文
	metrics, err := receiver.ParsePacket(packet)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}

	// 验证主温度
	if metrics.Temperature != 25.0 {
		t.Errorf("主温度错误: got %f, want %f", metrics.Temperature, 25.0)
	}

	// 验证热控温度数组
	if temps, ok := metrics.Payload["thermal_temps"].([]float64); ok {
		if len(temps) != 10 {
			t.Errorf("热控温度点数量错误: got %d, want 10", len(temps))
		}
		for i, temp := range temps {
			if temp != 23.0 {
				t.Errorf("热控温度点%d错误: got %f, want 23.0", i+1, temp)
			}
		}
	} else {
		t.Error("热控温度数组解析失败")
	}

	// 验证开关状态
	if v, ok := metrics.Payload["platform_heater_switch"].(bool); !ok || !v {
		t.Errorf("平台加热开关状态错误: got %v, want true", v)
	}
	if v, ok := metrics.Payload["battery_heater_switch"].(bool); !ok || !v {
		t.Errorf("蓄电池加热开关状态错误: got %v, want true", v)
	}
	if v, ok := metrics.Payload["tank_heater_switch"].(bool); !ok || !v {
		t.Errorf("储箱加热开关状态错误: got %v, want true", v)
	}
}

// TestCommPacketParsing 测试通信服务报文解析
func TestCommPacketParsing(t *testing.T) {
	dispatcher := NewDispatcher()
	receiver := NewReceiver(dispatcher)

	// 构造通信服务报文
	packet := make([]byte, 3+6)
	packet[0] = CompComm // 类型: 通信服务
	binary.BigEndian.PutUint16(packet[1:3], 6) // 数据长度

	packet[3] = 15 // SNR
	binary.BigEndian.PutUint16(packet[4:6], 9600) // rate
	packet[6] = 1 // can_status: 正常应答
	packet[7] = 1 // serial_status: 有正常遥测
	packet[8] = 1 // air_to_air_status: 正常

	// 解析报文
	metrics, err := receiver.ParsePacket(packet)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}

	// 验证字段
	if v, ok := metrics.Payload["SNR"].(uint8); !ok || v != 15 {
		t.Errorf("SNR错误: got %v, want 15", v)
	}
	if v, ok := metrics.Payload["can_status"].(uint8); !ok || v != 1 {
		t.Errorf("CAN状态错误: got %v, want 1", v)
	}
}

// TestTransceiverPacketParsing 测试通信机报文解析
func TestTransceiverPacketParsing(t *testing.T) {
	dispatcher := NewDispatcher()
	receiver := NewReceiver(dispatcher)

	// 构造通信机报文
	packet := make([]byte, 3+9)
	packet[0] = CompTransceiver // 类型: 通信机
	binary.BigEndian.PutUint16(packet[1:3], 9) // 数据长度

	packet[3] = 20  // power
	packet[4] = 1   // telemetry_encrypt_status: 密态
	packet[5] = 1   // telecontrol_encrypt_status: 密态
	packet[6] = 1   // transmit_switch: 打开
	packet[7] = 18  // info_channel_snr
	packet[9] = 100 // receive_rssi
	binary.BigEndian.PutUint16(packet[10:12], 1234) // air_to_air_control_count

	// 解析报文
	metrics, err := receiver.ParsePacket(packet)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}

	// 验证字段
	if v, ok := metrics.Payload["telemetry_encrypt_status"].(uint8); !ok || v != 1 {
		t.Errorf("遥测加密状态错误: got %v, want 1", v)
	}
	if v, ok := metrics.Payload["transmit_switch"].(uint8); !ok || v != 1 {
		t.Errorf("发射开关状态错误: got %v, want 1", v)
	}
}

// TestActuatorPacketParsing 测试姿态控制机构报文解析
func TestActuatorPacketParsing(t *testing.T) {
	dispatcher := NewDispatcher()
	receiver := NewReceiver(dispatcher)

	// 构造姿态控制机构报文
	packet := make([]byte, 3+8)
	packet[0] = CompActuator // 类型: 姿态控制机构
	binary.BigEndian.PutUint16(packet[1:3], 8) // 数据长度

	binary.BigEndian.PutUint16(packet[3:5], 100)  // wheelSpeed = 100
	binary.BigEndian.PutUint16(packet[5:7], 98)   // wheel_speed_x = 98
	binary.BigEndian.PutUint16(packet[7:9], 102)  // wheel_speed_y = 102
	binary.BigEndian.PutUint16(packet[9:11], 101) // wheel_speed_z = 101

	// 解析报文
	metrics, err := receiver.ParsePacket(packet)
	if err != nil {
		t.Fatalf("解析报文失败: %v", err)
	}

	// 验证字段
	if v, ok := metrics.Payload["wheelSpeed"].(int16); !ok || v != 100 {
		t.Errorf("主轮转速错误: got %v, want 100", v)
	}
	if v, ok := metrics.Payload["wheel_speed_x"].(int16); !ok || v != 98 {
		t.Errorf("X轴转速错误: got %v, want 98", v)
	}
}

// 注: 串口通信、计数器等指标已合并到通信服务(CompComm)中,
// 相关测试可在 TestCommPacketParsing 中进行

// TestInvalidPacket 测试无效报文处理
func TestInvalidPacket(t *testing.T) {
	dispatcher := NewDispatcher()
	receiver := NewReceiver(dispatcher)

	// 测试报文太短
	shortPacket := []byte{0x01, 0x00}
	_, err := receiver.ParsePacket(shortPacket)
	if err == nil {
		t.Error("应该返回错误:报文太短")
	}

	// 测试长度不匹配
	mismatchPacket := []byte{0x03, 0x00, 0x10, 0x01, 0x02} // 声称16字节但只有2字节数据
	_, err = receiver.ParsePacket(mismatchPacket)
	if err == nil {
		t.Error("应该返回错误:长度不匹配")
	}

	// 测试未知组件类型
	unknownPacket := []byte{0xFF, 0x00, 0x02, 0x01, 0x02}
	_, err = receiver.ParsePacket(unknownPacket)
	if err == nil {
		t.Error("应该返回错误:未知组件类型")
	}
}
