package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	diagnosisModels "fault-diagnosis/pkg/models"
	recovery "fault-tolerance/fault-recovery/pkg/recovery"
)

type scenarioExpectation struct {
	trees  []string
	faults []string
	plans  []string
}

type scenarioPerfTracker struct {
	mu                sync.Mutex
	scenario          string
	expect            scenarioExpectation
	alertByBasicEvent map[string]string
	alertTimes        map[string]int64

	healthAlertMs           int64
	diagnosisTriggerAlertMs int64
	diagnosisResultMs       int64
	recoveryPlanStartMs     int64
	recoveryPlanFinishMs    int64

	faultInjections   int
	correctDiagnoses  int
	lastTree          string
	lastFault         string
	lastPlan          string
	lastRecoveryState string
}

func newScenarioPerfTracker(scenario string) *scenarioPerfTracker {
	normalized := normalizeScenarioName(scenario)
	return &scenarioPerfTracker{
		scenario:   normalized,
		expect:     expectationForScenario(normalized),
		alertTimes: make(map[string]int64),
	}
}

func (t *scenarioPerfTracker) SetAlertByBasicEvent(mapping map[string]string) {
	if t == nil {
		return
	}
	copied := make(map[string]string, len(mapping))
	for eventID, alertID := range mapping {
		copied[eventID] = alertID
	}
	t.mu.Lock()
	t.alertByBasicEvent = copied
	t.mu.Unlock()
}

func (t *scenarioPerfTracker) ObserveAlert(alert *diagnosisModels.AlertEvent) {
	if t == nil || alert == nil || alert.IsResolved() {
		return
	}
	tMs := metadataInt64(alert.Metadata, "health_alert_ms")
	if tMs == 0 {
		tMs = nowMs()
	}

	t.mu.Lock()
	if t.healthAlertMs == 0 {
		t.healthAlertMs = tMs
	}
	if _, ok := t.alertTimes[alert.AlertID]; !ok {
		t.alertTimes[alert.AlertID] = tMs
	}
	t.mu.Unlock()

	fmt.Printf("[perf][health_alert] trace=%s t_ms=%d alert=%s fault=%s\n", alert.AlertID, tMs, alert.AlertID, alert.FaultCode)
}

func (t *scenarioPerfTracker) ObserveDiagnosis(result *diagnosisModels.DiagnosisResult) {
	if t == nil || result == nil {
		return
	}

	status := diagnosisStatus(result)
	tMs := nowMs()
	planID := primaryRecoveryPlanID(result.Metadata)

	t.mu.Lock()
	if status != recovery.EventStatusResolved {
		t.faultInjections++
		if t.diagnosisResultMs == 0 {
			t.diagnosisResultMs = tMs
		}
		if t.diagnosisTriggerAlertMs == 0 {
			t.diagnosisTriggerAlertMs = t.triggerAlertMsLocked(result)
		}
		if t.matchesExpectationLocked(result, planID) {
			t.correctDiagnoses++
		}
	}
	t.lastTree = result.FaultTreeID
	t.lastFault = result.FaultCode
	if planID != "" {
		t.lastPlan = planID
	}
	t.mu.Unlock()

	fmt.Printf("[perf][diagnosis_result] trace=%s t_ms=%d tree=%s fault=%s plan=%s status=%s\n",
		result.DiagnosisID, tMs, result.FaultTreeID, result.FaultCode, planID, status)
}

func (t *scenarioPerfTracker) ObserveRecoveryPlanStart(event recovery.RecoveryPlanEvent) {
	if t == nil {
		return
	}

	t.mu.Lock()
	if t.recoveryPlanStartMs == 0 {
		t.recoveryPlanStartMs = event.TimestampMs
	}
	t.mu.Unlock()

	fmt.Printf("[perf][recovery_plan_start] trace=%s t_ms=%d plan=%s\n", event.TraceID, event.TimestampMs, event.PlanID)
}

func (t *scenarioPerfTracker) ObserveRecoveryPlanFinish(event recovery.RecoveryPlanEvent) {
	if t == nil {
		return
	}

	t.mu.Lock()
	t.recoveryPlanFinishMs = event.TimestampMs
	t.lastRecoveryState = event.Status
	t.mu.Unlock()

	fmt.Printf("[perf][recovery_plan_finish] trace=%s t_ms=%d plan=%s status=%s error=%q\n",
		event.TraceID, event.TimestampMs, event.PlanID, event.Status, event.Error)
}

