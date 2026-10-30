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

	"github.com/rejisterjack/fuse/internal/budget"
	"github.com/rejisterjack/fuse/internal/cost"
)

// Handler is an OpenAI-compatible reverse proxy with budget enforcement.
type Handler struct {
	Upstream  *url.URL
	APIKey    string
	Tracker   *budget.Tracker
	OnSoftHit func(scope budget.Scope, decision budget.Decision)
	Proxy     *httputil.ReverseProxy
}

// New builds a reverse proxy pointing at upstreamBase.
func New(upstreamBase, apiKey string, tracker *budget.Tracker) (*Handler, error) {
	u, err := url.Parse(strings.TrimRight(upstreamBase, "/"))
	if err != nil {
		return nil, err
	}
	h := &Handler{Upstream: u, APIKey: apiKey, Tracker: tracker}
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
	if h.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.APIKey)
	}
}

// ServeHTTP enforces budgets then forwards compatible chat/completions traffic.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	scope := budget.Scope{
		Agent: r.Header.Get("X-Fuse-Agent"),
		User:  r.Header.Get("X-Fuse-User"),
		Task:  r.Header.Get("X-Fuse-Task"),
	}
	if scope.Agent == "" {
		scope.Agent = "default"
	}

	decision := h.Tracker.Check(scope)
	if decision.SoftHit && h.OnSoftHit != nil {
		h.OnSoftHit(scope, decision)
	}
	if !decision.Allowed {
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

	ctx := r.Context()
	r = r.WithContext(withScope(ctx, scope))
	h.Proxy.ServeHTTP(w, r)
}

func (h *Handler) modifyResponse(resp *http.Response) error {
	scope, ok := scopeFromContext(resp.Request.Context())
	if !ok {
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

	var parsed struct {
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		usd := cost.EstimateUSD(parsed.Model, parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens)
		if usd > 0 {
			h.Tracker.Record(scope, usd)
		}
	}
	return nil
}
