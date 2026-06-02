/* 定义所有指标结构体：

NodeMetric

ContainerMetric

ServiceMetric

BusinessMetric（业务层） */

package model

// ============ 共享类型定义 ============

// CPUUsage CPU使用情况
type CPUUsage struct {
	Total float64   `json:"total"`
	Cores []float64 `json:"cores"`
}

// NodeNetInfo 节点网络信息
type NodeNetInfo struct {
	NetworkName string  `json:"networkName"`
	UpNet       float64 `json:"upNet"`
	DownNet     float64 `json:"downNet"`
}

// ============ 微服务指标集 ============

// 微服务总指标集
type MicroServiceMetricsSet struct {
	NodeMetrics      []NodeMetrics
	ContainerMetrics []ContainerMetrics
	ServiceMetrics   []ServiceMetrics
}

// ---------------- Node ----------------
type NodeMetrics struct {
	ID                   string
	Status               string
	MemoryTotal          int64
	MemoryFree           int64
	DiskTotal            float64
	DiskFree             float64
	CPUUsage             interface{}
	ProcessCount         int
	ContainerTotal       int
	ContainerRunning     int
	ContainerEcsmTotal   int
	ContainerEcsmRunning int
	Net                  []NodeNetInfo
}

// ---------------- Container ----------------
type ContainerMetrics struct {
	ID              string
	Status          string
	Uptime          int
	StartedTime     string
	CreatedTime     string
	TaskCreatedTime string
	DeployStatus    string
	FailedMessage   *string
	RestartCount    int
	DeployNum       int
	CPUUsage        CPUUsage
	MemoryLimit     int64
	MemoryUsage     int64
	MemoryMaxUsage  int64
	SizeUsage       int64
	SizeLimit       int64
	ServiceID       string
	ServiceName     string
}

// ---------------- Service ----------------
type ServiceMetrics struct {
	ID                   string
	Status               string
	ContainerStatusGroup []string
	Healthy              bool
	Factor               int
	Policy               string
	InstanceOnline       int
	InstanceActive       int
	BusinessCheckSuccess int // 业务校验成功次数
	BusinessCheckFail    int // 业务校验失败次数
}

// ---------------- Business ----------------
// BusinessMetrics 业务层健康监测指标基础结构
type BusinessMetrics struct {
	Timestamp int64       // 时间戳
	Data      interface{} // 具体组件的指标数据
}

// ========== 供电服务检测指标 ==========
// PowerMetrics 供电服务指标
type PowerMetrics struct {
	Timestamp int64 // 时间戳

	// 电压类指标
	BatteryVoltage    float64 // TMEZD01095: 蓄电池电压, 正常[21, 29.4]V
	BusVoltage        float64 // TMEZD01096: 母线电压, 正常[21, 29.4]V
	CPUVoltage        float64 // TMEZD01011: CPU板电压, 正常[3.1, 3.5]V
	ThermalRefVoltage float64 // TMEZD01100: 热敏基准电压, 正常[4.5, 5.5]V

	// 电流类指标
	LoadCurrent float64 // TMEZD01247: 负载电流, 正常[0.5, 5]A

	// 设备上电状态类指标，true=on，false=off
	CommunicationPowerStatus bool // Communication_power_status
	GNSSAPowerStatus         bool // GNSSA_power_status
	GNSSBPowerStatus         bool // GNSSB_power_status
	GyroscopePowerStatus     bool // Gyroscope_power_status
	MEMSPowerStatus          bool // MEMS_power_status
	StarTrackerlPowerStatus  bool // StarTrackerl_power_status
	MomentumWheelPowerStatus bool // MomentumWheel_power_status

	// 故障关联编号
	FaultCodes []string // 关联的故障编号
}

