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

func TestCountAlertStatuses(t *testing.T) {
	firing, resolved := countAlertStatuses([]*model.AlertEvent{
		{AlertID: "A", Status: model.AlertStatusFiring},
		{AlertID: "B", Status: model.AlertStatusResolved},
		{AlertID: "C"},
		nil,
	})
	if firing != 2 || resolved != 1 {
		t.Fatalf("unexpected counts: firing=%d resolved=%d", firing, resolved)
	}
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

func TestCheckActuatorThresholdsWithStateChecksNoTelemetryIncrease(t *testing.T) {
	sm, err := state.NewStateManager()
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}
	defer sm.Close()

	storeBusinessMetric(t, sm, &model.ActuatorMetrics{
		WheelSpeedX:      100,
		WheelSpeedY:      100,
		WheelSpeedZ:      100,
		NoTelemetryCount: 1,
	})

	alerts := CheckActuatorThresholdsWithState(&model.ActuatorMetrics{
		WheelSpeedX:      100,
		WheelSpeedY:      100,
		WheelSpeedZ:      100,
		NoTelemetryCount: 2,
	}, sm)

	if !hasAlert(alerts, "YW-MomentumWheel-MomentumWheel_No_telemetry_count") {
		t.Fatalf("expected momentum wheel no telemetry count increase alert")
	}
}

func TestCheckActuatorThresholdsChecksWheelSpeedOnlyWhenCommandSet(t *testing.T) {
	alerts := CheckActuatorThresholdsWithState(&model.ActuatorMetrics{
		SendCmdK53029: 0,
		WheelSpeedX:   120,
		WheelSpeedY:   80,
		WheelSpeedZ:   120,
	}, nil)
	if hasAlert(alerts, "YW-MomentumWheel-WheelSpeed_X") ||
		hasAlert(alerts, "YW-MomentumWheel-WheelSpeed_Y") ||
		hasAlert(alerts, "YW-MomentumWheel-WheelSpeed_Z") {
		t.Fatalf("did not expect wheel speed alerts when SendCmd_K53029 is not 1")
	}

	alerts = CheckActuatorThresholdsWithState(&model.ActuatorMetrics{
		SendCmdK53029: 1,
		WheelSpeedX:   120,
		WheelSpeedY:   80,
		WheelSpeedZ:   120,
	}, nil)
	if !hasAlert(alerts, "YW-MomentumWheel-WheelSpeed_X") ||
		!hasAlert(alerts, "YW-MomentumWheel-WheelSpeed_Y") ||
		!hasAlert(alerts, "YW-MomentumWheel-WheelSpeed_Z") {
		t.Fatalf("expected wheel speed alerts when SendCmd_K53029 is 1")
	}
}

func TestCheckActuatorThresholdsWithStateChecksErrorCountIncreases(t *testing.T) {
	sm, err := state.NewStateManager()
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}
	defer sm.Close()

	storeBusinessMetric(t, sm, &model.ActuatorMetrics{
		WheelSpeedX:           100,
		WheelSpeedY:           100,
		WheelSpeedZ:           100,
		CheckErrorCount:       1,
		FrameHeaderErrorCount: 1,
		FrameLengthErrorCount: 1,
		ResetCount:            1,
	})

	alerts := CheckActuatorThresholdsWithState(&model.ActuatorMetrics{
		WheelSpeedX:           100,
		WheelSpeedY:           100,
		WheelSpeedZ:           100,
		CheckErrorCount:       2,
		FrameHeaderErrorCount: 2,
		FrameLengthErrorCount: 2,
		ResetCount:            2,
	}, sm)

	expectedAlerts := []string{
		"YW-AttitudeOrbitControl-TMEZD01013_CheckErrorCount",
		"YW-AttitudeOrbitControl-TMEZD01014_FrameHeaderErrorCount",
		"YW-AttitudeOrbitControl-TMEZD01015_FrameLengthErrorCount",
		"YW-AttitudeOrbitControl-TMEZD01016_ResetCount",
	}
	for _, id := range expectedAlerts {
		if !hasAlert(alerts, id) {
			t.Fatalf("expected alert %s", id)
		}
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

func TestCheckAttitudeOrbitControlIncreaseRules(t *testing.T) {
	sm, err := state.NewStateManager()
	if err != nil {
		t.Fatalf("NewStateManager failed: %v", err)
	}
	defer sm.Close()

	storeBusinessMetric(t, sm, &model.AttitudeOrbitControlMetrics{
		Values: map[string]interface{}{
			"GNSS_No_telemetry_count":         1,
			"TMEZD01054_ResetCount":           1,
			"Gyroscope_No_telemetry_count":    1,
			"TMEZD01021_CheckErrorCount":      1,
			"MEMS_No_telemetry_count":         1,
			"TMEZD01025_CheckErrorCount":      1,
			"StarTrackerl_No_telemetry_count": 1,
			"TMEZD01033_CheckErrorCount":      1,
		},
	})

	alerts := CheckAttitudeOrbitControlThresholdsWithState(&model.AttitudeOrbitControlMetrics{
		Values: map[string]interface{}{
			"GNSS_No_telemetry_count":         2,
			"TMEZD01054_ResetCount":           2,
			"Gyroscope_No_telemetry_count":    2,
			"TMEZD01021_CheckErrorCount":      2,
			"MEMS_No_telemetry_count":         2,
			"TMEZD01025_CheckErrorCount":      2,
			"StarTrackerl_No_telemetry_count": 2,
			"TMEZD01033_CheckErrorCount":      2,
		},
	}, sm)

	expectedAlerts := []string{
		"YW-AttitudeOrbitControl-GNSS_No_telemetry_count",
		"YW-AttitudeOrbitControl-TMEZD01044_ResetCount",
		"YW-AttitudeOrbitControl-Gyroscope_No_telemetry_count",
		"YW-AttitudeOrbitControl-TMEZD01021_CheckErrorCount",
		"YW-AttitudeOrbitControl-MEMS_No_telemetry_count",
		"YW-AttitudeOrbitControl-TMEZD01025_CheckErrorCount",
		"YW-AttitudeOrbitControl-StarTrackerl_No_telemetry_count",
		"YW-AttitudeOrbitControl-TMEZD01033_CheckErrorCount",
	}
	for _, id := range expectedAlerts {
		if !hasAlert(alerts, id) {
			t.Fatalf("expected alert %s", id)
		}
	}
}
