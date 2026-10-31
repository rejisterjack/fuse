package provider

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
)

// OpenAI handles OpenAI and OpenAI-compatible APIs.
type OpenAI struct{}

func (OpenAI) Name() string { return "openai" }

func (OpenAI) RewriteRequest(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Del("x-api-key")
	}
}

func (OpenAI) ParseUsage(body []byte) (Usage, bool) {
	var parsed struct {
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Usage{}, false
	}
	if parsed.Usage.PromptTokens == 0 && parsed.Usage.CompletionTokens == 0 {
		return Usage{}, false
	}
	return Usage{
		Model:            parsed.Model,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
	}, true
}

func (o OpenAI) ParseStreamUsage(sseBody []byte) (Usage, bool) {
	// OpenAI streams usage on the final chunk when stream_options.include_usage is set,
	// or sometimes as a trailing data line with usage.
	lines := bytes.Split(sseBody, []byte("\n"))
	var last Usage
	found := false
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		var chunk struct {
			Model string `json:"model"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(payload, &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil && (chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0) {
			last = Usage{
				Model:            chunk.Model,
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
			}
			found = true
		}
	}
	if !found {
		// Fallback: whole body might be one JSON object
		return o.ParseUsage(sseBody)
	}
	if last.Model == "" {
		last.Model = "unknown"
	}
	return last, true
}

// IsStreamResponse reports whether the response looks like SSE.
func IsStreamResponse(resp *http.Response, reqBodyHint []byte) bool {
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return true
	}
	return bytes.Contains(reqBodyHint, []byte(`"stream":true`)) || bytes.Contains(reqBodyHint, []byte(`"stream": true`))
}
