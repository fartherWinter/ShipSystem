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
        default_steps = MODULE.selected_steps(include_runtime=False, include_runtime_precheck=False, include_db_integration=False, include_backup_restore_drill=False, include_runtime_observability_snapshot=False)
        runtime_steps = MODULE.selected_steps(include_runtime=True, include_runtime_precheck=False, include_db_integration=False, include_backup_restore_drill=False, include_runtime_observability_snapshot=False)
        runtime_precheck_steps = MODULE.selected_steps(include_runtime=False, include_runtime_precheck=True, include_db_integration=False, include_backup_restore_drill=False, include_runtime_observability_snapshot=False)
        db_steps = MODULE.selected_steps(include_runtime=False, include_runtime_precheck=False, include_db_integration=True, include_backup_restore_drill=False, include_runtime_observability_snapshot=False)
        backup_steps = MODULE.selected_steps(include_runtime=False, include_runtime_precheck=False, include_db_integration=False, include_backup_restore_drill=True, include_runtime_observability_snapshot=False)
        observability_steps = MODULE.selected_steps(include_runtime=False, include_runtime_precheck=False, include_db_integration=False, include_backup_restore_drill=False, include_runtime_observability_snapshot=True)

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

    def test_main_writes_manifest_even_when_a_step_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            args = MODULE.argparse.Namespace(
                output_dir=tmpdir,
                include_runtime=False,
                include_runtime_precheck=False,
                include_db_integration=False,
                include_backup_restore_drill=False,
                include_runtime_observability_snapshot=False,
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
        self.assertEqual("smoke-check", manifest["steps"][0]["name"])


if __name__ == "__main__":
    unittest.main()
