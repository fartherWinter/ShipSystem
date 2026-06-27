#!/usr/bin/env python3
"""Run an isolated PostgreSQL backup/restore drill against a temporary PostGIS container."""

from __future__ import annotations

import argparse
import os
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
TEMP_DIR = ROOT / ".docker-codex"
DEFAULT_PROJECT = "shipsystem-backup-drill"
DEFAULT_DB_NAME = "shipsystem_backup_drill"
DEFAULT_RESTORE_DB_NAME = "shipsystem_backup_drill_restore"
DEFAULT_DB_USER = "shipsystem"
DEFAULT_DB_PASSWORD = "shipsystem"
DEFAULT_HOST_PORT = 16432
CONTAINER_PORT = 5432
READY_TIMEOUT_SECONDS = 60
SERVICE_NAME = "postgres-drill"
SAMPLE_SHIP_ID = 1001
SAMPLE_LOCATION_ID = 2001
SAMPLE_ALARM_ID = 3001
SAMPLE_DISPATCH_EVENT_ID = 4001
SAMPLE_SESSION_ID = "backup-drill-session"
SAMPLE_RADAR_TARGET_ID = 6001
SAMPLE_BATTLE_EVENT_ID = 7001
SAMPLE_BATTLE_SNAPSHOT_ID = 8001


def main() -> int:
    args = parse_args()
    ensure_tool("docker")
    ensure_tool("go")

    host_port = choose_port(args.port)
    compose_file = Path(args.compose_file).resolve()
    compose_file.parent.mkdir(parents=True, exist_ok=True)
    dump_file = Path(args.dump_file).resolve()
    dump_file.parent.mkdir(parents=True, exist_ok=True)
    project = args.project

    dsn = build_dsn(host_port, args.db_name, args.db_user, args.db_password)
    restore_dsn = build_dsn(host_port, args.restore_db_name, args.db_user, args.db_password)

    compose_file.write_text(
        build_compose_yaml(
            host_port=host_port,
            db_name=args.db_name,
            db_user=args.db_user,
            db_password=args.db_password,
        ),
        encoding="utf-8",
    )

    print(f"compose_file={compose_file}")
    print(f"project={project}")
    print(f"dsn={mask_dsn(dsn)}")
    print(f"restore_dsn={mask_dsn(restore_dsn)}")
    print(f"dump_file={dump_file}")

    keep_database = args.keep_database
    keep_artifacts = args.keep_artifacts
    try:
        cleanup_previous_resources(project, compose_file)
        run_compose(project, compose_file, ["up", "-d", SERVICE_NAME])
        wait_for_postgres(project, compose_file, args.db_user, args.db_name, args.db_password, timeout_seconds=args.ready_timeout)

        run_migrations(dsn, action="up")
        run_seed_sample(project, compose_file, args.db_name, args.db_user, args.db_password)

        source_counts = query_table_counts(project, compose_file, args.db_name, args.db_user, args.db_password)
        print_counts("source_counts", source_counts)

        dump_bytes = dump_database(project, compose_file, args.db_name, args.db_user, args.db_password)
        dump_file.write_bytes(dump_bytes)
        print(f"dump_bytes={len(dump_bytes)}")

        recreate_database(project, compose_file, args.restore_db_name, args.db_user, args.db_password)
        restore_database(project, compose_file, args.restore_db_name, args.db_user, args.db_password, dump_bytes)
        run_migrations(restore_dsn, action="check")

        restore_counts = query_table_counts(project, compose_file, args.restore_db_name, args.db_user, args.db_password)
        print_counts("restore_counts", restore_counts)
        compare_counts(source_counts, restore_counts)
        verify_restored_sample(project, compose_file, args.restore_db_name, args.db_user, args.db_password)

        print("backup restore drill passed")
        return 0
    finally:
        if not keep_database:
            down_result = run_compose(project, compose_file, ["down", "-v"], check=False)
            if down_result.returncode != 0:
                print("[WARN] failed to clean up temporary backup drill database", file=sys.stderr)
        if not keep_artifacts:
            unlink_if_exists(dump_file)
            unlink_if_exists(compose_file)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", default=DEFAULT_PROJECT, help="Docker Compose project name")
    parser.add_argument("--port", type=int, default=DEFAULT_HOST_PORT, help="Preferred published PostgreSQL port")
    parser.add_argument("--db-name", default=DEFAULT_DB_NAME, help="Source drill database name")
    parser.add_argument("--restore-db-name", default=DEFAULT_RESTORE_DB_NAME, help="Restore target database name")
    parser.add_argument("--db-user", default=DEFAULT_DB_USER, help="Temporary database user")
    parser.add_argument("--db-password", default=DEFAULT_DB_PASSWORD, help="Temporary database password")
    parser.add_argument(
        "--compose-file",
        default=str(TEMP_DIR / "docker-compose.backup-drill.yml"),
        help="Path to the generated temporary compose file",
    )
    parser.add_argument(
        "--dump-file",
        default=str(TEMP_DIR / "backup-drill.dump"),
        help="Path to the temporary pg_dump custom-format artifact",
    )
    parser.add_argument("--keep-database", action="store_true", help="Keep the temporary database container and volume after the drill")
    parser.add_argument("--keep-artifacts", action="store_true", help="Keep the generated compose file and dump artifact")
    parser.add_argument("--ready-timeout", type=int, default=READY_TIMEOUT_SECONDS, help="Seconds to wait for PostgreSQL readiness")
    return parser.parse_args()