func (t *scenarioPerfTracker) PrintSummary() {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	total := t.faultInjections
	accuracy := "N/A"
	if total > 0 {
		accuracy = fmt.Sprintf("%.2f%%", float64(t.correctDiagnoses)/float64(total)*100)
	}

	fmt.Println("\n========== 场景级性能结果 ==========")
	if t.scenario != "" {
		fmt.Printf("场景: %s\n", t.scenario)
	}
	fmt.Printf("P-001 故障检测准确率: %s (%d/%d)\n", accuracy, t.correctDiagnoses, total)
	fmt.Printf("P-002 故障检测时延: %s\n", durationText(t.diagnosisTriggerAlertMs, t.diagnosisResultMs))
	fmt.Printf("P-003 恢复调度时延: %s\n", durationText(t.diagnosisResultMs, t.recoveryPlanStartMs))
	fmt.Printf("P-004 恢复计划执行时间: %s\n", durationText(t.recoveryPlanStartMs, t.recoveryPlanFinishMs))
	fmt.Printf("P-005 端到端故障修复时间: %s\n", durationText(t.diagnosisTriggerAlertMs, t.recoveryPlanFinishMs))
	fmt.Printf("诊断结果: tree=%s fault=%s plan=%s recovery_status=%s\n", t.lastTree, t.lastFault, t.lastPlan, t.lastRecoveryState)
	fmt.Println("====================================")
}

func (t *scenarioPerfTracker) triggerAlertMsLocked(result *diagnosisModels.DiagnosisResult) int64 {
	var latest int64
	for _, eventID := range result.BasicEvents {
		alertID := t.alertByBasicEvent[eventID]
		if alertID == "" {
			continue
		}
		if tMs := t.alertTimes[alertID]; tMs > latest {
			latest = tMs
		}
	}
	if latest != 0 {
		return latest
	}
	return t.healthAlertMs
}

func (t *scenarioPerfTracker) matchesExpectationLocked(result *diagnosisModels.DiagnosisResult, planID string) bool {
	if len(t.expect.trees) == 0 && len(t.expect.faults) == 0 && len(t.expect.plans) == 0 {
		return true
	}
	return containsString(t.expect.trees, result.FaultTreeID) &&
		containsString(t.expect.faults, result.FaultCode) &&
		containsString(t.expect.plans, planID)
}

func expectationForScenario(scenario string) scenarioExpectation {
	switch scenario {
	case "s-001", "power_dispatch":
		return scenarioExpectation{trees: []string{"TREE-003"}, faults: []string{"YW-RG-ZD-3"}, plans: []string{"RP-003"}}
	case "s-002", "ad_dispatch":
		return scenarioExpectation{trees: []string{"TREE-003"}, faults: []string{"YW-RG-ZD-3"}, plans: []string{"RP-004"}}
	case "s-003", "thermal_sensor_fault":
		return scenarioExpectation{trees: []string{"TREE-004"}, faults: []string{"YW-RG-ZD-4"}, plans: []string{"RP-004", "RP-005"}}
	case "s-004":
		return scenarioExpectation{
			trees:  []string{"TREE-005", "TREE-006", "TREE-009"},
			faults: []string{"YW-RG-ZD-5", "YW-RG-ZD-6", "YW-O2-ZD-1"},
			plans:  []string{"RP-006", "RP-007", "RP-008", "RP-009", "RP-010", "RP-011", "RP-012", "RP-013", "RP-014"},
		}
	case "heater_platform_fault":
		return scenarioExpectation{trees: []string{"TREE-005"}, faults: []string{"YW-RG-ZD-5"}, plans: []string{"RP-006", "RP-007", "RP-008"}}
	case "heater_battery_fault":
		return scenarioExpectation{trees: []string{"TREE-006"}, faults: []string{"YW-RG-ZD-6"}, plans: []string{"RP-009", "RP-010", "RP-011"}}
	case "heater_tank_fault":
		return scenarioExpectation{trees: []string{"TREE-009"}, faults: []string{"YW-O2-ZD-1"}, plans: []string{"RP-012", "RP-013", "RP-014"}}
	case "s-005", "can_noresponse":
		return scenarioExpectation{trees: []string{"TREE-002"}, faults: []string{"YW-RG-ZD-2"}, plans: []string{"RP-002"}}
	case "s-006":
		return scenarioExpectation{trees: []string{"TREE-011", "TREE-012", "TREE-013", "TREE-014", "TREE-015", "TREE-016"}, faults: nil, plans: []string{"RP-015", "RP-016", "RP-017", "RP-018", "RP-019", "RP-020", "RP-021", "RP-022"}}
	case "comm_start_fail", "comm_telemetry_fault":
		return scenarioExpectation{trees: []string{"TREE-011", "TREE-012"}, faults: []string{"YW-O2-CS-1", "YW-O2-CS-2"}, plans: []string{"RP-015"}}
	case "comm_transmit_switch_fault":
		return scenarioExpectation{trees: []string{"TREE-013"}, faults: []string{"YW-O2-CS-3"}, plans: []string{"RP-016", "RP-017"}}
	case "comm_air_link_fault":
		return scenarioExpectation{trees: []string{"TREE-014"}, faults: []string{"YW-O2-CS-4"}, plans: []string{"RP-018"}}
	case "comm_telemetry_encrypt_fault":
		return scenarioExpectation{trees: []string{"TREE-015"}, faults: []string{"YW-O2-CS-5"}, plans: []string{"RP-019", "RP-020"}}
	case "comm_remote_encrypt_fault":
		return scenarioExpectation{trees: []string{"TREE-016"}, faults: []string{"YW-O2-CS-6"}, plans: []string{"RP-021", "RP-022"}}
	case "s-007", "gnss_telemetry_fault":
		return scenarioExpectation{trees: []string{"TREE-017", "TREE-018"}, faults: []string{"YW-O2-CS-7", "YW-O2-CS-8"}, plans: []string{"RP-023"}}
	case "s-008", "gyro_telemetry_fault":
		return scenarioExpectation{trees: []string{"TREE-019", "TREE-020"}, faults: []string{"YW-O2-CS-9", "YW-O2-CS-10"}, plans: []string{"RP-024"}}
	case "s-009", "mems_telemetry_fault":
		return scenarioExpectation{trees: []string{"TREE-021", "TREE-022"}, faults: []string{"YW-O2-CS-11", "YW-O2-CS-12"}, plans: []string{"RP-025"}}
	case "s-010", "startracker_telemetry_fault":
		return scenarioExpectation{trees: []string{"TREE-023", "TREE-024"}, faults: []string{"YW-O2-CS-13", "YW-O2-CS-14"}, plans: []string{"RP-026"}}
	case "s-011":
		return scenarioExpectation{trees: []string{"TREE-025", "TREE-026", "TREE-027"}, faults: []string{"YW-O2-CS-15", "YW-O2-CS-16", "YW-O2-CS-17"}, plans: []string{"RP-027", "RP-028", "RP-029", "RP-030"}}
	case "momentum_start_fail":
		return scenarioExpectation{trees: []string{"TREE-025"}, faults: []string{"YW-O2-CS-15"}, plans: []string{"RP-027"}}
	case "momentum_recheck_ok", "momentum_recheck_fail":
		return scenarioExpectation{trees: []string{"TREE-026"}, faults: []string{"YW-O2-CS-16"}, plans: []string{"RP-029"}}
	case "momentum_direct_dispatch":
		return scenarioExpectation{trees: []string{"TREE-026"}, faults: []string{"YW-O2-CS-16"}, plans: []string{"RP-028"}}
	case "momentum_telemetry_fault":
		return scenarioExpectation{trees: []string{"TREE-027"}, faults: []string{"YW-O2-CS-17"}, plans: []string{"RP-030"}}
	case "s-012", "power_resolved_cancel":
		return scenarioExpectation{trees: []string{"TREE-003"}, faults: []string{"YW-RG-ZD-3"}, plans: []string{"RP-003"}}
	default:
		return scenarioExpectation{}
	}
}

