# Providers

Fuse selects a provider adapter with `upstream_provider` (or `FUSE_UPSTREAM_PROVIDER`):

| Value | Behavior |
|-------|----------|
| `openai` (default) | `Authorization: Bearer` + OpenAI `usage.prompt_tokens` / `completion_tokens` |
| `openai_compatible` | Same as `openai` (any OpenAI-shaped base URL) |
| `anthropic` | `x-api-key` + `anthropic-version` + `usage.input_tokens` / `output_tokens` |

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
