package recovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type StateChecker interface {
	CheckBoolean(ctx context.Context, metricName string) (bool, bool, error)
}

type RecoveryServiceConfig struct {
	QueueSize    int
	OnPlanStart  func(RecoveryPlanEvent)
	OnPlanFinish func(RecoveryPlanEvent)
}

type RecoveryService struct {
	registry *PlanRegistry
	client   ContainerClient
	checker  StateChecker
	faults   *faultStatusTracker
	sm       StateManager

	queue chan *recoveryTask

	mu     sync.Mutex
	active map[string]*recoveryTask
	queued map[string]*recoveryTask

	onPlanStart  func(RecoveryPlanEvent)
	onPlanFinish func(RecoveryPlanEvent)
}

var errRecoveryPlanComplete = errors.New("recovery plan complete")

type RecoveryPlanEvent struct {
	TraceID     string
	PlanID      string
	TargetID    string
	FaultCode   string
	Status      string
	Error       string
	TimestampMs int64
}

type recoveryTask struct {
	event  NormalizedEvent
	plan   RecoveryPlan
	key    string
	ctx    context.Context
	cancel context.CancelFunc
}

func NewRecoveryService(registry *PlanRegistry, client ContainerClient, checker StateChecker, sm StateManager, cfg RecoveryServiceConfig) *RecoveryService {
	qsize := cfg.QueueSize
	if qsize <= 0 {
		qsize = 100
	}
	if sm == nil {
		sm = NewInMemoryStateManager()
	}
	return &RecoveryService{
		registry: registry,
		client:   client,
		checker:  checker,
		faults:   newFaultStatusTracker(),
		sm:       sm,
		queue:    make(chan *recoveryTask, qsize),
		active:   make(map[string]*recoveryTask),
		queued:   make(map[string]*recoveryTask),

		onPlanStart:  cfg.OnPlanStart,
		onPlanFinish: cfg.OnPlanFinish,
	}
}

func NewDefaultRecoveryService() (*RecoveryService, error) {
	registry, err := LoadPlanRegistry("")
	if err != nil {
		return nil, err
	}
	return NewRecoveryService(registry, NewVSOAContainerClientFromEnv(), nil, NewInMemoryStateManager(), RecoveryServiceConfig{}), nil
}

func (s *RecoveryService) ReceiveConfig(queueSize int) ReceiveConfig {
	return ReceiveConfig{
		QueueSize: queueSize,
		OnNormalized: func(ev NormalizedEvent) {
			if err := s.SubmitNormalized(ev); err != nil {
				log.Printf("[recovery][service] submit normalized event failed: %v", err)
			}
		},
	}
}

func (s *RecoveryService) Start(ctx context.Context) {
	if s == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				s.cancelAll("service context canceled")
				return
			case task := <-s.queue:
				s.runTask(task)
			}
		}
	}()
}

func (s *RecoveryService) SubmitNormalized(ev NormalizedEvent) error {
	if s == nil {
		return fmt.Errorf("recovery service is nil")
	}
	s.faults.Observe(ev)
	if ev.Status == EventStatusResolved {
		s.cancelMatching(ev)
		return nil
	}
	if ev.Status != EventStatusFiring {
		return fmt.Errorf("unsupported event status: %s", ev.Status)
	}

	planIDs := recoveryPlanIDsFromMetadata(ev.Metadata)
	if len(planIDs) == 0 {
		return s.report(ev, "", ResultNoAction, "NO_ACTION", "diagnosis has no recovery_plan_id")
	}

	for _, planID := range planIDs {
		plan, ok := s.registry.Get(planID)
		if !ok {
			_ = s.report(ev, planID, ResultNoAction, "PLAN_NOT_FOUND", "recovery plan not found")
			continue
		}
		if len(plan.InstructionIDs) == 0 {
			if plan.IsLogExecutor() {
				log.Printf("[recovery][log] trace=%s plan=%s target=%s fault=%s description=%s", ev.TraceID, plan.PlanID, ev.TargetID, ev.FaultCode, plan.Description)
				_ = s.report(ev, planID, ResultSuccess, "LOG_ONLY", "recovery plan handled by log executor")
				continue
			}
			_ = s.report(ev, planID, ResultNoAction, "NO_ACTION", "recovery plan has no instruction_id")
			continue
		}
		if err := s.enqueue(ev, plan); err != nil {
			_ = s.report(ev, planID, ResultRejected, "QUEUE_FULL", err.Error())
		}
	}
	return nil
}