def ensure_tool(name: str) -> None:
    if shutil.which(name) is None:
        raise SystemExit(f"[FAIL] required tool not found in PATH: {name}")


def choose_port(preferred: int, search_window: int = 50) -> int:
    if preferred > 0 and port_is_free(preferred):
        return preferred
    for offset in range(1, search_window + 1):
        candidate = preferred + offset
        if port_is_free(candidate):
            return candidate
    raise SystemExit(f"[FAIL] no free port found near {preferred}")


def port_is_free(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.2)
        return sock.connect_ex(("127.0.0.1", port)) != 0


def build_compose_yaml(*, host_port: int, db_name: str, db_user: str, db_password: str) -> str:
    return (
        "services:\n"
        f"  {SERVICE_NAME}:\n"
        "    image: postgis/postgis:17-3.5\n"
        "    environment:\n"
        f"      POSTGRES_DB: {db_name}\n"
        f"      POSTGRES_USER: {db_user}\n"
        f"      POSTGRES_PASSWORD: {db_password}\n"
        "      TZ: Asia/Shanghai\n"
        "    ports:\n"
        f"      - \"{host_port}:{CONTAINER_PORT}\"\n"
        "    volumes:\n"
        "      - backup_drill_postgres_data:/var/lib/postgresql/data\n"
        "      - ./backend/migrations:/docker-entrypoint-initdb.d:ro\n"
        "    healthcheck:\n"
        f"      test: [\"CMD-SHELL\", \"pg_isready -U {db_user} -d {db_name}\"]\n"
        "      interval: 5s\n"
        "      timeout: 5s\n"
        "      retries: 12\n"
        "volumes:\n"
        "  backup_drill_postgres_data:\n"
    )


def build_dsn(host_port: int, db_name: str, db_user: str, db_password: str) -> str:
    return (
        f"host=127.0.0.1 user={db_user} password={db_password} dbname={db_name} "
        f"port={host_port} sslmode=disable TimeZone=Asia/Shanghai"
    )


def mask_dsn(dsn: str) -> str:
    parts = []
    for item in dsn.split():
        if item.startswith("password="):
            parts.append("password=***")
        else:
            parts.append(item)
    return " ".join(parts)


