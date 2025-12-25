# 业务层报文格式说明

## 报文通用格式

```
+--------+--------+--------+---------------+
| 类型   | 长度(2B)        | 数据负载       |
| (1B)   | (Big Endian)   | (N bytes)     |
+--------+--------+--------+---------------+
```

- **类型 (1 byte)**: 组件编号，标识报文类型
- **长度 (2 bytes)**: 数据负载的字节数 (Big Endian)
- **数据负载 (N bytes)**: 具体指标数据

## 组件类型编号

| 编号 | 名称 | 说明 |
|------|------|------|
| 0x01 | CompRunMgr | 运行管理 |
| 0x02 | CompComm | 通信服务 (CAN、串口等) |
| 0x03 | CompPower | 供电服务 |
| 0x04 | CompRailCtrl | 轨道控制 |
| 0x05 | CompPayload | 载荷 |
| 0x06 | CompThermal | 热控服务 |
| 0x07 | CompAttCtrl | 姿态控制 |
| 0x08 | CompMeasure | 测量 |
| 0x09 | CompOptical | 光电设备 |
| 0x0A | CompSensor | 敏感器 |
| 0x0B | CompActuator | 姿态控制机构 (动量轮) |
| 0x0C | CompTransceiver | 通信机 |
| 0x0D | CompThruster | 推进器 |
| 0x0E | CompEPS | 电源 |

**注**: 串口通信、开关状态、计数器等指标已归属到各自所在的组件中

## 各组件报文格式详细说明

### 1. 供电服务 (0x03 - CompPower)

基于 metrics.md 中的供电服务检测指标实现。

```
偏移  长度  字段名                     说明
0     2     voltage                   主电压 (mV)
2     2     current                   主电流 (mA)
4     2     power_module_12v          12V功率模块电压 (mV) - TMAN01046, 正常约13V
6     2     battery_voltage           蓄电池电压 (mV) - TMEZD01095, 正常[21,29.4]V
8     2     bus_voltage               母线电压 (mV) - TMEZD01096, 正常[21,29.4]V
10    2     cpu_voltage               CPU板电压 (mV) - TMEZD01011, 正常[3.1,3.5]V
12    2     thermal_ref_voltage       热敏基准电压 (mV) - TMEZD01100, 正常[4.5,5.5]V
14    2     bracket_12v_current       通用连接机构12V电流 (mA) - TMAN01050, 正常约1.2A
16    2     load_current              负载电流 (mA) - TMEZD01247, 正常[0.5,5]A
```

### 2. 热控服务 (0x06 - CompThermal)

基于 metrics.md 中的热控服务检测指标实现。

```
偏移  长度  字段名                     说明
0     2     temperature               主温度 (0.1℃)
2     20    thermal_temps[10]         cjb热控温度1-10 (0.1℃) - TMEZD01066-01075
22    2     battery_temp_1            蓄电池温度1 (0.1℃) - TMEZD01084
24    2     battery_temp_2            蓄电池温度2 (0.1℃) - TMEZD01085
26    2     platform_thermal_temp     平台热控温度 (0.1℃)
28    2     tank_thermal_temp         储箱热控温度 (0.1℃)
30    1     switch_state              开关状态位标志:
                                      bit0: 平台加热总开关 - TMEZD01121
                                      bit1: 蓄电池加热总开关 - TMEZD01254
                                      bit2: 储箱加热总开关 - TMEZD01115
```

### 3. 通信服务 (0x02 - CompComm)

基于 metrics.md 中的通信服务检测指标实现。

```
偏移  长度  字段名                     说明
0     1     SNR                       信噪比
1     2     rate                      通信速率
3     1     can_status                CAN通信状态 (1=正常应答, 0=无应答)
4     1     serial_status             串口通信状态 (1=有正常遥测, 0=无遥测)
5     1     air_to_air_status         空空通信状态 (1=正常, 0=异常)
```

### 4. 通信机 (0x0C - CompTransceiver)

基于 metrics.md 中的通信机相关指标实现。

```
偏移  长度  字段名                        说明
0     1     power                        发射功率
1     1     telemetry_encrypt_status     遥测明/密状态 - TMEZD01167 (1=密态)
2     1     telecontrol_encrypt_status   遥控明/密状态 - TMEZD01168 (1=密态)
3     1     transmit_switch              发射通道开关 - TMEZD01155 (1=打开)
4     1     info_channel_snr             信息通道接收信噪比 - TMEZD01145
6     1     receive_rssi                 接收RSSI - TMEZD01147 (有符号)
7     2     air_to_air_control_count     空空遥控计数 - TMEZD01150
```

### 5. 姿态控制机构 (0x0B - CompActuator)

基于 metrics.md 中的动量轮转速指标实现。

