# Ship Simulation Training System

This repository contains a training/demo simulation system for ship situational awareness. It is intentionally bounded to simulated or recorded data.

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

Production mode requires authentication:

```powershell
$env:SHIP_SIM_ENV="production"
$env:SHIP_SIM_AUTH_MODE="token"
$env:SHIP_SIM_AUTH_TOKEN="replace-with-a-secret"
```

`SHIP_SIM_AUTH_MODE=proxy` is also supported for OIDC/auth-proxy deployments; set
`SHIP_SIM_AUTH_USER_HEADER` to the trusted user header from the proxy.
When authentication is enabled, run listing and run-scoped APIs are limited to
the authenticated token user or proxy user.

## Cloud Demo Deployment

Copy `.env.example` to `.env`, change `SHIP_SIM_AUTH_TOKEN`,
`SHIP_SIM_ALLOWED_ORIGINS`, and the PostgreSQL password, then start:

```powershell
docker compose up --build
```

The Compose file runs the app and PostGIS. The migration is mounted into
`/docker-entrypoint-initdb.d` and is applied when the database volume is first
created. For an existing database, apply `migrations/001_init.sql` and then
`migrations/002_snapshot_frames.sql` manually.

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
$env:SHIP_SIM_MAX_TRACK_POINTS_PER_RUN="0"
```

Set `SHIP_SIM_RETENTION_DAYS` to prune events, contacts, track points, and
snapshots older than that cutoff at server startup. Set
`SHIP_SIM_MAX_TRACK_POINTS_PER_RUN` to keep only the newest points per run. Use
the preview API before a manual prune:

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/retention/preview?days=30"
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/retention/prune" -ContentType application/json -Body '{"days":30}'
```

Manual retention also accepts `cutoff`, `ended_before`, and
`max_track_points_per_run`. In token or proxy authentication mode, retention,
run listing, run-scoped replay, report, event, track, and snapshot APIs are
limited to the authenticated owner.

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