func (s *RecoveryService) enqueue(ev NormalizedEvent, plan RecoveryPlan) error {
	taskCtx, cancel := context.WithCancel(context.Background())
	task := &recoveryTask{
		event:  ev,
		plan:   plan,
		key:    taskKey(ev, plan.PlanID),
		ctx:    taskCtx,
		cancel: cancel,
	}

	s.mu.Lock()
	if _, ok := s.active[task.key]; ok {
		s.mu.Unlock()
		cancel()
		return fmt.Errorf("recovery task already running: %s", task.key)
	}
	if _, ok := s.queued[task.key]; ok {
		s.mu.Unlock()
		cancel()
		return fmt.Errorf("recovery task already queued: %s", task.key)
	}
	s.queued[task.key] = task
	s.mu.Unlock()

	select {
	case s.queue <- task:
		return nil
	default:
		s.mu.Lock()
		delete(s.queued, task.key)
		s.mu.Unlock()
		cancel()
		return fmt.Errorf("recovery queue full")
	}
}

func (s *RecoveryService) runTask(task *recoveryTask) {
	if task == nil {
		return
	}

	s.mu.Lock()
	if _, ok := s.queued[task.key]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.queued, task.key)
	s.active[task.key] = task
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.active, task.key)
		s.mu.Unlock()
		task.cancel()
	}()

	if err := task.ctx.Err(); err != nil {
		_ = s.report(task.event, task.plan.PlanID, ResultCanceled, "CANCELED", err.Error())
		return
	}

	started := nowUnix()
	status := ResultSuccess
	message := "SUCCESS"
	errText := ""
	s.notifyPlanStart(task.event, task.plan.PlanID)

	ctx := task.ctx
	if timeout := task.plan.Timeout(); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(task.ctx, timeout)
		defer cancel()
	}

	if err := s.executePlan(ctx, task.event, task.plan); err != nil {
		if ctx.Err() != nil || task.ctx.Err() != nil {
			status = ResultCanceled
			message = "CANCELED"
			errText = err.Error()
		} else {
			status = ResultFailed
			message = "FAILED"
			errText = err.Error()
		}
	}

	result := RecoveryResult{
		TargetID:   task.event.TargetID,
		FaultCode:  task.event.FaultCode,
		Action:     task.plan.PlanID,
		Status:     status,
		Message:    message,
		StartedAt:  started,
		FinishedAt: nowUnix(),
		Error:      errText,
	}
	_ = s.sm.ReportResult(result)
	s.notifyPlanFinish(task.event, task.plan.PlanID, status, errText)
	if status == ResultFailed {
		log.Printf("[recovery][plan] plan=%s trace=%s target=%s status=RETRY_EXHAUSTED err=%s", task.plan.PlanID, task.event.TraceID, task.event.TargetID, errText)
	}
}

func (s *RecoveryService) executePlan(ctx context.Context, ev NormalizedEvent, plan RecoveryPlan) error {
	for idx, instructionID := range plan.InstructionIDs {
		instructionID = strings.TrimSpace(instructionID)
		if instructionID == "" {
			continue
		}
		if err := s.executeWithRetry(ctx, ev, plan, idx, instructionID); err != nil {
			if errors.Is(err, errRecoveryPlanComplete) {
				log.Printf("[recovery][plan] plan=%s trace=%s target=%s status=RECHECK_CLEAR", plan.PlanID, ev.TraceID, ev.TargetID)
				return nil
			}
			return err
		}
	}
	return nil
}

