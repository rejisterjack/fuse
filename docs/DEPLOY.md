# Deploy

## Docker Compose (default self-host)

```bash
cp .env.example .env
# set FUSE_UPSTREAM_API_KEY
docker compose up -d --build
```

Dashboard: http://localhost:8080  
Proxy base URL for agents: http://localhost:8080

## Demo traffic (no real API key)

```bash
FUSE_UPSTREAM_BASE_URL=http://demogen:18080 \
  docker compose -f docker-compose.yml -f docker-compose.demo.yml --profile demo up --build
```

`fuse-demogen` serves a mock OpenAI `/v1/chat/completions` and posts through Fuse so the spend board fills.

## Neon

Self-host Compose Postgres is enough for most installs. For hosted Postgres (Neon):

1. Create a Neon project and copy the **pooled** connection string.
2. Set `FUSE_DATABASE_URL` (include `sslmode=require`).
3. Run the Fuse binary or container **without** the Compose Postgres service.

No Neon-specific SDK is required — Fuse uses `pgx`.

## Railway

`railway.toml` builds the Dockerfile and health-checks `/healthz`.

For a fake-traffic demo on Railway:

1. Deploy service **fuse** with `FUSE_DATABASE_URL` (Neon or Railway Postgres) and `FUSE_UPSTREAM_BASE_URL` pointing at the demogen service private URL.
2. Deploy service **demogen** with start command `/fuse-demogen -mock-addr=:18080 -fuse-url=http://fuse.railway.internal:8080` (adjust host to your private networking).
3. Do not put a real provider API key on the demo; use `sk-demo`.

Use fake traffic only on public demos.