// ========== 热控服务检测指标 ==========
// ThermalMetrics 热控服务指标
type ThermalMetrics struct {
	Timestamp int64 // 时间戳

	// 温度类指标
	ThermalTemps        [10]float64 // TMEZD01066-01075: cjb热控温度1~10
	BatteryTemp1        float64     // TMEZD01084: cjb蓄电池温度1
	BatteryTemp2        float64     // TMEZD01085: cjb蓄电池温度2
	PlatformThermalTemp float64     // 平台热控温度
	BatteryThermalTemp  float64     // 蓄电池热控温度
	TankThermalTemp     float64     // 储箱热控温度

	// 开关状态类指标
	PlatformHeaterSwitch bool // TMEZD01121: 平台加热总开关状态, 1=打开
	BatteryHeaterSwitch  bool // TMEZD01254: 蓄电池加热总开关状态, 1=打开
	TankHeaterSwitch     bool // TMEZD01115: 储箱加热总开关状态, 1=打开

	// 测试场景随遥测载荷下发的期望状态；为空时使用阈值配置。
	PlatformHeatingExpected *bool // PlatformHeatingSwitch_ExpectedState
	BatteryHeatingExpected  *bool // BatteryHeatingSwitch_ExpectedState
	TankHeatingExpected     *bool // TankHeatingSwitch_ExpectedState

	// 故障关联编号
	FaultCodes []string // 关联的故障编号
}

// ========== 通信服务检测指标 ==========
// CommMetrics 通信服务指标(包含串口通信)
type CommMetrics struct {
	Timestamp int64 // 时间戳

	// 通信状态类指标
	CANStatus      uint8 // CAN通信状态: 1=正常应答, 0=无应答
	SerialStatus   uint8 // 串口通信状态: 1=有正常遥测, 0=无遥测
	AirToAirStatus uint8 // 空空通信状态: 1=正常收发, 0=异常

	// 通用通信参数
	SNR  uint8  // 信噪比
	Rate uint16 // 通信速率

	// 串口错误计数类指标
	ParityErrorCount      uint16 // TMEZD01046: 串口校验错计数
	FrameHeaderErrorCount uint16 // TMEZD01047: 帧头错误计数
	FrameLengthErrorCount uint16 // TMEZD01048: 帧长度错误计数
	SerialResetCount      uint16 // TMEZD01052: 串口复位计数

	// 命令接收计数
	ReceiveCmdCount             uint32 // TMEZD01004: 接收命令计数
	SoftwareRecInstructionCount uint32 // Com_SoftwareRecInstructionCount
	CorrectRecInstructionCount  uint32 // Com_CorrectRecInstructionCount
	CommandSerialPortCount      uint32 // Com_CommandSeriaPortCount

	// O1响应计数类指标
	O1ReceiveDeviceResponseCount        uint32 // O1_ReceiveDeviceResponseCount
	O1ReceiveTelemetryResponseCount     uint32 // O1_ReceiveTelemetryResponseCount
	O1ReceiveRemoteControlResponseCount uint32 // O1_ReceiveRemoteControlResponseCount

	// 无遥测计数
	NoTelemetryCount uint32 // Com_No_telemetry_count

	// 通信机状态与计数
	TransmitSwitch           uint8  // TMEZD01155: 发射通道开关状态
	ReceiveCTACount          uint32 // TMEZD01150: 空空遥控计数
	TelemetryEncryptStatus   uint8  // TMEZD01167: 遥测明/密状态
	TelecontrolEncryptStatus uint8  // TMEZD01168: 遥控明/密状态
	ReceiveRSSI              int16  // TMEZD01147: 接收RSSI

	// 测试场景随遥测载荷下发的期望状态；为空时使用阈值配置。
	TransmissionExpected  *int // transmission_channel_ExpectedState
	TelemetryExpected     *int // Communication_Telemetry_ExpectedState
	RemoteControlExpected *int // Communicator_RemoteControl_ExpectedState

	// 故障关联编号
	FaultCodes []string // 关联的故障编号
}

