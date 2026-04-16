package recovery

import (
	"errors"
	"fmt"
	"time"
)

func NormalizeDiagnosisEvent(in DiagnosisResult) (NormalizedEvent, error) {
	if in.FaultCode == "" {
		return NormalizedEvent{}, errors.New("fault_code is required")
	}

	targetID := DiagnosisTargetID(in)
	if targetID == "" {
		return NormalizedEvent{}, errors.New("target_id is required")
	}

	status := DiagnosisStatus(in)
	if status != EventStatusFiring && status != EventStatusResolved {
		return NormalizedEvent{}, fmt.Errorf("invalid status: %s", status)
	}

	ts := in.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	traceID := in.DiagnosisID
	if traceID == "" {
		traceID = fmt.Sprintf("trace-%d", ts.UnixNano())
	}

	return NormalizedEvent{
		TraceID:       traceID,
		FaultCode:     in.FaultCode,
		TargetID:      targetID,
		Status:        status,
		DiagnosisTime: ts.Unix(),
		Metadata:      in.Metadata,
	}, nil
}