def build_seed_sql() -> str:
    return f"""
BEGIN;
INSERT INTO ships (id, name, mmsi, ship_type, flag, status, created_at, updated_at)
VALUES ({SAMPLE_SHIP_ID}, 'Backup Drill Ship', '123456789', 'frigate', 'CN', 'active', now(), now());

INSERT INTO ship_locations (id, ship_id, longitude, latitude, speed_knots, course, reported_at, created_at, geom)
VALUES (
  {SAMPLE_LOCATION_ID},
  {SAMPLE_SHIP_ID},
  121.4900,
  31.2300,
  18.5,
  90,
  now(),
  now(),
  ST_SetSRID(ST_MakePoint(121.4900, 31.2300), 4326)::geography
);

INSERT INTO alarms (id, ship_id, type, level, title, message, status, created_at, updated_at)
VALUES ({SAMPLE_ALARM_ID}, {SAMPLE_SHIP_ID}, 'TEST', 'WARN', 'Backup Drill Alarm', 'backup restore verification', 'ACKED', now(), now());

INSERT INTO dispatch_events (id, ship_id, title, description, status, priority, created_at, updated_at)
VALUES ({SAMPLE_DISPATCH_EVENT_ID}, {SAMPLE_SHIP_ID}, 'Backup Drill Dispatch', 'backup restore verification', 'COMPLETED', 'normal', now(), now());

INSERT INTO battle_sessions (id, session_id, name, scenario_code, status, started_at, stopped_at, created_at, updated_at)
VALUES (5001, '{SAMPLE_SESSION_ID}', 'Backup Drill Session', 'open-water-duel', 'stopped', now(), now(), now(), now());

INSERT INTO radar_targets (id, session_id, radar_id, target_id, side, longitude, latitude, course, speed_knots, confidence, detected, scan_time, created_at)
VALUES ({SAMPLE_RADAR_TARGET_ID}, '{SAMPLE_SESSION_ID}', 'radar-1', 'target-1', 'RED', 122.1000, 30.9000, 180, 22.5, 0.95, true, now(), now());

INSERT INTO battle_events (id, session_id, event_id, type, severity, message, source_unit_id, target_unit_id, occurred_at, created_at)
VALUES ({SAMPLE_BATTLE_EVENT_ID}, '{SAMPLE_SESSION_ID}', 'event-1', 'SCAN', 'INFO', 'backup restore event', 'blue-1', 'red-1', now(), now());

INSERT INTO battle_snapshots (id, session_id, tick, snapshot_time, units_json, projectiles_json, radar_targets_json, events_json, created_at)
VALUES (
  {SAMPLE_BATTLE_SNAPSHOT_ID},
  '{SAMPLE_SESSION_ID}',
  7,
  now(),
  '[{{"unitId":"blue-1","side":"BLUE"}}]'::jsonb,
  '[]'::jsonb,
  '[{{"targetId":"target-1"}}]'::jsonb,
  '[{{"eventId":"event-1"}}]'::jsonb,
  now()
);
COMMIT;
""".strip()


def build_table_counts_sql() -> str:
    return """
SELECT key || E'\t' || value
FROM (
  SELECT 'alarms' AS key, COUNT(*)::bigint AS value FROM alarms
  UNION ALL SELECT 'battle_events', COUNT(*)::bigint FROM battle_events
  UNION ALL SELECT 'battle_sessions', COUNT(*)::bigint FROM battle_sessions
  UNION ALL SELECT 'battle_snapshots', COUNT(*)::bigint FROM battle_snapshots
  UNION ALL SELECT 'dispatch_events', COUNT(*)::bigint FROM dispatch_events
  UNION ALL SELECT 'radar_targets', COUNT(*)::bigint FROM radar_targets
  UNION ALL SELECT 'schema_migrations', COUNT(*)::bigint FROM schema_migrations
  UNION ALL SELECT 'ship_locations', COUNT(*)::bigint FROM ship_locations
  UNION ALL SELECT 'ships', COUNT(*)::bigint FROM ships
) counts
ORDER BY key;
""".strip()


def build_restore_verification_sql() -> str:
    return f"""
SELECT COUNT(*)::bigint
FROM (
  SELECT 1 FROM ships WHERE id = {SAMPLE_SHIP_ID} AND name = 'Backup Drill Ship'
  UNION ALL
  SELECT 1 FROM ship_locations WHERE id = {SAMPLE_LOCATION_ID} AND geom IS NOT NULL
  UNION ALL
  SELECT 1 FROM alarms WHERE id = {SAMPLE_ALARM_ID} AND status = 'ACKED'
  UNION ALL
  SELECT 1 FROM dispatch_events WHERE id = {SAMPLE_DISPATCH_EVENT_ID} AND status = 'COMPLETED'
  UNION ALL
  SELECT 1 FROM battle_snapshots WHERE id = {SAMPLE_BATTLE_SNAPSHOT_ID} AND tick = 7 AND jsonb_array_length(units_json) = 1
) checks;
""".strip()


