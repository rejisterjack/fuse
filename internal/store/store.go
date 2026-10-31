package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the Postgres handle for usage records and budget rules.
type Store struct {
	Pool *pgxpool.Pool
}

// BudgetRule is a per-scope soft/hard limit (USD and/or request count).
type BudgetRule struct {
	ID            int64    `json:"id"`
	ScopeType     string   `json:"scope_type"`
	ScopeKey      string   `json:"scope_key"`
	SoftUSD       *float64 `json:"soft_usd"`
	HardUSD       *float64 `json:"hard_usd"`
	SoftRequests  *int     `json:"soft_requests"`
	HardRequests  *int     `json:"hard_requests"`
	WindowSeconds *int     `json:"window_seconds"`
}

// UsageEvent is a single recorded API call cost.
type UsageEvent struct {
	Agent            string
	User             string
	Task             string
	Model            string
	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
}

// ProxyKey is a Fuse-issued scoped API key (hash at rest).
type ProxyKey struct {
	ID        int64      `json:"id"`
	Prefix    string     `json:"prefix"`
	Agent     string     `json:"agent"`
	UserLabel string     `json:"user_label"`
	Label     string     `json:"label"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// SeriesPoint is hourly spend for charts.
type SeriesPoint struct {
	Bucket time.Time `json:"bucket"`
	USD    float64   `json:"usd"`
}

// NamedSpend is a rollup row.
type NamedSpend struct {
	Name string  `json:"name"`
	USD  float64 `json:"usd"`
}

// Connect opens a pool and ensures the schema exists.
func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("empty database url")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	s := &Store{Pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS usage_events (
  id BIGSERIAL PRIMARY KEY,
  agent TEXT NOT NULL DEFAULT 'default',
  "user" TEXT NOT NULL DEFAULT '',
  task TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  prompt_tokens INT NOT NULL DEFAULT 0,
  completion_tokens INT NOT NULL DEFAULT 0,
  cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS budget_rules (
  id BIGSERIAL PRIMARY KEY,
  scope_type TEXT NOT NULL,
  scope_key TEXT NOT NULL,
  soft_usd DOUBLE PRECISION,
  hard_usd DOUBLE PRECISION,
  soft_requests INT,
  hard_requests INT,
  window_seconds INT,
  UNIQUE (scope_type, scope_key)
);

CREATE TABLE IF NOT EXISTS alert_history (
  id BIGSERIAL PRIMARY KEY,
  scope_type TEXT NOT NULL,
  scope_key TEXT NOT NULL,
  kind TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS request_events (
  id BIGSERIAL PRIMARY KEY,
  agent TEXT NOT NULL DEFAULT 'default',
  "user" TEXT NOT NULL DEFAULT '',
  task TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS proxy_keys (
  id BIGSERIAL PRIMARY KEY,
  key_hash TEXT NOT NULL UNIQUE,
  prefix TEXT NOT NULL,
  agent TEXT NOT NULL DEFAULT 'default',
  user_label TEXT NOT NULL DEFAULT '',
  label TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS usage_events_created_at_idx ON usage_events (created_at);
CREATE INDEX IF NOT EXISTS usage_events_agent_created_idx ON usage_events (agent, created_at);
CREATE INDEX IF NOT EXISTS request_events_created_at_idx ON request_events (created_at);
CREATE INDEX IF NOT EXISTS alert_history_lookup_idx ON alert_history (scope_type, scope_key, kind, created_at);

ALTER TABLE budget_rules ADD COLUMN IF NOT EXISTS soft_requests INT;
ALTER TABLE budget_rules ADD COLUMN IF NOT EXISTS hard_requests INT;
ALTER TABLE budget_rules ADD COLUMN IF NOT EXISTS window_seconds INT;
`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Close releases the pool.
func (s *Store) Close() {
	if s != nil && s.Pool != nil {
		s.Pool.Close()
	}
}

// InsertUsage records a completed call.
func (s *Store) InsertUsage(ctx context.Context, e UsageEvent) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO usage_events (agent, "user", task, model, prompt_tokens, completion_tokens, cost_usd)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.Agent, e.User, e.Task, e.Model, e.PromptTokens, e.CompletionTokens, e.CostUSD)
	return err
}

