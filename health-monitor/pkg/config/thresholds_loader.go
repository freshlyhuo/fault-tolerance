package config

import (
	"encoding/json"
	"os"
	"sync"
)

type MetricThreshold struct {
	Value *float64 `json:"value"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Unit  string   `json:"unit,omitempty"`
}

type IntThreshold struct {
	Value *int   `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

type BoolThreshold struct {
	Value *bool  `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

type IndexedMetricThreshold struct {
	Index int      `json:"index"`
	Value *float64 `json:"value"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Unit  string   `json:"unit,omitempty"`
}

// ThresholdConfig 告警阈值配置
// 支持从 JSON 覆盖默认值。
type ThresholdConfig struct {
	Power struct {
		TMAN0104612VVoltage       MetricThreshold `json:"TMAN0104612V_Voltage"`
		TMAN01050Current          MetricThreshold `json:"TMAN01050_Current"`
		TMEZD01095CjBBatteryVolt MetricThreshold `json:"TMEZD01095cjb_BatteryVoltage"`
		TMEZD01096CjBBusVolt     MetricThreshold `json:"TMEZD01096cjb_BusVoltage"`
		TMEZD01011CjBCPUVolt     MetricThreshold `json:"TMEZD01011cjb_CPUVoltage"`
		TMEZD01100CjBThermalRef  MetricThreshold `json:"TMEZD01100cjb_ThermalRefVoltage"`
		TMEZD01247LoadCurrent    MetricThreshold `json:"TMEZD01247_LoadCurrent"`
	} `json:"power"`

	Thermal struct {
		ThermalTemps         []IndexedMetricThreshold `json:"TMEZD01066-TMEZD01075_ThermalTemps"`
		BatteryTemp1         MetricThreshold          `json:"TMEZD01084_BatteryTemp1"`
		BatteryTemp2         MetricThreshold          `json:"TMEZD01085_BatteryTemp2"`
		PlatformThermalTemp  MetricThreshold          `json:"PlatformThermalTemp"`
		BatteryThermalTemp   MetricThreshold          `json:"BatteryThermalTemp"`
		TankThermalTemp      MetricThreshold          `json:"TankThermalTemp"`
		SendCmdPlatformHeat  BoolThreshold            `json:"SendCmd_PlatformHeatingSwitchON"`
		SendCmdBatteryHeat   BoolThreshold            `json:"SendCmd_BatteryHeatingSwitchON"`
		SendCmdTankHeat      BoolThreshold            `json:"SendCmd_TankHeatingSwitchON"`
		HeaterSwitchState    struct {
			PlatformHeater *bool `json:"TMEZD01121_PlatformHeater"`
			BatteryHeater  *bool `json:"TMEZD01254_BatteryHeater"`
			TankHeater     *bool `json:"TMEZD01115_TankHeater"`
		} `json:"HeaterSwitchState"`
	} `json:"thermal"`

	Comm struct {
		ComSerialPort              IntThreshold `json:"ComSerialPort"`
		InstructionCount           IntThreshold `json:"TMEZD01004cjb_InstructionCount"`
		SendInstruction            IntThreshold `json:"SendInstruction"`
		CheckErrorCount            IntThreshold `json:"TMEZD01046_CheckErrorCount"`
		FrameHeaderErrorCount      IntThreshold `json:"TMEZD01047_FrameHeaderErrorCount"`
		FrameLengthErrorCount      IntThreshold `json:"TMEZD01048_FrameLengthErrorCount"`
		ResetCount                 IntThreshold `json:"TMEZD01052_ResetCount"`
		SendCmdK52519              IntThreshold `json:"SendCmd_K52519"`
		SwitchState                IntThreshold `json:"TMEZD01155_SwitchState"`
		ReceiveCTACount            IntThreshold `json:"TMEZD01150_ReceiveCTACount"`
		SNR                        IntThreshold `json:"TMEZD01145_SNR"`
		ReceiveRSSI                IntThreshold `json:"TMEZD01147_ReceiveRSSI"`
		SendCmdK50502              IntThreshold `json:"SendCmd_K50502"`
		TelemetryEncryptStatus     IntThreshold `json:"TMEZD01167_TelemetryEncryptStatus"`
		SendCmdK50504              IntThreshold `json:"SendCmd_K50504"`
		TelecontrolEncryptStatus   IntThreshold `json:"TMEZD01168_TelemetryEncryptStatus"`
	} `json:"comm"`

	AttitudeOrbitControl struct {
		GNSSSerialPort            IntThreshold `json:"GNSSSerialPort"`
		TMEZD01041CheckErrorCount IntThreshold `json:"TMEZD01041_CheckErrorCount"`
		TMEZD01042FrameHeaderErr  IntThreshold `json:"TMEZD01042_FrameHeaderErrorCount"`
		TMEZD01043FrameLengthErr  IntThreshold `json:"TMEZD01043_FrameLengthErrorCount"`
		TMEZD01054ResetCount      IntThreshold `json:"TMEZD01054_ResetCount"`
		GyroscopeSerialPort       IntThreshold `json:"GyroscopeSerialPort"`
		TMEZD01021CheckErrorCount IntThreshold `json:"TMEZD01021_CheckErrorCount"`
		TMEZD01022FrameHeaderErr  IntThreshold `json:"TMEZD01022_FrameHeaderErrorCount"`
		TMEZD01023FrameLengthErr  IntThreshold `json:"TMEZD01023_FrameLengthErrorCount"`
		TMEZD01024ResetCount      IntThreshold `json:"TMEZD01024_ResetCount"`
		MEMSerialPort             IntThreshold `json:"MEMSerialPort"`
		TMEZD01025CheckErrorCount IntThreshold `json:"TMEZD01025_CheckErrorCount"`
		TMEZD01026FrameHeaderErr  IntThreshold `json:"TMEZD01026_FrameHeaderErrorCount"`
		TMEZD01027FrameLengthErr  IntThreshold `json:"TMEZD01027_FrameLengthErrorCount"`
		TMEZD01028ResetCount      IntThreshold `json:"TMEZD01028_ResetCount"`
		TMEZD01033CheckErrorCount IntThreshold `json:"TMEZD01033_CheckErrorCount"`
		TMEZD01034FrameHeaderErr  IntThreshold `json:"TMEZD01034_FrameHeaderErrorCount"`
		TMEZD01035FrameLengthErr  IntThreshold `json:"TMEZD01035_FrameLengthErrorCount"`
		TMEZD01036ResetCount      IntThreshold `json:"TMEZD01036_ResetCount"`
		TMEZD01037CheckErrorCount IntThreshold `json:"TMEZD01037_CheckErrorCount"`
		TMEZD01038FrameHeaderErr  IntThreshold `json:"TMEZD01038_FrameHeaderErrorCount"`
		TMEZD01039FrameLengthErr  IntThreshold `json:"TMEZD01039_FrameLengthErrorCount"`
		TMEZD01040ResetCount      IntThreshold `json:"TMEZD01040_ResetCount"`
	} `json:"AttitudeOrbitControl"`

	MomentumWheel struct {
		MomentumWheelSerialPort   IntThreshold `json:"MomentumWheelSerialPort"`
		SendCmdK53029             IntThreshold `json:"SendCmd_K53029"`
		WheelSpeed struct {
			X MetricThreshold `json:"X"`
			Y MetricThreshold `json:"Y"`
			Z MetricThreshold `json:"Z"`
		} `json:"WheelSpeed"`
		WheelSpeedTolerance       *float64     `json:"wheel_speed_tolerance,omitempty"`
		TMEZD01013CheckErrorCount IntThreshold `json:"TMEZD01013_CheckErrorCount"`
		TMEZD01014FrameHeaderErr  IntThreshold `json:"TMEZD01014_FrameHeaderErrorCount"`
		TMEZD01015FrameLengthErr  IntThreshold `json:"TMEZD01015_FrameLengthErrorCount"`
		TMEZD01016ResetCount      IntThreshold `json:"TMEZD01016_ResetCount"`
	} `json:"MomentumWheel"`

	Node struct {
		CPUUsageMax    float64 `json:"cpu_usage_max"`
		MemoryUsageMax float64 `json:"memory_usage_max"`
		DiskUsageMax   float64 `json:"disk_usage_max"`
	} `json:"node"`

	Container struct {
		CPUUsageMax    float64 `json:"cpu_usage_max"`
		MemoryUsageMax float64 `json:"memory_usage_max"`
		DiskUsageMax   float64 `json:"disk_usage_max"`
	} `json:"container"`

	Service struct {
		RequireHealthy    bool `json:"require_healthy"`
		InstanceOnlineMin int  `json:"instance_online_min"`
	} `json:"service"`
}

var (
	thresholdOnce sync.Once
	thresholdCfg  *ThresholdConfig
)

func defaultThresholdConfig() *ThresholdConfig {
	cfg := &ThresholdConfig{}
	cfg.Node.CPUUsageMax = 85.0
	cfg.Node.MemoryUsageMax = 90.0
	cfg.Node.DiskUsageMax = 90.0

	cfg.Container.CPUUsageMax = 65.0
	cfg.Container.MemoryUsageMax = 80.0
	cfg.Container.DiskUsageMax = 65.0

	cfg.Service.RequireHealthy = true
	cfg.Service.InstanceOnlineMin = 1
	return cfg
}

func candidateThresholdPaths() []string {
	if p := os.Getenv("HM_THRESHOLD_CONFIG"); p != "" {
		return []string{p}
	}
	return []string{
		"./health-monitor/pkg/config/thresholds.json",
		"./pkg/config/thresholds.json",
		"./thresholds.json",
	}
}

// GetThresholdConfig 获取阈值配置（进程内懒加载）
// 未找到配置文件或解析失败时自动回退到默认值。
func GetThresholdConfig() *ThresholdConfig {
	thresholdOnce.Do(func() {
		cfg := defaultThresholdConfig()
		for _, p := range candidateThresholdPaths() {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			_ = json.Unmarshal(b, cfg)
			break
		}
		thresholdCfg = cfg
	})
	return thresholdCfg
}
