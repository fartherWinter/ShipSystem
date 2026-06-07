# Database Operations

ShipSystem stores training, demonstration, replay, and audit data only. Do not connect the database workflow to real weapon-control, fire-control, electronic-warfare, radar control, or device-command systems.

## Migration Gate

PostgreSQL mode requires the application schema to reach `schema_migrations.name='ship_sim'` version `3`.

On startup, the server:

1. Connects to PostgreSQL.
2. Reads the migration status.
3. Fails before starting HTTP if the current version is lower than the required version.

An empty database or a database without `schema_migrations` is treated as version `0`. A database with only `migrations/001_init.sql` applied is version `1`, and a database with `migrations/002_snapshot_frames.sql` applied is version `2`. All must be migrated to version `3` before the app starts in PostgreSQL mode.

Apply migrations in order:

```powershell
psql $env:SHIP_SIM_DATABASE_URL -f migrations/001_init.sql
psql $env:SHIP_SIM_DATABASE_URL -f migrations/002_snapshot_frames.sql
psql $env:SHIP_SIM_DATABASE_URL -f migrations/003_training_product.sql
```

For Docker Compose, migrations are mounted into `/docker-entrypoint-initdb.d` and run when the database volume is first created. Existing volumes are not re-migrated by the Postgres image; apply new migration files manually before restarting the app.

`003_training_product.sql` is additive. It adds managed scenario metadata, run training metadata, event annotations, and audit log tables. It does not delete, truncate, or rewrite existing replay history. Still take a database backup or platform snapshot before applying migrations to shared, staging, or production databases.

## Destructive Operations

Do not run destructive operations against shared, staging, or production data without a backup or documented preview.

- For retention, call `GET /api/retention/preview` before `POST /api/retention/prune`, or use `scripts/retention.ps1` without `-Apply`.
- For manual SQL maintenance, record the intended SQL, expected row counts, and backup location before running `DELETE`, `DROP`, `TRUNCATE`, or migration rollback commands.
- For production PostgreSQL, take a `pg_dump` or platform snapshot before destructive maintenance.

The Postgres integration test script uses a separate Compose project named `shipsim-test` and removes only that project's test containers and volumes by default. See `docs/retention.md` for retention capacity limits and growth estimates.

## Postgres Integration Tests

Run the store contract against an isolated PostGIS database:

```powershell
.\scripts\test-postgres.ps1
```

To inspect the database after a failed test:

```powershell
.\scripts\test-postgres.ps1 -KeepDatabase
docker compose -p shipsim-test -f docker-compose.test.yml down -v
```

If `make` is available:

```powershell
make postgres-test
```

The script sets:

```text
TEST_DATABASE_URL=postgres://shipsim_test:shipsim-test-only@127.0.0.1:15432/shipsim_test?sslmode=disable
```

Do not point `TEST_DATABASE_URL` at a shared or production database. The store tests create and prune training history as part of the contract.

## Snapshot Writes

`SaveSnapshot` writes a replay frame plus raw contacts, current tracks, and track points in one transaction. The PostgreSQL implementation batches per-contact and per-track inserts with `pgx.Batch` to reduce network round trips.

Runtime snapshot persistence is bounded by `SHIP_SIM_SNAPSHOT_WRITE_TIMEOUT`, default `5s`. When the app provides a deadline, the Postgres transaction derives a local `statement_timeout` from that deadline so database work is bounded by the same write window. Increase the timeout only after measuring load and reviewing database capacity.

Async snapshot queues are intentionally not introduced in this stage. They can improve simulation tick isolation, but they also need queue sizing, backpressure, shutdown draining, and loss accounting before they are suitable for audit-grade replay data.
