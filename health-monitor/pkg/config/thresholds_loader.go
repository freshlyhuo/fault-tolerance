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
		PowerModule12V    MetricThreshold `json:"PowerModule12V"`
		BatteryVoltage    MetricThreshold `json:"BatteryVoltage"`
		BusVoltage        MetricThreshold `json:"BusVoltage"`
		CPUVoltage        MetricThreshold `json:"CPUVoltage"`
		ThermalRefVoltage MetricThreshold `json:"ThermalRefVoltage"`
		Bracket12VCurrent MetricThreshold `json:"Bracket12VCurrent"`
		LoadCurrent       MetricThreshold `json:"LoadCurrent"`
	} `json:"power"`

	Thermal struct {
		ThermalTemps         []IndexedMetricThreshold `json:"ThermalTemps"`
		BatteryTemp1         MetricThreshold          `json:"BatteryTemp1"`
		BatteryTemp2         MetricThreshold          `json:"BatteryTemp2"`
		PlatformThermalTemp  MetricThreshold          `json:"PlatformThermalTemp"`
		BatteryThermalTemp   MetricThreshold          `json:"BatteryThermalTemp"`
		TankThermalTemp      MetricThreshold          `json:"TankThermalTemp"`
		HeaterSwitchState    struct {
			PlatformHeater *bool `json:"PlatformHeater"`
			BatteryHeater  *bool `json:"BatteryHeater"`
			TankHeater     *bool `json:"TankHeater"`
		} `json:"HeaterSwitchState"`
	} `json:"thermal"`

	Comm struct {
		SNR                     IntThreshold  `json:"SNR"`
		ReceiveRSSI             IntThreshold  `json:"ReceiveRSSI"`
		CANStatus               IntThreshold  `json:"CANStatus"`
		SerialStatus            IntThreshold  `json:"SerialStatus"`
		AirToAirStatus          IntThreshold  `json:"AirToAirStatus"`
		ReceiveCmdCount         IntThreshold  `json:"ReceiveCmdCount"`
		ReceiveCTACount         IntThreshold  `json:"ReceiveCTACount"`
		SwitchState             IntThreshold  `json:"SwitchState"`
		TelemetryEncryptStatus  IntThreshold  `json:"TelemetryEncryptStatus"`
		TelecontrolEncryptStatus IntThreshold `json:"TelecontrolEncryptStatus"`
	} `json:"comm"`

	Actuator struct {
		WheelSpeed struct {
			X MetricThreshold `json:"X"`
			Y MetricThreshold `json:"Y"`
			Z MetricThreshold `json:"Z"`
		} `json:"WheelSpeed"`
		WheelSpeedTolerance *float64 `json:"wheel_speed_tolerance,omitempty"`
	} `json:"actuator"`

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
