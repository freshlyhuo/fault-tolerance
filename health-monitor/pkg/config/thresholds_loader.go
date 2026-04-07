package config

import (
	"errors"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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
	thresholdMu   sync.RWMutex
	thresholdCfg  *ThresholdConfig
	thresholdMeta ThresholdConfigStatus
	thresholdPath string
)

var (
	ErrChecksumMismatch = errors.New("checksum mismatch")
	ErrConfigParse      = errors.New("config parse error")
)

// ThresholdConfigStatus 表示当前生效配置版本与校验和。
type ThresholdConfigStatus struct {
	CurrentVersion  string `json:"current_version"`
	CurrentChecksum string `json:"current_checksum"`
}

type ThresholdFileMeta struct {
	ActiveVersion  string `json:"active_version"`
	ActiveChecksum string `json:"active_checksum"`
	UpdatedAt      string `json:"updated_at"`
}

type thresholdPersistedFile struct {
	Meta ThresholdFileMeta `json:"_meta"`
	ThresholdConfig
}

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

func initializeThresholdConfig() {
	cfg := defaultThresholdConfig()
	meta := ThresholdConfigStatus{CurrentVersion: "DEFAULT"}
	activePath := ""
	legacyFileLoaded := false

	for _, p := range candidateThresholdPaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(b, cfg); err != nil {
			continue
		}
		if fileMeta, ok := extractStatusFromPersistedMeta(b); ok {
			meta = fileMeta
		} else {
			meta.CurrentVersion = "BOOTSTRAP"
			if cfgChecksum, err := checksumForConfig(cfg); err == nil {
				meta.CurrentChecksum = cfgChecksum
			} else {
				meta.CurrentChecksum = formatCRC32Hex(crc32.ChecksumIEEE(b))
			}
			legacyFileLoaded = true
		}
		activePath = p
		break
	}

	if meta.CurrentChecksum == "" {
		if b, err := json.Marshal(cfg); err == nil {
			meta.CurrentChecksum = formatCRC32Hex(crc32.ChecksumIEEE(b))
		}
	}

	thresholdMu.Lock()
	thresholdCfg = cfg
	thresholdMeta = meta
	thresholdPath = activePath
	thresholdMu.Unlock()

	if activePath != "" && legacyFileLoaded {
		_ = persistThresholdConfig(activePath, cfg, meta)
	}
}

func checksumForConfig(cfg *ThresholdConfig) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return formatCRC32Hex(crc32.ChecksumIEEE(b)), nil
}

func extractStatusFromPersistedMeta(raw []byte) (ThresholdConfigStatus, bool) {
	var m struct {
		Meta ThresholdFileMeta `json:"_meta"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ThresholdConfigStatus{}, false
	}
	if m.Meta.ActiveVersion == "" || m.Meta.ActiveChecksum == "" {
		return ThresholdConfigStatus{}, false
	}
	return ThresholdConfigStatus{
		CurrentVersion:  m.Meta.ActiveVersion,
		CurrentChecksum: normalizeChecksumToken(m.Meta.ActiveChecksum),
	}, true
}

func resolveThresholdWritePath() string {
	for _, p := range candidateThresholdPaths() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	candidates := candidateThresholdPaths()
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "./thresholds.json"
}

func persistThresholdConfig(path string, cfg *ThresholdConfig, status ThresholdConfigStatus) error {
	if cfg == nil {
		return fmt.Errorf("threshold config is nil")
	}

	payload := thresholdPersistedFile{
		Meta: ThresholdFileMeta{
			ActiveVersion:  status.CurrentVersion,
			ActiveChecksum: status.CurrentChecksum,
			UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
		},
		ThresholdConfig: *cfg,
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal threshold file: %w", err)
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create threshold dir: %w", err)
		}
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write threshold temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace threshold file: %w", err)
	}

	return nil
}

func formatCRC32Hex(v uint32) string {
	return fmt.Sprintf("%08X", v)
}

func normalizeChecksumToken(s string) string {
	v := strings.TrimSpace(strings.ToUpper(s))
	v = strings.TrimPrefix(v, "0X")
	return v
}

func checksumMatched(data []byte, provided string) bool {
	provided = normalizeChecksumToken(provided)
	if provided == "" {
		return false
	}

	actual := crc32.ChecksumIEEE(data)
	if provided == formatCRC32Hex(actual) {
		return true
	}

	if u, err := strconv.ParseUint(provided, 16, 32); err == nil && uint32(u) == actual {
		return true
	}

	if u, err := strconv.ParseUint(provided, 10, 32); err == nil && uint32(u) == actual {
		return true
	}

	return false
}

// GetThresholdConfig 获取阈值配置（进程内懒加载）
// 未找到配置文件或解析失败时自动回退到默认值。
func GetThresholdConfig() *ThresholdConfig {
	thresholdOnce.Do(func() {
		initializeThresholdConfig()
	})
	thresholdMu.RLock()
	defer thresholdMu.RUnlock()
	return thresholdCfg
}

// UpdateThresholdConfig 运行时更新阈值配置。
func UpdateThresholdConfig(version, checksum, configData string) error {
	thresholdOnce.Do(func() {
		initializeThresholdConfig()
	})

	raw := []byte(configData)
	if !checksumMatched(raw, checksum) {
		return ErrChecksumMismatch
	}

	cfg := defaultThresholdConfig()
	if err := json.Unmarshal(raw, cfg); err != nil {
		return ErrConfigParse
	}

	status := ThresholdConfigStatus{
		CurrentVersion:  version,
		CurrentChecksum: formatCRC32Hex(crc32.ChecksumIEEE(raw)),
	}

	thresholdMu.RLock()
	path := thresholdPath
	thresholdMu.RUnlock()
	if path == "" {
		path = resolveThresholdWritePath()
	}

	if err := persistThresholdConfig(path, cfg, status); err != nil {
		return fmt.Errorf("persist threshold config failed: %w", err)
	}

	thresholdMu.Lock()
	thresholdCfg = cfg
	thresholdMeta = status
	thresholdPath = path
	thresholdMu.Unlock()

	return nil
}

// GetThresholdConfigStatus 返回当前生效配置状态。
func GetThresholdConfigStatus() ThresholdConfigStatus {
	thresholdOnce.Do(func() {
		initializeThresholdConfig()
	})

	thresholdMu.RLock()
	defer thresholdMu.RUnlock()
	return thresholdMeta
}

// ResetThresholdCachesForTest resets threshold runtime caches for isolated tests.
func ResetThresholdCachesForTest() {
	thresholdMu.Lock()
	defer thresholdMu.Unlock()
	thresholdOnce = sync.Once{}
	thresholdCfg = nil
	thresholdMeta = ThresholdConfigStatus{}
	thresholdPath = ""
}
