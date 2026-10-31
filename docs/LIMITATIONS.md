# Fuse limitations

## Cost estimates ≠ invoices

Fuse estimates USD from token counts using illustrative rates in `internal/cost`. Real bills vary by model tier, caching, batch discounts, and provider pricing changes. Treat dashboard numbers as a circuit-breaker signal, not accounting.

## Proxy latency

Every call passes through Fuse. Sample local overhead against a mock upstream (see `scripts/bench-latency.sh` and `go test -bench=BenchmarkProxyRoundTrip ./internal/proxy`):

| Path | Sample (Apple M5, `go test -bench`) | Notes |
|------|--------------------------------------|--------|
| Via Fuse → mock upstream | ~80 µs/op (~0.08 ms) | `BenchmarkProxyRoundTrip` local httptest |

Expect higher latency on real networks and with Postgres-backed spend sums. Re-run the bench on your hardware before quoting numbers externally.

## First-call / pre-check spend risk

Budget **Check** runs **before** the upstream request. Cost is **Record**ed **after** a successful response (including streaming close). Therefore:

1. A call that pushes spend from under the hard limit to over still reaches the provider and incurs cost.
2. The **next** call is blocked with `402` / `fuse_budget_exceeded`.
3. Set conservative soft/hard limits so one expensive completion cannot blow the budget unnoticed.

Streaming responses are accounted when the response body is fully consumed/closed.
