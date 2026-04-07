package config

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func setupThresholdConfigFileForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "thresholds.json")
	seed := `{"node":{"cpu_usage_max":85}}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("write seed config failed: %v", err)
	}
	t.Setenv("HM_THRESHOLD_CONFIG", path)
	ResetThresholdCachesForTest()
	t.Cleanup(ResetThresholdCachesForTest)
	return path
}

func TestUpdateThresholdConfigSuccess(t *testing.T) {
	path := setupThresholdConfigFileForTest(t)

	configData := `{"node":{"cpu_usage_max":12.5}}`
	checksum := fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(configData)))

	err := UpdateThresholdConfig("V2.1", checksum, configData)
	if err != nil {
		t.Fatalf("UpdateThresholdConfig returned error: %v", err)
	}

	cfg := GetThresholdConfig()
	if cfg.Node.CPUUsageMax != 12.5 {
		t.Fatalf("expected cpu_usage_max=12.5, got %v", cfg.Node.CPUUsageMax)
	}

	status := GetThresholdConfigStatus()
	if status.CurrentVersion != "V2.1" {
		t.Fatalf("expected current version V2.1, got %s", status.CurrentVersion)
	}
	if status.CurrentChecksum != checksum {
		t.Fatalf("expected checksum %s, got %s", checksum, status.CurrentChecksum)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted file failed: %v", err)
	}

	var persisted map[string]json.RawMessage
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("unmarshal persisted file failed: %v", err)
	}

	metaRaw, ok := persisted["_meta"]
	if !ok {
		t.Fatalf("expected _meta in persisted thresholds.json")
	}
	var meta struct {
		ActiveVersion  string `json:"active_version"`
		ActiveChecksum string `json:"active_checksum"`
		UpdatedAt      string `json:"updated_at"`
	}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("unmarshal _meta failed: %v", err)
	}
	if meta.ActiveVersion != "V2.1" {
		t.Fatalf("expected persisted active_version=V2.1, got %s", meta.ActiveVersion)
	}
	if meta.ActiveChecksum != checksum {
		t.Fatalf("expected persisted active_checksum=%s, got %s", checksum, meta.ActiveChecksum)
	}
	if meta.UpdatedAt == "" {
		t.Fatalf("expected persisted updated_at not empty")
	}
}

func TestUpdateThresholdConfigChecksumMismatch(t *testing.T) {
	_ = setupThresholdConfigFileForTest(t)

	configData := `{"node":{"cpu_usage_max":15.0}}`
	err := UpdateThresholdConfig("V2.2", "BAD-CHECKSUM", configData)
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
	if err != ErrChecksumMismatch {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
}

func TestUpdateThresholdConfigParseError(t *testing.T) {
	_ = setupThresholdConfigFileForTest(t)

	configData := `{"node":{"cpu_usage_max":` // broken JSON
	checksum := fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(configData)))

	err := UpdateThresholdConfig("V2.3", checksum, configData)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if err != ErrConfigParse {
		t.Fatalf("expected ErrConfigParse, got %v", err)
	}
}

func TestGetThresholdConfigStatusAfterFailedUpdate(t *testing.T) {
	_ = setupThresholdConfigFileForTest(t)

	okConfig := `{"node":{"cpu_usage_max":20.0}}`
	okChecksum := fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(okConfig)))
	if err := UpdateThresholdConfig("V3.0", okChecksum, okConfig); err != nil {
		t.Fatalf("setup update failed: %v", err)
	}

	badConfig := `{"node":{"cpu_usage_max":25.0}}`
	if err := UpdateThresholdConfig("V3.1", "WRONG", badConfig); err != ErrChecksumMismatch {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}

	status := GetThresholdConfigStatus()
	if status.CurrentVersion != "V3.0" {
		t.Fatalf("expected current version to stay V3.0, got %s", status.CurrentVersion)
	}
	if status.CurrentChecksum != okChecksum {
		t.Fatalf("expected checksum to stay %s, got %s", okChecksum, status.CurrentChecksum)
	}
}

func TestThresholdStatusReloadFromPersistedMeta(t *testing.T) {
	_ = setupThresholdConfigFileForTest(t)

	configData := `{"node":{"cpu_usage_max":30.0}}`
	checksum := fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(configData)))
	if err := UpdateThresholdConfig("V4.2", checksum, configData); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// simulate process restart
	ResetThresholdCachesForTest()

	status := GetThresholdConfigStatus()
	if status.CurrentVersion != "V4.2" {
		t.Fatalf("expected reloaded version V4.2, got %s", status.CurrentVersion)
	}
	if status.CurrentChecksum != checksum {
		t.Fatalf("expected reloaded checksum %s, got %s", checksum, status.CurrentChecksum)
	}
}

func TestLegacyThresholdFileAutoUpgradeMetaOnLoad(t *testing.T) {
	path := setupThresholdConfigFileForTest(t)

	status := GetThresholdConfigStatus()
	if status.CurrentVersion != "BOOTSTRAP" {
		t.Fatalf("expected bootstrap version on legacy load, got %s", status.CurrentVersion)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read thresholds file failed: %v", err)
	}

	var persisted map[string]json.RawMessage
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("unmarshal thresholds file failed: %v", err)
	}

	metaRaw, ok := persisted["_meta"]
	if !ok {
		t.Fatalf("expected _meta to be auto-upgraded on load")
	}

	var meta struct {
		ActiveVersion  string `json:"active_version"`
		ActiveChecksum string `json:"active_checksum"`
		UpdatedAt      string `json:"updated_at"`
	}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("unmarshal _meta failed: %v", err)
	}
	if meta.ActiveVersion != "BOOTSTRAP" {
		t.Fatalf("expected active_version=BOOTSTRAP, got %s", meta.ActiveVersion)
	}
	if meta.ActiveChecksum == "" {
		t.Fatalf("expected active_checksum not empty")
	}
	if meta.UpdatedAt == "" {
		t.Fatalf("expected updated_at not empty")
	}
}
