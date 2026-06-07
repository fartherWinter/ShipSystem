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

## Frontend

Install dependencies and start Vite:

```powershell
cd web
npm ci
npm run dev
```

The default frontend development URL is `http://127.0.0.1:5173`. Keep `SHIP_SIM_ALLOWED_ORIGINS` aligned with the browser origin used during development.

## Database

Without `SHIP_SIM_DATABASE_URL` or `DATABASE_URL`, the backend uses in-memory storage for demo runs.

For PostgreSQL/PostGIS mode, create a database and apply migrations in order:

```powershell
$env:SHIP_SIM_DATABASE_URL="postgres://user:password@localhost:5432/shipsim?sslmode=disable"
psql $env:SHIP_SIM_DATABASE_URL -f migrations/001_init.sql
psql $env:SHIP_SIM_DATABASE_URL -f migrations/002_snapshot_frames.sql
go run ./cmd/sim-server
```

Do not run destructive database operations against shared or production data without first taking a backup or using a documented preview/dry-run path. Retention pruning supports a preview API; use it before manual pruning.

## Unified Commands

With `make` available from the repository root:

```powershell
make dev
make test
make build
make lint
make docker-build
```

The targets expand to the same underlying commands:

- `test`: `go test ./...` and `npm test` in `web/`.
- `build`: Go backend build and `npm run build` in `web/`.
- `lint`: `go vet ./...` and frontend type checking.
- `docker-build`: local Docker image build, default tag `shipsim:local`.
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
