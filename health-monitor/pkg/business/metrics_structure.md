# 业务层健康监测指标结构设计

## 设计原则

业务层健康监测指标按照不同组件设计独立的结构体，每个指标都明确列出在对应的组件结构体内，便于：
1. 类型安全 - 每个字段都有明确的数据类型
2. 清晰可读 - 指标名称和注释明确对应到 metrics.md 中的监测参数
3. 故障追踪 - 每个结构体包含故障关联编号字段
4. 易于扩展 - 新增指标只需在对应结构体中添加字段

## 核心结构

### BusinessMetrics (业务层指标基础结构)
```go
type BusinessMetrics struct {
    ComponentType uint8      // 组件类型编号
    Timestamp     int64      // 时间戳
    Data          interface{} // 具体组件的指标数据
}
```

`Data` 字段将包含下列具体组件指标结构体之一。

---

## 1. 供电服务检测指标 (PowerMetrics)

**对应 metrics.md**: 一、（一）供电服务检测指标

```go
type PowerMetrics struct {
    Timestamp           int64   // 时间戳
    
    // 电压类指标
    PowerModule12V      float64 // TMAN01046: 12V功率模块电压, 正常约13V
    BatteryVoltage      float64 // TMEZD01095: 蓄电池电压, 正常[21, 29.4]V
    BusVoltage          float64 // TMEZD01096: 母线电压, 正常[21, 29.4]V
    CPUVoltage          float64 // TMEZD01011: CPU板电压, 正常[3.1, 3.5]V
    ThermalRefVoltage   float64 // TMEZD01100: 热敏基准电压, 正常[4.5, 5.5]V
    
    // 电流类指标
    Bracket12VCurrent   float64 // TMAN01050: 通用连接机构12V供电电流, 正常约1.2A
    LoadCurrent         float64 // TMEZD01247: 负载电流, 正常[0.5, 5]A
    
    // 故障关联编号
    FaultCodes          []string // 关联的故障编号
}
```

**报文格式** (14字节):
```
偏移  长度  字段
0     2     PowerModule12V (mV)
2     2     BatteryVoltage (mV)
4     2     BusVoltage (mV)
6     2     CPUVoltage (mV)
8     2     ThermalRefVoltage (mV)
10    2     Bracket12VCurrent (mA)
12    2     LoadCurrent (mA)
```

**关联故障编号**:
- CJB-RG-ZD-1: 12V功率模块电压、12V供电电流
- CJB-RG-ZD-3: 蓄电池电压、母线电压、CPU板电压

---

## 2. 热控服务检测指标 (ThermalMetrics)

**对应 metrics.md**: 一、（二）热控服务检测指标

```go
type ThermalMetrics struct {
    Timestamp           int64   // 时间戳
    
    // 温度类指标
    ThermalTemps        [10]float64 // TMEZD01066-01075: cjb热控温度1~10
    BatteryTemp1        float64     // TMEZD01084: cjb蓄电池温度1
    BatteryTemp2        float64     // TMEZD01085: cjb蓄电池温度2
    PlatformThermalTemp float64     // 平台热控温度
    BatteryThermalTemp  float64     // 蓄电池热控温度
    TankThermalTemp     float64     // 储箱热控温度
    
    // 开关状态类指标
    PlatformHeaterSwitch bool       // TMEZD01121: 平台加热总开关状态, 1=打开
    BatteryHeaterSwitch  bool       // TMEZD01254: 蓄电池加热总开关状态, 1=打开
    TankHeaterSwitch     bool       // TMEZD01115: 储箱加热总开关状态, 1=打开
    
    // 故障关联编号
    FaultCodes          []string    // 关联的故障编号
}
```

**报文格式** (31字节):
```
偏移  长度  字段
0     20    ThermalTemps[10] (每个2字节, 0.1℃)
20    2     BatteryTemp1 (0.1℃)
22    2     BatteryTemp2 (0.1℃)
24    2     PlatformThermalTemp (0.1℃)
26    2     BatteryThermalTemp (0.1℃)
28    2     TankThermalTemp (0.1℃)
30    1     开关状态位: bit0=平台, bit1=蓄电池, bit2=储箱
```

**关联故障编号**:
- CJB-RG-ZD-4: cjb热控温度、蓄电池温度
- CJB-RG-ZD-5: 平台加热总开关
- CJB-RG-ZD-6: 蓄电池加热总开关
- CJB-RG-ZD-7: 平台热控温度
- CJB-RG-ZD-8: 蓄电池热控温度
- CJB-O2-ZD-1: 储箱加热总开关
- CJB-O2-ZD-2: 储箱热控温度

---

## 3. 通信服务检测指标 (CommMetrics)

**对应 metrics.md**: 一、（三）通信服务检测指标（部分）

