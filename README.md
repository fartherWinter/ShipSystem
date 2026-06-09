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
- PostgreSQL/PostGIS migrations in `migrations/001_init.sql` through `migrations/005_metrics_history.sql`
- OpenAPI contract in `docs/openapi.json`
- React/MapLibre training console in `web`
- Docker/Compose packaging for small cloud deployments

## Current Capabilities

- Scenario workflow: built-in/file scenarios, managed database scenarios, JSON upload/copy/update, map-assisted sensor/zone authoring, reusable visual templates, assessment-rule form editing, enable/disable controls, client/server validation, version metadata, optional assessment profiles, and configurable record-completeness rules.
- Run workflow: create, start, pause, stop, live WebSocket snapshots, snapshot replay, event audit, track history, training metadata, annotations, and archive state.
- Training actions: abstract `maneuver`, `decoy`, and `training_response` actions with training-only adjudication and persisted audit records.
- Review workflow: replay window navigation, event jump-to-nearest-frame, replay anchors, browser-local review bookmarks, replay quality hints, final track summaries, configurable abstract training-record assessment, and JSON/CSV/HTML/PDF report exports.
- Operations workflow: memory or PostgreSQL/PostGIS storage, migration gate, retention preview/prune, run archive export, role-aware proxy auth with console access summary, capacity pressure metrics with per-run detail, health/readiness endpoints, JSON and Prometheus metrics, Docker/Compose packaging, and documented release checks.
- Course workflow: file and managed course templates pair validated scenarios with expected metadata and report review checklists, then create managed scenarios through the console or API.

See `docs/glossary.md` for product terminology, `docs/course-templates.md` for reusable exercise packages, `docs/proxy-identity-runbook.md` for proxy role deployment, and `docs/optimization-roadmap.md` for the product polish and improvement backlog.

## Run Backend

Memory mode:

```powershell
go run ./cmd/sim-server
```

Useful local configuration:

```powershell
$env:SHIP_SIM_SCENARIO_DIR="scenarios"
$env:SHIP_SIM_COURSE_TEMPLATE_DIR="course-templates"
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
psql $env:DATABASE_URL -f migrations/003_training_product.sql
psql $env:DATABASE_URL -f migrations/004_course_templates.sql
psql $env:DATABASE_URL -f migrations/005_metrics_history.sql
```

PostgreSQL mode has a startup migration gate. The app requires
`schema_migrations.name='ship_sim'` to be at the current version before HTTP
startup. Empty databases are treated as version `0`; databases with only
`001_init.sql` are version `1`, databases migrated through
`002_snapshot_frames.sql` are version `2`, and databases migrated through
`003_training_product.sql` are version `3`, and `004_course_templates.sql`
is version `4`. Apply `005_metrics_history.sql`
to reach the current version.

Production mode requires authentication:

```powershell
$env:SHIP_SIM_ENV="production"
$env:SHIP_SIM_AUTH_MODE="token"
$env:SHIP_SIM_AUTH_TOKEN="replace-with-a-secret"
```

Token auth is intended for demo or simple single-user deployments. It is not a
multi-user production identity system. `SHIP_SIM_AUTH_MODE=proxy` is supported
for OIDC/auth-proxy deployments; set `SHIP_SIM_AUTH_USER_HEADER` to the trusted
user header from the proxy and optionally `SHIP_SIM_AUTH_ROLE_HEADER` to
`viewer`, `operator`, `instructor`, or `admin`. If the proxy cannot send a role
header, set `SHIP_SIM_AUTH_ROLE_MAP` with comma-separated `user=role` entries
and optionally `SHIP_SIM_AUTH_DEFAULT_ROLE` for unmapped authenticated users.
The reverse proxy must remove any incoming copy of those headers before setting
them, and the application must not be exposed directly to the public internet in
proxy-auth mode. When authentication is enabled, run listing and run-scoped APIs
are limited to the authenticated token user or proxy user.

`GET /api/session` returns the current auth mode, authenticated user, role, and
role source plus descriptive permission flags. The React console uses it to show
the active Access summary; backend authorization remains the source of truth for
every mutation.

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

The frontend uses Vite build-time settings for the browser API base,
authentication prompt mode, and MapLibre raster tile source:

```powershell
$env:VITE_API_BASE="https://training.example.com"
$env:VITE_AUTH_MODE="proxy"
$env:VITE_MAP_TILE_URL="https://tiles.internal.example/{z}/{x}/{y}.png"
$env:VITE_MAP_TILE_ATTRIBUTION="Internal training map tiles"
```

Set `VITE_MAP_TILE_URL` to a local or internal tile server before building when
the deployment must run without public internet access. The UI displays a map
error state if configured tiles cannot be loaded.

The Compose file runs the app and PostGIS. The migrations are mounted into
`/docker-entrypoint-initdb.d` and are applied when the database volume is first
created. For an existing database, apply `migrations/001_init.sql` and then
`migrations/002_snapshot_frames.sql`, `migrations/003_training_product.sql`,
`migrations/004_course_templates.sql`, and `migrations/005_metrics_history.sql`
manually.

Run the optional Postgres integration test against an isolated test database:

