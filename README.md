# ShipSystem

ShipSystem (船舶管理) is a training/demo simulation system for ship situational awareness. It is intentionally bounded to simulated or recorded data.

Safety boundary:

- No real weapon-control interface.
- No real fire-control computation.
- No real electronic-warfare or countermeasure device command.
- No live radar integration in v1.
- All abstract threat and response effects are training-only adjudications.

## Components

- Go backend in `cmd/sim-server`
- Simulation engine in `internal/sim`
- REST/WebSocket API in `internal/api`
- PostgreSQL/PostGIS migrations in `migrations/001_init.sql` and `migrations/002_snapshot_frames.sql`
- React/MapLibre frontend skeleton in `web`
- Docker/Compose packaging for small cloud deployments

## Run Backend

Memory mode:

```powershell
go run ./cmd/sim-server
```

Useful local configuration:

```powershell
$env:SHIP_SIM_SCENARIO_DIR="scenarios"
$env:SHIP_SIM_REQUEST_BODY_LIMIT="1048576"
```

Limit browser origins for local UI access:

```powershell
$env:SHIP_SIM_ALLOWED_ORIGINS="http://127.0.0.1:5173,http://localhost:5173"
```

PostgreSQL/PostGIS mode:

```powershell
$env:DATABASE_URL="postgres://user:password@localhost:5432/shipsim?sslmode=disable"
go run ./cmd/sim-server
```

Apply the migrations in order before using PostgreSQL mode:

```powershell
psql $env:DATABASE_URL -f migrations/001_init.sql
psql $env:DATABASE_URL -f migrations/002_snapshot_frames.sql
```

PostgreSQL mode has a startup migration gate. The app requires
`schema_migrations.name='ship_sim'` to be at the current version before HTTP
startup. Empty databases are treated as version `0`; databases with only
`001_init.sql` are version `1` and must apply `002_snapshot_frames.sql`.

Production mode requires authentication:

```powershell
$env:SHIP_SIM_ENV="production"
$env:SHIP_SIM_AUTH_MODE="token"
$env:SHIP_SIM_AUTH_TOKEN="replace-with-a-secret"
```

Token auth is intended for demo or simple single-user deployments. It is not a
multi-user production identity system. `SHIP_SIM_AUTH_MODE=proxy` is supported
for OIDC/auth-proxy deployments; set `SHIP_SIM_AUTH_USER_HEADER` to the trusted
user header from the proxy. The reverse proxy must remove any incoming copy of
that header before setting it, and the application must not be exposed directly
to the public internet in proxy-auth mode. When authentication is enabled, run
listing and run-scoped APIs are limited to the authenticated token user or proxy
user.

Long-lived credentials are not accepted through the `access_token` query
parameter. Browser report export uses authenticated `fetch` requests and Blob
downloads so credentials stay in headers. WebSocket clients first request a
short-lived one-time ticket from `POST /api/runs/{run_id}/ws-ticket`, then
connect to `/ws/runs/{run_id}?ticket=...`; the ticket is bound to that run and
is consumed on use.

The HTTP server sets baseline security headers on all responses:
`Content-Security-Policy`, `X-Content-Type-Options: nosniff`,
`Referrer-Policy: no-referrer`, and `X-Frame-Options: DENY`.

## Cloud Demo Deployment

For a local Compose demo, copy `.env.local.example` to `.env` and start:

```powershell
Copy-Item .env.local.example .env
docker compose up --build
```

The Compose defaults are development-oriented and use authentication off plus a
local-only database password. Production deployments must use
`.env.production.example` as a template, replace all placeholders with secret
manager values, set a narrow `SHIP_SIM_ALLOWED_ORIGINS`, and choose token or
proxy authentication explicitly. See `docs/deployment.md` for secret management,
HTTP timeout, graceful shutdown, and Compose validation guidance.

The Compose file runs the app and PostGIS. The migrations are mounted into
`/docker-entrypoint-initdb.d` and are applied when the database volume is first
created. For an existing database, apply `migrations/001_init.sql` and then
`migrations/002_snapshot_frames.sql` manually.

Run the optional Postgres integration test against an isolated test database:

```powershell
.\scripts\test-postgres.ps1
```

See `docs/database.md` for database migration, backup/preview, and snapshot
write reliability guidance.

## API Quick Start

```powershell
$run = Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/runs -ContentType application/json -Body '{}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/runs/$($run.id)/start"
Invoke-RestMethod -Uri "http://localhost:8080/api/runs/$($run.id)/tracks"
```

