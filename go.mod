module fault-tolerance

go 1.24.5

require (
	fault-diagnosis v0.0.0
	health-monitor v0.0.0
	go.uber.org/zap v1.27.1
)

require go.uber.org/multierr v1.10.0 // indirect

replace (
	fault-diagnosis => ./fault-diagnosis
	health-monitor => ./health-monitor
)
