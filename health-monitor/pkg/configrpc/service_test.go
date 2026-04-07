package configrpc

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"testing"
)

func makeUpdatePayload(moduleName, version, configData, checksum string) []byte {
	req := UpdateConfigRequest{
		ModuleName: moduleName,
		Version:    version,
		Checksum:   checksum,
		ConfigData: configData,
	}
	b, _ := json.Marshal(req)
	return b
}

func makeChecksum(configData string) string {
	return fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(configData)))
}

func TestHandleUpdateConfigPayloadSuccess(t *testing.T) {
	svc := NewRuntimeConfigService()

	configData := `{"node":{"cpu_usage_max":12.5}}`
	resp := svc.HandleUpdateConfigPayload(makeUpdatePayload(DefaultModuleName, "V2.1", configData, makeChecksum(configData)))

	if resp.StatusCode != StatusCodeSuccess {
		t.Fatalf("expected status_code=%d, got %d", StatusCodeSuccess, resp.StatusCode)
	}
	if resp.ActiveVersion != "V2.1" {
		t.Fatalf("expected active_version=V2.1, got %s", resp.ActiveVersion)
	}
}

func TestHandleUpdateConfigPayloadChecksumMismatch(t *testing.T) {
	svc := NewRuntimeConfigService()

	configData := `{"node":{"cpu_usage_max":13.5}}`
	resp := svc.HandleUpdateConfigPayload(makeUpdatePayload(DefaultModuleName, "V2.2", configData, "BAD-CHECKSUM"))

	if resp.StatusCode != StatusCodeChecksumError {
		t.Fatalf("expected status_code=%d, got %d", StatusCodeChecksumError, resp.StatusCode)
	}
}

func TestHandleUpdateConfigPayloadConfigParseError(t *testing.T) {
	svc := NewRuntimeConfigService()

	configData := `{"node":{"cpu_usage_max":`
	resp := svc.HandleUpdateConfigPayload(makeUpdatePayload(DefaultModuleName, "V2.3", configData, makeChecksum(configData)))

	if resp.StatusCode != StatusCodeParseError {
		t.Fatalf("expected status_code=%d, got %d", StatusCodeParseError, resp.StatusCode)
	}
}

func TestHandleUpdateConfigPayloadInvalidModule(t *testing.T) {
	svc := NewRuntimeConfigService()

	configData := `{"node":{"cpu_usage_max":14.5}}`
	resp := svc.HandleUpdateConfigPayload(makeUpdatePayload("wrong/module", "V2.4", configData, makeChecksum(configData)))

	if resp.StatusCode != StatusCodeInternalError {
		t.Fatalf("expected status_code=%d, got %d", StatusCodeInternalError, resp.StatusCode)
	}
}

func TestHandleGetStatusPayload(t *testing.T) {
	svc := NewRuntimeConfigService()

	configData := `{"node":{"cpu_usage_max":15.5}}`
	_ = svc.HandleUpdateConfigPayload(makeUpdatePayload(DefaultModuleName, "V3.0", configData, makeChecksum(configData)))

	resp := svc.HandleGetStatusPayload(nil)
	if resp.CurrentVersion != "V3.0" {
		t.Fatalf("expected current_version=V3.0, got %s", resp.CurrentVersion)
	}
	if resp.CurrentChecksum != makeChecksum(configData) {
		t.Fatalf("expected current_checksum=%s, got %s", makeChecksum(configData), resp.CurrentChecksum)
	}
}
