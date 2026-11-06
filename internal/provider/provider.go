package provider

import (
	"net/http"
	"strings"
)

// Usage is normalized token usage from an upstream response.
type Usage struct {
	Model            string
	PromptTokens     int
	CompletionTokens int
}

// Provider rewrites outbound requests and parses usage from responses.
type Provider interface {
	Name() string
	RewriteRequest(req *http.Request, apiKey string)
	ParseUsage(body []byte) (Usage, bool)
	ParseStreamUsage(sseBody []byte) (Usage, bool)
}

// Lookup returns a provider by name.
// openai_compatible, google, and groq alias the OpenAI request/usage shape.
func Lookup(name string) Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "anthropic":
		return Anthropic{}
	case "openai_compatible", "openai", "google", "groq", "":
		return OpenAI{}
	default:
		return OpenAI{}
	}
}
