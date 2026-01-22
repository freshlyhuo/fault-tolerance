module fault-tolerance

go 1.25.0

require (
	fault-diagnosis v0.0.0
	go.uber.org/zap v1.27.1
	health-monitor v0.0.0
)

require (
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
)

replace (
	fault-diagnosis => ./fault-diagnosis
	health-monitor => ./health-monitor
)
