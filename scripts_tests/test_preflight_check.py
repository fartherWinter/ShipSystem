import argparse
import importlib.util
import io
import os
import sys
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "preflight_check.py"
SPEC = importlib.util.spec_from_file_location("preflight_check_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class PreflightCheckScriptTest(unittest.TestCase):
    def test_default_selected_checks_include_script_unit(self) -> None:
        args = argparse.Namespace(
            only=None,
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertIn("script unit tests", names)
        self.assertEqual(
            [
                "go tests",
                "analytics tests",
                "script unit tests",
                "compose baseline",
                "event contract",
                "callback contract",
                "openapi contract",
                "frontend api contract",
                "rbac matrix",
                "frontend build",
            ],
            names,
        )

    def test_only_script_unit_selects_single_check(self) -> None:
        args = argparse.Namespace(
            only=["script-unit"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)

        self.assertEqual(1, len(checks))
        self.assertEqual("script unit tests", checks[0].name)
        self.assertEqual(
            [sys.executable, "-m", "unittest", "discover", "-s", "scripts_tests"],
            checks[0].command,
        )

    def test_runtime_smoke_is_opt_in_for_default_preflight(self) -> None:
        args = argparse.Namespace(
            only=None,
            include_runtime_smoke=True,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertIn("runtime smoke precheck", names)
        self.assertEqual(["runtime smoke precheck", "runtime smoke"], names[-2:])

    def test_runtime_smoke_check_uses_smoke_script(self) -> None:
        args = argparse.Namespace(
            only=None,
            include_runtime_smoke=True,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)

        self.assertEqual([sys.executable, "scripts/smoke_check.py"], checks[-1].command)

    def test_runtime_smoke_check_name_is_stable(self) -> None:
        args = argparse.Namespace(
            only=None,
            include_runtime_smoke=True,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)

        self.assertEqual("runtime smoke", checks[-1].name)

    def test_only_go_with_runtime_smoke_appends_runtime_checks(self) -> None:
        args = argparse.Namespace(
            only=["go"],
            include_runtime_smoke=True,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertEqual(["go tests", "runtime smoke precheck", "runtime smoke"], names)

    def test_only_go_with_runtime_observability_snapshot_appends_snapshot(self) -> None:
        args = argparse.Namespace(
            only=["go"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=True,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertEqual(["go tests", "runtime observability snapshot"], names)

    def test_only_go_with_repository_integration_appends_db_check(self) -> None:
        args = argparse.Namespace(
            only=["go"],
            include_runtime_smoke=False,
            include_db_integration=True,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        dsn = "host=localhost user=shipsystem password=shipsystem dbname=shipsystem_test"
        with mock.patch.dict(os.environ, {"SHIPSYSTEM_REPOSITORY_TEST_DSN": dsn}, clear=True):
            checks = MODULE.selected_checks(args)

        names = [check.name for check in checks]
        self.assertEqual(["go tests", "repository DB integration tests"], names)

    def test_only_go_with_backup_restore_drill_appends_drill(self) -> None:
        args = argparse.Namespace(
            only=["go"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=True,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertEqual(["go tests", "backup restore drill"], names)

    def test_only_go_with_compose_override_appends_override(self) -> None:
        args = argparse.Namespace(
            only=["go"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=True,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertEqual(["go tests", "compose override"], names)

    def test_only_go_with_retention_preview_appends_preview(self) -> None:
        args = argparse.Namespace(
            only=["go"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=True,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertEqual(["go tests", "retention preview"], names)

    def test_runtime_smoke_summary_is_printed_only_when_check_is_selected(self) -> None:
        args = argparse.Namespace(
            only=["go"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)

        self.assertFalse(any(check.name == "runtime smoke" for check in checks))

    def test_compose_override_is_opt_in_for_default_preflight(self) -> None:
        args = argparse.Namespace(
            only=None,
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=True,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertIn("compose override", names)
        self.assertEqual("compose override", names[-1])

    def test_retention_preview_is_opt_in_for_default_preflight(self) -> None:
        args = argparse.Namespace(
            only=None,
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=True,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertIn("retention preview", names)
        self.assertEqual("retention preview", names[-1])
        self.assertNotIn("--apply", checks[-1].command)

    def test_only_compose_override_selects_generator_script(self) -> None:
        args = argparse.Namespace(
            only=["compose-override"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)

        self.assertEqual(1, len(checks))
        self.assertEqual("compose override", checks[0].name)
        self.assertEqual([sys.executable, "scripts/generate_compose_local_override.py"], checks[0].command)

    def test_only_retention_preview_selects_preview_script(self) -> None:
        args = argparse.Namespace(
            only=["retention-preview"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)

        self.assertEqual(1, len(checks))
        self.assertEqual("retention preview", checks[0].name)
        self.assertEqual([sys.executable, "scripts/retention_maintenance.py"], checks[0].command)

    def test_repository_integration_requires_dsn(self) -> None:
        with mock.patch.dict(os.environ, {}, clear=True):
            with self.assertRaises(SystemExit) as ctx:
                MODULE.repository_integration_check()
        self.assertIn("SHIPSYSTEM_REPOSITORY_TEST_DSN", str(ctx.exception))

    def test_repository_integration_uses_env_dsn(self) -> None:
        dsn = "host=localhost user=shipsystem password=shipsystem dbname=shipsystem_test"
        with mock.patch.dict(os.environ, {"SHIPSYSTEM_REPOSITORY_TEST_DSN": dsn}, clear=True):
            check = MODULE.repository_integration_check()

        self.assertEqual("repository DB integration tests", check.name)
        self.assertEqual(dsn, check.env["SHIPSYSTEM_REPOSITORY_TEST_DSN"])
        self.assertIn("./internal/repositories", check.command)

    def test_backup_restore_drill_is_opt_in_for_default_preflight(self) -> None:
        args = argparse.Namespace(
            only=None,
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=True,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertIn("backup restore drill", names)
        self.assertEqual("backup restore drill", names[-1])

    def test_only_backup_restore_drill_selects_script(self) -> None:
        args = argparse.Namespace(
            only=["backup-restore-drill"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)

        self.assertEqual(1, len(checks))
        self.assertEqual("backup restore drill", checks[0].name)
        self.assertEqual([sys.executable, "scripts/run_backup_restore_drill.py"], checks[0].command)

    def test_frontend_check_captures_output_on_windows(self) -> None:
        args = argparse.Namespace(
            only=["frontend"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        with mock.patch.object(MODULE.os, "name", "nt"):
            checks = MODULE.selected_checks(args)

        self.assertEqual(1, len(checks))
        self.assertEqual("frontend build", checks[0].name)
        self.assertTrue(checks[0].capture_output)

    def test_runtime_observability_snapshot_is_opt_in_for_default_preflight(self) -> None:
        args = argparse.Namespace(
            only=None,
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=True,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertIn("runtime observability snapshot", names)
        self.assertEqual("runtime observability snapshot", names[-1])

    def test_only_runtime_observability_snapshot_selects_script(self) -> None:
        args = argparse.Namespace(
            only=["runtime-observability-snapshot"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=False,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)

        self.assertEqual(1, len(checks))
        self.assertEqual("runtime observability snapshot", checks[0].name)
        self.assertEqual([sys.executable, "scripts/run_runtime_observability_snapshot.py"], checks[0].command)

    def test_only_explicit_and_include_same_check_does_not_duplicate(self) -> None:
        args = argparse.Namespace(
            only=["compose-override"],
            include_runtime_smoke=False,
            include_db_integration=False,
            include_runtime_observability_snapshot=False,
            include_backup_restore_drill=False,
            include_compose_override=True,
            include_retention_preview=False,
        )

        checks = MODULE.selected_checks(args)
        names = [check.name for check in checks]

        self.assertEqual(["compose override"], names)

    def test_known_windows_node_realpath_eperm_is_detected(self) -> None:
        completed = MODULE.subprocess.CompletedProcess(
            args=["npm", "run", "build"],
            returncode=1,
            stdout="",
            stderr=(
                "Error: EPERM: operation not permitted, lstat 'C:\\Users\\chenn'\n"
                "    at Object.realpathSync (node:fs:2729:29)\n"
            ),
        )

        self.assertTrue(MODULE.is_known_windows_node_realpath_eperm(completed))

    def test_unknown_frontend_failure_is_not_misclassified(self) -> None:
        completed = MODULE.subprocess.CompletedProcess(
            args=["npm", "run", "build"],
            returncode=1,
            stdout="TypeScript error",
            stderr="",
        )

        self.assertFalse(MODULE.is_known_windows_node_realpath_eperm(completed))

    def test_relay_completed_output_falls_back_on_unicode_encode_error(self) -> None:
        completed = MODULE.subprocess.CompletedProcess(
            args=["npm", "run", "build"],
            returncode=0,
            stdout="prefix ✓\n",
            stderr="",
        )

        class LimitedStream(io.StringIO):
            encoding = "gbk"

            def write(self, s):
                if "✓" in s:
                    raise UnicodeEncodeError("gbk", s, 0, 1, "illegal multibyte sequence")
                return super().write(s)

        stream = LimitedStream()
        with mock.patch.object(sys, "stdout", stream):
            MODULE.relay_completed_output(completed)

        self.assertIn("prefix ?", stream.getvalue())


if __name__ == "__main__":
    unittest.main()