func (s *RecoveryService) executeWithRetry(ctx context.Context, ev NormalizedEvent, plan RecoveryPlan, idx int, instructionID string) error {
	var lastErr error
	for attempt := 0; attempt <= plan.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := plan.Backoff(attempt)
			log.Printf("[recovery][retry] plan=%s instruction=%s attempt=%d delay=%s err=%v", plan.PlanID, instructionID, attempt, delay, lastErr)
			if err := sleepContext(ctx, delay); err != nil {
				return err
			}
		}

		if strings.HasPrefix(instructionID, "CTRL_") {
			lastErr = s.executeControl(ctx, ev, instructionID)
		} else {
			lastErr = s.dispatchInstruction(ctx, ev, plan, idx, instructionID)
		}
		if errors.Is(lastErr, errRecoveryPlanComplete) {
			return lastErr
		}
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("instruction %s failed after %d retries: %w", instructionID, plan.MaxRetries, lastErr)
}

func (s *RecoveryService) dispatchInstruction(ctx context.Context, ev NormalizedEvent, plan RecoveryPlan, idx int, instructionID string) error {
	if s.client == nil {
		return fmt.Errorf("repair container client is nil")
	}
	log.Printf("[recovery][dispatch] trace=%s plan=%s target=%s step=%d instruction=%s", ev.TraceID, plan.PlanID, ev.TargetID, idx, instructionID)
	_, err := s.client.Dispatch(ctx, instructionID, map[string]float64{})
	return err
}

func (s *RecoveryService) executeControl(ctx context.Context, ev NormalizedEvent, instructionID string) error {
	if strings.HasPrefix(instructionID, "CTRL_WAIT_") && strings.HasSuffix(instructionID, "ms") {
		raw := strings.TrimSuffix(strings.TrimPrefix(instructionID, "CTRL_WAIT_"), "ms")
		var ms int
		if _, err := fmt.Sscanf(raw, "%d", &ms); err != nil || ms < 0 {
			return fmt.Errorf("invalid wait control instruction: %s", instructionID)
		}
		return sleepContext(ctx, time.Duration(ms)*time.Millisecond)
	}

	if strings.HasPrefix(instructionID, "CTRL_RECHECK_") {
		if instructionID == "CTRL_RECHECK_error" {
			active := s.faults.IsActive(ev)
			log.Printf("[recovery][control] recheck fault trace=%s tree=%s top=%s fault=%s target=%s active=%v",
				ev.TraceID, ev.FaultTreeID, ev.TopEventID, ev.FaultCode, ev.TargetID, active)
			if !active {
				return errRecoveryPlanComplete
			}
			return nil
		}
		parts := strings.Split(strings.TrimPrefix(instructionID, "CTRL_RECHECK_"), "_")
		if len(parts) < 2 {
			return fmt.Errorf("legacy recheck control instruction requires migration: %s", instructionID)
		}
		expectedText := parts[len(parts)-1]
		metricName := strings.Join(parts[:len(parts)-1], "_")
		expected, ok := parseExpectedSwitch(expectedText)
		if !ok {
			return fmt.Errorf("invalid recheck expected value: %s", instructionID)
		}
		if s.checker == nil {
			return fmt.Errorf("state checker is nil for %s", instructionID)
		}
		actual, exists, err := s.checker.CheckBoolean(ctx, metricName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("state metric not found: %s", metricName)
		}
		if actual != expected {
			return fmt.Errorf("state metric %s mismatch: got=%v want=%v", metricName, actual, expected)
		}
		return nil
	}

	return fmt.Errorf("unsupported control instruction: %s", instructionID)
}

