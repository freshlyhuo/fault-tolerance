package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"fault-diagnosis/pkg/config"
	"fault-diagnosis/pkg/engine"
	"fault-diagnosis/pkg/models"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "./configs/fault_trees_multi_template.json", "故障树配置文件路径")
	alertID := flag.String("alert-id", "YW-comm-noresponse", "用于触发诊断的告警ID")
	expectedTreeID := flag.String("expected-tree", "TREE-002", "预期触发的故障树ID")
	expectedFaultCode := flag.String("expected-fault-code", "YW-RG-ZD-2", "预期触发的故障码")
	expectedTopEvent := flag.String("expected-top-event", "T2-TOP-001", "预期触发路径中的顶层事件ID")
	expectedBasicEvent := flag.String("expected-basic-event", "T2-EVT-001", "预期触发的基本事件ID")
	flag.Parse()

	if err := run(*configPath, *alertID, *expectedTreeID, *expectedFaultCode, *expectedTopEvent, *expectedBasicEvent); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("PASS: fault diagnosis alert flow is working")
}

func run(configPath, alertID, expectedTreeID, expectedFaultCode, expectedTopEvent, expectedBasicEvent string) error {
	loader := config.NewLoader(configPath)
	faultTrees, err := loader.LoadFaultTrees()
	if err != nil {
		return fmt.Errorf("load fault trees: %w", err)
	}

	diagnosisEngine, err := engine.NewMultiDiagnosisEngine(faultTrees, zap.NewNop())
	if err != nil {
		return fmt.Errorf("create multi diagnosis engine: %w", err)
	}

	results := make(chan *models.DiagnosisResult, 8)
	diagnosisEngine.SetCallback(func(result *models.DiagnosisResult) {
		results <- result
	})

	firing := &models.AlertEvent{
		AlertID:   alertID,
		Status:    models.AlertStatusFiring,
		Source:    "diagnosis-alert-test",
		Message:   "diagnosis alert test firing",
		Timestamp: time.Now().Unix(),
	}
	diagnosisEngine.ProcessAlert(firing)

	triggered, err := nextDiagnosis(results)
	if err != nil {
		return fmt.Errorf("firing alert did not emit diagnosis: %w", err)
	}
	if err := assertDiagnosis(triggered, expectedTreeID, expectedFaultCode, expectedTopEvent, expectedBasicEvent); err != nil {
		return err
	}
	fmt.Printf("trigger diagnosis OK: tree=%s fault_code=%s top=%s basic=%v\n",
		triggered.FaultTreeID, triggered.FaultCode, triggered.TopEventID, triggered.BasicEvents)

	resolved := &models.AlertEvent{
		AlertID:   alertID,
		Status:    models.AlertStatusResolved,
		Source:    "diagnosis-alert-test",
		Message:   "diagnosis alert test resolved",
		Timestamp: time.Now().Unix(),
	}
	diagnosisEngine.ProcessAlert(resolved)

	recovered, err := nextDiagnosis(results)
	if err != nil {
		return fmt.Errorf("resolved alert did not emit diagnosis: %w", err)
	}
	if recovered.FaultTreeID != expectedTreeID {
		return fmt.Errorf("resolved FaultTreeID mismatch: got %q, want %q", recovered.FaultTreeID, expectedTreeID)
	}
	if recovered.Metadata["status"] != "RESOLVED" {
		return fmt.Errorf("resolved status mismatch: got %#v, want RESOLVED", recovered.Metadata["status"])
	}
	fmt.Printf("resolved diagnosis OK: tree=%s status=%v\n", recovered.FaultTreeID, recovered.Metadata["status"])

	diagnosisEngine.ProcessAlert(&models.AlertEvent{
		AlertID:   "UNKNOWN-ALERT-ID",
		Status:    models.AlertStatusFiring,
		Source:    "diagnosis-alert-test",
		Message:   "diagnosis alert test unmatched",
		Timestamp: time.Now().Unix(),
	})
	if result := tryDiagnosis(results); result != nil {
		return fmt.Errorf("unmatched alert emitted unexpected diagnosis: tree=%s fault_code=%s", result.FaultTreeID, result.FaultCode)
	}
	fmt.Println("unmatched alert OK: no diagnosis emitted")

	return nil
}

func nextDiagnosis(results <-chan *models.DiagnosisResult) (*models.DiagnosisResult, error) {
	select {
	case result := <-results:
		if result == nil {
			return nil, fmt.Errorf("nil diagnosis result")
		}
		return result, nil
	case <-time.After(2 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
}

func tryDiagnosis(results <-chan *models.DiagnosisResult) *models.DiagnosisResult {
	select {
	case result := <-results:
		return result
	default:
		return nil
	}
}

func assertDiagnosis(result *models.DiagnosisResult, expectedTreeID, expectedFaultCode, expectedTopEvent, expectedBasicEvent string) error {
	if result.FaultTreeID != expectedTreeID {
		return fmt.Errorf("FaultTreeID mismatch: got %q, want %q", result.FaultTreeID, expectedTreeID)
	}
	if result.FaultCode != expectedFaultCode {
		return fmt.Errorf("FaultCode mismatch: got %q, want %q", result.FaultCode, expectedFaultCode)
	}
	if !contains(result.TriggerPath, expectedTopEvent) {
		return fmt.Errorf("TriggerPath should contain %q, got %v", expectedTopEvent, result.TriggerPath)
	}
	if !contains(result.BasicEvents, expectedBasicEvent) {
		return fmt.Errorf("BasicEvents should contain %q, got %v", expectedBasicEvent, result.BasicEvents)
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
