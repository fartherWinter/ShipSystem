#!/usr/bin/env python3
"""Preview or prune old ShipSystem operational data."""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_DSN = (
    "host=localhost user=shipsystem password=shipsystem "
    "dbname=shipsystem port=5432 sslmode=disable TimeZone=Asia/Shanghai"
)
DEFAULT_DAYS = 30


@dataclass(frozen=True)
class RetentionPolicy:
    days: int
    max_track_points_per_ship: int
    max_battle_snapshots_per_session: int
    max_battle_events_per_session: int
    max_radar_targets_per_session: int

    @property
    def has_age_cutoff(self) -> bool:
        return self.days > 0


@dataclass(frozen=True)
class MaintenanceStep:
    key: str
    description: str
    preview_sql: str
    prune_sql: str


def main() -> int:
    args = parse_args()
    policy = RetentionPolicy(
        days=args.days,
        max_track_points_per_ship=args.max_track_points_per_ship,
        max_battle_snapshots_per_session=args.max_battle_snapshots_per_session,
        max_battle_events_per_session=args.max_battle_events_per_session,
        max_radar_targets_per_session=args.max_radar_targets_per_session,
    )
    if not any(
        [
            policy.has_age_cutoff,
            policy.max_track_points_per_ship > 0,
            policy.max_battle_snapshots_per_session > 0,
            policy.max_battle_events_per_session > 0,
            policy.max_radar_targets_per_session > 0,
        ]
    ):
        raise SystemExit("At least one retention threshold must be greater than 0")

    dsn = args.dsn or os.environ.get("DATABASE_DSN") or DEFAULT_DSN
    steps = build_steps(policy)
    print(f"mode={'apply' if args.apply else 'preview'}")
    print(f"dsn={mask_dsn(dsn)}")
    print(f"days={policy.days}")
    print(f"steps={len(steps)}")

    psql = args.psql or shutil.which("psql") or "psql"
    for step in steps:
        sql = step.prune_sql if args.apply else step.preview_sql
        value = run_scalar(psql, dsn, sql)
        verb = "deleted" if args.apply else "matched"
        print(f"{step.key}\t{verb}={value}\t{step.description}")
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dsn", default="", help="PostgreSQL DSN. Defaults to DATABASE_DSN or local compose DSN.")
    parser.add_argument("--psql", default="", help="Optional psql executable path.")
    parser.add_argument("--apply", action="store_true", help="Delete matching rows. Omit for preview-only mode.")
    parser.add_argument("--days", type=positive_int_or_zero, default=DEFAULT_DAYS, help="Age cutoff in days. 0 disables age-based pruning.")
    parser.add_argument(
        "--max-track-points-per-ship",
        type=positive_int_or_zero,
        default=0,
        help="Keep only the newest N ship_locations per ship. 0 disables this cap.",
    )
    parser.add_argument(
        "--max-battle-snapshots-per-session",
        type=positive_int_or_zero,
        default=0,
        help="Keep only the newest N battle_snapshots per stopped battle session. 0 disables this cap.",
    )
    parser.add_argument(
        "--max-battle-events-per-session",
        type=positive_int_or_zero,
        default=0,
        help="Keep only the newest N battle_events per stopped battle session. 0 disables this cap.",
    )
    parser.add_argument(
        "--max-radar-targets-per-session",
        type=positive_int_or_zero,
        default=0,
        help="Keep only the newest N radar_targets per stopped battle session. 0 disables this cap.",
    )
    return parser.parse_args()


