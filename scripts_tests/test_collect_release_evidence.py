import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "collect_release_evidence.py"
SPEC = importlib.util.spec_from_file_location("collect_release_evidence_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def completed(stdout: str = "", stderr: str = "", returncode: int = 0):
    return MODULE.subprocess.CompletedProcess(args=["test"], returncode=returncode, stdout=stdout, stderr=stderr)


class CollectReleaseEvidenceScriptTest(unittest.TestCase):
    def test_selected_steps_default_and_runtime_modes(self) -> None:
        default_steps = MODULE.selected_steps(
            include_runtime=False,
            include_runtime_precheck=False,
            include_db_integration=False,
            include_backup_restore_drill=False,
            include_runtime_observability_snapshot=False,
            include_retention_preview=False,
            include_capacity_estimate=False,
        )
        runtime_steps = MODULE.selected_steps(
            include_runtime=True,
            include_runtime_precheck=False,
            include_db_integration=False,
            include_backup_restore_drill=False,
            include_runtime_observability_snapshot=False,
            include_retention_preview=False,
            include_capacity_estimate=False,
        )
        runtime_precheck_steps = MODULE.selected_steps(
            include_runtime=False,
            include_runtime_precheck=True,
            include_db_integration=False,
            include_backup_restore_drill=False,
            include_runtime_observability_snapshot=False,
            include_retention_preview=False,
            include_capacity_estimate=False,
        )
        db_steps = MODULE.selected_steps(
            include_runtime=False,
            include_runtime_precheck=False,
            include_db_integration=True,
            include_backup_restore_drill=False,
            include_runtime_observability_snapshot=False,
            include_retention_preview=False,
            include_capacity_estimate=False,
        )
        backup_steps = MODULE.selected_steps(
            include_runtime=False,
            include_runtime_precheck=False,
            include_db_integration=False,
            include_backup_restore_drill=True,
            include_runtime_observability_snapshot=False,
            include_retention_preview=False,
            include_capacity_estimate=False,
        )
        observability_steps = MODULE.selected_steps(
            include_runtime=False,
            include_runtime_precheck=False,
            include_db_integration=False,
            include_backup_restore_drill=False,
            include_runtime_observability_snapshot=True,
            include_retention_preview=False,
            include_capacity_estimate=False,
        )
        frontend_e2e_steps = MODULE.selected_steps(
            include_runtime=False,
            include_runtime_precheck=False,
            include_db_integration=False,
            include_backup_restore_drill=False,
            include_runtime_observability_snapshot=False,
            include_retention_preview=False,
            include_capacity_estimate=False,
            include_frontend_e2e=True,
        )
        retention_steps = MODULE.selected_steps(
            include_runtime=False,
            include_runtime_precheck=False,
            include_db_integration=False,
            include_backup_restore_drill=False,
            include_runtime_observability_snapshot=False,
            include_retention_preview=True,
            include_capacity_estimate=False,
        )
        capacity_steps = MODULE.selected_steps(
            include_runtime=False,
            include_runtime_precheck=False,
            include_db_integration=False,
            include_backup_restore_drill=False,
            include_runtime_observability_snapshot=False,
            include_retention_preview=False,
            include_capacity_estimate=True,
        )

        self.assertEqual(["migration-status", "preflight"], [item.name for item in default_steps])
        self.assertEqual(
            ["migration-status", "preflight", "smoke-check"],
            [item.name for item in runtime_steps],
        )
        self.assertEqual(
            ["migration-status", "preflight", "runtime-precheck"],
            [item.name for item in runtime_precheck_steps],
        )
        self.assertEqual(["migration-status", "preflight", "db-integration"], [item.name for item in db_steps])
        self.assertEqual(["migration-status", "preflight", "backup-restore-drill"], [item.name for item in backup_steps])
        self.assertEqual("03-backup-restore-drill.txt", backup_steps[-1].output_file)
        self.assertEqual(["migration-status", "preflight", "runtime-observability-snapshot"], [item.name for item in observability_steps])
        self.assertEqual("03-runtime-observability-snapshot.txt", observability_steps[-1].output_file)
        self.assertEqual(["migration-status", "preflight", "frontend-e2e"], [item.name for item in frontend_e2e_steps])
        self.assertEqual("03-frontend-e2e.txt", frontend_e2e_steps[-1].output_file)
        self.assertEqual(["migration-status", "preflight", "retention-preview"], [item.name for item in retention_steps])
        self.assertEqual("03-retention-preview.txt", retention_steps[-1].output_file)
        self.assertEqual(["migration-status", "preflight", "capacity-estimate"], [item.name for item in capacity_steps])
        self.assertEqual("03-capacity-estimate.txt", capacity_steps[-1].output_file)

    def test_selected_steps_stacks_optional_evidence_in_stable_order(self) -> None:
        steps = MODULE.selected_steps(
            include_runtime=True,
            include_runtime_precheck=True,
            include_db_integration=True,
            include_backup_restore_drill=True,
            include_runtime_observability_snapshot=True,
            include_retention_preview=True,
            include_capacity_estimate=True,
            include_frontend_e2e=True,
        )

        self.assertEqual(
            [
                "migration-status",
                "preflight",
                "frontend-e2e",
                "runtime-precheck",
                "smoke-check",
                "db-integration",
                "backup-restore-drill",
                "runtime-observability-snapshot",
                "retention-preview",
                "capacity-estimate",
            ],
            [item.name for item in steps],
        )
        self.assertEqual(
            [
                "01-migrate-status.txt",
                "02-preflight.txt",
                "03-frontend-e2e.txt",
                "04-runtime-precheck.txt",
                "05-smoke-check.txt",
                "06-db-integration.txt",
                "07-backup-restore-drill.txt",
                "08-runtime-observability-snapshot.txt",
                "09-retention-preview.txt",
                "10-capacity-estimate.txt",
            ],
            [item.output_file for item in steps],
        )

    def test_run_step_writes_output_file_and_returns_result(self) -> None:
        step = MODULE.EvidenceStep("preflight", [sys.executable, "scripts/preflight_check.py"], ROOT, "02-preflight.txt")
        with tempfile.TemporaryDirectory() as tmpdir:
            with mock.patch.object(MODULE.subprocess, "run", return_value=completed(stdout="ok\n", stderr="", returncode=0)):
                result = MODULE.run_step(step, Path(tmpdir))

            output = (Path(tmpdir) / "02-preflight.txt").read_text(encoding="utf-8")

        self.assertEqual("preflight", result.name)
        self.assertEqual(0, result.exitCode)
        self.assertIn("stdout:", output)
        self.assertIn("ok", output)

    def test_run_step_records_missing_command_as_failed_output(self) -> None:
        step = MODULE.EvidenceStep("frontend-e2e", ["npm", "run", "test:e2e"], ROOT / "frontend", "03-frontend-e2e.txt")
        missing = FileNotFoundError(2, "No such file or directory", "npm")
        with tempfile.TemporaryDirectory() as tmpdir:
            with mock.patch.object(MODULE.subprocess, "run", side_effect=missing):
                result = MODULE.run_step(step, Path(tmpdir))

            output = (Path(tmpdir) / "03-frontend-e2e.txt").read_text(encoding="utf-8")

        self.assertEqual("frontend-e2e", result.name)
        self.assertEqual(127, result.exitCode)
        self.assertIn("could not start command: npm", output)

    def test_main_writes_manifest_even_when_a_step_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            args = MODULE.argparse.Namespace(
                output_dir=tmpdir,
                include_runtime=False,
                include_runtime_precheck=False,
                include_db_integration=False,
                include_backup_restore_drill=False,
                include_runtime_observability_snapshot=False,
                include_retention_preview=False,
                include_capacity_estimate=False,
                continue_on_failure=False,
            )
            fake_steps = [
                MODULE.EvidenceStep("migration-status", ["go"], ROOT, "01-migrate-status.txt"),
                MODULE.EvidenceStep("preflight", ["python"], ROOT, "02-preflight.txt"),
            ]
            fake_results = [
                MODULE.StepResult("migration-status", ["go"], str(ROOT), "01-migrate-status.txt", 0),
                MODULE.StepResult("preflight", ["python"], str(ROOT), "02-preflight.txt", 1),
            ]
            with mock.patch.object(MODULE, "parse_args", return_value=args):
                with mock.patch.object(MODULE, "selected_steps", return_value=fake_steps):
                    with mock.patch.object(MODULE, "run_step", side_effect=fake_results):
                        exit_code = MODULE.main()

            manifest = json.loads((Path(tmpdir) / "manifest.json").read_text(encoding="utf-8"))

        self.assertEqual(1, exit_code)
        self.assertEqual(2, len(manifest["steps"]))
        self.assertEqual(1, manifest["steps"][-1]["exitCode"])

    def test_main_success_writes_manifest(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            args = MODULE.argparse.Namespace(
                output_dir=tmpdir,
                include_runtime=True,
                include_runtime_precheck=False,
                include_db_integration=True,
                include_backup_restore_drill=True,
                include_runtime_observability_snapshot=True,
                include_frontend_e2e=True,
                include_retention_preview=True,
                include_capacity_estimate=True,
                continue_on_failure=False,
            )
            fake_steps = [MODULE.EvidenceStep("smoke-check", ["python"], ROOT, "04-smoke-check.txt")]
            fake_result = MODULE.StepResult("smoke-check", ["python"], str(ROOT), "04-smoke-check.txt", 0)
            with mock.patch.object(MODULE, "parse_args", return_value=args):
                with mock.patch.object(MODULE, "selected_steps", return_value=fake_steps):
                    with mock.patch.object(MODULE, "run_step", return_value=fake_result):
                        exit_code = MODULE.main()

            manifest = json.loads((Path(tmpdir) / "manifest.json").read_text(encoding="utf-8"))

        self.assertEqual(0, exit_code)
        self.assertTrue(manifest["includeRuntime"])
        self.assertFalse(manifest["includeRuntimePrecheck"])
        self.assertTrue(manifest["includeDbIntegration"])
        self.assertTrue(manifest["includeBackupRestoreDrill"])
        self.assertTrue(manifest["includeRuntimeObservabilitySnapshot"])
        self.assertTrue(manifest["includeFrontendE2E"])
        self.assertTrue(manifest["includeRetentionPreview"])
        self.assertTrue(manifest["includeCapacityEstimate"])
        self.assertEqual("smoke-check", manifest["steps"][0]["name"])

    def test_main_can_continue_collecting_after_failure_when_enabled(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            args = MODULE.argparse.Namespace(
                output_dir=tmpdir,
                include_runtime=False,
                include_runtime_precheck=False,
                include_db_integration=False,
                include_backup_restore_drill=False,
                include_runtime_observability_snapshot=False,
                include_retention_preview=True,
                include_capacity_estimate=True,
                continue_on_failure=True,
            )
            fake_steps = [
                MODULE.EvidenceStep("migration-status", ["go"], ROOT, "01-migrate-status.txt"),
                MODULE.EvidenceStep("retention-preview", ["python"], ROOT, "03-retention-preview.txt"),
                MODULE.EvidenceStep("capacity-estimate", ["python"], ROOT, "04-capacity-estimate.txt"),
            ]
            fake_results = [
                MODULE.StepResult("migration-status", ["go"], str(ROOT), "01-migrate-status.txt", 1),
                MODULE.StepResult("retention-preview", ["python"], str(ROOT), "03-retention-preview.txt", 0),
                MODULE.StepResult("capacity-estimate", ["python"], str(ROOT), "04-capacity-estimate.txt", 0),
            ]
            with mock.patch.object(MODULE, "parse_args", return_value=args):
                with mock.patch.object(MODULE, "selected_steps", return_value=fake_steps):
                    with mock.patch.object(MODULE, "run_step", side_effect=fake_results):
                        exit_code = MODULE.main()

            manifest = json.loads((Path(tmpdir) / "manifest.json").read_text(encoding="utf-8"))

        self.assertEqual(1, exit_code)
        self.assertEqual(
            ["migration-status", "retention-preview", "capacity-estimate"],
            [item["name"] for item in manifest["steps"]],
        )


if __name__ == "__main__":
    unittest.main()