def tool_path(name: str) -> str:
    return shutil.which(name) or name


def compose_base_command(project: str, compose_file: Path) -> list[str]:
    return [tool_path("docker"), "compose", "-p", project, "-f", str(compose_file)]


def compose_exec_command(project: str, compose_file: Path, db_password: str, args: list[str]) -> list[str]:
    return compose_base_command(project, compose_file) + ["exec", "-T", "-e", f"PGPASSWORD={db_password}", SERVICE_NAME] + args


def run_compose(project: str, compose_file: Path, args: list[str], *, check: bool = True) -> subprocess.CompletedProcess[str]:
    command = compose_base_command(project, compose_file) + args
    completed = subprocess.run(command, cwd=ROOT, check=False, text=True, encoding="utf-8", errors="replace")
    if check and completed.returncode != 0:
        raise SystemExit(completed.returncode)
    return completed


def cleanup_previous_resources(project: str, compose_file: Path) -> None:
    run_compose(project, compose_file, ["down", "-v", "--remove-orphans"], check=False)


def wait_for_postgres(project: str, compose_file: Path, db_user: str, db_name: str, db_password: str, *, timeout_seconds: int) -> None:
    deadline = time.monotonic() + timeout_seconds
    command = compose_exec_command(
        project,
        compose_file,
        db_password,
        ["pg_isready", "-h", "127.0.0.1", "-U", db_user, "-d", db_name],
    )
    while time.monotonic() < deadline:
        completed = subprocess.run(command, cwd=ROOT, check=False, capture_output=True, text=True, encoding="utf-8", errors="replace")
        if completed.returncode == 0:
            return
        time.sleep(1)
    print_service_logs(project, compose_file)
    raise SystemExit("[FAIL] temporary PostGIS backup drill database did not become ready in time")


def run_migrations(dsn: str, *, action: str) -> None:
    env = os.environ.copy()
    env["APP_ENV"] = "development"
    env["DATABASE_DSN"] = dsn
    command = [tool_path("go"), "run", "./cmd/migrate", f"-action={action}"]
    completed = subprocess.run(command, cwd=ROOT / "backend", env=env, check=False, capture_output=True, text=True, encoding="utf-8", errors="replace")
    if completed.stdout:
        print(completed.stdout, end="" if completed.stdout.endswith("\n") else "\n")
    if completed.returncode != 0:
        if completed.stderr:
            print(completed.stderr, end="" if completed.stderr.endswith("\n") else "\n", file=sys.stderr)
        raise SystemExit(f"[FAIL] migration {action} failed for {mask_dsn(dsn)}")


def run_seed_sample(project: str, compose_file: Path, db_name: str, db_user: str, db_password: str) -> None:
    run_psql(project, compose_file, db_name, db_user, db_password, build_seed_sql())


def query_table_counts(project: str, compose_file: Path, db_name: str, db_user: str, db_password: str) -> dict[str, int]:
    output = run_psql(project, compose_file, db_name, db_user, db_password, build_table_counts_sql(), raw=True)
    return parse_count_lines(output)


def parse_count_lines(output: str) -> dict[str, int]:
    counts: dict[str, int] = {}
    for raw_line in output.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        parts = line.split("\t")
        if len(parts) != 2:
            raise SystemExit(f"[FAIL] unexpected count row: {line}")
        counts[parts[0]] = int(parts[1])
    return counts


def print_counts(label: str, counts: dict[str, int]) -> None:
    summary = ", ".join(f"{key}={counts[key]}" for key in sorted(counts))
    print(f"{label}: {summary}")


