package provider

import (
	"net/http"
	"testing"
)

func TestOpenAIParseUsage(t *testing.T) {
	body := []byte(`{"model":"gpt-4o-mini","usage":{"prompt_tokens":10,"completion_tokens":20}}`)
	u, ok := OpenAI{}.ParseUsage(body)
	if !ok || u.PromptTokens != 10 || u.CompletionTokens != 20 {
		t.Fatalf("got %+v ok=%v", u, ok)
	}
}

func TestAnthropicParseUsage(t *testing.T) {
	body := []byte(`{"model":"claude-3-5-sonnet","usage":{"input_tokens":11,"output_tokens":22}}`)
	u, ok := Anthropic{}.ParseUsage(body)
	if !ok || u.PromptTokens != 11 || u.CompletionTokens != 22 {
		t.Fatalf("got %+v ok=%v", u, ok)
	}
}

func TestAnthropicRewrite(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer skip")
	Anthropic{}.RewriteRequest(req, "sk-ant-test")
	if req.Header.Get("x-api-key") != "sk-ant-test" {
		t.Fatalf("x-api-key=%q", req.Header.Get("x-api-key"))
	}
	if req.Header.Get("Authorization") != "" {
		t.Fatal("Authorization should be cleared")
	}
}

func TestOpenAIStreamUsage(t *testing.T) {
	sse := []byte("data: {\"id\":\"1\"}\n\ndata: {\"model\":\"gpt-4o\",\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":7}}\n\ndata: [DONE]\n")
	u, ok := OpenAI{}.ParseStreamUsage(sse)
	if !ok || u.PromptTokens != 5 || u.CompletionTokens != 7 {
		t.Fatalf("got %+v ok=%v", u, ok)
	}
}

func TestAnthropicStreamUsage(t *testing.T) {
	sse := []byte("data: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-3-5-haiku\",\"usage\":{\"input_tokens\":3,\"output_tokens\":0}}}\n\ndata: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":3,\"output_tokens\":9}}\n")
	u, ok := Anthropic{}.ParseStreamUsage(sse)
	if !ok || u.CompletionTokens != 9 {
		t.Fatalf("got %+v ok=%v", u, ok)
	}
}

func TestLookup(t *testing.T) {
	if Lookup("anthropic").Name() != "anthropic" {
		t.Fatal("anthropic")
	}
	if Lookup("openai_compatible").Name() != "openai" {
		t.Fatal("alias")
	}
}
