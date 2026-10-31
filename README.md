# Fuse

Agent cost circuit breaker.

> Agents should never run up an uncapped bill. Fuse makes sure they can't.

**Home:** [fuse.rejisterjack.com](https://fuse.rejisterjack.com) · **Module:** `github.com/rejisterjack/fuse`

![Demo: runaway agent blocked](docs/demo.gif)

## Quick start

```bash
cp .env.example .env
# set FUSE_UPSTREAM_API_KEY
docker compose up -d --build
```

Point your agent at `http://localhost:8080` instead of the real API base URL. Optionally send:

- `X-Fuse-Agent`
- `X-Fuse-User`
- `X-Fuse-Task`

Or mint a scoped `fuse_…` key from the dashboard and use it as `Authorization: Bearer fuse_…`.

Open the dashboard at [http://localhost:8080](http://localhost:8080) to view spend charts, edit budget rules, and manage keys.

## Local without Docker

```bash
go run ./cmd/fuse -config config.example.yaml
```

Postgres is optional at boot; without it Fuse still enforces in-memory soft/hard USD limits. With Postgres, spend, rules, alerts, and keys persist across restarts.

## Demo traffic (no real API key)

```bash
docker compose -f docker-compose.yml -f docker-compose.demo.yml --profile demo up --build
```

## Features

- Reverse proxy for OpenAI, Anthropic (`upstream_provider: anthropic`), or OpenAI-compatible bases
- Soft (alert) and hard (block) USD limits, plus optional request-count limits
- Per-agent / per-user / per-task rules from the dashboard
- Slack webhook and SMTP alerts (once per scope per billing window)
- Streaming and non-streaming usage accounting
- Self-hosted Compose; Neon-compatible `FUSE_DATABASE_URL` (see [docs/DEPLOY.md](docs/DEPLOY.md))

## Docs

- [Limitations](docs/LIMITATIONS.md) — cost estimates, latency, first-call risk
- [Providers](docs/PROVIDERS.md) — adapters and how to add one
- [Deploy](docs/DEPLOY.md) — Compose, Neon, Railway demo

## License

MIT
