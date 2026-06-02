package recovery

import (
	"strings"
	"sync"
)

type faultStatusTracker struct {
	mu     sync.RWMutex
	active map[string]bool
}

func newFaultStatusTracker() *faultStatusTracker {
	return &faultStatusTracker{active: make(map[string]bool)}
}

func (t *faultStatusTracker) Observe(ev NormalizedEvent) {
	if t == nil {
		return
	}
	key := faultStatusKey(ev)
	if key == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	switch ev.Status {
	case EventStatusFiring:
		t.active[key] = true
	case EventStatusResolved:
		t.active[key] = false
	}
}

func (t *faultStatusTracker) IsActive(ev NormalizedEvent) bool {
	if t == nil {
		return ev.Status == EventStatusFiring
	}
	key := faultStatusKey(ev)
	if key == "" {
		return ev.Status == EventStatusFiring
	}

	t.mu.RLock()
	active, exists := t.active[key]
	t.mu.RUnlock()
	if !exists {
		return ev.Status == EventStatusFiring
	}
	return active
}

func faultStatusKey(ev NormalizedEvent) string {
	parts := []string{
		strings.TrimSpace(ev.FaultTreeID),
		strings.TrimSpace(ev.TopEventID),
		strings.TrimSpace(ev.FaultCode),
		strings.TrimSpace(ev.TargetID),
	}
	for _, part := range parts {
		if part != "" {
			return strings.Join(parts, "\x1f")
		}
	}
	return ""
}