func (s *RecoveryService) cancelMatching(ev NormalizedEvent) {
	planIDs := recoveryPlanIDsFromMetadata(ev.Metadata)
	planSet := make(map[string]bool, len(planIDs))
	for _, planID := range planIDs {
		planSet[planID] = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for key, task := range s.queued {
		if task.matchesResolved(ev, planSet) {
			delete(s.queued, key)
			task.cancel()
			if task.plan.HasLegacyErrorRecheck() {
				_ = s.reportLocked(task.event, task.plan.PlanID, ResultSuccess, "RECHECK_CLEAR", "fault resolved before recovery plan started")
			} else {
				_ = s.reportLocked(task.event, task.plan.PlanID, ResultCanceled, "CANCELED", "canceled by resolved diagnosis")
			}
		}
	}
	for _, task := range s.active {
		if task.matchesResolved(ev, planSet) {
			if task.plan.HasLegacyErrorRecheck() {
				log.Printf("[recovery][control] plan=%s trace=%s keeps running until CTRL_RECHECK_error", task.plan.PlanID, task.event.TraceID)
				continue
			}
			task.cancel()
		}
	}
}

func (s *RecoveryService) cancelAll(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, task := range s.queued {
		delete(s.queued, key)
		task.cancel()
		_ = s.reportLocked(task.event, task.plan.PlanID, ResultCanceled, "CANCELED", reason)
	}
	for _, task := range s.active {
		task.cancel()
	}
}

func (t *recoveryTask) matchesResolved(ev NormalizedEvent, planSet map[string]bool) bool {
	if t == nil {
		return false
	}
	if ev.TargetID != "" && ev.TargetID != t.event.TargetID {
		return false
	}
	if ev.FaultTreeID != "" && t.event.FaultTreeID != "" && ev.FaultTreeID != t.event.FaultTreeID {
		return false
	}
	if ev.FaultCode != "" && ev.FaultCode != t.event.FaultCode {
		return false
	}
	if len(planSet) > 0 && !planSet[t.plan.PlanID] {
		return false
	}
	return true
}

func (s *RecoveryService) report(ev NormalizedEvent, action, status, message, errText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reportLocked(ev, action, status, message, errText)
}

func (s *RecoveryService) reportLocked(ev NormalizedEvent, action, status, message, errText string) error {
	if action != "" {
		s.notifyPlanStart(ev, action)
	}
	result := RecoveryResult{
		TargetID:   ev.TargetID,
		FaultCode:  ev.FaultCode,
		Action:     action,
		Status:     status,
		Message:    message,
		StartedAt:  nowUnix(),
		FinishedAt: nowUnix(),
		Error:      errText,
	}
	err := s.sm.ReportResult(result)
	s.notifyPlanFinish(ev, action, status, errText)
	return err
}

func (s *RecoveryService) notifyPlanStart(ev NormalizedEvent, planID string) {
	if s == nil || s.onPlanStart == nil {
		return
	}
	s.onPlanStart(RecoveryPlanEvent{
		TraceID:     ev.TraceID,
		PlanID:      planID,
		TargetID:    ev.TargetID,
		FaultCode:   ev.FaultCode,
		TimestampMs: nowUnixMilli(),
	})
}

func (s *RecoveryService) notifyPlanFinish(ev NormalizedEvent, planID, status, errText string) {
	if s == nil || s.onPlanFinish == nil {
		return
	}
	s.onPlanFinish(RecoveryPlanEvent{
		TraceID:     ev.TraceID,
		PlanID:      planID,
		TargetID:    ev.TargetID,
		FaultCode:   ev.FaultCode,
		Status:      status,
		Error:       errText,
		TimestampMs: nowUnixMilli(),
	})
}

func taskKey(ev NormalizedEvent, planID string) string {
	return strings.Join([]string{ev.TraceID, ev.TargetID, ev.FaultTreeID, planID}, "|")
}

func recoveryPlanIDsFromMetadata(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}
	out := make([]string, 0)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}

	switch v := metadata["recovery_plan_ids"].(type) {
	case []string:
		for _, item := range v {
			add(item)
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	case string:
		for _, item := range strings.Split(v, ",") {
			add(item)
		}
	}
	if len(out) == 0 {
		if v, ok := metadata["primary_recovery_plan_id"].(string); ok {
			add(v)
		}
	}

	seen := make(map[string]bool, len(out))
	deduped := out[:0]
	for _, item := range out {
		if seen[item] {
			continue
		}
		seen[item] = true
		deduped = append(deduped, item)
	}
	return deduped
}

func parseExpectedSwitch(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "true", "1":
		return true, true
	case "off", "false", "0":
		return false, true
	default:
		return false, false
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
