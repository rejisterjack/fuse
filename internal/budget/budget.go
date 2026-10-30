package budget

import (
	"sync"
	"time"
)

// Scope identifies whose spend is being tracked.
type Scope struct {
	Agent string
	User  string
	Task  string
}

// Decision is the outcome of a budget check.
type Decision struct {
	Allowed   bool
	SoftHit   bool
	HardHit   bool
	SpendUSD  float64
	LimitUSD  float64
	Message   string
}

// Tracker keeps in-memory spend counters. Persistent store can replace this later.
type Tracker struct {
	mu       sync.Mutex
	spend    map[string]float64
	softUSD  float64
	hardUSD  float64
	window   time.Duration
	resetAt  map[string]time.Time
}

// NewTracker creates a monthly-window spend tracker with soft/hard USD limits.
func NewTracker(softUSD, hardUSD float64) *Tracker {
	return &Tracker{
		spend:   make(map[string]float64),
		softUSD: softUSD,
		hardUSD: hardUSD,
		window:  30 * 24 * time.Hour,
		resetAt: make(map[string]time.Time),
	}
}

func key(s Scope) string {
	return s.Agent + "\x00" + s.User + "\x00" + s.Task
}

// Check returns whether the next call is allowed under current spend.
func (t *Tracker) Check(s Scope) Decision {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(s)
	t.maybeReset(k)
	spend := t.spend[k]
	d := Decision{Allowed: true, SpendUSD: spend, LimitUSD: t.hardUSD}
	if spend >= t.hardUSD {
		d.Allowed = false
		d.HardHit = true
		d.Message = "hard budget limit exceeded"
		return d
	}
	if spend >= t.softUSD {
		d.SoftHit = true
		d.Message = "soft budget limit reached"
	}
	return d
}

// Record adds estimated USD cost for a completed call.
func (t *Tracker) Record(s Scope, usd float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(s)
	t.maybeReset(k)
	t.spend[k] += usd
}

// Snapshot returns current spend keyed by scope string for the dashboard.
func (t *Tracker) Snapshot() map[string]float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]float64, len(t.spend))
	for k, v := range t.spend {
		out[k] = v
	}
	return out
}

func (t *Tracker) maybeReset(k string) {
	now := time.Now()
	if reset, ok := t.resetAt[k]; !ok || now.After(reset) {
		t.spend[k] = 0
		t.resetAt[k] = now.Add(t.window)
	}
}
