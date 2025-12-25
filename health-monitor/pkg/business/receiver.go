/*
提供共性服务提交指标的接口
当有输入时才启动
报文格式
报文格式存在几个问题
1.有些指标给我们的文件中没有明确是属于那个服务的，我们先按照自己的理解分
2.给我们的是原始数据还是处理过的数据，如果是原始数据，我们还需要知道处理方法，暂时按照处理过的数据设计
3.有些指标没有范围暂时未编入
解析完数据后，交由alert/threshold判断
*/
package business

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"health-monitor/pkg/models"
)

// 业务层 Receiver：用于监听业务报文、解析、分发
type Receiver struct {
	dispatcher *Dispatcher
	inputChan  chan []byte
	stopChan   chan struct{}
}

func NewReceiver(dispatcher *Dispatcher) *Receiver {
	return &Receiver{
		dispatcher: dispatcher,
		inputChan:  make(chan []byte, 100),
		stopChan:   make(chan struct{}),
	}
}

////////////////////////////////////////////////////////////////////////////////
//                           1. 提供共性服务提交接口
////////////////////////////////////////////////////////////////////////////////

// 只有收到了业务层输入才会触发解析与分发
func (r *Receiver) Submit(data []byte) error {
	if len(data) < 3 { // 至少需要：类型 + 长度字段
		return errors.New("invalid business packet")
	}
	r.inputChan <- data
	return nil
}

////////////////////////////////////////////////////////////////////////////////
//                           监听器启动/停止
////////////////////////////////////////////////////////////////////////////////

func (r *Receiver) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case <-r.stopChan:
				return

			case packet := <-r.inputChan:
				metrics, err := r.ParsePacket(packet)
				if err != nil {
					fmt.Println("[业务层] 报文解析失败:", err)
					continue
				}

				// 分发到业务层的 dispatcher
				r.dispatcher.HandleBusinessMetrics(ctx, metrics)
			}
		}
	}()
}

func (r *Receiver) Stop() {
	close(r.stopChan)
}

////////////////////////////////////////////////////////////////////////////////
//                           2. 报文解析模块
////////////////////////////////////////////////////////////////////////////////

// 组件编号（首字节）
const (
	CompRunMgr       = 0x01 // 运行管理
	CompComm         = 0x02 // 通信服务（CAN、串口等）
	CompPower        = 0x03 // 供电服务
	CompRailCtrl     = 0x04 // 轨道控制
	CompPayload      = 0x05 // 载荷
	CompThermal      = 0x06 // 热控服务
	CompAttCtrl      = 0x07 // 姿态控制
	CompMeasure      = 0x08 // 测量
	CompOptical      = 0x09 // 光电设备
	CompSensor       = 0x0A // 敏感器
	CompActuator     = 0x0B // 姿态控制机构(动量轮)
	CompTransceiver  = 0x0C // 通信机
	CompThruster     = 0x0D // 推进器
	CompEPS          = 0x0E // 电源
)

// 解析业务层报文
func (r *Receiver) ParsePacket(packet []byte) (*model.BusinessMetrics, error) {

	component := packet[0]
	length := binary.BigEndian.Uint16(packet[1:3])

	if int(length) > len(packet)-3 {
		return nil, errors.New("length mismatch in business packet")
	}

	payload := packet[3 : 3+length]

	out := &model.BusinessMetrics{
		ComponentType: component,
		Timestamp:     time.Now().Unix(),
		Data:          make(map[string]interface{}),
	}

	switch component {

	case CompRunMgr:
		r.parseRunMgr(payload, out)

	case CompComm:
		r.parseComm(payload, out)

	case CompPower:
		r.parsePower(payload, out)

	case CompRailCtrl:
		r.parseRailCtrl(payload, out)

	case CompPayload:
		r.parsePayload(payload, out)

	case CompThermal:
		r.parseThermal(payload, out)

	case CompAttCtrl:
		r.parseAttCtrl(payload, out)

	case CompMeasure:
		r.parseMeasure(payload, out)

	case CompOptical:
		r.parseOptical(payload, out)

	case CompSensor:
		r.parseSensor(payload, out)

	case CompActuator:
		r.parseActuator(payload, out)

	case CompTransceiver:
		r.parseTransceiver(payload, out)

	case CompThruster:
		r.parseThruster(payload, out)

	case CompEPS:
		r.parseEPS(payload, out)

	default:
		return nil, fmt.Errorf("unknown business packet type: 0x%02X", component)
	}

	return out, nil
}

