package configrpc

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"testing"

	vsoaProtocol "github.com/acoinfo/vsoa/protocol"
)

func TestHandleUpdateConfigRPCBridge(t *testing.T) {
	s := NewVSOAServer("127.0.0.1:3001", NewRuntimeConfigService())
	configData := `{"node":{"cpu_usage_max":16.5}}`
	checksum := fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(configData)))

	req := vsoaProtocol.NewMessage()
	req.Param = makeUpdatePayload(DefaultModuleName, "V4.0", configData, checksum)
	res := vsoaProtocol.NewMessage()

	s.handleUpdateConfigRPC(req, res)

	var payload UpdateConfigResponse
	if err := json.Unmarshal(res.Param, &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if payload.StatusCode != StatusCodeSuccess {
		t.Fatalf("expected status_code=%d, got %d", StatusCodeSuccess, payload.StatusCode)
	}
	if payload.ActiveVersion != "V4.0" {
		t.Fatalf("expected active_version=V4.0, got %s", payload.ActiveVersion)
	}
}

func TestHandleGetStatusRPCBridge(t *testing.T) {
	s := NewVSOAServer("127.0.0.1:3001", NewRuntimeConfigService())

	req := vsoaProtocol.NewMessage()
	res := vsoaProtocol.NewMessage()

	s.handleGetStatusRPC(req, res)

	var payload GetStatusResponse
	if err := json.Unmarshal(res.Param, &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if payload.CurrentVersion == "" {
		t.Fatalf("expected current_version not empty")
	}
	if payload.CurrentChecksum == "" {
		t.Fatalf("expected current_checksum not empty")
	}
}
