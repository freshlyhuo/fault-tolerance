package recovery

// NormalizedEvent is the minimal normalized diagnosis payload used by recovery mapping.
type NormalizedEvent struct {
	TraceID       string
	FaultCode     string
	TargetID      string
	Status        string
	DiagnosisTime int64
	Metadata      map[string]interface{}
}