////////////////////////////////////////////////////////////////////////////////
//                     3. 各组件的解析函数（可继续扩展）
////////////////////////////////////////////////////////////////////////////////
//运行管理
func (r *Receiver) parseRunMgr(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 5 {
		return
	}
	
	metrics := &model.RunMgrMetrics{
		Timestamp: time.Now().Unix(),
		Temperature: float64(binary.BigEndian.Uint16(payload[0:2])) / 10.0,
		Voltage: float64(binary.BigEndian.Uint16(payload[2:4])) / 1000.0,
		StatusCode: int(payload[4]),
		Payload: make(map[string]interface{}),
	}
	
	out.Data = metrics
}
//通信服务:包含CAN、串口、空空通信状态,以及串口错误计数、命令接收计数
func (r *Receiver) parseComm(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 16 {
		return
	}
	
	metrics := &model.CommMetrics{
		Timestamp: time.Now().Unix(),
		SNR: payload[0],
		Rate: binary.BigEndian.Uint16(payload[1:3]),
		CANStatus: payload[3],
		SerialStatus: payload[4],
		AirToAirStatus: payload[5],
		// 串口错误计数
		ParityErrorCount: binary.BigEndian.Uint16(payload[6:8]),
		FrameHeaderErrorCount: binary.BigEndian.Uint16(payload[8:10]),
		FrameLengthErrorCount: binary.BigEndian.Uint16(payload[10:12]),
		SerialResetCount: binary.BigEndian.Uint16(payload[12:14]),
		// 命令接收计数
		ReceiveCmdCount: binary.BigEndian.Uint32(payload[14:18]),
	}
	
	out.Data = metrics
}
//供电服务：包含电压类和电流类指标
func (r *Receiver) parsePower(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 14 {
		return
	}
	
	metrics := &model.PowerMetrics{
		Timestamp: time.Now().Unix(),
	}
	
	// 解析所有电压和电流指标
	metrics.PowerModule12V = float64(binary.BigEndian.Uint16(payload[0:2])) / 1000.0     // TMAN01046
	metrics.BatteryVoltage = float64(binary.BigEndian.Uint16(payload[2:4])) / 1000.0     // TMEZD01095
	metrics.BusVoltage = float64(binary.BigEndian.Uint16(payload[4:6])) / 1000.0         // TMEZD01096
	metrics.CPUVoltage = float64(binary.BigEndian.Uint16(payload[6:8])) / 1000.0         // TMEZD01011
	metrics.ThermalRefVoltage = float64(binary.BigEndian.Uint16(payload[8:10])) / 1000.0 // TMEZD01100
	metrics.Bracket12VCurrent = float64(binary.BigEndian.Uint16(payload[10:12])) / 1000.0 // TMAN01050
	metrics.LoadCurrent = float64(binary.BigEndian.Uint16(payload[12:14])) / 1000.0      // TMEZD01247
	
	out.Data = metrics
}
//轨道控制
func (r *Receiver) parseRailCtrl(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 1 {
		return
	}
	
	metrics := &model.RailCtrlMetrics{
		Timestamp: time.Now().Unix(),
		OrbitMode: payload[0],
	}
	
	out.Data = metrics
}
//载荷
func (r *Receiver) parsePayload(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 1 {
		return
	}
	
	metrics := &model.PayloadMetrics{
		Timestamp: time.Now().Unix(),
		WorkMode: payload[0],
	}
	
	out.Data = metrics
}
//热控服务：包含温度类和开关状态类指标
func (r *Receiver) parseThermal(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 31 {
		return
	}
	
	metrics := &model.ThermalMetrics{
		Timestamp: time.Now().Unix(),
	}
	
	// 解析10个热控温度点（TMEZD01066-01075）
	for i := 0; i < 10; i++ {
		metrics.ThermalTemps[i] = float64(int16(binary.BigEndian.Uint16(payload[i*2:i*2+2]))) / 10.0
	}
	
	// 蓄电池温度（TMEZD01084、TMEZD01085）
	metrics.BatteryTemp1 = float64(int16(binary.BigEndian.Uint16(payload[20:22]))) / 10.0
	metrics.BatteryTemp2 = float64(int16(binary.BigEndian.Uint16(payload[22:24]))) / 10.0
	
	// 其他热控温度
	metrics.PlatformThermalTemp = float64(int16(binary.BigEndian.Uint16(payload[24:26]))) / 10.0
	metrics.BatteryThermalTemp = float64(int16(binary.BigEndian.Uint16(payload[26:28]))) / 10.0
	metrics.TankThermalTemp = float64(int16(binary.BigEndian.Uint16(payload[28:30]))) / 10.0
	
	// 开关状态（TMEZD01121, TMEZD01254, TMEZD01115）
	switchState := payload[30]
	metrics.PlatformHeaterSwitch = (switchState & 0x01) != 0
	metrics.BatteryHeaterSwitch = (switchState & 0x02) != 0
	metrics.TankHeaterSwitch = (switchState & 0x04) != 0
	
	out.Data = metrics
}
//姿态控制
func (r *Receiver) parseAttCtrl(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 1 {
		return
	}
	
	metrics := &model.AttCtrlMetrics{
		Timestamp: time.Now().Unix(),
		ControlMode: payload[0],
	}
	
	out.Data = metrics
}
//测量
func (r *Receiver) parseMeasure(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 4 {
		return
	}
	
	metrics := &model.MeasureMetrics{
		Timestamp: time.Now().Unix(),
		SensorValue: binary.BigEndian.Uint32(payload[0:4]),
	}
	
	out.Data = metrics
}
//光电设备
func (r *Receiver) parseOptical(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 2 {
		return
	}
	
	metrics := &model.OpticalMetrics{
		Timestamp: time.Now().Unix(),
		PhotoCurrent: binary.BigEndian.Uint16(payload[0:2]),
	}
	
	out.Data = metrics
}
//敏感器
func (r *Receiver) parseSensor(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 6 {
		return
	}
	
	metrics := &model.SensorMetrics{
		Timestamp: time.Now().Unix(),
		AccX: int16(binary.BigEndian.Uint16(payload[0:2])),
		AccY: int16(binary.BigEndian.Uint16(payload[2:4])),
		AccZ: int16(binary.BigEndian.Uint16(payload[4:6])),
	}
	
	out.Data = metrics
}
//姿态控制机构：动量轮三轴转速
func (r *Receiver) parseActuator(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 6 {
		return
	}
	
	metrics := &model.ActuatorMetrics{
		Timestamp: time.Now().Unix(),
		WheelSpeedX: int16(binary.BigEndian.Uint16(payload[0:2])), // TMEGNC2029
		WheelSpeedY: int16(binary.BigEndian.Uint16(payload[2:4])), // TMEGNC2030
		WheelSpeedZ: int16(binary.BigEndian.Uint16(payload[4:6])), // TMEGNC2031
	}
	
	out.Data = metrics
}
//通信机:包含发射功率、信噪比、RSSI、开关状态等
func (r *Receiver) parseTransceiver(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 7 {
		return
	}
	
	metrics := &model.TransceiverMetrics{
		Timestamp: time.Now().Unix(),
		TransmitPower: payload[0],
		TelemetryEncryptStatus: payload[1],     // TMEZD01167
		TelecontrolEncryptStatus: payload[2],   // TMEZD01168
		TransmitSwitch: payload[3],             // TMEZD01155
		InfoChannelSNR: payload[4],             // TMEZD01145
		ReceiveRSSI: int8(payload[6]),          // TMEZD01147
	}
	
	out.Data = metrics
}
//推进器：燃料、管路开关、压力传感器
func (r *Receiver) parseThruster(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 5 {
		return
	}
	
	metrics := &model.ThrusterMetrics{
		Timestamp: time.Now().Unix(),
		FuelLevel: binary.BigEndian.Uint16(payload[0:2]),
		PipelineSwitch: payload[2],
		PressureSensor: binary.BigEndian.Uint16(payload[3:5]),
	}
	
	out.Data = metrics
}
//电源
func (r *Receiver) parseEPS(payload []byte, out *model.BusinessMetrics) {
	if len(payload) < 4 {
		return
	}
	
	metrics := &model.EPSMetrics{
		Timestamp: time.Now().Unix(),
		Voltage: float64(binary.BigEndian.Uint16(payload[0:2])) / 1000.0,
		Current: float64(binary.BigEndian.Uint16(payload[2:4])) / 1000.0,
	}
	
	out.Data = metrics
}