```powershell
.\scripts\test-postgres.ps1
```

See `docs/database.md` for database migration, backup/preview, and snapshot
write reliability guidance.

CI/CD, release tagging, SBOM, and image scanning guidance is in
`docs/release.md`.

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
Invoke-WebRequest -Uri "http://localhost:8080/api/runs/$($run.id)/report?format=html" -OutFile report.html
Invoke-WebRequest -Uri "http://localhost:8080/api/runs/$($run.id)/report?format=pdf" -OutFile report.pdf
```

Export a completed run archive before retention pruning or cold-storage handoff:

```powershell
.\scripts\export-run-archive.ps1 -RunID $run.id
.\scripts\export-run-archive.ps1 -RunID $run.id -Compress
```

Compressed archives can be uploaded through a pre-signed object-store URL:

```powershell
.\scripts\export-run-archive.ps1 -RunID $run.id -Compress -UploadUrl $env:SHIP_SIM_ARCHIVE_UPLOAD_URL
```

Restore replay evidence from an archive into a PostgreSQL/PostGIS store:

```powershell
go run ./cmd/archive-restore -archive outputs/run-$($run.id)-archive.zip -database-url $env:DATABASE_URL
```

Reports currently use `version: 2` and are intentionally limited to training
summary plus audit data. They include duration, track totals, action counts,
threat summary, final track states, event audit summary, event annotations,
abstract training-record assessment, persisted audit logs, snapshot coverage
(`from`, `to`, `count`, `average_interval_ms`), raw audit events, and the safety
notice. Assessment is based on record completeness and replay coverage; it is
not tactical advice, real fire-control data, or device-command guidance.

Managed scenario and training record APIs:

```powershell
$scenario = Get-Content scenarios/demo.json -Raw
$saved = Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/scenarios -ContentType application/json -Body $scenario
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/scenarios/$($saved.id)/copy" -ContentType application/json -Body '{"name":"demo-copy"}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/scenarios/$($saved.id)/disable"
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/scenarios/$($saved.id)/enable"
Invoke-RestMethod -Method Put -Uri "http://localhost:8080/api/runs/$($run.id)/metadata" -ContentType application/json -Body '{"tags":["demo"],"trainees":["student-a"],"instructor_notes":"Training review only.","archived":false}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/runs/$($run.id)/annotations" -ContentType application/json -Body '{"note":"Instructor review note."}'
Invoke-RestMethod -Uri "http://localhost:8080/api/runs/$($run.id)/audit"
```

Reusable course templates are loaded from `course-templates/` and can also be
stored as managed database templates. The console Course panel can create a
managed scenario from the selected template. API usage:

```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/course-templates
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/course-templates/quick-review-baseline/scenario
```

Template scenarios, expected metadata, and review checklists remain
training-only setup and audit aids; they are not tactical recommendations.

New runs persist full snapshot frames for replay. Runs created before the
snapshot migration still return reports with `replay_mode: "legacy"`; the UI
keeps showing historical track lines and events, but exact state replay is not
available for those older runs.

Replay anchors can be copied from the console and opened later with:

```text
http://localhost:5173/?run=<run-id>&at=<RFC3339-time>
```

The console loads the run and jumps to the nearest persisted replay frame.
Browser-local replay bookmarks are review conveniences; use annotations or
instructor notes for evidence that must be preserved in reports.

Readiness and scenarios:

```powershell
Invoke-RestMethod -Uri http://localhost:8080/readyz
Invoke-RestMethod -Uri http://localhost:8080/api/scenarios
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/runs -ContentType application/json -Body '{"scenario_id":"demo"}'
```

Runtime metrics:

```powershell
Invoke-RestMethod -Uri http://localhost:8080/metrics
Invoke-WebRequest -Uri http://localhost:8080/metrics/prometheus
```

The metrics payload includes sample time, active/listed run counts, WebSocket
connection count, total snapshot/event/track-point/contact counts, per-run
count maps, capacity pressure for snapshots/events/track points, configured
retention limits, snapshot write count/failures/latency, HTTP request
counters/latency, engine counts, DB readiness, and backing-store table/index/
total bytes where available. The console Capacity panel keeps a rolling trend
window for growth, write latency, DB size, and archive-watch guidance.
Prometheus format is available at `/metrics/prometheus`. See
`docs/observability.md` for probe authentication and log field guidance.

API contract:

```powershell
cd web
npm run generate:types
```

The OpenAPI source is `docs/openapi.json`. Frontend API types are generated into
`web/src/generated/api-types.ts` and re-exported through `web/src/types.ts`.
See `docs/api.md` for report, scenario, run, snapshot versioning and API error
code policy. See `docs/training-product.md` for scenario CRUD, run metadata,
annotations, assessment, report templates, audit logs, and the training-only
boundary.

## Operations

Apply PostgreSQL migrations in order: `001_init.sql` first, then
`002_snapshot_frames.sql`, `003_training_product.sql`,
`004_course_templates.sql`, and `005_metrics_history.sql`. Existing runs from
before `002_snapshot_frames.sql` are not backfilled with snapshots; they remain
available through legacy replay mode.

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