def positive_int_or_zero(value: str) -> int:
    try:
        parsed = int(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be an integer") from exc
    if parsed < 0:
        raise argparse.ArgumentTypeError("must be greater than or equal to 0")
    return parsed


def build_steps(policy: RetentionPolicy) -> list[MaintenanceStep]:
    steps: list[MaintenanceStep] = []
    if policy.has_age_cutoff:
        steps.extend(age_based_steps(policy.days))
    if policy.max_track_points_per_ship > 0:
        steps.append(cap_step("ship_locations", "ship_id", "reported_at", policy.max_track_points_per_ship, "old ship track points beyond per-ship cap"))
    if policy.max_battle_snapshots_per_session > 0:
        steps.append(
            stopped_session_cap_step(
                "battle_snapshots",
                "snapshot_time",
                policy.max_battle_snapshots_per_session,
                "old battle replay snapshots beyond per-session cap",
            )
        )
    if policy.max_battle_events_per_session > 0:
        steps.append(
            stopped_session_cap_step(
                "battle_events",
                "occurred_at",
                policy.max_battle_events_per_session,
                "old battle events beyond per-session cap",
            )
        )
    if policy.max_radar_targets_per_session > 0:
        steps.append(
            stopped_session_cap_step(
                "radar_targets",
                "scan_time",
                policy.max_radar_targets_per_session,
                "old radar target rows beyond per-session cap",
            )
        )
    return steps


def age_based_steps(days: int) -> list[MaintenanceStep]:
    cutoff = f"now() - interval '{days} days'"
    stopped_sessions = (
        "SELECT session_id FROM battle_sessions "
        f"WHERE status <> 'running' AND COALESCE(stopped_at, updated_at, started_at) < {cutoff}"
    )
    return [
        simple_step(
            "ship_locations_age",
            "ship_locations",
            f"reported_at < {cutoff}",
            "ship track points older than cutoff",
        ),
        simple_step(
            "acked_alarms_age",
            "alarms",
            f"status = 'ACKED' AND COALESCE(ack_at, updated_at, created_at) < {cutoff}",
            "acknowledged alarms older than cutoff; OPEN alarms are retained",
        ),
        simple_step(
            "dispatch_event_logs_age",
            "dispatch_event_logs",
            f"created_at < {cutoff}",
            "dispatch status transition logs older than cutoff",
        ),
        simple_step(
            "closed_dispatch_events_age",
            "dispatch_events",
            f"status IN ('COMPLETED', 'CANCELLED') AND updated_at < {cutoff}",
            "closed dispatch events older than cutoff",
        ),
        session_child_step(
            "battle_snapshots_age",
            "battle_snapshots",
            stopped_sessions,
            "snapshots for stopped battle sessions older than cutoff",
        ),
        session_child_step(
            "battle_events_age",
            "battle_events",
            stopped_sessions,
            "battle events for stopped battle sessions older than cutoff",
        ),
        session_child_step(
            "radar_targets_age",
            "radar_targets",
            stopped_sessions,
            "radar targets for stopped battle sessions older than cutoff",
        ),
        simple_step(
            "battle_projectiles_age",
            "battle_projectiles",
            f"session_id IN ({stopped_sessions})",
            "projectiles for stopped battle sessions older than cutoff",
        ),
        simple_step(
            "battle_units_age",
            "battle_units",
            f"session_id IN ({stopped_sessions})",
            "units for stopped battle sessions older than cutoff",
        ),
        simple_step(
            "battle_sessions_age",
            "battle_sessions",
            f"session_id IN ({stopped_sessions})",
            "stopped battle session rows older than cutoff after children are removed",
        ),
    ]


def simple_step(key: str, table: str, where_sql: str, description: str) -> MaintenanceStep:
    return MaintenanceStep(
        key=key,
        description=description,
        preview_sql=f"SELECT COUNT(*) FROM {table} WHERE {where_sql};",
        prune_sql=f"WITH deleted AS (DELETE FROM {table} WHERE {where_sql} RETURNING 1) SELECT COUNT(*) FROM deleted;",
    )


def session_child_step(key: str, table: str, session_sql: str, description: str) -> MaintenanceStep:
    where_sql = f"session_id IN ({session_sql})"
    return simple_step(key, table, where_sql, description)


def cap_step(table: str, partition_column: str, order_column: str, max_rows: int, description: str) -> MaintenanceStep:
    ranked = (
        f"SELECT id FROM ("
        f"SELECT id, row_number() OVER (PARTITION BY {partition_column} ORDER BY {order_column} DESC, id DESC) AS rn "
        f"FROM {table}"
        f") ranked WHERE rn > {max_rows}"
    )
    return MaintenanceStep(
        key=f"{table}_cap",
        description=description,
        preview_sql=f"SELECT COUNT(*) FROM ({ranked}) rows_to_prune;",
        prune_sql=f"WITH rows_to_prune AS ({ranked}), deleted AS (DELETE FROM {table} WHERE id IN (SELECT id FROM rows_to_prune) RETURNING 1) SELECT COUNT(*) FROM deleted;",
    )


def stopped_session_cap_step(table: str, order_column: str, max_rows: int, description: str) -> MaintenanceStep:
    ranked = (
        f"SELECT item.id FROM ("
        f"SELECT {table}.id, row_number() OVER (PARTITION BY {table}.session_id ORDER BY {table}.{order_column} DESC, {table}.id DESC) AS rn "
        f"FROM {table} "
        f"JOIN battle_sessions ON battle_sessions.session_id = {table}.session_id "
        f"WHERE battle_sessions.status <> 'running'"
        f") item WHERE item.rn > {max_rows}"
    )
    return MaintenanceStep(
        key=f"{table}_cap",
        description=description,
        preview_sql=f"SELECT COUNT(*) FROM ({ranked}) rows_to_prune;",
        prune_sql=f"WITH rows_to_prune AS ({ranked}), deleted AS (DELETE FROM {table} WHERE id IN (SELECT id FROM rows_to_prune) RETURNING 1) SELECT COUNT(*) FROM deleted;",
    )


def run_scalar(psql: str, dsn: str, sql: str) -> str:
    command = [psql, dsn, "-X", "-q", "-t", "-A", "-v", "ON_ERROR_STOP=1", "-c", sql]
    try:
        completed = subprocess.run(
            command,
            cwd=ROOT,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
        )
    except FileNotFoundError as exc:
        missing = exc.filename or psql
        raise SystemExit(f"[FAIL] retention query could not start command: {missing}") from exc
    if completed.returncode != 0:
        detail = completed.stderr.strip() or completed.stdout.strip() or "psql failed"
        raise SystemExit(f"[FAIL] retention query failed: {detail}")
    return completed.stdout.strip()


def mask_dsn(dsn: str) -> str:
    parts = dsn.split()
    masked: list[str] = []
    for part in parts:
        if part.lower().startswith("password="):
            masked.append("password=***")
        else:
            masked.append(part)
    return " ".join(masked)


if __name__ == "__main__":
    sys.exit(main())
