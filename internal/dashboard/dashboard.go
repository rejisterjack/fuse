package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rejisterjack/fuse/internal/auth"
	"github.com/rejisterjack/fuse/internal/budget"
	"github.com/rejisterjack/fuse/internal/store"
)

// Pages serves an Alpine.js spend dashboard from the Go process.
type Pages struct {
	Tracker *budget.Tracker
	Store   *store.Store
	// Demo is true for the public fake-traffic host (no real API key).
	Demo bool
}

// Mount registers dashboard routes on mux.
func (p *Pages) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", p.index)
	mux.HandleFunc("GET /api/spend", p.spend)
	mux.HandleFunc("GET /api/spend/series", p.series)
	mux.HandleFunc("GET /api/spend/by-agent", p.byAgent)
	mux.HandleFunc("GET /api/spend/by-user", p.byUser)
	mux.HandleFunc("GET /api/rules", p.listRules)
	mux.HandleFunc("POST /api/rules", p.upsertRule)
	mux.HandleFunc("DELETE /api/rules/{id}", p.deleteRule)
	mux.HandleFunc("GET /api/keys", p.listKeys)
	mux.HandleFunc("POST /api/keys", p.createKey)
	mux.HandleFunc("POST /api/keys/{id}/revoke", p.revokeKey)
}

func (p *Pages) spend(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p.Tracker.Snapshot())
}

func (p *Pages) series(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if p.Store == nil {
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}
	lookback := 24 * time.Hour
	if r.URL.Query().Get("range") == "7d" {
		lookback = 7 * 24 * time.Hour
	}
	pts, err := p.Store.SpendSeries(r.Context(), lookback)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if pts == nil {
		pts = []store.SeriesPoint{}
	}
	_ = json.NewEncoder(w).Encode(pts)
}

func (p *Pages) byAgent(w http.ResponseWriter, r *http.Request) {
	p.rollup(w, r, true)
}

func (p *Pages) byUser(w http.ResponseWriter, r *http.Request) {
	p.rollup(w, r, false)
}