Submit an abstract training action:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/runs/$($run.id)/actions" -ContentType application/json -Body '{"type":"training_response"}'
```

List recent runs and paged events:

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/runs?limit=10"
Invoke-RestMethod -Uri "http://localhost:8080/api/runs/$($run.id)/events?limit=20"
Invoke-RestMethod -Uri "http://localhost:8080/api/runs/$($run.id)/track-points?limit=200"
```

Read snapshot replay frames and the nearest frame for a time:

```powershell
$frames = Invoke-RestMethod -Uri "http://localhost:8080/api/runs/$($run.id)/snapshots?limit=200"
$at = [System.Uri]::EscapeDataString($frames[0].sampled_at)
Invoke-RestMethod -Uri "http://localhost:8080/api/runs/$($run.id)/snapshots/nearest?at=$at"
```

Build a training review report:

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/runs/$($run.id)/report"
Invoke-WebRequest -Uri "http://localhost:8080/api/runs/$($run.id)/report" -OutFile report.json
Invoke-WebRequest -Uri "http://localhost:8080/api/runs/$($run.id)/report?format=csv" -OutFile report.csv
```

Reports currently use `version: 1` and are intentionally limited to training
summary plus audit data. They include duration, track totals, action counts,
threat summary, final track states, event audit summary, snapshot coverage
(`from`, `to`, `count`, `average_interval_ms`), raw audit events, and the safety
notice. They do not include scoring, tactical recommendations, real
fire-control data, or device-command guidance.

New runs persist full snapshot frames for replay. Runs created before the
snapshot migration still return reports with `replay_mode: "legacy"`; the UI
keeps showing historical track lines and events, but exact state replay is not
available for those older runs.

Readiness and scenarios:

```powershell
Invoke-RestMethod -Uri http://localhost:8080/readyz
Invoke-RestMethod -Uri http://localhost:8080/api/scenarios
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/runs -ContentType application/json -Body '{"scenario_id":"demo"}'
```

Runtime metrics:

```powershell
Invoke-RestMethod -Uri http://localhost:8080/metrics
```

The metrics payload includes active/listed run counts, WebSocket connection
count, total snapshot frame count, `snapshot_frames_by_run`, snapshot write
count, snapshot write failures, and snapshot write last/average/max duration in
milliseconds.

## Operations

Apply PostgreSQL migrations in order: `001_init.sql` first, then
`002_snapshot_frames.sql`. Existing runs from before `002_snapshot_frames.sql`
are not backfilled with snapshots; they remain available through legacy replay
mode.

Snapshot and audit retention is disabled by default:

```powershell
$env:SHIP_SIM_RETENTION_DAYS="0"
$env:SHIP_SIM_RETENTION_INTERVAL="0s"
$env:SHIP_SIM_MAX_TRACK_POINTS_PER_RUN="0"
$env:SHIP_SIM_MAX_EVENTS_PER_RUN="0"
$env:SHIP_SIM_MAX_SNAPSHOTS_PER_RUN="0"
```

Set `SHIP_SIM_RETENTION_DAYS` to prune events, contacts, track points, and
snapshots older than that cutoff at server startup and every
`SHIP_SIM_RETENTION_INTERVAL` when the interval is greater than zero. Set
`SHIP_SIM_MAX_TRACK_POINTS_PER_RUN`, `SHIP_SIM_MAX_EVENTS_PER_RUN`, and
`SHIP_SIM_MAX_SNAPSHOTS_PER_RUN` to keep only the newest rows per run. Use the
preview API or operations script before a manual prune:

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/retention/preview?days=30"
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/retention/prune" -ContentType application/json -Body '{"days":30}'
.\scripts\retention.ps1 -Days 30
```

Manual retention also accepts `cutoff`, `ended_before`, and
`max_track_points_per_run`, `max_events_per_run`, and
`max_snapshots_per_run`. In token or proxy authentication mode, retention, run
listing, run-scoped replay, report, event, track, and snapshot APIs are limited
to the authenticated owner. See `docs/retention.md` for capacity estimates and
the 5/20/100 track smoke script.

Security notes:

- Do not put long-lived API tokens in URLs. Use `Authorization: Bearer ...` or
  `X-Ship-Sim-Token` for simple token deployments.
- WebSocket access in authenticated deployments requires a one-time ticket from
  the authenticated API.
- Proxy-auth deployments require a trusted reverse proxy that overwrites and
  sanitizes `SHIP_SIM_AUTH_USER_HEADER`; direct public exposure can let clients
  spoof identity headers.
- The system remains training-only. Do not add real weapon-control,
  fire-control, electronic-warfare, or radar control integrations.

API errors use a stable shape:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "validation failed",
    "details": ["tick_hz must be between 1 and 60"]
  }
}
```
