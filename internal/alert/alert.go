package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rejisterjack/fuse/internal/budget"
)

// Notifier sends soft/hard budget alerts.
type Notifier struct {
	SlackWebhookURL string
}

// SoftHit notifies when a soft limit is crossed.
func (n *Notifier) SoftHit(scope budget.Scope, d budget.Decision) {
	if n.SlackWebhookURL == "" {
		return
	}
	text := fmt.Sprintf("Fuse soft limit: agent=%s user=%s task=%s spend=$%.4f limit=$%.2f",
		scope.Agent, scope.User, scope.Task, d.SpendUSD, d.LimitUSD)
	n.postSlack(text)
}

func (n *Notifier) postSlack(text string) {
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