func normalizeScenarioName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "_", "-")
	switch s {
	case "s-001", "s001":
		return "s-001"
	case "s-002", "s002":
		return "s-002"
	case "s-003", "s003":
		return "s-003"
	case "s-004", "s004":
		return "s-004"
	case "s-005", "s005":
		return "s-005"
	case "s-006", "s006":
		return "s-006"
	case "s-007", "s007":
		return "s-007"
	case "s-008", "s008":
		return "s-008"
	case "s-009", "s009":
		return "s-009"
	case "s-010", "s010":
		return "s-010"
	case "s-011", "s011":
		return "s-011"
	case "s-012", "s012":
		return "s-012"
	default:
		return strings.ReplaceAll(s, "-", "_")
	}
}

func containsString(allowed []string, value string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if item == value {
			return true
		}
	}
	return false
}

func primaryRecoveryPlanID(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	if v, ok := metadata["primary_recovery_plan_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	if v, ok := metadata["recovery_plan_ids"].([]string); ok && len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	if v, ok := metadata["recovery_plan_ids"].([]interface{}); ok && len(v) > 0 {
		if s, ok := v[0].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func diagnosisStatus(result *diagnosisModels.DiagnosisResult) string {
	if result == nil || result.Metadata == nil {
		return recovery.EventStatusFiring
	}
	if v, ok := result.Metadata["status"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.ToUpper(strings.TrimSpace(v))
	}
	return recovery.EventStatusFiring
}

func metadataInt64(metadata map[string]interface{}, key string) int64 {
	if metadata == nil {
		return 0
	}
	switch v := metadata[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case uint64:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		var out int64
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &out); err == nil {
			return out
		}
	}
	return 0
}

func durationText(startMs, finishMs int64) string {
	if startMs == 0 || finishMs == 0 {
		return "待补充"
	}
	if finishMs < startMs {
		return "时间戳异常"
	}
	return fmt.Sprintf("%d ms", finishMs-startMs)
}

func nowMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
