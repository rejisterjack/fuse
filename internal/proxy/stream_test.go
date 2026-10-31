package proxy

import (
	"testing"

	"github.com/rejisterjack/fuse/internal/provider"
)

func TestParseStreamOpenAI(t *testing.T) {
	sse := []byte("data: {\"model\":\"gpt-4o\",\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":3}}\n\ndata: [DONE]\n")
	u, ok := parseStreamUsage(provider.OpenAI{}, sse)
	if !ok || u.PromptTokens != 2 || u.CompletionTokens != 3 {
		t.Fatalf("got %+v ok=%v", u, ok)
	}
}

func TestParseStreamAnthropic(t *testing.T) {
	sse := []byte("data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":4,\"output_tokens\":8}}\n")
	u, ok := parseStreamUsage(provider.Anthropic{}, sse)
	if !ok || u.CompletionTokens != 8 {
		t.Fatalf("got %+v ok=%v", u, ok)
	}
}
