import importlib.util
import sys
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "run_backup_restore_drill.py"
SPEC = importlib.util.spec_from_file_location("run_backup_restore_drill_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def completed(*, stdout="", stderr="", returncode=0, args=None):
    return MODULE.subprocess.CompletedProcess(args=args or ["docker"], returncode=returncode, stdout=stdout, stderr=stderr)


class RunBackupRestoreDrillTest(unittest.TestCase):
    def test_build_compose_yaml_contains_expected_service(self) -> None:
        yaml_text = MODULE.build_compose_yaml(
            host_port=16432,
            db_name="shipsystem_backup_drill",
            db_user="shipsystem",
            db_password="shipsystem",
        )

        self.assertIn("postgres-drill", yaml_text)
        self.assertIn("postgis/postgis:17-3.5", yaml_text)
        self.assertIn('"16432:5432"', yaml_text)
        self.assertIn("./backend/migrations:/docker-entrypoint-initdb.d:ro", yaml_text)

    def test_mask_dsn_redacts_password(self) -> None:
        dsn = "host=127.0.0.1 user=shipsystem password=secret dbname=test"

        self.assertEqual(
            "host=127.0.0.1 user=shipsystem password=*** dbname=test",
            MODULE.mask_dsn(dsn),
        )

    def test_parse_count_lines_reads_tab_delimited_rows(self) -> None:
        counts = MODULE.parse_count_lines("alarms\t1\nships\t2\n")

        self.assertEqual({"alarms": 1, "ships": 2}, counts)

    def test_compare_counts_rejects_mismatch(self) -> None:
        with self.assertRaises(SystemExit) as ctx:
            MODULE.compare_counts({"ships": 1}, {"ships": 2})

        self.assertIn("restore counts mismatch", str(ctx.exception))

    def test_build_restore_verification_sql_checks_all_sample_rows(self) -> None:
        sql = MODULE.build_restore_verification_sql()

        self.assertIn("Backup Drill Ship", sql)
        self.assertIn("jsonb_array_length(units_json) = 1", sql)
        self.assertIn("SELECT COUNT(*)::bigint", sql)

    def test_wait_for_postgres_retries_until_ready(self) -> None:
        responses = [
            completed(returncode=1, stderr="not ready"),
            completed(returncode=0, stdout="ready"),
        ]

        with mock.patch.object(MODULE.subprocess, "run", side_effect=responses) as run_mock:
            with mock.patch.object(MODULE.time, "sleep") as sleep_mock:
                MODULE.wait_for_postgres("proj", Path("compose.yml"), "shipsystem", "shipsystem_backup_drill", "shipsystem", timeout_seconds=5)

        self.assertEqual(2, run_mock.call_count)
        sleep_mock.assert_called_once_with(1)

    def test_wait_for_postgres_prints_logs_before_timeout_failure(self) -> None:
        with mock.patch.object(
            MODULE.subprocess,
            "run",
            return_value=completed(returncode=1, stderr="not ready"),
        ):
            with mock.patch.object(MODULE.time, "monotonic", side_effect=[0, 10]):
                with mock.patch.object(MODULE, "print_service_logs") as logs_mock:
                    with self.assertRaises(SystemExit):
                        MODULE.wait_for_postgres("proj", Path("compose.yml"), "shipsystem", "shipsystem_backup_drill", "shipsystem", timeout_seconds=5)

        logs_mock.assert_called_once()

    def test_dump_database_returns_stdout_bytes(self) -> None:
        payload = b"PGDMPbinary"
        with mock.patch.object(MODULE.subprocess, "run", return_value=MODULE.subprocess.CompletedProcess(args=["pg_dump"], returncode=0, stdout=payload, stderr=b"")):
            result = MODULE.dump_database("proj", Path("compose.yml"), "db", "user", "pass")

        self.assertEqual(payload, result)

    def test_cleanup_previous_resources_uses_compose_down(self) -> None:
        with mock.patch.object(MODULE, "run_compose") as run_compose:
            MODULE.cleanup_previous_resources("proj", Path("compose.yml"))

        run_compose.assert_called_once_with("proj", Path("compose.yml"), ["down", "-v", "--remove-orphans"], check=False)

    def test_restore_database_reports_failure(self) -> None:
        with mock.patch.object(
            MODULE.subprocess,
            "run",
            return_value=MODULE.subprocess.CompletedProcess(args=["pg_restore"], returncode=1, stdout=b"", stderr=b"restore failed"),
        ):
            with self.assertRaises(SystemExit) as ctx:
                MODULE.restore_database("proj", Path("compose.yml"), "db", "user", "pass", b"dump")

        self.assertIn("pg_restore failed", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