// ========== 通信机检测指标 ==========
// TransceiverMetrics 通信机指标
type TransceiverMetrics struct {
	Timestamp int64 // 时间戳

	// 状态类指标
	TelemetryEncryptStatus   uint8 // TMEZD01167: 遥测明/密状态, 1=密态
	TelecontrolEncryptStatus uint8 // TMEZD01168: 遥控明/密状态, 1=密态
	TransmitSwitch           uint8 // TMEZD01155: 发射通道开关状态, 1=打开

	// 信号质量指标
	InfoChannelSNR uint8 // TMEZD01145: 信息通道接收信噪比
	ReceiveRSSI    int8  // TMEZD01147: 接收RSSI

	// 功率
	TransmitPower uint8 // 发射功率

	// 故障关联编号
	FaultCodes []string // 关联的故障编号
}

// ========== 姿态控制机构检测指标 ==========
// ActuatorMetrics 姿态控制机构(动量轮)指标
type ActuatorMetrics struct {
	Timestamp int64 // 时间戳

	// 指令状态
	SendCmdK53029 uint8 // SendCmd_K53029: 动量轮转速设置指令

	// 动量轮转速指标
	WheelSpeedX int16 // TMEGNC2029: X轴动量轮转速(反馈), 正常约100转
	WheelSpeedY int16 // TMEGNC2030: Y轴动量轮转速(反馈), 正常约100转
	WheelSpeedZ int16 // TMEGNC2031: Z轴动量轮转速(反馈), 正常约100转

	// 无遥测计数
	NoTelemetryCount uint32 // MomentumWheel_No_telemetry_count

	// 串口错误计数类指标
	CheckErrorCount       uint32 // TMEZD01013_CheckErrorCount
	FrameHeaderErrorCount uint32 // TMEZD01014_FrameHeaderErrorCount
	FrameLengthErrorCount uint32 // TMEZD01015_FrameLengthErrorCount
	ResetCount            uint32 // TMEZD01016_ResetCount

	// 故障关联编号
	FaultCodes []string // 关联的故障编号
}

// ========== 姿态轨道控制检测指标 ==========
// AttitudeOrbitControlMetrics 姿态轨道控制指标（按阈值配置键值动态上报）
type AttitudeOrbitControlMetrics struct {
	Timestamp int64                  // 时间戳
	Values    map[string]interface{} // 阈值配置中的键到运行时值的映射
}

// ========== 推进系统检测指标 ==========
// ThrusterMetrics 推进器指标
type ThrusterMetrics struct {
	Timestamp int64 // 时间戳

	// 开关状态
	PipelineSwitch uint8 // 推进管路开关状态, 1=打开

	// 传感器数据
	PressureSensor uint16 // 压力传感器数据

	// 燃料
	FuelLevel uint16 // 燃料量

	// 故障关联编号
	FaultCodes []string // 关联的故障编号
}

// ========== 其他组件指标 ==========
// RunMgrMetrics 运行管理指标
type RunMgrMetrics struct {
	Timestamp   int64                  // 时间戳
	Temperature float64                // 温度
	Voltage     float64                // 电压
	StatusCode  int                    // 状态码
	Payload     map[string]interface{} // 其他动态字段
}

// EPSMetrics 电源指标
type EPSMetrics struct {
	Timestamp int64   // 时间戳
	Voltage   float64 // 电压
	Current   float64 // 电流
}

// SensorMetrics 敏感器指标
type SensorMetrics struct {
	Timestamp int64 // 时间戳
	AccX      int16 // X轴加速度
	AccY      int16 // Y轴加速度
	AccZ      int16 // Z轴加速度
}

// MeasureMetrics 测量指标
type MeasureMetrics struct {
	Timestamp   int64  // 时间戳
	SensorValue uint32 // 传感器值
}

// AttCtrlMetrics 姿态控制指标
type AttCtrlMetrics struct {
	Timestamp   int64 // 时间戳
	ControlMode uint8 // 控制模式
}

// OpticalMetrics 光电设备指标
type OpticalMetrics struct {
	Timestamp    int64  // 时间戳
	PhotoCurrent uint16 // 光电流
}

// PayloadMetrics 载荷指标
type PayloadMetrics struct {
	Timestamp int64 // 时间戳
	WorkMode  uint8 // 工作模式
}

// RailCtrlMetrics 轨道控制指标
type RailCtrlMetrics struct {
	Timestamp int64 // 时间戳
	OrbitMode uint8 // 轨道模式
}