// SumSpend returns total USD for a composite scope since since.
func (s *Store) SumSpend(ctx context.Context, agent, user, task string, since time.Time) (float64, error) {
	var sum float64
	err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(SUM(cost_usd), 0) FROM usage_events
WHERE agent = $1 AND "user" = $2 AND task = $3 AND created_at >= $4`,
		agent, user, task, since).Scan(&sum)
	return sum, err
}

// SumSpendByDim sums spend for one dimension (agent|user|task) since since.
func (s *Store) SumSpendByDim(ctx context.Context, dim, key string, since time.Time) (float64, error) {
	col := "agent"
	switch dim {
	case "user":
		col = `"user"`
	case "task":
		col = "task"
	case "agent":
		col = "agent"
	default:
		return 0, fmt.Errorf("unknown dim %q", dim)
	}
	var sum float64
	q := fmt.Sprintf(`SELECT COALESCE(SUM(cost_usd), 0) FROM usage_events WHERE %s = $1 AND created_at >= $2`, col)
	err := s.Pool.QueryRow(ctx, q, key, since).Scan(&sum)
	return sum, err
}

// InsertRequest records an allowed forward.
func (s *Store) InsertRequest(ctx context.Context, agent, user, task string) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO request_events (agent, "user", task) VALUES ($1, $2, $3)`, agent, user, task)
	return err
}

// CountRequests counts requests for composite scope since since.
func (s *Store) CountRequests(ctx context.Context, agent, user, task string, since time.Time) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
SELECT COUNT(*) FROM request_events
WHERE agent = $1 AND "user" = $2 AND task = $3 AND created_at >= $4`,
		agent, user, task, since).Scan(&n)
	return n, err
}

// CountRequestsByDim counts requests for one dimension.
func (s *Store) CountRequestsByDim(ctx context.Context, dim, key string, since time.Time) (int, error) {
	col := "agent"
	switch dim {
	case "user":
		col = `"user"`
	case "task":
		col = "task"
	case "agent":
		col = "agent"
	default:
		return 0, fmt.Errorf("unknown dim %q", dim)
	}
	var n int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM request_events WHERE %s = $1 AND created_at >= $2`, col)
	err := s.Pool.QueryRow(ctx, q, key, since).Scan(&n)
	return n, err
}

// UpsertRule creates or updates a budget rule.
func (s *Store) UpsertRule(ctx context.Context, r BudgetRule) (BudgetRule, error) {
	err := s.Pool.QueryRow(ctx, `
INSERT INTO budget_rules (scope_type, scope_key, soft_usd, hard_usd, soft_requests, hard_requests, window_seconds)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (scope_type, scope_key) DO UPDATE SET
  soft_usd = EXCLUDED.soft_usd,
  hard_usd = EXCLUDED.hard_usd,
  soft_requests = EXCLUDED.soft_requests,
  hard_requests = EXCLUDED.hard_requests,
  window_seconds = EXCLUDED.window_seconds
RETURNING id, scope_type, scope_key, soft_usd, hard_usd, soft_requests, hard_requests, window_seconds`,
		r.ScopeType, r.ScopeKey, r.SoftUSD, r.HardUSD, r.SoftRequests, r.HardRequests, r.WindowSeconds,
	).Scan(&r.ID, &r.ScopeType, &r.ScopeKey, &r.SoftUSD, &r.HardUSD, &r.SoftRequests, &r.HardRequests, &r.WindowSeconds)
	return r, err
}