```go
type CommMetrics struct {
    Timestamp           int64   // 时间戳
    
    // 通信状态类指标
    CANStatus           uint8   // CAN通信状态: 1=正常应答, 0=无应答
    SerialStatus        uint8   // 串口通信状态: 1=有正常遥测, 0=无遥测
    AirToAirStatus      uint8   // 空空通信状态: 1=正常收发, 0=异常
    
    // 通用通信参数
    SNR                 uint8   // 信噪比
    Rate                uint16  // 通信速率
    
    // 故障关联编号
    FaultCodes          []string // 关联的故障编号
}
```

**报文格式** (6字节):
```
偏移  长度  字段
0     1     SNR
1     2     Rate
3     1     CANStatus
4     1     SerialStatus
5     1     AirToAirStatus
```

**关联故障编号**:
- CJB-RG-ZD-2: CAN通信状态
- CJB-O2-CS-1~15: 串口通信状态
- CJB-O2-CS-4: 空空通信状态

---

## 4. 通信机检测指标 (TransceiverMetrics)

**对应 metrics.md**: 一、（三）通信服务检测指标（通信机部分）

```go
type TransceiverMetrics struct {
    Timestamp                 int64   // 时间戳
    
    // 状态类指标
    TelemetryEncryptStatus    uint8   // TMEZD01167: 遥测明/密状态, 1=密态
    TelecontrolEncryptStatus  uint8   // TMEZD01168: 遥控明/密状态, 1=密态
    TransmitSwitch            uint8   // TMEZD01155: 发射通道开关状态, 1=打开
    
    // 信号质量指标
    InfoChannelSNR            uint8   // TMEZD01145: 信息通道接收信噪比
    ReceiveRSSI               int8    // TMEZD01147: 接收RSSI
    
    // 功率
    TransmitPower             uint8   // 发射功率
    
    // 故障关联编号
    FaultCodes                []string // 关联的故障编号
}
```

**报文格式** (7字节):
```
偏移  长度  字段
0     1     TransmitPower
1     1     TelemetryEncryptStatus
2     1     TelecontrolEncryptStatus
3     1     TransmitSwitch
4     1     InfoChannelSNR
6     1     ReceiveRSSI (有符号)
```

**关联故障编号**:
- CJB-O2-CS-3: 发射通道开关
- CJB-O2-CS-4: 信号质量
- CJB-O2-CS-5: 遥测明/密状态
- CJB-O2-CS-6: 遥控明/密状态

**注**: 空空遥控计数已移至 CommMetrics

---

**注**: 串口通信相关指标已合并到 CommMetrics 结构体中

---

## 5. 姿态控制机构检测指标 (ActuatorMetrics)

**对应 metrics.md**: 一、（四）其他关键设备检测指标（动量轮）

```go
type ActuatorMetrics struct {
    Timestamp           int64   // 时间戳
    
    // 动量轮转速指标
    WheelSpeedX         int16   // TMEGNC2029: X轴动量轮转速(反馈), 正常约100转
    WheelSpeedY         int16   // TMEGNC2030: Y轴动量轮转速(反馈), 正常约100转
    WheelSpeedZ         int16   // TMEGNC2031: Z轴动量轮转速(反馈), 正常约100转
    
    // 故障关联编号
    FaultCodes          []string // 关联的故障编号
}
```

**报文格式** (6字节):
```
偏移  长度  字段
0     2     WheelSpeedX (有符号)
2     2     WheelSpeedY (有符号)
4     2     WheelSpeedZ (有符号)
```

**关联故障编号**:
- CJB-O2-CS-16: 动量轮转速

---

## 7. 推进系统检测指标 (ThrusterMetrics)

**对应 metrics.md**: 一、（四）其他关键设备检测指标（推进管路、压力传感器）

```go
type ThrusterMetrics struct {
    Timestamp           int64   // 时间戳
    
    // 开关状态
    PipelineSwitch      uint8   // 推进管路开关状态, 1=打开
    
    // 传感器数据
    PressureSensor      uint16  // 压力传感器数据
    
    // 燃料
    FuelLevel           uint16  // 燃料量
    
    // 故障关联编号
    FaultCodes          []string // 关联的故障编号
}
```

**报文格式** (5字节):
```
偏移  长度  字段
0     2     FuelLevel
2     1     PipelineSwitch
3     2     PressureSensor
```

**关联故障编号**:
- CJB-O2-CS-16: 压力传感器
- CJB-O2-CS-17: 推进管路开关

---

**注**: 计数器相关指标已合并到 CommMetrics 结构体中

---

## 其他组件指标

### RunMgrMetrics (运行管理)
```go
type RunMgrMetrics struct {
    Timestamp   int64
    Temperature float64
    Voltage     float64
    StatusCode  int
    Payload     map[string]interface{} // 其他动态字段
}
```

