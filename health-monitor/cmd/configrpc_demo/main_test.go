package main

import (
	"fmt"
	"hash/crc32"
	"testing"

	"health-monitor/pkg/configrpc"
)

func TestBuildUpdateDemoCasesCoversStatusCodes(t *testing.T) {
	moduleName := configrpc.DefaultModuleName
	baseVersion := "V9.9"
	validConfig := `{"node":{"cpu_usage_max":42.5}}`

	cases := buildUpdateDemoCases(moduleName, baseVersion, validConfig)
	if len(cases) != 3 {
		t.Fatalf("expected 3 demo cases, got %d", len(cases))
	}

	if cases[0].ExpectedStatusCode != configrpc.StatusCodeSuccess {
		t.Fatalf("case 0 expected status %d, got %d", configrpc.StatusCodeSuccess, cases[0].ExpectedStatusCode)
	}

	if cases[1].ExpectedStatusCode != configrpc.StatusCodeChecksumError {
		t.Fatalf("case 1 expected status %d, got %d", configrpc.StatusCodeChecksumError, cases[1].ExpectedStatusCode)
	}
	if cases[1].Request.Checksum == fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(validConfig))) {
		t.Fatalf("case 1 should use an incorrect checksum")
	}

	if cases[2].ExpectedStatusCode != configrpc.StatusCodeParseError {
		t.Fatalf("case 2 expected status %d, got %d", configrpc.StatusCodeParseError, cases[2].ExpectedStatusCode)
	}
	if cases[2].Request.ConfigData == validConfig {
		t.Fatalf("case 2 should use malformed config_data")
	}
	wantBrokenChecksum := fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(cases[2].Request.ConfigData)))
	if cases[2].Request.Checksum != wantBrokenChecksum {
		t.Fatalf("case 2 should use checksum of malformed config_data, got %s want %s", cases[2].Request.Checksum, wantBrokenChecksum)
	}
}
