package main

import (
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
	if cases[1].Request.Checksum == checksumHex(validConfig) {
		t.Fatalf("case 1 should use an incorrect checksum")
	}

	if cases[2].ExpectedStatusCode != configrpc.StatusCodeParseError {
		t.Fatalf("case 2 expected status %d, got %d", configrpc.StatusCodeParseError, cases[2].ExpectedStatusCode)
	}
	if cases[2].Request.ConfigData == validConfig {
		t.Fatalf("case 2 should use malformed config_data")
	}
	wantBrokenChecksum := checksumHex(cases[2].Request.ConfigData)
	if cases[2].Request.Checksum != wantBrokenChecksum {
		t.Fatalf("case 2 should use checksum of malformed config_data, got %s want %s", cases[2].Request.Checksum, wantBrokenChecksum)
	}
}

func TestCRC16CCITTFalseKnownVector(t *testing.T) {
	if got := checksumHex("123456789"); got != "29B1" {
		t.Fatalf("CRC-16/CCITT-FALSE mismatch, got=%s want=29B1", got)
	}
}