// ListRules returns all budget rules.
func (s *Store) ListRules(ctx context.Context) ([]BudgetRule, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, scope_type, scope_key, soft_usd, hard_usd, soft_requests, hard_requests, window_seconds
FROM budget_rules ORDER BY scope_type, scope_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BudgetRule
	for rows.Next() {
		var r BudgetRule
		if err := rows.Scan(&r.ID, &r.ScopeType, &r.ScopeKey, &r.SoftUSD, &r.HardUSD, &r.SoftRequests, &r.HardRequests, &r.WindowSeconds); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRule removes a rule by id.
func (s *Store) DeleteRule(ctx context.Context, id int64) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM budget_rules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// RulesForScope returns rules matching agent/user/task keys.
func (s *Store) RulesForScope(ctx context.Context, agent, user, task string) ([]BudgetRule, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, scope_type, scope_key, soft_usd, hard_usd, soft_requests, hard_requests, window_seconds
FROM budget_rules
WHERE (scope_type = 'agent' AND scope_key = $1)
   OR (scope_type = 'user' AND scope_key = $2 AND $2 <> '')
   OR (scope_type = 'task' AND scope_key = $3 AND $3 <> '')`,
		agent, user, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BudgetRule
	for rows.Next() {
		var r BudgetRule
		if err := rows.Scan(&r.ID, &r.ScopeType, &r.ScopeKey, &r.SoftUSD, &r.HardUSD, &r.SoftRequests, &r.HardRequests, &r.WindowSeconds); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// InsertAlert records an alert and returns true if inserted (caller may still send).
func (s *Store) InsertAlert(ctx context.Context, scopeType, scopeKey, kind, message string) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO alert_history (scope_type, scope_key, kind, message)
VALUES ($1, $2, $3, $4)`, scopeType, scopeKey, kind, message)
	return err
}

// HasRecentAlert returns true if an alert of kind exists within window.
func (s *Store) HasRecentAlert(ctx context.Context, scopeType, scopeKey, kind string, window time.Duration) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
SELECT COUNT(*) FROM alert_history
WHERE scope_type = $1 AND scope_key = $2 AND kind = $3 AND created_at >= $4`,
		scopeType, scopeKey, kind, time.Now().Add(-window)).Scan(&n)
	return n > 0, err
}

// SpendSeries returns hourly buckets over the lookback duration.
func (s *Store) SpendSeries(ctx context.Context, lookback time.Duration) ([]SeriesPoint, error) {
	since := time.Now().Add(-lookback)
	rows, err := s.Pool.Query(ctx, `
SELECT date_trunc('hour', created_at) AS bucket, COALESCE(SUM(cost_usd), 0)
FROM usage_events
WHERE created_at >= $1
GROUP BY 1
ORDER BY 1`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SeriesPoint
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(&p.Bucket, &p.USD); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SpendByAgent rolls up cost by agent since since.
func (s *Store) SpendByAgent(ctx context.Context, since time.Time) ([]NamedSpend, error) {
	return s.spendByCol(ctx, "agent", since)
}

// SpendByUser rolls up cost by user since since.
func (s *Store) SpendByUser(ctx context.Context, since time.Time) ([]NamedSpend, error) {
	return s.spendByCol(ctx, `"user"`, since)
}

func (s *Store) spendByCol(ctx context.Context, col string, since time.Time) ([]NamedSpend, error) {
	q := fmt.Sprintf(`
SELECT COALESCE(%s, ''), COALESCE(SUM(cost_usd), 0)
FROM usage_events WHERE created_at >= $1
GROUP BY 1 ORDER BY 2 DESC`, col)
	rows, err := s.Pool.Query(ctx, q, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NamedSpend
	for rows.Next() {
		var n NamedSpend
		if err := rows.Scan(&n.Name, &n.USD); err != nil {
			return nil, err
		}
		if n.Name == "" {
			n.Name = "(empty)"
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SnapshotSpend returns composite-key spend map for the window.
func (s *Store) SnapshotSpend(ctx context.Context, since time.Time) (map[string]float64, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT agent, "user", task, COALESCE(SUM(cost_usd), 0)
FROM usage_events WHERE created_at >= $1
GROUP BY agent, "user", task`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var agent, user, task string
		var v float64
		if err := rows.Scan(&agent, &user, &task, &v); err != nil {
			return nil, err
		}
		out[agent+"\x00"+user+"\x00"+task] = v
	}
	return out, rows.Err()
}

// HashKey returns sha256 hex of a plaintext fuse key.
func HashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// InsertProxyKey stores a hashed key; plaintext is not retained.
func (s *Store) InsertProxyKey(ctx context.Context, plaintext, agent, userLabel, label string) (ProxyKey, error) {
	prefix := plaintext
	if len(prefix) > 12 {
		prefix = plaintext[:12]
	}
	var k ProxyKey
	err := s.Pool.QueryRow(ctx, `
INSERT INTO proxy_keys (key_hash, prefix, agent, user_label, label)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, prefix, agent, user_label, label, created_at, revoked_at`,
		HashKey(plaintext), prefix, agent, userLabel, label,
	).Scan(&k.ID, &k.Prefix, &k.Agent, &k.UserLabel, &k.Label, &k.CreatedAt, &k.RevokedAt)
	return k, err
}

// LookupProxyKey returns an active (non-revoked) key by plaintext.
func (s *Store) LookupProxyKey(ctx context.Context, plaintext string) (ProxyKey, error) {
	var k ProxyKey
	err := s.Pool.QueryRow(ctx, `
SELECT id, prefix, agent, user_label, label, created_at, revoked_at
FROM proxy_keys WHERE key_hash = $1 AND revoked_at IS NULL`, HashKey(plaintext)).
		Scan(&k.ID, &k.Prefix, &k.Agent, &k.UserLabel, &k.Label, &k.CreatedAt, &k.RevokedAt)
	return k, err
}

// ListProxyKeys lists all keys (including revoked).
func (s *Store) ListProxyKeys(ctx context.Context) ([]ProxyKey, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, prefix, agent, user_label, label, created_at, revoked_at
FROM proxy_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProxyKey
	for rows.Next() {
		var k ProxyKey
		if err := rows.Scan(&k.ID, &k.Prefix, &k.Agent, &k.UserLabel, &k.Label, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeProxyKey marks a key revoked.
func (s *Store) RevokeProxyKey(ctx context.Context, id int64) error {
	ct, err := s.Pool.Exec(ctx, `
UPDATE proxy_keys SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