```
偏移  长度  字段名                     说明
0     2     wheelSpeed                主轮转速 (有符号)
2     2     wheel_speed_x             X轴动量轮转速 - TMEGNC2029 (正常约100转)
4     2     wheel_speed_y             Y轴动量轮转速 - TMEGNC2030
6     2     wheel_speed_z             Z轴动量轮转速 - TMEGNC2031
```

### 6. 推进器 (0x0D - CompThruster)

基于 metrics.md 中的推进管路相关指标实现。

```
偏移  长度  字段名                     说明
0     2     fuel                      燃料量
2     1     pipeline_switch           推进管路开关状态 (1=打开)
3     2     pressure_sensor           压力传感器数据
```

**注**: 
- 串口通信相关指标已合并到通信服务 (0x02 - CompComm) 中
- 开关状态已分别包含在各自的组件中 (如热控服务的加热开关、推进器的管路开关等)
- 计数器相关指标已合并到通信服务 (0x02 - CompComm) 中

## 故障关联编号映射

根据 metrics.md 中的故障编号关联，各指标与故障的对应关系：

### 供电服务相关
- **CJB-RG-ZD-1**: 12V功率模块电压、12V供电电流
- **CJB-RG-ZD-3**: 蓄电池电压、母线电压、CPU板电压

### 热控服务相关
- **CJB-RG-ZD-4**: cjb热控温度、蓄电池温度、CPU板电压、热敏基准电压
- **CJB-RG-ZD-5**: 平台加热总开关、CPU板电压、接收命令计数、OC模块
- **CJB-RG-ZD-6**: 蓄电池加热总开关、接收命令计数、OC模块
- **CJB-RG-ZD-7**: 平台热控温度、热敏基准电压、接收命令计数
- **CJB-RG-ZD-8**: 蓄电池热控温度、接收命令计数
- **CJB-O2-ZD-1**: 储箱加热总开关、热敏基准电压、接收命令计数、OC模块
- **CJB-O2-ZD-2**: 储箱热控温度、热敏基准电压、接收命令计数

### 通信服务相关
- **CJB-RG-ZD-2**: CAN通信状态
- **CJB-O2-CS-1**: 串口通信状态、负载电流
- **CJB-O2-CS-2**: 串口错误计数
- **CJB-O2-CS-3**: 通信机发射通道开关、接收命令计数、空空遥控计数
- **CJB-O2-CS-4**: 空空通信状态、信息通道接收信噪比、接收RSSI
- **CJB-O2-CS-5**: 遥测明/密状态、接收命令计数、空空遥控计数
- **CJB-O2-CS-6**: 遥控明/密状态、接收命令计数、空空遥控计数
- **CJB-O2-CS-7~15**: 串口通信状态、串口错误计数、负载电流
- **CJB-O2-CS-16**: 动量轮转速、压力传感器、接收命令计数
- **CJB-O2-CS-17**: 推进管路开关、串口错误计数

## 使用示例

### 发送供电服务报文

```go
// 构造一个供电服务报文
packet := make([]byte, 3 + 18) // 类型(1) + 长度(2) + 数据(18)
packet[0] = 0x03 // CompPower
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

// 提交报文
receiver.Submit(packet)
```

### 发送热控服务报文

```go
// 构造一个热控服务报文
packet := make([]byte, 3 + 31) // 类型(1) + 长度(2) + 数据(31)
packet[0] = 0x06 // CompThermal
binary.BigEndian.PutUint16(packet[1:3], 31) // 数据长度

// 填充温度数据
binary.BigEndian.PutUint16(packet[3:5], 250)  // temperature = 25.0℃

// 10个热控温度点
for i := 0; i < 10; i++ {
    binary.BigEndian.PutUint16(packet[5+i*2:7+i*2], 230) // 23.0℃
}

// 蓄电池温度
binary.BigEndian.PutUint16(packet[25:27], 280)  // battery_temp_1 = 28.0℃
binary.BigEndian.PutUint16(packet[27:29], 275)  // battery_temp_2 = 27.5℃

// 其他温度
binary.BigEndian.PutUint16(packet[29:31], 240)  // platform_thermal_temp = 24.0℃
binary.BigEndian.PutUint16(packet[31:33], 220)  // tank_thermal_temp = 22.0℃

// 开关状态: 全部打开
packet[33] = 0x07  // bit0=1, bit1=1, bit2=1

receiver.Submit(packet)
```

## 注意事项

1. **字节序**: 所有多字节数值采用 Big Endian (网络字节序)
2. **单位转换**: 
   - 电压以 mV 为单位传输，解析后除以1000得到V
   - 电流以 mA 为单位传输，解析后除以1000得到A
   - 温度以 0.1℃ 为单位传输，解析后除以10得到℃
3. **扩展性**: 各报文格式支持可选字段，解析时需检查payload长度
4. **容错性**: 解析函数对长度不足的报文会提前返回，不会崩溃
5. **阈值判断**: 解析后的数据需要传递给 `alert/threshold` 模块进行阈值判断
