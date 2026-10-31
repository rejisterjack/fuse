package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rejisterjack/fuse/internal/budget"
)

func TestSoftHitDedupe(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	n := &Notifier{SlackWebhookURL: srv.URL}
	scope := budget.Scope{Agent: "a"}
	d := budget.Decision{SpendUSD: 5, LimitUSD: 5, Message: "soft"}
	n.SoftHit(scope, d)
	n.SoftHit(scope, d)
	if hits.Load() != 1 {
		t.Fatalf("expected 1 slack post, got %d", hits.Load())
	}
}

func TestHardHitDistinct(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]string
		_ = json.Unmarshal(b, &m)
		bodies = append(bodies, m["text"])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	n := &Notifier{SlackWebhookURL: srv.URL}
	scope := budget.Scope{Agent: "a"}
	n.SoftHit(scope, budget.Decision{SpendUSD: 1, LimitUSD: 1})
	n.HardHit(scope, budget.Decision{SpendUSD: 2, LimitUSD: 2, Message: "hard"})
	if len(bodies) != 2 {
		t.Fatalf("expected soft+hard, got %v", bodies)
	}
}
