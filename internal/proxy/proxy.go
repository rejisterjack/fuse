package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/rejisterjack/fuse/internal/auth"
	"github.com/rejisterjack/fuse/internal/budget"
	"github.com/rejisterjack/fuse/internal/cost"
	"github.com/rejisterjack/fuse/internal/provider"
	"github.com/rejisterjack/fuse/internal/store"
)

// Handler is an OpenAI-compatible reverse proxy with budget enforcement.
type Handler struct {
	Upstream  *url.URL
	APIKey    string
	Tracker   *budget.Tracker
	Provider  provider.Provider
	Store     *store.Store
	OnSoftHit func(scope budget.Scope, decision budget.Decision)
	OnHardHit func(scope budget.Scope, decision budget.Decision)
	Proxy     *httputil.ReverseProxy
}

// New builds a reverse proxy pointing at upstreamBase.
func New(upstreamBase, apiKey string, tracker *budget.Tracker, prov provider.Provider) (*Handler, error) {
	u, err := url.Parse(strings.TrimRight(upstreamBase, "/"))
	if err != nil {
		return nil, err
	}
	if prov == nil {
		prov = provider.OpenAI{}
	}
	h := &Handler{Upstream: u, APIKey: apiKey, Tracker: tracker, Provider: prov}
	h.Proxy = &httputil.ReverseProxy{
		Director:       h.director,
		ModifyResponse: h.modifyResponse,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		},
	}
	return h, nil
}

func (h *Handler) director(req *http.Request) {
	req.URL.Scheme = h.Upstream.Scheme
	req.URL.Host = h.Upstream.Host
	req.Host = h.Upstream.Host
	h.Provider.RewriteRequest(req, h.APIKey)
}

// ServeHTTP enforces budgets then forwards compatible chat/completions traffic.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	scope := budget.Scope{
		Agent: r.Header.Get("X-Fuse-Agent"),
		User:  r.Header.Get("X-Fuse-User"),
		Task:  r.Header.Get("X-Fuse-Task"),
	}

	if tok := auth.BearerToken(r.Header.Get("Authorization")); auth.IsFuseKey(tok) {
		if h.Store == nil {
			http.Error(w, `{"error":{"message":"fuse keys require postgres","type":"fuse_auth"}}`, http.StatusUnauthorized)
			return
		}
		k, err := h.Store.LookupProxyKey(r.Context(), tok)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid or revoked fuse key","type":"fuse_auth"}}`))
			return
		}
		if scope.Agent == "" || scope.Agent == "default" {
			scope.Agent = k.Agent
		}
		if scope.User == "" {
			scope.User = k.UserLabel
		}
	}

	if scope.Agent == "" {
		scope.Agent = "default"
	}

	decision := h.Tracker.Check(scope)
	if decision.SoftHit && decision.Allowed && h.OnSoftHit != nil {
		h.OnSoftHit(scope, decision)
	}
	if !decision.Allowed {
		if decision.HardHit && h.OnHardHit != nil {
			h.OnHardHit(scope, decision)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": decision.Message,
				"type":    "fuse_budget_exceeded",
				"spend":   decision.SpendUSD,
				"limit":   decision.LimitUSD,
			},
		})
		return
	}

	h.Tracker.RecordRequest(scope)

	ctx := r.Context()
	r = r.WithContext(withScope(ctx, scope))
	h.Proxy.ServeHTTP(w, r)
}

func (h *Handler) modifyResponse(resp *http.Response) error {
	scope, ok := scopeFromContext(resp.Request.Context())
	if !ok {
		return nil
	}

	ct := resp.Header.Get("Content-Type")
	isStream := strings.Contains(ct, "text/event-stream")

	if isStream {
		buf := &bytes.Buffer{}
		orig := resp.Body
		resp.Body = &streamCloser{
			ReadCloser: &teeReadCloser{r: orig, buf: buf},
			onClose: func() {
				if u, ok := h.Provider.ParseStreamUsage(buf.Bytes()); ok {
					h.recordUsage(scope, u)
				}
			},
		}
		resp.Header.Del("Content-Length")
		resp.ContentLength = -1
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))

	if u, ok := h.Provider.ParseUsage(body); ok {
		h.recordUsage(scope, u)
	}
	return nil
}

func (h *Handler) recordUsage(scope budget.Scope, u provider.Usage) {
	usd := cost.EstimateUSD(u.Model, u.PromptTokens, u.CompletionTokens)
	if usd > 0 {
		h.Tracker.RecordDetail(scope, usd, budget.UsageDetail{
			Model: u.Model, PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens,
		})
	}
}

type streamCloser struct {
	io.ReadCloser
	onClose func()
	closed  bool
}

func (s *streamCloser) Close() error {
	err := s.ReadCloser.Close()
	if !s.closed {
		s.closed = true
		if s.onClose != nil {
			s.onClose()
		}
	}
	return err
}
