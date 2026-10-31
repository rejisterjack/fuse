#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
echo "Running Go benchmark (mock upstream via httptest)…"
go test -bench=BenchmarkProxyRoundTrip -benchtime=2s -count=1 ./internal/proxy
echo
echo "Paste p50/ns-per-op into docs/LIMITATIONS.md after adjusting for your machine."