def compare_counts(source_counts: dict[str, int], restore_counts: dict[str, int]) -> None:
    if source_counts != restore_counts:
        raise SystemExit(f"[FAIL] restore counts mismatch: source={source_counts} restore={restore_counts}")


def verify_restored_sample(project: str, compose_file: Path, db_name: str, db_user: str, db_password: str) -> None:
    output = run_psql(project, compose_file, db_name, db_user, db_password, build_restore_verification_sql(), raw=True)
    if output.strip() != "5":
        raise SystemExit(f"[FAIL] restore sample verification failed: expected 5 checks, got {output.strip()!r}")


def dump_database(project: str, compose_file: Path, db_name: str, db_user: str, db_password: str) -> bytes:
    command = compose_exec_command(
        project,
        compose_file,
        db_password,
        ["pg_dump", "-Fc", "-h", "127.0.0.1", "-U", db_user, "-d", db_name],
    )
    completed = subprocess.run(command, cwd=ROOT, check=False, capture_output=True)
    if completed.returncode != 0:
        stderr = completed.stderr.decode("utf-8", errors="replace")
        raise SystemExit(f"[FAIL] pg_dump failed: {stderr.strip()}")
    return completed.stdout


def recreate_database(project: str, compose_file: Path, db_name: str, db_user: str, db_password: str) -> None:
    for args in (
        ["dropdb", "-h", "127.0.0.1", "-U", db_user, "--if-exists", db_name],
        ["createdb", "-h", "127.0.0.1", "-U", db_user, db_name],
    ):
        command = compose_exec_command(project, compose_file, db_password, args)
        completed = subprocess.run(command, cwd=ROOT, check=False, capture_output=True, text=True, encoding="utf-8", errors="replace")
        if completed.returncode != 0:
            raise SystemExit(f"[FAIL] {' '.join(args[:1])} failed for {db_name}: {completed.stderr.strip()}")


def restore_database(project: str, compose_file: Path, db_name: str, db_user: str, db_password: str, dump_bytes: bytes) -> None:
    command = compose_exec_command(
        project,
        compose_file,
        db_password,
        [
            "pg_restore",
            "-h",
            "127.0.0.1",
            "-U",
            db_user,
            "-d",
            db_name,
            "--clean",
            "--if-exists",
            "--no-owner",
            "--no-privileges",
        ],
    )
    completed = subprocess.run(command, cwd=ROOT, check=False, capture_output=True, input=dump_bytes)
    if completed.returncode != 0:
        stderr = completed.stderr.decode("utf-8", errors="replace")
        raise SystemExit(f"[FAIL] pg_restore failed: {stderr.strip()}")


def run_psql(
    project: str,
    compose_file: Path,
    db_name: str,
    db_user: str,
    db_password: str,
    sql: str,
    *,
    raw: bool = False,
) -> str:
    command = compose_exec_command(
        project,
        compose_file,
        db_password,
        ["psql", "-h", "127.0.0.1", "-U", db_user, "-d", db_name, "-v", "ON_ERROR_STOP=1", "-At", "-c", sql],
    )
    completed = subprocess.run(command, cwd=ROOT, check=False, capture_output=True, text=True, encoding="utf-8", errors="replace")
    if completed.returncode != 0:
        raise SystemExit(f"[FAIL] psql command failed for {db_name}: {completed.stderr.strip()}")
    if raw:
        return completed.stdout
    return completed.stdout.strip()


def unlink_if_exists(path: Path) -> None:
    if path.exists():
        path.unlink()


def print_service_logs(project: str, compose_file: Path) -> None:
    command = compose_base_command(project, compose_file) + ["logs", "--no-color", SERVICE_NAME]
    completed = subprocess.run(command, cwd=ROOT, check=False, capture_output=True, text=True, encoding="utf-8", errors="replace")
    if completed.stdout:
        print("service_logs:")
        print(completed.stdout, end="" if completed.stdout.endswith("\n") else "\n")
    if completed.stderr:
        print(completed.stderr, end="" if completed.stderr.endswith("\n") else "\n", file=sys.stderr)


if __name__ == "__main__":
    raise SystemExit(main())
