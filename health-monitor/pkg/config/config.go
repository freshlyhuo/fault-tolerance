/* 包括：

阈值配置（CPU、内存、业务报文 delay 等）

滑动窗口长度

去抖动时间

微服务查询周期

日志等级

ECSM API 地址

提供函数：

LoadConfig()
ReloadConfig()（热更新可选） */

//报文设计
/*Byte 0   : 组件类型（下面 14 类之一）
Byte 1-2 : 数据长度（uint16，大端）
Byte 3-n : 数据内容（不同组件不同格式）
Byte n+1 : CRC（可选）*/
const (
	CompRunMgr       = 0x01 // 运行管理
	CompComm         = 0x02 // 通信
	CompPower        = 0x03 // 供电
	CompRailCtrl     = 0x04 // 轨道控制
	CompPayload      = 0x05 // 载荷
	CompThermal      = 0x06 // 热控
	CompAttCtrl      = 0x07 // 姿态控制
	CompMeasure      = 0x08 // 测量
	CompOptical      = 0x09 // 光电设备
	CompSensor       = 0x0A // 敏感器
	CompActuator     = 0x0B // 姿态控制机构
	CompTransceiver  = 0x0C // 通信机
	CompThruster     = 0x0D // 推进器
	CompEPS          = 0x0E // 电源
)