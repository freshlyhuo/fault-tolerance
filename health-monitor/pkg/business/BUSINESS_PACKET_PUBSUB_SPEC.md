# 业务层报文规范 + Pub/Sub 推送接口说明（Health Monitor）

> 本文档基于当前 health-monitor 业务层解析实现整理。
> 适用范围：服务器端/数据源端通过 Pub/Sub 推送业务层二进制报文到监测端（health-monitor）。

## 1. 背景与目标

health-monitor 的业务层处理链路为：

1) 接收业务层报文（原始二进制）
2) 解析为 `BusinessMetrics`
3) 交由 `Dispatcher` 分发：写入 `StateManager` + 触发阈值告警

为支持跨进程/跨节点通信，建议由服务器端通过 Pub/Sub 向监测端推送“原始二进制报文”（监测端保持对字段语义的唯一解释源）。

## 2. Pub/Sub 推送接口（建议）

### 2.1 Topic

- Topic 名称（建议）：`healthmonitor.business.packet.v1`

> 说明：
>
> - `v1` 表示消息体字段/语义版本（与二进制报文格式版本可同步，也可独立演进）。

### 2.2 消息体（建议）

建议在 Pub/Sub 消息外层携带最小元数据，便于去重、排障、跨网络传输诊断。

- `packet`：bytes（必填）
  - 即“业务层二进制报文”，格式见第 3 节。
- `sequence`：uint64（建议）
  - 单调递增序号，用于去重/乱序处理。
- `source`：string（建议）
  - 数据源标识（如节点名、设备号）。
- `server_timestamp`：int64（建议，Unix 秒或毫秒二选一，需明确）
  - 服务器产生/采集该报文的时间戳。

兼容性策略：
- 新增字段优先“向后兼容”（旧订阅者忽略未知字段）。
- `packet` 字段必须存在且符合基础帧格式，否则监测端应丢弃该消息并记录错误。

## 3. 业务层二进制报文：基础帧格式（通用）

所有业务层报文采用相同基础帧：

| 字段 | 偏移 | 长度 | 类型 | 端序 | 说明 |
|---|---:|---:|---|---|---|
| ComponentType | 0 | 1 | uint8 | - | 组件编号/报文类型 |
| PayloadLength | 1 | 2 | uint16 | Big Endian | 负载长度 N |
| Payload | 3 | N | bytes | - | 负载数据 |

校验规则：
- 若 `PayloadLength > len(packet) - 3`：判定为非法报文（长度不匹配），直接丢弃。
- 若 `ComponentType` 未定义：判定为非法报文（unknown type），直接丢弃。

## 4. ComponentType（组件编号）定义

| 组件 | 值(hex) | 说明 |
|---|---:|---|
| RunMgr | 0x01 | 运行管理 |
| Comm | 0x02 | 通信服务（CAN/串口/空空通信等） |
| Power | 0x03 | 供电服务 |
| RailCtrl | 0x04 | 轨道控制 |
| Payload | 0x05 | 载荷 |
| Thermal | 0x06 | 热控服务 |
| AttCtrl | 0x07 | 姿态控制 |
| Measure | 0x08 | 测量 |
| Optical | 0x09 | 光电设备 |
| Sensor | 0x0A | 敏感器 |
| Actuator | 0x0B | 姿态控制机构（动量轮） |
| Transceiver | 0x0C | 通信机 |
| Thruster | 0x0D | 推进器 |
| EPS | 0x0E | 电源 |

## 5. Payload 格式定义（逐组件）

说明：
- 表格中的“偏移”相对于 Payload 起始位置（Payload[0]）。
- 多字节整数均使用 Big Endian。
- `int16/int8` 表示有符号整型（补码）。

### 5.1 RunMgr（0x01）

- PayloadLength：5

| 字段 | 偏移 | 长度 | 类型 | 缩放/单位 | 说明 |
|---|---:|---:|---|---|---|
| Temperature | 0 | 2 | uint16 | 值/10 ℃ | 温度 |
| Voltage | 2 | 2 | uint16 | 值/1000 V | 电压 |
| StatusCode | 4 | 1 | uint8 | - | 状态码 |

### 5.2 Comm（0x02）

- PayloadLength：18（强烈建议固定为 18）

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| SNR | 0 | 1 | uint8 | 信噪比 |
| Rate | 1 | 2 | uint16 | 速率 |
| CANStatus | 3 | 1 | uint8 | CAN 状态 |
| SerialStatus | 4 | 1 | uint8 | 串口状态 |
| AirToAirStatus | 5 | 1 | uint8 | 空空通信状态 |
| ParityErrorCount | 6 | 2 | uint16 | 奇偶校验错误计数 |
| FrameHeaderErrorCount | 8 | 2 | uint16 | 帧头错误计数 |
| FrameLengthErrorCount | 10 | 2 | uint16 | 帧长错误计数 |
| SerialResetCount | 12 | 2 | uint16 | 串口复位计数 |
| ReceiveCmdCount | 14 | 4 | uint32 | 命令接收计数 |

实现备注（与当前解析逻辑对齐）：
- 解析会读取到 offset 17（即 18 字节）。服务器端必须发送足够长度，避免监测端越界或解析失败。

### 5.3 Power（0x03）

- PayloadLength：14

