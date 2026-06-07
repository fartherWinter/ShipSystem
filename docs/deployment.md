# Deployment Guide

ShipSystem remains a training, demonstration, replay, and audit simulator. Do not connect deployments to real weapon-control, fire-control, electronic-warfare, radar control, or device-command chains.

## Environment Files

Use the examples as templates only. Do not commit real secrets.

- `.env.local.example`: local Docker Compose demo settings. Authentication is off and the database password is marked `shipsim-local-dev-only`.
- `.env.production.example`: production-oriented settings. Replace every placeholder, especially `POSTGRES_PASSWORD`, `SHIP_SIM_ALLOWED_ORIGINS`, and authentication settings.
- `.env.example`: backward-compatible local demo template.

The Compose defaults are intentionally development-oriented. They no longer default to `SHIP_SIM_ENV=production`, token auth, or a production-looking placeholder token. Production deployments must provide an explicit environment file or platform configuration.

## Frontend Build Configuration

The React frontend is built by Vite, so these settings are embedded when `npm run build` or the Docker `web-build` stage runs:

- `VITE_API_BASE`: public browser-facing API origin, such as `https://training.example.com`.
- `VITE_AUTH_MODE`: `off` for local demos, `token` for simple token demos, or `proxy` for authenticated reverse-proxy deployments.
- `VITE_MAP_TILE_URL`: raster tile URL template. Use an internal tile server for disconnected or intranet deployments.
- `VITE_MAP_TILE_ATTRIBUTION`: attribution for the configured tile source.

`docker-compose.yml` passes these values as build args. Rebuild the image after changing them:

```powershell
docker compose build app
docker compose up -d
```

Do not rely on public OpenStreetMap tiles in production environments that must operate without public internet access. Provide a local or internal tile service and set `VITE_MAP_TILE_URL` before building the frontend.

## Secrets

Recommended secret sources:

- Docker Compose: keep secrets in an untracked `.env` file for local-only demos. For production Compose, prefer Docker secrets mounted as files and convert them to environment variables in the entrypoint or deployment wrapper.
- Kubernetes: use `Secret` objects or an external-secrets operator backed by a cloud secret manager. Mount database credentials and authentication settings from secrets, not ConfigMaps.
- Cloud platforms: use the platform secret manager, such as AWS Secrets Manager, Azure Key Vault, Google Secret Manager, or the platform's managed application secrets.

Never store real database passwords, API tokens, OIDC client secrets, or TLS private keys in git. Rotate any secret that was copied into logs, URLs, screenshots, shell history, or committed files.

## Authentication

Production mode must not run with `SHIP_SIM_AUTH_MODE=off`.

Token auth is acceptable for demos and simple single-user deployments. It is not a multi-user production identity system. For multi-user production deployments, put ShipSystem behind an authentication proxy and set `SHIP_SIM_AUTH_MODE=proxy`.

In proxy-auth mode, the reverse proxy must strip any incoming `SHIP_SIM_AUTH_USER_HEADER` value before setting the trusted authenticated identity. Do not expose the app directly to the public internet in proxy-auth mode.

Long-lived credentials must stay out of URLs. Report downloads use authenticated `fetch` requests. WebSockets use a short-lived one-time ticket from `POST /api/runs/{run_id}/ws-ticket`.

## HTTP Server Timeouts

The server applies these defaults:

- `SHIP_SIM_HTTP_READ_TIMEOUT=10s`
- `SHIP_SIM_HTTP_READ_HEADER_TIMEOUT=5s`
- `SHIP_SIM_HTTP_WRITE_TIMEOUT=30s`
- `SHIP_SIM_HTTP_IDLE_TIMEOUT=60s`
- `SHIP_SIM_SHUTDOWN_TIMEOUT=15s`
- `SHIP_SIM_SNAPSHOT_WRITE_TIMEOUT=5s`

Use Go duration syntax such as `500ms`, `10s`, or `2m`. Set longer write timeouts only when reports or replay APIs need more time under known load.
`SHIP_SIM_SNAPSHOT_WRITE_TIMEOUT` bounds replay/audit snapshot persistence. PostgreSQL snapshot transactions derive their local statement timeout from this request deadline. Increase it only after measuring database write latency and capacity.

## Graceful Shutdown

The process handles `SIGINT` and `SIGTERM`.

On shutdown:

1. The HTTP server stops accepting new requests and waits up to `SHIP_SIM_SHUTDOWN_TIMEOUT` for active HTTP requests to complete.
2. Running simulation engines are stopped.
3. Each stopped run is saved.
4. A final snapshot is written where possible.

If final persistence fails, the process logs the run id and exits non-zero. This shutdown path is best effort; use persistent PostgreSQL storage for deployments where replay and audit data must survive process restarts.

## Database Safety

Apply migrations before using PostgreSQL mode. The app refuses to start with PostgreSQL unless `schema_migrations` reports the current required version. Empty databases are treated as version `0`, a database with only `migrations/001_init.sql` is version `1`, and a database with migrations through `002_snapshot_frames.sql` is version `2`.

Apply `migrations/003_training_product.sql` before deploying the training product workflow to PostgreSQL. It is additive and stores managed scenarios, run metadata, event annotations, and audit logs. Take a backup or platform snapshot before applying migrations to shared, staging, or production data.

Do not run destructive database operations against shared or production data without a backup or a documented preview. Retention pruning supports `GET /api/retention/preview`; use it before `POST /api/retention/prune`.

See `docs/database.md` for migration gate behavior, test database workflow, and snapshot write reliability notes. See `docs/retention.md` for background retention, preview-first pruning, capacity limits, and sizing estimates.

## Probes and Metrics

Use `/healthz` for liveness and `/readyz` for readiness. `/readyz` checks store readiness and migration status, so it can return `503` while `/healthz` remains live. Metrics are available as JSON at `/metrics` and Prometheus text at `/metrics/prometheus`.

In authenticated deployments, `/readyz`, `/metrics`, and `/metrics/prometheus` require authentication. Route probes and scrapers through the same trusted proxy or provide a secret-backed token for token-mode deployments. See `docs/observability.md`.

## API Contract

Publish the matching `docs/openapi.json` with each release. The contract version, `RunReport.version`, scenario versions, and image tag should be traceable together in release notes. See `docs/api.md` for compatibility rules and error-code policy.

Training product deployments should document who can create or disable scenarios, who can archive runs, and how exported reports are retained. The system remains training-only; the abstract assessment module must not be described as tactical guidance or real-world engagement advice.

## Compose Checks

Validate the Compose model before deployment:

```powershell
docker compose config
```

For local demo:

```powershell
Copy-Item .env.local.example .env
docker compose up --build
```

For production, create an untracked environment file from `.env.production.example`, replace placeholders with real secret-backed values, and verify authentication and allowed origins before exposing the service.
