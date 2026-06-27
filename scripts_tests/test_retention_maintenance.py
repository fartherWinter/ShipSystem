import argparse
import importlib.util
import sys
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "retention_maintenance.py"
SPEC = importlib.util.spec_from_file_location("retention_maintenance_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def completed(stdout: str = "", stderr: str = "", returncode: int = 0):
    return MODULE.subprocess.CompletedProcess(args=["psql"], returncode=returncode, stdout=stdout, stderr=stderr)


class RetentionMaintenanceTest(unittest.TestCase):
    def test_age_policy_builds_conservative_preview_steps(self) -> None:
        steps = MODULE.build_steps(
            MODULE.RetentionPolicy(
                days=14,
                max_track_points_per_ship=0,
                max_battle_snapshots_per_session=0,
                max_battle_events_per_session=0,
                max_radar_targets_per_session=0,
            )
        )

        keys = [step.key for step in steps]

        self.assertIn("ship_locations_age", keys)
        self.assertIn("acked_alarms_age", keys)
        self.assertIn("battle_sessions_age", keys)
        self.assertNotIn("open_alarms_age", keys)
        self.assertIn("status = 'ACKED'", steps[1].preview_sql)
        self.assertIn("status <> 'running'", "\n".join(step.preview_sql for step in steps))
        self.assertIn("session_id IN (SELECT session_id FROM battle_sessions", "\n".join(step.preview_sql for step in steps))

    def test_cap_policy_builds_per_group_window_queries(self) -> None:
        steps = MODULE.build_steps(
            MODULE.RetentionPolicy(
                days=0,
                max_track_points_per_ship=500,
                max_battle_snapshots_per_session=100,
                max_battle_events_per_session=50,
                max_radar_targets_per_session=25,
            )
        )

        by_key = {step.key: step for step in steps}

        self.assertIn("ship_locations_cap", by_key)
        self.assertIn("PARTITION BY ship_id", by_key["ship_locations_cap"].preview_sql)
        self.assertIn("rn > 500", by_key["ship_locations_cap"].preview_sql)
        self.assertIn("battle_sessions.status <> 'running'", by_key["battle_snapshots_cap"].preview_sql)
        self.assertIn("battle_sessions.status <> 'running'", by_key["battle_events_cap"].preview_sql)
        self.assertIn("battle_sessions.status <> 'running'", by_key["radar_targets_cap"].preview_sql)

    def test_prune_sql_returns_deleted_count(self) -> None:
        step = MODULE.simple_step("acked_alarms_age", "alarms", "status = 'ACKED'", "old acked alarms")

        self.assertIn("DELETE FROM alarms", step.prune_sql)
        self.assertIn("RETURNING 1", step.prune_sql)
        self.assertTrue(step.prune_sql.endswith("SELECT COUNT(*) FROM deleted;"))

    def test_mask_dsn_redacts_password(self) -> None:
        dsn = "host=localhost user=shipsystem password=secret dbname=shipsystem"

        self.assertEqual(
            "host=localhost user=shipsystem password=*** dbname=shipsystem",
            MODULE.mask_dsn(dsn),
        )

    def test_positive_int_or_zero_rejects_negative_values(self) -> None:
        self.assertEqual(0, MODULE.positive_int_or_zero("0"))
        self.assertEqual(30, MODULE.positive_int_or_zero("30"))

        with self.assertRaises(argparse.ArgumentTypeError):
            MODULE.positive_int_or_zero("-1")

    def test_run_scalar_uses_psql_with_error_stop(self) -> None:
        with mock.patch.object(MODULE.subprocess, "run", return_value=completed(stdout="42\n")) as run:
            value = MODULE.run_scalar("psql", "host=localhost password=secret", "SELECT 42;")

        self.assertEqual("42", value)
        command = run.call_args.args[0]
        self.assertEqual("psql", command[0])
        self.assertIn("-v", command)
        self.assertIn("ON_ERROR_STOP=1", command)
        self.assertIn("SELECT 42;", command)

    def test_run_scalar_reports_psql_failure(self) -> None:
        with mock.patch.object(MODULE.subprocess, "run", return_value=completed(stderr="syntax error", returncode=1)):
            with self.assertRaises(SystemExit) as ctx:
                MODULE.run_scalar("psql", "host=localhost", "SELECT broken;")

        self.assertIn("retention query failed", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
