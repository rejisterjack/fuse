# Providers

Fuse selects a provider adapter with `upstream_provider` (or `FUSE_UPSTREAM_PROVIDER`):

| Value | Behavior |
|-------|----------|
| `openai` (default) | `Authorization: Bearer` + OpenAI `usage.prompt_tokens` / `completion_tokens` |
| `openai_compatible` | Same as `openai` (any OpenAI-shaped base URL) |
| `google` | Same as `openai` — point `upstream_base_url` at an OpenAI-compatible Google endpoint |
| `groq` | Same as `openai` — point `upstream_base_url` at Groq's OpenAI-compatible API |
| `anthropic` | `x-api-key` + `anthropic-version` + `usage.input_tokens` / `output_tokens` |

`google` and `groq` are registered aliases of the OpenAI adapter. They do not change auth or usage parsing; they exist so config can name the vendor while reusing the OpenAI-compatible wire format. Set `upstream_base_url` (or `FUSE_UPSTREAM_BASE_URL`) to that vendor's OpenAI-compatible base.

Adapters live in `internal/provider` and implement:

```go
type Provider interface {
    Name() string
    RewriteRequest(req *http.Request, apiKey string)
    ParseUsage(body []byte) (Usage, bool)
    ParseStreamUsage(sseBody []byte) (Usage, bool)
}
```

## Adding a provider

1. Create `internal/provider/myprovider.go` implementing `Provider`.
2. Register it in `Lookup` inside `provider.go`.
3. Add ParseUsage / ParseStreamUsage fixtures in `provider_test.go`.
4. Document the name in this file and in `config.example.yaml`.