| 字段 | 偏移 | 长度 | 类型 | 单位 | 说明 |
|---|---:|---:|---|---|---|
| PowerModule12V | 0 | 2 | uint16 | mv | 12V 模块电压 |
| BatteryVoltage | 2 | 2 | uint16 | mv   | 电池电压 |
| BusVoltage | 4 | 2 | uint16 | mv   | 总线电压 |
| CPUVoltage | 6 | 2 | uint16 | mv   | CPU 电压 |
| ThermalRefVoltage | 8 | 2 | uint16 | mv   | 热控基准电压 |
| Bracket12VCurrent | 10 | 2 | uint16 | mA | 支架 12V 电流 |
| LoadCurrent | 12 | 2 | uint16 | mA | 负载电流 |

### 5.4 RailCtrl（0x04）

- PayloadLength：1

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| OrbitMode | 0 | 1 | uint8 | 轨控模式 |

### 5.5 Payload（0x05）

- PayloadLength：1

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| WorkMode | 0 | 1 | uint8 | 载荷工作模式 |

### 5.6 Thermal（0x06）

- PayloadLength：31

| 字段 | 偏移 | 长度 | 类型 | 缩放/单位 | 说明 |
|---|---:|---:|---|---|---|
| ThermalTemps[0..9] | 0 | 20 | 10×int16 | 0.1 ℃ | 10 个温度点（每点 2 字节） |
| BatteryTemp1 | 20 | 2 | int16 | 0.1 ℃ | 蓄电池温度 1 |
| BatteryTemp2 | 22 | 2 | int16 | 0.1 ℃ | 蓄电池温度 2 |
| PlatformThermalTemp | 24 | 2 | int16 | 0.1 ℃ | 平台热控温度 |
| BatteryThermalTemp | 26 | 2 | int16 | 0.1 ℃ | 电池热控温度 |
| TankThermalTemp | 28 | 2 | int16 | 0.1 ℃ | 罐体/储罐热控温度 |
| SwitchState | 30 | 1 | uint8 | 位标志 | 加热器开关状态 |

SwitchState 位定义：
- bit0 (0x01)：PlatformHeaterSwitch（平台加热器）
- bit1 (0x02)：BatteryHeaterSwitch（电池加热器）
- bit2 (0x04)：TankHeaterSwitch（罐体加热器）
- 其余位保留

### 5.7 AttCtrl（0x07）

- PayloadLength：1

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| ControlMode | 0 | 1 | uint8 | 姿态控制模式 |

### 5.8 Measure（0x08）

- PayloadLength：4

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| SensorValue | 0 | 4 | uint32 | 测量值 |

### 5.9 Optical（0x09）

- PayloadLength：2

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| PhotoCurrent | 0 | 2 | uint16 | 光电流 |

### 5.10 Sensor（0x0A）

- PayloadLength：6

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| AccX | 0 | 2 | int16 | 加速度 X |
| AccY | 2 | 2 | int16 | 加速度 Y |
| AccZ | 4 | 2 | int16 | 加速度 Z |

### 5.11 Actuator（0x0B）

- PayloadLength：6

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| WheelSpeedX | 0 | 2 | int16 | 动量轮 X 轴转速 |
| WheelSpeedY | 2 | 2 | int16 | 动量轮 Y 轴转速 |
| WheelSpeedZ | 4 | 2 | int16 | 动量轮 Z 轴转速 |

### 5.12 Transceiver（0x0C）

- PayloadLength：7

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| TransmitPower | 0 | 1 | uint8 | 发射功率 |
| TelemetryEncryptStatus | 1 | 1 | uint8 | 遥测加密状态 |
| TelecontrolEncryptStatus | 2 | 1 | uint8 | 遥控加密状态 |
| TransmitSwitch | 3 | 1 | uint8 | 发射开关 |
| InfoChannelSNR | 4 | 1 | uint8 | 信息通道信噪比 |
| Reserved | 5 | 1 | uint8 | 保留（当前未使用） |
| ReceiveRSSI | 6 | 1 | int8 | 接收 RSSI |

### 5.13 Thruster（0x0D）

- PayloadLength：5

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|---|---:|---:|---|---|
| FuelLevel | 0 | 2 | uint16 | 燃料余量/液位 |
| PipelineSwitch | 2 | 1 | uint8 | 管路开关 |
| PressureSensor | 3 | 2 | uint16 | 压力传感器 |

### 5.14 EPS（0x0E）

- PayloadLength：4

| 字段 | 偏移 | 长度 | 类型 | 缩放/单位 | 说明 |
|---|---:|---:|---|---|---|
| Voltage | 0 | 2 | uint16 | 值/1000 V | 电压 |
| Current | 2 | 2 | uint16 | 值/1000 A | 电流 |

## 6. 发送端实现要求（服务器端）

- 严格使用 Big Endian 编码多字节整数。
- `PayloadLength` 必须等于实际 Payload 字节数。
- 对每个 ComponentType 建议固定 `PayloadLength`，与第 5 节一致。
- 若需要扩展：优先在 payload 中新增“保留字段/尾部字段”，并通过 topic 版本或 ComponentType 子版本控制兼容。

## 7. 示例（结构示意）

以 RunMgr（0x01）为例：

```
+-------------+----------------+-------------------------------+
| 0x01        | 0x00 0x05      | Temp[2] Volt[2] Status[1]     |
| (type)      | (len=5)        | payload                       |
+-------------+----------------+-------------------------------+
```

## 8. 监测端处理行为（概述）

监测端收到 Pub/Sub 消息后：
- 解析 `packet` → `BusinessMetrics`
- 写入 `StateManager`
- 执行阈值判断生成告警（业务层目前以阈值告警为主）

> 如需在 Pub/Sub 层面实现 ACK/重试/堆积策略，请结合你们使用的 VSOA 中间件能力补充（不同实现差异较大）。
