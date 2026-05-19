package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrChecksumMismatch = errors.New("checksum mismatch")
	ErrConfigParse      = errors.New("config parse error")
)

// RecoveryPlanConfigStatus 表示当前生效配置版本与校验和。
type RecoveryPlanConfigStatus struct {
	CurrentVersion  string `json:"current_version"`
	CurrentChecksum string `json:"current_checksum"`
}

var (
	recoveryPlanStatusOnce sync.Once
	recoveryPlanStatusMu   sync.RWMutex
	recoveryPlanStatus     RecoveryPlanConfigStatus
	recoveryPlanConfigPath string
)

type recoveryPlanConfigFile struct {
	Version string                     `json:"version"`
	Plans   map[string]json.RawMessage `json:"plans"`
}

func candidateRecoveryPlanPaths() []string {
	if p := strings.TrimSpace(os.Getenv("FR_RECOVERY_PLAN_CONFIG")); p != "" {
		return []string{p}
	}
	return []string{
		"./fault-recovery/configs/recovery_plan_mapping_template.json",
		"./configs/recovery_plan_mapping_template.json",
	}
}

func formatCRC32Hex(v uint32) string {
	return fmt.Sprintf("%08X", v)
}

func normalizeChecksumToken(v string) string {
	t := strings.ToUpper(strings.TrimSpace(v))
	t = strings.TrimPrefix(t, "0X")
	return t
}

func validateRecoveryPlanConfigData(configData string) error {
	var cfg recoveryPlanConfigFile
	if err := json.Unmarshal([]byte(configData), &cfg); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigParse, err)
	}
	if strings.TrimSpace(cfg.Version) == "" {
		return fmt.Errorf("%w: version is required", ErrConfigParse)
	}
	if cfg.Plans == nil || len(cfg.Plans) == 0 {
		return fmt.Errorf("%w: plans is required", ErrConfigParse)
	}
	return nil
}

func initializeRecoveryPlanConfigStatus() {
	status := RecoveryPlanConfigStatus{CurrentVersion: "DEFAULT"}
	activePath := ""

	for _, p := range candidateRecoveryPlanPaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := validateRecoveryPlanConfigData(string(b)); err != nil {
			continue
		}
		status.CurrentVersion = "BOOTSTRAP"
		status.CurrentChecksum = formatCRC32Hex(crc32.ChecksumIEEE(b))
		activePath = p
		break
	}

	recoveryPlanStatusMu.Lock()
	recoveryPlanStatus = status
	recoveryPlanConfigPath = activePath
	recoveryPlanStatusMu.Unlock()
}

func ensureRecoveryPlanStatusInitialized() {
	recoveryPlanStatusOnce.Do(initializeRecoveryPlanConfigStatus)
}

func resolveRecoveryPlanWritePath() string {
	recoveryPlanStatusMu.RLock()
	if recoveryPlanConfigPath != "" {
		p := recoveryPlanConfigPath
		recoveryPlanStatusMu.RUnlock()
		return p
	}
	recoveryPlanStatusMu.RUnlock()

	for _, p := range candidateRecoveryPlanPaths() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}

	candidates := candidateRecoveryPlanPaths()
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "./fault-recovery/configs/recovery_plan_mapping_template.json"
}

func writeRecoveryPlanConfigAtomically(path, configData string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config dir failed: %w", err)
		}
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(configData), 0o644); err != nil {
		return fmt.Errorf("write temp config file failed: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace config file failed: %w", err)
	}
	return nil
}

// UpdateRecoveryPlanConfig 在运行时更新修复映射配置。
func UpdateRecoveryPlanConfig(version, checksum, configData string) error {
	ensureRecoveryPlanStatusInitialized()

	version = strings.TrimSpace(version)
	checksum = normalizeChecksumToken(checksum)
	if version == "" || checksum == "" || strings.TrimSpace(configData) == "" {
		return fmt.Errorf("invalid update request")
	}

	actualChecksum := formatCRC32Hex(crc32.ChecksumIEEE([]byte(configData)))
	if checksum != actualChecksum {
		return ErrChecksumMismatch
	}

	if err := validateRecoveryPlanConfigData(configData); err != nil {
		return err
	}

	writePath := resolveRecoveryPlanWritePath()
	if err := writeRecoveryPlanConfigAtomically(writePath, configData); err != nil {
		return err
	}

	recoveryPlanStatusMu.Lock()
	recoveryPlanStatus = RecoveryPlanConfigStatus{
		CurrentVersion:  version,
		CurrentChecksum: actualChecksum,
	}
	recoveryPlanConfigPath = writePath
	recoveryPlanStatusMu.Unlock()

	return nil
}

// GetRecoveryPlanConfigStatus 返回当前修复映射配置状态。
func GetRecoveryPlanConfigStatus() RecoveryPlanConfigStatus {
	ensureRecoveryPlanStatusInitialized()
	recoveryPlanStatusMu.RLock()
	defer recoveryPlanStatusMu.RUnlock()
	return recoveryPlanStatus
}

// ResetRecoveryPlanConfigStatusForTest resets runtime caches for isolated tests.
func ResetRecoveryPlanConfigStatusForTest() {
	recoveryPlanStatusMu.Lock()
	defer recoveryPlanStatusMu.Unlock()
	recoveryPlanStatusOnce = sync.Once{}
	recoveryPlanStatus = RecoveryPlanConfigStatus{}
	recoveryPlanConfigPath = ""
}
