package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrChecksumMismatch = errors.New("checksum mismatch")
	ErrConfigParse      = errors.New("config parse error")
)

// FaultTreeConfigStatus 表示当前生效配置版本与校验和。
type FaultTreeConfigStatus struct {
	CurrentVersion  string `json:"current_version"`
	CurrentChecksum string `json:"current_checksum"`
}

var (
	faultTreeStatusOnce sync.Once
	faultTreeStatusMu   sync.RWMutex
	faultTreeStatus     FaultTreeConfigStatus
	faultTreeConfigPath string
)

func candidateFaultTreePaths() []string {
	if p := strings.TrimSpace(os.Getenv("FD_FAULT_TREE_CONFIG")); p != "" {
		return []string{p}
	}
	return []string{
		"./configs/fault_trees_multi_template.json",
		"./fault-diagnosis/configs/fault_trees_multi_template.json",
	}
}

func formatCRC16Hex(v uint16) string {
	return fmt.Sprintf("%04X", v)
}

func crc16CCITTFalse(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func normalizeChecksumToken(v string) string {
	t := strings.ToUpper(strings.TrimSpace(v))
	return strings.TrimPrefix(t, "0X")
}

func checksumMatched(data []byte, provided string) bool {
	provided = normalizeChecksumToken(provided)
	if provided == "" {
		return false
	}

	actual := crc16CCITTFalse(data)
	if provided == formatCRC16Hex(actual) {
		return true
	}

	if u, err := strconv.ParseUint(provided, 16, 16); err == nil && uint16(u) == actual {
		return true
	}

	if u, err := strconv.ParseUint(provided, 10, 16); err == nil && uint16(u) == actual {
		return true
	}

	return false
}

func initializeFaultTreeConfigStatus() {
	status := FaultTreeConfigStatus{CurrentVersion: "DEFAULT"}
	activePath := ""

	for _, p := range candidateFaultTreePaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		loader := NewLoader(p)
		if _, err := loader.LoadFaultTrees(); err != nil {
			continue
		}

		status.CurrentVersion = "BOOTSTRAP"
		status.CurrentChecksum = formatCRC16Hex(crc16CCITTFalse(b))
		activePath = p
		break
	}

	faultTreeStatusMu.Lock()
	faultTreeStatus = status
	faultTreeConfigPath = activePath
	faultTreeStatusMu.Unlock()
}

func ensureFaultTreeStatusInitialized() {
	faultTreeStatusOnce.Do(initializeFaultTreeConfigStatus)
}

// SetFaultTreeConfigPath 指定运行时配置更新要写入的故障树配置文件。
func SetFaultTreeConfigPath(path string) {
	ensureFaultTreeStatusInitialized()
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}

	faultTreeStatusMu.Lock()
	faultTreeConfigPath = path
	faultTreeStatusMu.Unlock()
}

func resolveFaultTreeWritePath() string {
	faultTreeStatusMu.RLock()
	if faultTreeConfigPath != "" {
		p := faultTreeConfigPath
		faultTreeStatusMu.RUnlock()
		return p
	}
	faultTreeStatusMu.RUnlock()

	for _, p := range candidateFaultTreePaths() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}

	candidates := candidateFaultTreePaths()
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "./configs/fault_trees_multi_template.json"
}

func validateFaultTreeConfigData(configData string) error {
	tmpFile, err := os.CreateTemp("", "fault-tree-config-*.json")
	if err != nil {
		return fmt.Errorf("create temp config file failed: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(configData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp config file failed: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp config file failed: %w", err)
	}

	loader := NewLoader(tmpPath)
	if _, err := loader.LoadFaultTrees(); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigParse, err)
	}
	return nil
}

func writeFaultTreeConfigAtomically(path, configData string) error {
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

// UpdateFaultTreeConfig 在运行时更新故障树配置。
func UpdateFaultTreeConfig(version, checksum, configData string) error {
	ensureFaultTreeStatusInitialized()

	version = strings.TrimSpace(version)
	checksum = normalizeChecksumToken(checksum)
	if version == "" || checksum == "" || strings.TrimSpace(configData) == "" {
		return fmt.Errorf("invalid update request")
	}

	raw := []byte(configData)
	actualChecksum := formatCRC16Hex(crc16CCITTFalse(raw))
	if !checksumMatched(raw, checksum) {
		return ErrChecksumMismatch
	}

	if err := validateFaultTreeConfigData(configData); err != nil {
		return err
	}

	writePath := resolveFaultTreeWritePath()
	if err := writeFaultTreeConfigAtomically(writePath, configData); err != nil {
		return err
	}

	faultTreeStatusMu.Lock()
	faultTreeStatus = FaultTreeConfigStatus{
		CurrentVersion:  version,
		CurrentChecksum: actualChecksum,
	}
	faultTreeConfigPath = writePath
	faultTreeStatusMu.Unlock()
	return nil
}

// GetFaultTreeConfigStatus 返回当前故障树配置状态。
func GetFaultTreeConfigStatus() FaultTreeConfigStatus {
	ensureFaultTreeStatusInitialized()
	faultTreeStatusMu.RLock()
	defer faultTreeStatusMu.RUnlock()
	return faultTreeStatus
}
