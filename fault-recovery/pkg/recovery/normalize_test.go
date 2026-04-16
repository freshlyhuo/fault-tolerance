package recovery

import (
	"testing"
	"time"
)

func TestNormalizeDiagnosisEvent_ExtractFaultCodeAndTarget(t *testing.T) {
	e := DiagnosisResult{
		DiagnosisID: "diag-1",
		FaultCode:   "BUSINESS-IMAGE-START",
		Source:      "svc-a",
		Timestamp:   time.Unix(1710000000, 0),
		Metadata: map[string]interface{}{
			"status": "FIRING",
		},
	}

	n, err := NormalizeDiagnosisEvent(e)
	if err != nil {
		t.Fatalf("NormalizeDiagnosisEvent error: %v", err)
	}
	if n.FaultCode != "BUSINESS-IMAGE-START" {
		t.Fatalf("fault_code mismatch: %s", n.FaultCode)
	}
	if n.TargetID != "svc-a" {
		t.Fatalf("target_id mismatch: %s", n.TargetID)
	}
	if n.Status != EventStatusFiring {
		t.Fatalf("status mismatch: %s", n.Status)
	}
}

func TestNormalizeDiagnosisEvent_StatusDefaultsToFiring(t *testing.T) {
	e := DiagnosisResult{
		DiagnosisID: "diag-2",
		FaultCode:   "BUSINESS-IMAGE-START",
		Source:      "svc-b",
		Timestamp:   time.Unix(1710000001, 0),
	}

	n, err := NormalizeDiagnosisEvent(e)
	if err != nil {
		t.Fatalf("NormalizeDiagnosisEvent error: %v", err)
	}
	if n.Status != EventStatusFiring {
		t.Fatalf("status mismatch: %s", n.Status)
	}
}

func TestNormalizeDiagnosisEvent_InvalidWhenFaultCodeMissing(t *testing.T) {
	e := DiagnosisResult{Source: "svc-a", Timestamp: time.Now()}
	_, err := NormalizeDiagnosisEvent(e)
	if err == nil {
		t.Fatal("expected error when fault_code missing")
	}
}
