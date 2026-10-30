# Fuse

Agent cost circuit breaker.

> Agents should never run up an uncapped bill. Fuse makes sure they can't.

**Home:** [fuse.rejisterjack.com](https://fuse.rejisterjack.com) · **Module:** `github.com/rejisterjack/fuse`

## Quick start

```bash
cp .env.example .env
# set FUSE_UPSTREAM_API_KEY
docker compose up -d
```

Point your agent at `http://localhost:8080` instead of the real API base URL. Optionally send:

- `X-Fuse-Agent`
- `X-Fuse-User`
- `X-Fuse-Task`

Open the dashboard at [http://localhost:8080](http://localhost:8080).

## Local without Docker

```bash
go run ./cmd/fuse -config config.example.yaml
```

Postgres is optional at boot; without it Fuse still enforces in-memory soft/hard USD limits.

## Status

Scaffold with reverse proxy, budget checks, Alpine.js spend board, Docker Compose, and Postgres schema stubs. Soft/hard limits and Slack alerts are wired for the soft path.
