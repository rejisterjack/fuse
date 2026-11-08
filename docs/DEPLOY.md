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

## Render (public demo)

Deploy the Dockerfile as two free web services in the same region:

1. **fuse** — start command `/fuse -config /config.example.yaml`. Set `FUSE_DEMO=true`, `FUSE_DATABASE_URL` (Neon), `FUSE_UPSTREAM_PROVIDER=openai`, `FUSE_UPSTREAM_API_KEY=sk-demo`, and `FUSE_UPSTREAM_BASE_URL=http://fuse-demogen:10000` (Render private network; both services listen on `$PORT`, typically `10000`).
2. **fuse-demogen** — docker command `/fuse-demogen -mock-addr=:10000 -fuse-url=http://fuse:10000 -agent=demo -interval=3s`.
3. Optionally put a Vercel rewrite in `edge/` in front of the Fuse public URL for `fuse.rejisterjack.com`.

Do not put a real provider API key on the public demo.
