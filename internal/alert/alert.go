package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"sync"
	"time"

	"github.com/rejisterjack/fuse/internal/budget"
	"github.com/rejisterjack/fuse/internal/store"
)

const dedupeWindow = 30 * 24 * time.Hour

// Notifier sends soft/hard budget alerts via Slack and/or SMTP.
type Notifier struct {
	SlackWebhookURL string
	SMTPAddr        string
	SMTPFrom        string
	SMTPTo          string
	Store           *store.Store

	mu    sync.Mutex
	local map[string]time.Time // kind|scope -> last sent
}

func (n *Notifier) ensureLocal() {
	if n.local == nil {
		n.local = make(map[string]time.Time)
	}
}

func alertScopeKey(scope budget.Scope) (scopeType, scopeKey string) {
	if scope.Task != "" {
		return "task", scope.Task
	}
	if scope.User != "" {
		return "user", scope.User
	}
	return "agent", scope.Agent
}

func (n *Notifier) shouldSend(scopeType, scopeKey, kind string) bool {
	ctx := context.Background()
	if n.Store != nil {
		ok, err := n.Store.HasRecentAlert(ctx, scopeType, scopeKey, kind, dedupeWindow)
		if err == nil && ok {
			return false
		}
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ensureLocal()
	k := kind + "|" + scopeType + "|" + scopeKey
	if t, ok := n.local[k]; ok && time.Since(t) < dedupeWindow {
		return false
	}
	return true
}

func (n *Notifier) markSent(scopeType, scopeKey, kind, message string) {
	if n.Store != nil {
		_ = n.Store.InsertAlert(context.Background(), scopeType, scopeKey, kind, message)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ensureLocal()
	n.local[kind+"|"+scopeType+"|"+scopeKey] = time.Now()
}

// SoftHit notifies when a soft limit is crossed (once per scope per window).
func (n *Notifier) SoftHit(scope budget.Scope, d budget.Decision) {
	n.notify("soft", scope, d)
}

// HardHit notifies when a hard limit blocks a call (once per scope per window).
func (n *Notifier) HardHit(scope budget.Scope, d budget.Decision) {
	n.notify("hard", scope, d)
}

func (n *Notifier) notify(kind string, scope budget.Scope, d budget.Decision) {
	scopeType, scopeKey := alertScopeKey(scope)
	if !n.shouldSend(scopeType, scopeKey, kind) {
		return
	}
	text := fmt.Sprintf("Fuse %s limit: agent=%s user=%s task=%s spend=$%.4f limit=$%.2f msg=%s",
		kind, scope.Agent, scope.User, scope.Task, d.SpendUSD, d.LimitUSD, d.Message)
	n.postSlack(text)
	n.sendSMTP(kind+" budget limit", text)
	n.markSent(scopeType, scopeKey, kind, text)
}

func (n *Notifier) postSlack(text string) {
	if n.SlackWebhookURL == "" {
		return
	}
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(n.SlackWebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (n *Notifier) sendSMTP(subject, body string) {
	if n.SMTPAddr == "" || n.SMTPFrom == "" || n.SMTPTo == "" {
		return
	}
	msg := []byte("To: " + n.SMTPTo + "\r\n" +
		"From: " + n.SMTPFrom + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" + body + "\r\n")
	_ = smtp.SendMail(n.SMTPAddr, nil, n.SMTPFrom, []string{n.SMTPTo}, msg)
}

// SendSMTPForTest exposes SMTP send for tests.
func (n *Notifier) SendSMTPForTest(subject, body string) {
	n.sendSMTP(subject, body)
}