### EPSMetrics (电源)
```go
type EPSMetrics struct {
    Timestamp int64
    Voltage   float64
    Current   float64
}
```

**注**: 开关状态指标已包含在各自的组件中(如热控服务的加热开关、推进系统的管路开关等)

### SensorMetrics (敏感器)
```go
type SensorMetrics struct {
    Timestamp int64
    AccX      int16 // X轴加速度
    AccY      int16 // Y轴加速度
    AccZ      int16 // Z轴加速度
}
```

### MeasureMetrics (测量)
```go
type MeasureMetrics struct {
    Timestamp   int64
    SensorValue uint32
}
```

### AttCtrlMetrics (姿态控制)
```go
type AttCtrlMetrics struct {
    Timestamp   int64
    ControlMode uint8
}
```

### OpticalMetrics (光电设备)
```go
type OpticalMetrics struct {
    Timestamp    int64
    PhotoCurrent uint16
}
```

### PayloadMetrics (载荷)
```go
type PayloadMetrics struct {
    Timestamp int64
    WorkMode  uint8
}
```

### RailCtrlMetrics (轨道控制)
```go
type RailCtrlMetrics struct {
    Timestamp int64
    OrbitMode uint8
}
```

---

## 使用示例

### 1. 创建和发送供电服务指标报文

```go
// 构造PowerMetrics
powerMetrics := &model.PowerMetrics{
    Timestamp:         time.Now().Unix(),
    PowerModule12V:    13.0,
    BatteryVoltage:    25.0,
    BusVoltage:        24.5,
    CPUVoltage:        3.3,
    ThermalRefVoltage: 5.0,
    Bracket12VCurrent: 1.2,
    LoadCurrent:       2.0,
    FaultCodes:        []string{},
}

// 构造业务层指标
businessMetrics := &model.BusinessMetrics{
    ComponentType: business.CompPower,
    Timestamp:     time.Now().Unix(),
    Data:          powerMetrics,
}
```

### 2. 解析报文并获取具体指标

```go
// 解析报文
metrics, err := receiver.ParsePacket(packet)
if err != nil {
    log.Fatal(err)
}

// 类型断言获取具体组件指标
switch metrics.ComponentType {
case business.CompPower:
    powerData := metrics.Data.(*model.PowerMetrics)
    fmt.Printf("Battery Voltage: %.2fV\n", powerData.BatteryVoltage)
    fmt.Printf("CPU Voltage: %.2fV\n", powerData.CPUVoltage)
    
case business.CompThermal:
    thermalData := metrics.Data.(*model.ThermalMetrics)
    fmt.Printf("Thermal Temps: %v\n", thermalData.ThermalTemps)
    fmt.Printf("Platform Heater: %v\n", thermalData.PlatformHeaterSwitch)
    
case business.CompActuator:
    actuatorData := metrics.Data.(*model.ActuatorMetrics)
    fmt.Printf("Wheel Speed X: %d\n", actuatorData.WheelSpeedX)
}
```

### 3. 阈值检查

```go
func checkPowerThresholds(metrics *model.PowerMetrics) []string {
    var faults []string
    
    // 检查12V功率模块电压 (正常约13V)
    if metrics.PowerModule12V < 12.5 || metrics.PowerModule12V > 13.5 {
        faults = append(faults, "CJB-RG-ZD-1")
    }
    
    // 检查蓄电池电压 (正常[21, 29.4]V)
    if metrics.BatteryVoltage < 21.0 || metrics.BatteryVoltage > 29.4 {
        faults = append(faults, "CJB-RG-ZD-3")
    }
    
    // 检查CPU板电压 (正常[3.1, 3.5]V)
    if metrics.CPUVoltage < 3.1 || metrics.CPUVoltage > 3.5 {
        faults = append(faults, "CJB-RG-ZD-3")
    }
    
    // 检查热敏基准电压 (正常[4.5, 5.5]V)
    if metrics.ThermalRefVoltage < 4.5 || metrics.ThermalRefVoltage > 5.5 {
        faults = append(faults, "CJB-RG-ZD-4")
    }
    
    return faults
}
```

---

## 优势总结

1. **类型安全**: 每个字段都有明确的类型，编译时可检查
2. **文档清晰**: 每个字段都有注释说明其对应的监测参数编号和正常阈值
3. **易于维护**: 修改某个组件的指标只需修改对应的结构体
4. **故障追踪**: 每个指标结构体都包含 FaultCodes 字段用于关联故障
5. **扩展性强**: 新增组件只需定义新的结构体并在 ParsePacket 中添加分支
6. **代码可读**: 使用结构体字段比 map[string]interface{} 更清晰易读
