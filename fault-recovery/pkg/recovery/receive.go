package recovery

import (
	"context"
	"errors"
	"log"
)

type ReceiveConfig struct {
	QueueSize    int
	OnNormalized func(NormalizedEvent)
}

type ReceiveService struct {
	queue        chan DiagnosisResult
	onNormalized func(NormalizedEvent)
}

func NewReceiveService(cfg ReceiveConfig) *ReceiveService {
	qsize := cfg.QueueSize
	if qsize <= 0 {
		qsize = 100
	}

	return &ReceiveService{
		queue:        make(chan DiagnosisResult, qsize),
		onNormalized: cfg.OnNormalized,
	}
}

func (s *ReceiveService) Start(ctx context.Context) {
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
				return
			case in := <-s.queue:
				s.process(in)
			}
		}
	}()
}

func (s *ReceiveService) Submit(in DiagnosisResult) error {
	if s == nil {
		return errors.New("receive service is nil")
	}

	select {
	case s.queue <- in:
		return nil
	default:
		return errors.New("receive queue full")
	}
}

func (s *ReceiveService) process(in DiagnosisResult) {
	normalized, err := NormalizeDiagnosisEvent(in)
	if err != nil {
		log.Printf("[recovery][receive] drop invalid diagnosis event: %v", err)
		return
	}

	if s.onNormalized != nil {
		s.onNormalized(normalized)
		return
	}

	log.Printf("[recovery][receive] accepted trace=%s fault=%s target=%s status=%s", normalized.TraceID, normalized.FaultCode, normalized.TargetID, normalized.Status)
}
