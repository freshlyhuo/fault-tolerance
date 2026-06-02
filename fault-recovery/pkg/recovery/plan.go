package recovery

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type RetryBackoff struct {
	InitialMS int `json:"initial_ms"`
	MaxMS     int `json:"max_ms"`
}

type RecoveryPlan struct {
	PlanID         string       `json:"plan_id"`
	Description    string       `json:"description"`
	InstructionIDs []string     `json:"instruction_id"`
	Domain         string       `json:"domain"`
	Executor       string       `json:"executor"`
	TimeoutMS      int          `json:"timeout_ms"`
	MaxRetries     int          `json:"max_retries"`
	RetryBackoff   RetryBackoff `json:"retry_backoff"`
}

type PlanRegistry struct {
	plans map[string]RecoveryPlan
}

type planConfigFile struct {
	Version string                  `json:"version"`
	Meta    map[string]interface{}  `json:"_meta"`
	Plans   map[string]RecoveryPlan `json:"plans"`
}

func LoadPlanRegistry(path string) (*PlanRegistry, error) {
	path = ResolvePlanConfigPath(path)

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read recovery plan config failed: %w", err)
	}

	var cfg planConfigFile
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse recovery plan config failed: %w", err)
	}
	if len(cfg.Plans) == 0 {
		return nil, fmt.Errorf("recovery plan config has no plans")
	}

	plans := make(map[string]RecoveryPlan, len(cfg.Plans))
	for id, plan := range cfg.Plans {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("recovery plan id is empty")
		}
		plan.PlanID = id
		plan.Description = strings.TrimSpace(plan.Description)
		plan.Executor = strings.TrimSpace(plan.Executor)
		if plan.Executor == "" {
			plan.Executor = "repair_container"
		}
		if plan.TimeoutMS <= 0 {
			plan.TimeoutMS = 10000
		}
		if plan.MaxRetries < 0 {
			plan.MaxRetries = 0
		}
		if plan.RetryBackoff.InitialMS <= 0 {
			plan.RetryBackoff.InitialMS = 1000
		}
		if plan.RetryBackoff.MaxMS <= 0 {
			plan.RetryBackoff.MaxMS = plan.RetryBackoff.InitialMS
		}
		if plan.RetryBackoff.MaxMS < plan.RetryBackoff.InitialMS {
			plan.RetryBackoff.MaxMS = plan.RetryBackoff.InitialMS
		}
		plans[id] = plan
	}

	return &PlanRegistry{plans: plans}, nil
}

func ResolvePlanConfigPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return defaultPlanConfigPath()
	}
	return path
}

func NewPlanRegistry(plans map[string]RecoveryPlan) *PlanRegistry {
	copied := make(map[string]RecoveryPlan, len(plans))
	for id, plan := range plans {
		if plan.PlanID == "" {
			plan.PlanID = id
		}
		copied[id] = plan
	}
	return &PlanRegistry{plans: copied}
}

func (r *PlanRegistry) Get(planID string) (RecoveryPlan, bool) {
	if r == nil {
		return RecoveryPlan{}, false
	}
	plan, ok := r.plans[strings.TrimSpace(planID)]
	return plan, ok
}

func (p RecoveryPlan) Timeout() time.Duration {
	return time.Duration(p.TimeoutMS) * time.Millisecond
}

func (p RecoveryPlan) IsLogExecutor() bool {
	return strings.EqualFold(strings.TrimSpace(p.Executor), "log")
}

func (p RecoveryPlan) HasLegacyErrorRecheck() bool {
	for _, instructionID := range p.InstructionIDs {
		if strings.TrimSpace(instructionID) == "CTRL_RECHECK_error" {
			return true
		}
	}
	return false
}

func (p RecoveryPlan) Backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	initial := p.RetryBackoff.InitialMS
	if initial <= 0 {
		initial = 1000
	}
	maxMS := p.RetryBackoff.MaxMS
	if maxMS <= 0 {
		maxMS = initial
	}
	delay := initial
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= maxMS {
			delay = maxMS
			break
		}
	}
	return time.Duration(delay) * time.Millisecond
}

func defaultPlanConfigPath() string {
	if p := strings.TrimSpace(os.Getenv("FR_RECOVERY_PLAN_CONFIG")); p != "" {
		return p
	}
	return "./fault-recovery/configs/recovery_plan_mapping_template.json"
}
