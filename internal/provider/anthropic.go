package provider

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Anthropic handles the Anthropic Messages API.
type Anthropic struct{}

func (Anthropic) Name() string { return "anthropic" }

func (Anthropic) RewriteRequest(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Del("Authorization")
	}
}

func (Anthropic) ParseUsage(body []byte) (Usage, bool) {
	var parsed struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Usage{}, false
	}
	if parsed.Usage.InputTokens == 0 && parsed.Usage.OutputTokens == 0 {
		return Usage{}, false
	}
	return Usage{
		Model:            parsed.Model,
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
	}, true
}

func (a Anthropic) ParseStreamUsage(sseBody []byte) (Usage, bool) {
	lines := bytes.Split(sseBody, []byte("\n"))
	var last Usage
	found := false
	model := ""
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		var evt struct {
			Type  string `json:"type"`
			Model string `json:"model"`
			Usage *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Message *struct {
				Model string `json:"model"`
				Usage *struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(payload, &evt); err != nil {
			continue
		}
		if evt.Model != "" {
			model = evt.Model
		}
		if evt.Message != nil && evt.Message.Model != "" {
			model = evt.Message.Model
		}
		u := evt.Usage
		if u == nil && evt.Message != nil {
			u = evt.Message.Usage
		}
		if u != nil && (u.InputTokens > 0 || u.OutputTokens > 0) {
			last = Usage{
				Model:            model,
				PromptTokens:     u.InputTokens,
				CompletionTokens: u.OutputTokens,
			}
			found = true
		}
	}
	if !found {
		return a.ParseUsage(sseBody)
	}
	if last.Model == "" {
		last.Model = model
	}
	return last, true
}
