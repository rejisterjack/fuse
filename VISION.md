# Fuse: Agent Cost Circuit Breaker

Private doc for me, Rupam. Not for public. Product name: Fuse. Home: fuse.rejisterjack.com. Go module under github.com/rejisterjack/fuse.

## The one-line idea

A Go proxy that sits in front of your AI API calls, tracks token usage and cost per agent, per user, and per task in real time, and hard-stops any agent that exceeds its budget — before the bill arrives.

## The problem it solves

AI agents can spiral into massive API bills with nothing to stop them. A bug in a loop, a runaway multi-step task, or a single user hammering the system — and the cost shows up days later with no warning. API providers offer account-level limits but nothing at the agent or user level. Developers bolt on ad-hoc counters in application code, which are fragile, inconsistent, and invisible to the rest of the team.

Runaway costs are one of the top nine challenges in AI agent development. There is no lightweight, self-hostable tool that enforces budgets at the agent level and cuts the circuit automatically. Fuse is that tool.

## What it does

- Sits as a proxy between your application and the AI API (OpenAI, Anthropic, or any OpenAI-compatible endpoint)
- Tracks token usage and estimated cost per agent, per user, and per task in real time
- Enforces configurable budget limits: when a limit is hit, the proxy returns an error instead of forwarding the request
- Supports soft limits (alert and log) and hard limits (block the call entirely)
- Dashboard shows spend by agent, by user, and over time with charts
- Alert via email or Slack webhook when an agent approaches or hits its budget
- Zero changes to your agent code beyond pointing it at the Fuse proxy URL instead of the API directly

## Who it is for

- Teams running AI agents in production who have been surprised by API bills
- Developers building multi-user products where each user's agent usage must be tracked and capped independently
- Solo developers who want a circuit breaker on their own API spend during development and production

## How it is used

1. Deploy Fuse with Docker Compose, which includes its own Postgres
2. Set your real API key in Fuse's config and get a Fuse proxy URL and a scoped key back
3. Point your agent at the Fuse proxy URL instead of the real API — no other code changes needed
4. Set budget rules in the dashboard: agent X gets 1,000 requests per day, user Y gets $5 of spend per month
5. When a limit is hit, Fuse blocks the call and fires the alert — the bill stops there

## How I portray it

- One-line pitch: "Agents should never run up an uncapped bill. Fuse makes sure they can't."
- Lead with the demo: show a runaway agent loop getting blocked at the budget limit with a clear error and a Slack alert
- Show the spend dashboard because it makes the invisible visible
- Position it as the financial safety layer that pairs with Keywarden's permission layer

## How it runs

- Self-hosted with Docker Compose, which bundles the Go proxy and Postgres together
- One command: `docker compose up -d`
- The developer points their agent at the Fuse proxy URL instead of the real API
- No data leaves the developer's own infrastructure
- Demo instance on Railway uses fake agent traffic only

## Tech stack

- Go for the proxy and the budget enforcement engine
- Postgres on Neon for storing usage records, budget rules, and alert history
- Simple HTML and Alpine.js dashboard served by the Go process itself
- Slack incoming webhooks and SMTP for alerts
- Compatible with OpenAI, Anthropic, and any OpenAI-compatible API

## What done looks like

- Docker Compose file that starts the proxy and dashboard in one command
- Budget rules working for per-agent, per-user, and per-task scopes
- Soft limit (alert) and hard limit (block) both working end to end
- Spend dashboard showing usage over time with charts
- Slack alert firing when a budget is hit
- Zero changes required to agent code beyond updating the base URL
- Clean README with a demo GIF of a runaway agent being blocked
- Repo at github.com/rejisterjack/fuse

## Honest notes

- Cost estimation from token counts is approximate; actual API pricing can vary by model and tier, so document this clearly
- The proxy adds a small amount of latency to every API call; benchmark and document this honestly
- This does not prevent an agent from being expensive before the first limit check fires; set conservative limits and document the first-call risk
- Compatible with OpenAI and Anthropic out of the box; other providers need a custom adapter

## Deployment

- Docker Compose for the self-hosted version (includes Go proxy and Postgres)
- Railway for the demo instance, using fake traffic only
- No Vercel needed; the dashboard is served by the Go process itself
- No Neon needed for self-hosting; the Docker Compose Postgres is sufficient
