package alert

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"health-monitor/pkg/config"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

func storeBusinessMetric(t *testing.T, sm *state.StateManager, data interface{}) {
	t.Helper()
	err := sm.UpdateMetric(&state.BusinessMetric{
		Data: &model.BusinessMetrics{
			Timestamp: time.Now().Unix(),
			Data:      data,
		},
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("UpdateMetric failed: %v", err)
	}
}

func hasAlert(alerts []*model.AlertEvent, id string) bool {
	for _, alert := range alerts {
		if alert.AlertID == id && alert.Status == model.AlertStatusFiring {
			return true
		}
	}
	return false
}

func TestCheckCommThresholdsWithStateUsesHistoryDeltas(t *testing.T) {
	sm, err := state.NewStateManager()
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}
	defer sm.Close()

	storeBusinessMetric(t, sm, &model.CommMetrics{
		ReceiveCmdCount:                     100,
		O1ReceiveDeviceResponseCount:        10,
		O1ReceiveTelemetryResponseCount:     20,
		O1ReceiveRemoteControlResponseCount: 30,
		ReceiveCTACount:                     40,
		ParityErrorCount:                    1,
		NoTelemetryCount:                    2,
	})

	alerts := CheckCommThresholdsWithState(&model.CommMetrics{
		ReceiveCmdCount:                     100,
		O1ReceiveDeviceResponseCount:        10,
		O1ReceiveTelemetryResponseCount:     20,
		O1ReceiveRemoteControlResponseCount: 30,
		ReceiveCTACount:                     40,
		ParityErrorCount:                    2,
		NoTelemetryCount:                    3,
	}, sm)

	if !hasAlert(alerts, "YW-comm-TMEZD01004cjb_InstructionCount") {
		t.Fatalf("expected instruction count no-increase alert")
	}
	if !hasAlert(alerts, "YW-comm-noresponse") {
		t.Fatalf("expected O1 no-response alert")
	}
	if !hasAlert(alerts, "YW-comm-TMEZD01150_ReceiveCTACount") {
		t.Fatalf("expected CTA count no-increase alert")
	}
	if !hasAlert(alerts, "YW-comm-TMEZD01046_CheckErrorCount") {
		t.Fatalf("expected check error count increase alert")
	}
	if !hasAlert(alerts, "YW-comm-Com_No_telemetry_count") {
		t.Fatalf("expected no telemetry count increase alert")
	}
}

func TestCheckCommThresholdsWithStateChecksExpectedStates(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "thresholds.json")
	cfg := []byte(`{
		"comm": {
			"transmission_channel_ExpectedState": { "value": 1 },
			"Communication_Telemetry_ExpectedState": { "value": 1 },
			"Communicator_RemoteControl_ExpectedState": { "value": 1 }
		}
	}`)
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	t.Setenv("HM_THRESHOLD_CONFIG", cfgPath)
	config.ResetThresholdCachesForTest()
	defer config.ResetThresholdCachesForTest()

	alerts := CheckCommThresholdsWithState(&model.CommMetrics{
		TransmitSwitch:           0,
		TelemetryEncryptStatus:   0,
		TelecontrolEncryptStatus: 0,
	}, nil)

	if !hasAlert(alerts, "YW-comm-TMEZD01155_SwitchState") {
		t.Fatalf("expected transmission switch state alert")
	}
	if !hasAlert(alerts, "YW-comm-TMEZD01167_TelemetryEncryptStatus") {
		t.Fatalf("expected telemetry encrypt state alert")
	}
	if !hasAlert(alerts, "YW-comm-TMEZD01168_TelemetryEncryptStatus") {
		t.Fatalf("expected remote-control encrypt state alert")
	}
}

func TestCheckAttitudeOrbitControlSkipsExpectedAndReceiveCounters(t *testing.T) {
	sm, err := state.NewStateManager()
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}
	defer sm.Close()

	storeBusinessMetric(t, sm, &model.AttitudeOrbitControlMetrics{
		Values: map[string]interface{}{
			"GNSS_ExpectedState":               1,
			"GNSS_SoftwareRecInstructionCount": 10,
			"TMEZD01041_CheckErrorCount":       1,
		},
	})

	alerts := CheckAttitudeOrbitControlThresholdsWithState(&model.AttitudeOrbitControlMetrics{
		Values: map[string]interface{}{
			"GNSS_ExpectedState":               0,
			"GNSS_SoftwareRecInstructionCount": 11,
			"TMEZD01041_CheckErrorCount":       2,
		},
	}, sm)

	if hasAlert(alerts, "YW-AttitudeOrbitControl-GNSS_ExpectedState") {
		t.Fatalf("did not expect expected-state alert")
	}
	if hasAlert(alerts, "YW-AttitudeOrbitControl-GNSS_SoftwareRecInstructionCount") {
		t.Fatalf("did not expect software receive count alert")
	}
	if !hasAlert(alerts, "YW-AttitudeOrbitControl-TMEZD01041_CheckErrorCount") {
		t.Fatalf("expected check error count increase alert")
	}
}
