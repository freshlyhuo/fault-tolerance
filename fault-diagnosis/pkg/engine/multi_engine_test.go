package engine

import (
	"sync"
	"testing"
	"time"

	"fault-diagnosis/pkg/models"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func makeTree(treeID, faultCode, topID, basicID, alertID string) *models.FaultTree {
	return &models.FaultTree{
		FaultTreeID: treeID,
		TopEvents: []models.Event{{
			EventID:   topID,
			Name:      "TOP-" + treeID,
			FaultCode: faultCode,
			GateType:  models.GateOR,
			Children:  []string{basicID},
		}},
		BasicEvents: []models.BasicEvent{{
			EventID: basicID,
			Name:    "BASIC-" + treeID,
			AlertID: alertID,
		}},
	}
}

func drain(ch <-chan *models.DiagnosisResult) int {
	return len(drainResults(ch))
}

func drainResults(ch <-chan *models.DiagnosisResult) []*models.DiagnosisResult {
	results := make([]*models.DiagnosisResult, 0)
	for {
		select {
		case d := <-ch:
			results = append(results, d)
		default:
			return results
		}
	}
}

func TestNewMultiDiagnosisEngine_InvalidInput(t *testing.T) {
	if _, err := NewMultiDiagnosisEngine(nil, zap.NewNop()); err == nil {
		t.Fatalf("expected error for empty input")
	}

	if _, err := NewMultiDiagnosisEngine([]*models.FaultTree{nil}, zap.NewNop()); err == nil {
		t.Fatalf("expected error for nil tree entry")
	}
}

func TestMultiDiagnosisEngine_FanoutForSharedAlert(t *testing.T) {
	t1 := makeTree("TREE-A", "FC-A", "TOP-A", "EVT-A", "ALERT-SHARED")
	t2 := makeTree("TREE-B", "FC-B", "TOP-B", "EVT-B", "ALERT-SHARED")

	eng, err := NewMultiDiagnosisEngine([]*models.FaultTree{t1, t2}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewMultiDiagnosisEngine failed: %v", err)
	}

	out := make(chan *models.DiagnosisResult, 8)
	eng.SetCallback(func(d *models.DiagnosisResult) { out <- d })

	eng.ProcessAlert(&models.AlertEvent{AlertID: "ALERT-SHARED", Status: models.AlertStatusFiring, Source: "unit"})

	results := drainResults(out)
	if len(results) != 2 {
		t.Fatalf("expected 2 diagnosis results, got %d", len(results))
	}

	codes := make(map[string]bool)
	for _, d := range results {
		codes[d.FaultCode] = true
	}

	if !codes["FC-A"] || !codes["FC-B"] {
		t.Fatalf("expected fault codes FC-A and FC-B, got %+v", codes)
	}
}

func TestMultiDiagnosisEngine_NoMatchShouldNotEmit(t *testing.T) {
	t1 := makeTree("TREE-A", "FC-A", "TOP-A", "EVT-A", "ALERT-A")

	eng, err := NewMultiDiagnosisEngine([]*models.FaultTree{t1}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewMultiDiagnosisEngine failed: %v", err)
	}

	out := make(chan *models.DiagnosisResult, 4)
	eng.SetCallback(func(d *models.DiagnosisResult) { out <- d })

	eng.ProcessAlert(&models.AlertEvent{AlertID: "ALERT-NOT-EXIST", Status: models.AlertStatusFiring, Source: "unit"})

	if got := drain(out); got != 0 {
		t.Fatalf("expected 0 diagnosis result, got %d", got)
	}
}

func TestMultiDiagnosisEngine_KeepSingleTreeDedupAndResolvedSemantics(t *testing.T) {
	t1 := makeTree("TREE-A", "FC-A", "TOP-A", "EVT-A", "ALERT-SHARED")
	t2 := makeTree("TREE-B", "FC-B", "TOP-B", "EVT-B", "ALERT-SHARED")

	eng, err := NewMultiDiagnosisEngine([]*models.FaultTree{t1, t2}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewMultiDiagnosisEngine failed: %v", err)
	}

	out := make(chan *models.DiagnosisResult, 16)
	eng.SetCallback(func(d *models.DiagnosisResult) { out <- d })

	eng.ProcessAlert(&models.AlertEvent{AlertID: "ALERT-SHARED", Status: models.AlertStatusFiring, Source: "unit"})
	if got := drain(out); got != 2 {
		t.Fatalf("first firing expected 2, got %d", got)
	}

	eng.ProcessAlert(&models.AlertEvent{AlertID: "ALERT-SHARED", Status: models.AlertStatusFiring, Source: "unit"})
	if got := drain(out); got != 0 {
		t.Fatalf("duplicate firing expected 0, got %d", got)
	}

	eng.ProcessAlert(&models.AlertEvent{AlertID: "ALERT-SHARED", Status: models.AlertStatusResolved, Source: "unit"})
	if got := drain(out); got != 2 {
		t.Fatalf("resolved expected 2, got %d", got)
	}
}

func TestMultiDiagnosisEngine_SingleTreeDuplicateAlertIDRoutesOnce(t *testing.T) {
	tree := &models.FaultTree{
		FaultTreeID: "TREE-DUP",
		TopEvents: []models.Event{{
			EventID:   "TOP-DUP",
			Name:      "TOP-DUP",
			FaultCode: "FC-DUP",
			GateType:  models.GateOR,
			Children:  []string{"EVT-DUP-A", "EVT-DUP-B"},
		}},
		BasicEvents: []models.BasicEvent{
			{EventID: "EVT-DUP-A", Name: "BASIC-A", AlertID: "ALERT-DUP"},
			{EventID: "EVT-DUP-B", Name: "BASIC-B", AlertID: "ALERT-DUP"},
		},
	}

	eng, err := NewMultiDiagnosisEngine([]*models.FaultTree{tree}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewMultiDiagnosisEngine failed: %v", err)
	}

	out := make(chan *models.DiagnosisResult, 8)
	eng.SetCallback(func(d *models.DiagnosisResult) { out <- d })

	eng.ProcessAlert(&models.AlertEvent{AlertID: "ALERT-DUP", Status: models.AlertStatusFiring, Source: "unit"})

	results := drainResults(out)
	if len(results) != 1 {
		t.Fatalf("expected 1 diagnosis result for duplicate alert routing, got %d", len(results))
	}

	if results[0].FaultCode != "FC-DUP" {
		t.Fatalf("expected fault code FC-DUP, got %s", results[0].FaultCode)
	}
}

func TestMultiDiagnosisEngine_PanicLogContainsFaultTreeID(t *testing.T) {
	core, observed := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	t1 := makeTree("TREE-A", "FC-A", "TOP-A", "EVT-A", "ALERT-A")
	eng, err := NewMultiDiagnosisEngine([]*models.FaultTree{t1}, logger)
	if err != nil {
		t.Fatalf("NewMultiDiagnosisEngine failed: %v", err)
	}

	eng.engines[0].stateManager = nil
	eng.ProcessAlert(&models.AlertEvent{AlertID: "ALERT-A", Status: models.AlertStatusFiring, Source: "unit"})

	entries := observed.FilterMessage("并行评估异常").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 panic log entry, got %d", len(entries))
	}

	ctx := entries[0].ContextMap()
	if got, ok := ctx["fault_tree_id"]; !ok || got != "TREE-A" {
		t.Fatalf("expected panic log with fault_tree_id=TREE-A, got %+v", ctx)
	}
}

func TestMultiDiagnosisEngine_ConcurrentProcessAlertSmoke(t *testing.T) {
	t1 := makeTree("TREE-A", "FC-A", "TOP-A", "EVT-A", "ALERT-A")
	t2 := makeTree("TREE-B", "FC-B", "TOP-B", "EVT-B", "ALERT-B")

	eng, err := NewMultiDiagnosisEngine([]*models.FaultTree{t1, t2}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewMultiDiagnosisEngine failed: %v", err)
	}

	out := make(chan *models.DiagnosisResult, 1024)
	eng.SetCallback(func(d *models.DiagnosisResult) { out <- d })

	const workers = 8
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				alertID := "ALERT-A"
				if (worker+j)%2 == 0 {
					alertID = "ALERT-B"
				}

				status := models.AlertStatusFiring
				if j%7 == 0 {
					status = models.AlertStatusResolved
				}

				eng.ProcessAlert(&models.AlertEvent{AlertID: alertID, Status: status, Source: "smoke"})
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("concurrent ProcessAlert appears blocked")
	}

	_ = drain(out)
}
