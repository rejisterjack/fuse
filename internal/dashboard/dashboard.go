package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/rejisterjack/fuse/internal/budget"
)

// Pages serves a minimal Alpine.js spend dashboard from the Go process.
type Pages struct {
	Tracker *budget.Tracker
}

// Mount registers dashboard routes on mux.
func (p *Pages) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /", p.index)
	mux.HandleFunc("GET /api/spend", p.spend)
}

func (p *Pages) spend(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p.Tracker.Snapshot())
}

func (p *Pages) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Fuse — Agent Cost Circuit Breaker</title>
  <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
  <style>
    :root { color-scheme: light; --ink:#14212b; --muted:#5b6b76; --line:#d7e0e7; --bg:#f3f7f9; --accent:#0f6e56; }
    * { box-sizing: border-box; }
    body { margin:0; font-family: ui-sans-serif, system-ui, sans-serif; background:var(--bg); color:var(--ink); }
    main { max-width: 720px; margin: 0 auto; padding: 3rem 1.25rem; }
    h1 { font-size: 1.75rem; margin: 0 0 .5rem; }
    p { color: var(--muted); line-height: 1.6; }
    table { width:100%; border-collapse: collapse; margin-top: 1.5rem; background:#fff; border:1px solid var(--line); }
    th, td { text-align:left; padding:.75rem 1rem; border-bottom:1px solid var(--line); font-size:.95rem; }
    th { font-size:.75rem; text-transform:uppercase; letter-spacing:.06em; color:var(--muted); }
    .brand { font-weight:700; letter-spacing:.14em; font-size:.8rem; color:var(--accent); }
  </style>
</head>
<body>
  <main x-data="spend()" x-init="load(); setInterval(load, 5000)">
    <p class="brand">FUSE</p>
    <h1>Spend dashboard</h1>
    <p>Live token cost estimates by agent / user / task. Hard limits stop the next API call.</p>
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
    <p x-show="rows.length === 0" style="margin-top:1rem">No spend recorded yet. Point an agent at this proxy and send a chat completion.</p>
  </main>
  <script>
    function spend() {
      return {
        rows: [],
        async load() {
          const res = await fetch('/api/spend');
          const data = await res.json();
          this.rows = Object.entries(data).map(([key, spend]) => ({ key: key.split('\u0000').filter(Boolean).join(' / ') || 'default', spend }));
        }
      }
    }
  </script>
</body>
</html>
`
