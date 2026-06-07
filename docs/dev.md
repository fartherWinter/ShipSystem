# Development Guide

ShipSystem is a training, demonstration, replay, and audit simulator. Keep it inside that boundary: do not connect it to real weapon-control, fire-control, electronic-warfare, or live radar control chains.

## Prerequisites

- Go 1.23 or newer.
- Node.js 20 or newer with npm.
- Docker and Docker Compose for container and PostgreSQL/PostGIS workflows.
- Optional: `make` for the root workflow shortcuts.

## Local Backend

Run the backend in in-memory demo mode:

```powershell
go run ./cmd/sim-server
```

Useful local settings:

```powershell
$env:SHIP_SIM_SCENARIO_DIR="scenarios"
$env:SHIP_SIM_ALLOWED_ORIGINS="http://127.0.0.1:5173,http://localhost:5173"
$env:SHIP_SIM_REQUEST_BODY_LIMIT="1048576"
```

Production mode must not run without authentication. For a simple demo or single-user deployment, token auth can be enabled with `SHIP_SIM_AUTH_MODE=token` and `SHIP_SIM_AUTH_TOKEN`; this is not a multi-user production authentication design. Proxy auth is intended for deployments where a trusted reverse proxy performs authentication and passes a sanitized user header.

Do not pass long-lived credentials through URLs. The backend rejects `access_token` as a normal authentication path. Browser report export must use `fetch` plus Blob download so `Authorization` stays in headers. Authenticated WebSocket connections must first call `POST /api/runs/{run_id}/ws-ticket` and then connect with the returned short-lived one-time `ticket` query parameter.

For `SHIP_SIM_AUTH_MODE=proxy`, the reverse proxy must remove any client-supplied copy of `SHIP_SIM_AUTH_USER_HEADER` before setting the trusted value. Do not expose the app directly to the public internet in proxy-auth mode, because the app trusts that header after the proxy has authenticated the user.

All HTTP responses should keep the baseline security headers enabled: `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and `X-Frame-Options: DENY`.

## Frontend

Install dependencies and start Vite:

```powershell
cd web
Copy-Item .env.example .env.local
npm ci
npm run dev
```

The default frontend development URL is `http://127.0.0.1:5173`. Keep `SHIP_SIM_ALLOWED_ORIGINS` aligned with the browser origin used during development.

Frontend runtime settings are Vite build-time variables:

- `VITE_API_BASE`: backend API origin, default `http://localhost:8080`.
- `VITE_AUTH_MODE`: UI mode for authentication prompts, one of `off`, `token`, or `proxy`.
- `VITE_MAP_TILE_URL`: MapLibre raster tile template, for example `https://tile.openstreetmap.org/{z}/{x}/{y}.png` or an internal tile server.
- `VITE_MAP_TILE_ATTRIBUTION`: attribution text shown by MapLibre.

For disconnected or intranet deployments, set `VITE_MAP_TILE_URL` to a local tile server before running `npm run build` or `docker compose build`. If map tiles fail to load, the UI shows a map error overlay instead of leaving the map blank.

## Database

Without `SHIP_SIM_DATABASE_URL` or `DATABASE_URL`, the backend uses in-memory storage for demo runs.

For PostgreSQL/PostGIS mode, create a database and apply migrations in order:

```powershell
$env:SHIP_SIM_DATABASE_URL="postgres://user:password@localhost:5432/shipsim?sslmode=disable"
psql $env:SHIP_SIM_DATABASE_URL -f migrations/001_init.sql
psql $env:SHIP_SIM_DATABASE_URL -f migrations/002_snapshot_frames.sql
psql $env:SHIP_SIM_DATABASE_URL -f migrations/003_training_product.sql
go run ./cmd/sim-server
```

PostgreSQL mode has a startup migration gate. The app requires `schema_migrations.name='ship_sim'` to be at the current version before it starts HTTP. Empty, v1, and v2 databases fail clearly instead of running in a half-migrated state.

Do not run destructive database operations against shared or production data without first taking a backup or using a documented preview/dry-run path. Retention pruning supports a preview API; use it before manual pruning.

Run the optional Postgres integration test against an isolated Docker database:

```powershell
.\scripts\test-postgres.ps1
```

The script uses the `shipsim-test` Compose project and removes only that test project's containers and volume by default. See `docs/database.md` for details.

Retention preview and capacity smoke commands:

```powershell
.\scripts\retention.ps1 -Days 30
.\scripts\run-capacity-smoke.ps1 -EstimateOnly
```

See `docs/retention.md` for background retention, destructive-operation preview rules, and sizing estimates.

Observability endpoints:

```powershell
Invoke-RestMethod -Uri http://localhost:8080/healthz
Invoke-RestMethod -Uri http://localhost:8080/readyz
Invoke-WebRequest -Uri http://localhost:8080/metrics/prometheus
```

See `docs/observability.md` for metrics, probe authentication, and log field guidance.

API contract and generated frontend types:

```powershell
cd web
npm run generate:types
```

The source contract is `docs/openapi.json`; generated frontend API types are written to `web/src/generated/api-types.ts`. See `docs/api.md` for versioning and error-code policy.

Training product workflow APIs support managed scenario upload/copy/enable/disable, run tags, trainees, instructor notes, event annotations, abstract training assessment, JSON/CSV/HTML/PDF report templates, and persisted audit logs. See `docs/training-product.md` before changing those workflows.

## Unified Commands

With `make` available from the repository root:

```powershell
make dev
make test
make build
make lint
make docker-build
make postgres-test
make api-types
```

The targets expand to the same underlying commands:

- `test`: `go test ./...` and `npm test` in `web/`.
- `build`: Go backend build and `npm run build` in `web/`.
- `lint`: `go vet ./...` and frontend type checking.
- `docker-build`: local Docker image build, default tag `shipsim:local`.
- `postgres-test`: isolated PostGIS store contract test using `docker-compose.test.yml`.
- `api-types`: generate frontend TypeScript API types from `docs/openapi.json`.
- `dev`: backend and Vite frontend development servers.

On systems without `make`, run the underlying Go, npm, and Docker commands directly.

## Docker Compose

For a local Compose demo:

```powershell
Copy-Item .env.example .env
docker compose up --build
```

Before any non-local deployment, replace all placeholder secrets, set a narrow `SHIP_SIM_ALLOWED_ORIGINS`, and verify the authentication mode. Do not commit real secrets; commit only `.env.example` style templates.

## Generated Artifacts

Keep generated output, logs, coverage files, temporary verification images, and local environment files out of git. The repository intentionally ignores `outputs/`, `logs/`, `*.log`, coverage output, temporary directories, dependency directories, and build directories. Commit source, migrations, documentation, scenarios, and small intentional examples only.