func (p *Pages) rollup(w http.ResponseWriter, r *http.Request, byAgent bool) {
	w.Header().Set("Content-Type", "application/json")
	if p.Store == nil {
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	var rows []store.NamedSpend
	var err error
	if byAgent {
		rows, err = p.Store.SpendByAgent(r.Context(), since)
	} else {
		rows, err = p.Store.SpendByUser(r.Context(), since)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if rows == nil {
		rows = []store.NamedSpend{}
	}
	_ = json.NewEncoder(w).Encode(rows)
}

func (p *Pages) listRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if p.Store == nil {
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}
	rules, err := p.Store.ListRules(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if rules == nil {
		rules = []store.BudgetRule{}
	}
	_ = json.NewEncoder(w).Encode(rules)
}

func (p *Pages) upsertRule(w http.ResponseWriter, r *http.Request) {
	if p.Store == nil {
		http.Error(w, "postgres required", 503)
		return
	}
	var body store.BudgetRule
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.ScopeType == "" || body.ScopeKey == "" {
		http.Error(w, "scope_type and scope_key required", 400)
		return
	}
	out, err := p.Store.UpsertRule(r.Context(), body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (p *Pages) deleteRule(w http.ResponseWriter, r *http.Request) {
	if p.Store == nil {
		http.Error(w, "postgres required", 503)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := p.Store.DeleteRule(r.Context(), id); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	w.WriteHeader(204)
}

func (p *Pages) listKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if p.Store == nil {
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}
	keys, err := p.Store.ListProxyKeys(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if keys == nil {
		keys = []store.ProxyKey{}
	}
	_ = json.NewEncoder(w).Encode(keys)
}

func (p *Pages) createKey(w http.ResponseWriter, r *http.Request) {
	if p.Store == nil {
		http.Error(w, "postgres required", 503)
		return
	}
	var body struct {
		Agent     string `json:"agent"`
		UserLabel string `json:"user_label"`
		Label     string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Agent == "" {
		body.Agent = "default"
	}
	plain, err := auth.GenerateKey()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	k, err := p.Store.InsertProxyKey(r.Context(), plain, body.Agent, body.UserLabel, body.Label)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"key":  plain,
		"meta": k,
	})
}

func (p *Pages) revokeKey(w http.ResponseWriter, r *http.Request) {
	if p.Store == nil {
		http.Error(w, "postgres required", 503)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := p.Store.RevokeProxyKey(r.Context(), id); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	w.WriteHeader(204)
}

func (p *Pages) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := indexHTML
	if p.Demo {
		html = strings.Replace(indexHTML, "<body>", "<body>\n"+demoBanner, 1)
	}
	_, _ = w.Write([]byte(html))
}

const demoBanner = `<div style="background:#fef3c7;border-bottom:1px solid #f59e0b;color:#78350f;padding:.65rem 1rem;text-align:center;font-size:.9rem">
  Public demo — fake agent traffic only. Do not put real API keys or secrets here. Self-host with Docker Compose for production.
</div>`

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Fuse — Agent Cost Circuit Breaker</title>
  <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
  <style>
    :root { color-scheme: light; --ink:#14212b; --muted:#5b6b76; --line:#d7e0e7; --bg:#f3f7f9; --accent:#0f6e56; --card:#fff; }
    * { box-sizing: border-box; }
    body { margin:0; font-family: ui-sans-serif, system-ui, sans-serif; background:var(--bg); color:var(--ink); }
    main { max-width: 920px; margin: 0 auto; padding: 2.5rem 1.25rem 4rem; }
    h1 { font-size: 1.75rem; margin: 0 0 .5rem; }
    h2 { font-size: 1.1rem; margin: 2rem 0 .75rem; }
    p { color: var(--muted); line-height: 1.6; }
    table { width:100%; border-collapse: collapse; margin-top: .75rem; background:var(--card); border:1px solid var(--line); }
    th, td { text-align:left; padding:.65rem .9rem; border-bottom:1px solid var(--line); font-size:.92rem; }
    th { font-size:.72rem; text-transform:uppercase; letter-spacing:.06em; color:var(--muted); }
    .brand { font-weight:700; letter-spacing:.14em; font-size:.8rem; color:var(--accent); }
    .grid { display:grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
    @media (max-width:700px) { .grid { grid-template-columns: 1fr; } }
    form.row { display:flex; flex-wrap:wrap; gap:.5rem; align-items:end; margin:.75rem 0; }
    label { display:flex; flex-direction:column; gap:.25rem; font-size:.75rem; color:var(--muted); }
    input, select, button { font: inherit; padding:.45rem .6rem; border:1px solid var(--line); border-radius:4px; background:#fff; }
    button { background:var(--accent); color:#fff; border-color:var(--accent); cursor:pointer; }
    button.ghost { background:#fff; color:var(--ink); }
    .chart { background:var(--card); border:1px solid var(--line); padding:1rem; margin-top:.75rem; }
    .foot { margin-top:2.5rem; font-size:.8rem; color:var(--muted); }
    .ok { color:var(--accent); font-size:.85rem; }
    .warn { color:#9a3412; font-size:.85rem; }
  </style>
</head>
<body>
  <main x-data="dash()" x-init="init()">
    <p class="brand">FUSE</p>
    <h1>Spend dashboard</h1>
    <p>Live token cost estimates by agent / user / task. Hard limits stop the next API call. Self-host on a trusted network — this UI has no auth.</p>

    <h2>Spend over time</h2>
    <div class="chart">
      <svg :viewBox="'0 0 ' + chartW + ' ' + chartH" width="100%" height="160" preserveAspectRatio="none">
        <polyline fill="none" stroke="#0f6e56" stroke-width="2"
          :points="seriesPoints" />
      </svg>
      <p x-show="series.length === 0" style="margin:0">No time-series yet (requires Postgres usage events).</p>
    </div>

    <div class="grid">
      <div>
        <h2>By agent</h2>
        <table>
          <thead><tr><th>Agent</th><th>USD</th></tr></thead>
          <tbody>
            <template x-for="row in byAgent" :key="row.name">
              <tr><td x-text="row.name"></td><td x-text="row.usd.toFixed(4)"></td></tr>
            </template>
          </tbody>
        </table>
      </div>
      <div>
        <h2>By user</h2>
        <table>
          <thead><tr><th>User</th><th>USD</th></tr></thead>
          <tbody>
            <template x-for="row in byUser" :key="row.name">
              <tr><td x-text="row.name"></td><td x-text="row.usd.toFixed(4)"></td></tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>

    <h2>Current scopes</h2>
    <table>
      <thead><tr><th>Scope</th><th>Spend (USD)</th></tr></thead>
      <tbody>
        <template x-for="row in rows" :key="row.key">
          <tr>
            <td x-text="row.key"></td>
            <td x-text="row.spend.toFixed(4)"></td>
          </tr>
        </template>
      </tbody>
    </table>
    <p x-show="rows.length === 0" style="margin-top:1rem">No spend recorded yet.</p>

    <h2>Budget rules</h2>
    <form class="row" @submit.prevent="saveRule">
      <label>Type
        <select x-model="rule.scope_type">
          <option value="agent">agent</option>
          <option value="user">user</option>
          <option value="task">task</option>
        </select>
      </label>
      <label>Key <input x-model="rule.scope_key" required /></label>
      <label>Soft USD <input type="number" step="0.01" x-model.number="rule.soft_usd" /></label>
      <label>Hard USD <input type="number" step="0.01" x-model.number="rule.hard_usd" /></label>
      <label>Soft reqs <input type="number" x-model.number="rule.soft_requests" /></label>
      <label>Hard reqs <input type="number" x-model.number="rule.hard_requests" /></label>
      <label>Window s <input type="number" x-model.number="rule.window_seconds" placeholder="86400" /></label>
      <button type="submit">Save rule</button>
    </form>
    <p class="warn" x-text="ruleErr"></p>
    <table>
      <thead><tr><th>ID</th><th>Type</th><th>Key</th><th>Soft/Hard USD</th><th>Reqs</th><th></th></tr></thead>
      <tbody>
        <template x-for="r in rules" :key="r.id">
          <tr>
            <td x-text="r.id"></td>
            <td x-text="r.scope_type"></td>
            <td x-text="r.scope_key"></td>
            <td x-text="(r.soft_usd ?? '—') + ' / ' + (r.hard_usd ?? '—')"></td>
            <td x-text="(r.soft_requests ?? '—') + ' / ' + (r.hard_requests ?? '—')"></td>
            <td><button type="button" class="ghost" @click="delRule(r.id)">Delete</button></td>
          </tr>
        </template>
      </tbody>
    </table>

    <h2>Scoped Fuse keys</h2>
    <form class="row" @submit.prevent="mintKey">
      <label>Agent <input x-model="keyForm.agent" placeholder="default" /></label>
      <label>User <input x-model="keyForm.user_label" /></label>
      <label>Label <input x-model="keyForm.label" /></label>
      <button type="submit">Mint key</button>
    </form>
    <p class="ok" x-show="newKey" x-text="'Copy now (shown once): ' + newKey"></p>
    <table>
      <thead><tr><th>Prefix</th><th>Agent</th><th>User</th><th>Label</th><th>Revoked</th><th></th></tr></thead>
      <tbody>
        <template x-for="k in keys" :key="k.id">
          <tr>
            <td x-text="k.prefix"></td>
            <td x-text="k.agent"></td>
            <td x-text="k.user_label"></td>
            <td x-text="k.label"></td>
            <td x-text="k.revoked_at ? 'yes' : 'no'"></td>
            <td><button type="button" class="ghost" x-show="!k.revoked_at" @click="revokeKey(k.id)">Revoke</button></td>
          </tr>
        </template>
      </tbody>
    </table>

    <p class="foot">Cost figures are estimates from token counts and may diverge from provider invoices. See docs/LIMITATIONS.md. Rates live in internal/cost.</p>
  </main>
  <script>
    function dash() {
      return {
        rows: [], series: [], byAgent: [], byUser: [], rules: [], keys: [],
        newKey: '', ruleErr: '',
        chartW: 600, chartH: 140,
        rule: { scope_type: 'agent', scope_key: '', soft_usd: 5, hard_usd: 25, soft_requests: null, hard_requests: null, window_seconds: 86400 },
        keyForm: { agent: 'default', user_label: '', label: '' },
        get seriesPoints() {
          if (!this.series.length) return '';
          const max = Math.max(...this.series.map(p => p.usd), 0.0001);
          const n = this.series.length;
          return this.series.map((p, i) => {
            const x = n === 1 ? this.chartW/2 : (i / (n-1)) * (this.chartW - 20) + 10;
            const y = this.chartH - 10 - (p.usd / max) * (this.chartH - 20);
            return x + ',' + y;
          }).join(' ');
        },
        async init() {
          await this.load();
          setInterval(() => this.load(), 5000);
        },
        async load() {
          const spend = await fetch('/api/spend').then(r => r.json());
          this.rows = Object.entries(spend).map(([key, spend]) => ({
            key: key.split('\u0000').filter(Boolean).join(' / ') || 'default', spend
          }));
          this.series = await fetch('/api/spend/series?range=24h').then(r => r.json()).catch(() => []);
          this.byAgent = await fetch('/api/spend/by-agent').then(r => r.json()).catch(() => []);
          this.byUser = await fetch('/api/spend/by-user').then(r => r.json()).catch(() => []);
          this.rules = await fetch('/api/rules').then(r => r.json()).catch(() => []);
          this.keys = await fetch('/api/keys').then(r => r.json()).catch(() => []);
        },
        async saveRule() {
          this.ruleErr = '';
          const body = { ...this.rule };
          ['soft_usd','hard_usd','soft_requests','hard_requests','window_seconds'].forEach(k => {
            if (body[k] === '' || body[k] === null || Number.isNaN(body[k])) body[k] = null;
          });
          const res = await fetch('/api/rules', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
          if (!res.ok) { this.ruleErr = await res.text(); return; }
          await this.load();
        },
        async delRule(id) {
          await fetch('/api/rules/' + id, { method: 'DELETE' });
          await this.load();
        },
        async mintKey() {
          const res = await fetch('/api/keys', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(this.keyForm) });
          const data = await res.json();
          this.newKey = data.key || '';
          await this.load();
        },
        async revokeKey(id) {
          await fetch('/api/keys/' + id + '/revoke', { method: 'POST' });
          await this.load();
        }
      }
    }
  </script>
</body>
</html>
`
