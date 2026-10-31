package budget

import (
	"context"
	"sync"
	"time"

	"github.com/rejisterjack/fuse/internal/store"
)

// Scope identifies whose spend is being tracked.
type Scope struct {
	Agent string
	User  string
	Task  string
}

// Decision is the outcome of a budget check.
type Decision struct {
	Allowed  bool
	SoftHit  bool
	HardHit  bool
	SpendUSD float64
	LimitUSD float64
	Requests int
	Message  string
}

// UsageDetail is optional metadata when recording spend.
type UsageDetail struct {
	Model            string
	PromptTokens     int
	CompletionTokens int
}

// Tracker keeps spend counters, optionally backed by Postgres.
type Tracker struct {
	mu      sync.Mutex
	spend   map[string]float64
	reqs    map[string]int
	softUSD float64
	hardUSD float64
	window  time.Duration
	resetAt map[string]time.Time
	store   *store.Store
}

// NewTracker creates a monthly-window spend tracker with soft/hard USD limits.
func NewTracker(softUSD, hardUSD float64) *Tracker {
	return &Tracker{
		spend:   make(map[string]float64),
		reqs:    make(map[string]int),
		softUSD: softUSD,
		hardUSD: hardUSD,
		window:  30 * 24 * time.Hour,
		resetAt: make(map[string]time.Time),
	}
}

// SetStore attaches an optional Postgres store for durable spend/rules.
func (t *Tracker) SetStore(s *store.Store) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store = s
}

func key(s Scope) string {
	return s.Agent + "\x00" + s.User + "\x00" + s.Task
}

func (t *Tracker) since() time.Time {
	return time.Now().Add(-t.window)
}

// Check returns whether the next call is allowed under current spend and rules.
func (t *Tracker) Check(s Scope) Decision {
	t.mu.Lock()
	defer t.mu.Unlock()

	softUSD, hardUSD := t.softUSD, t.hardUSD
	var softReq, hardReq *int
	reqWindow := 24 * time.Hour
	var rules []store.BudgetRule

	if t.store != nil {
		if r, err := t.store.RulesForScope(context.Background(), s.Agent, s.User, s.Task); err == nil {
			rules = r
		}
		for _, r := range rules {
			if r.HardUSD != nil && *r.HardUSD < hardUSD {
				hardUSD = *r.HardUSD
			}
			if r.SoftUSD != nil && *r.SoftUSD < softUSD {
				softUSD = *r.SoftUSD
			}
			if r.HardRequests != nil {
				if hardReq == nil || *r.HardRequests < *hardReq {
					v := *r.HardRequests
					hardReq = &v
				}
			}
			if r.SoftRequests != nil {
				if softReq == nil || *r.SoftRequests < *softReq {
					v := *r.SoftRequests
					softReq = &v
				}
			}
			if r.WindowSeconds != nil && *r.WindowSeconds > 0 {
				reqWindow = time.Duration(*r.WindowSeconds) * time.Second
			}
		}
	}

	compSpend := t.compositeSpendLocked(s)
	reqs := t.compositeReqsLocked(s, reqWindow)

	d := Decision{Allowed: true, SpendUSD: compSpend, LimitUSD: hardUSD, Requests: reqs}

	// Per-dimension hard USD: any matching rule exceeded → block.
	for _, r := range rules {
		if r.HardUSD == nil {
			continue
		}
		spend := t.dimSpendLocked(r.ScopeType, r.ScopeKey)
		if spend >= *r.HardUSD {
			d.Allowed = false
			d.HardHit = true
			d.SpendUSD = spend
			d.LimitUSD = *r.HardUSD
			d.Message = "hard budget limit exceeded"
			return d
		}
	}

	// Request hard limit on composite scope.
	if hardReq != nil && reqs >= *hardReq {
		d.Allowed = false
		d.HardHit = true
		d.Message = "hard request limit exceeded"
		d.LimitUSD = float64(*hardReq)
		return d
	}

	// Default hard against composite.
	if hardUSD > 0 && compSpend >= hardUSD {
		d.Allowed = false
		d.HardHit = true
		d.Message = "hard budget limit exceeded"
		return d
	}

	for _, r := range rules {
		if r.SoftUSD == nil {
			continue
		}
		spend := t.dimSpendLocked(r.ScopeType, r.ScopeKey)
		if spend >= *r.SoftUSD {
			d.SoftHit = true
			d.SpendUSD = spend
			d.Message = "soft budget limit reached"
		}
	}
	if softReq != nil && reqs >= *softReq {
		d.SoftHit = true
		if d.Message == "" {
			d.Message = "soft request limit reached"
		}
	}
	if softUSD > 0 && compSpend >= softUSD {
		d.SoftHit = true
		if d.Message == "" {
			d.Message = "soft budget limit reached"
		}
	}
	return d
}

func (t *Tracker) compositeSpendLocked(s Scope) float64 {
	if t.store != nil {
		v, err := t.store.SumSpend(context.Background(), s.Agent, s.User, s.Task, t.since())
		if err == nil {
			return v
		}
	}
	k := key(s)
	t.maybeReset(k)
	return t.spend[k]
}

func (t *Tracker) compositeReqsLocked(s Scope, window time.Duration) int {
	if t.store != nil {
		n, err := t.store.CountRequests(context.Background(), s.Agent, s.User, s.Task, time.Now().Add(-window))
		if err == nil {
			return n
		}
	}
	k := key(s)
	t.maybeReset(k)
	return t.reqs[k]
}

func (t *Tracker) dimSpendLocked(scopeType, scopeKey string) float64 {
	if t.store != nil {
		v, err := t.store.SumSpendByDim(context.Background(), scopeType, scopeKey, t.since())
		if err == nil {
			return v
		}
	}
	return 0
}

// Record adds estimated USD cost for a completed call.
func (t *Tracker) Record(s Scope, usd float64) {
	t.RecordDetail(s, usd, UsageDetail{})
}

// RecordDetail records spend and optionally persists a usage event.
func (t *Tracker) RecordDetail(s Scope, usd float64, d UsageDetail) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(s)
	t.maybeReset(k)
	t.spend[k] += usd
	if t.store != nil && usd > 0 {
		_ = t.store.InsertUsage(context.Background(), store.UsageEvent{
			Agent: s.Agent, User: s.User, Task: s.Task,
			Model: d.Model, PromptTokens: d.PromptTokens, CompletionTokens: d.CompletionTokens,
			CostUSD: usd,
		})
	}
}

// RecordRequest increments the request counter for an allowed forward.
func (t *Tracker) RecordRequest(s Scope) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(s)
	t.maybeReset(k)
	t.reqs[k]++
	if t.store != nil {
		_ = t.store.InsertRequest(context.Background(), s.Agent, s.User, s.Task)
	}
}

// Snapshot returns current spend keyed by scope string for the dashboard.
func (t *Tracker) Snapshot() map[string]float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.store != nil {
		m, err := t.store.SnapshotSpend(context.Background(), t.since())
		if err == nil {
			return m
		}
	}
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
		t.reqs[k] = 0
		t.resetAt[k] = now.Add(t.window)
	}
}
