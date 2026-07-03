#!/usr/bin/env python3
"""Collect ShipSystem release evidence into a timestamped artifact directory."""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_OUTPUT_ROOT = ROOT / ".release-evidence"


@dataclass(frozen=True)
class EvidenceStep:
    name: str
    command: list[str]
    cwd: Path
    output_file: str


@dataclass(frozen=True)
class StepResult:
    name: str
    command: list[str]
    cwd: str
    outputFile: str
    exitCode: int


def main() -> int:
    args = parse_args()
    output_dir = resolve_output_dir(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    steps = selected_steps(
        include_runtime=args.include_runtime,
        include_runtime_precheck=args.include_runtime_precheck,
        include_db_integration=args.include_db_integration,
        include_backup_restore_drill=args.include_backup_restore_drill,
        include_runtime_observability_snapshot=args.include_runtime_observability_snapshot,
        include_frontend_e2e=getattr(args, "include_frontend_e2e", False),
        include_retention_preview=args.include_retention_preview,
        include_capacity_estimate=args.include_capacity_estimate,
        skip_migration_status=getattr(args, "skip_migration_status", False),
    )
    manifest: dict[str, object] = {
        "generatedAt": datetime.now(timezone.utc).isoformat(),
        "outputDir": str(output_dir),
        "includeRuntime": args.include_runtime,
        "includeRuntimePrecheck": args.include_runtime_precheck,
        "includeDbIntegration": args.include_db_integration,
        "includeBackupRestoreDrill": args.include_backup_restore_drill,
        "includeRuntimeObservabilitySnapshot": args.include_runtime_observability_snapshot,
        "includeFrontendE2E": getattr(args, "include_frontend_e2e", False),
        "includeRetentionPreview": args.include_retention_preview,
        "includeCapacityEstimate": args.include_capacity_estimate,
        "skipMigrationStatus": getattr(args, "skip_migration_status", False),
        "steps": [],
    }

    failed = False
    for step in steps:
        result = run_step(step, output_dir)
        manifest["steps"].append(asdict(result))
        if result.exitCode != 0:
            failed = True
            if not args.continue_on_failure:
                write_manifest(output_dir, manifest)
                print(f"[FAIL] {step.name} failed; see {output_dir}")
                return 1

    write_manifest(output_dir, manifest)
    if failed:
        print(f"[FAIL] one or more evidence steps failed; see {output_dir}")
        return 1
    print(f"Release evidence collected in {output_dir}")
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output-dir",
        help="Optional output directory. Defaults to .release-evidence/<UTC timestamp>.",
    )
    parser.add_argument(
        "--include-runtime",
        action="store_true",
        help="Also run smoke_check.py. Requires a running stack.",
    )
    parser.add_argument(
        "--include-runtime-precheck",
        action="store_true",
        help="Also run runtime_precheck.py. This is intended for pre-start evidence before the stack is running.",
    )
    parser.add_argument(
        "--include-db-integration",
        action="store_true",
        help="Also run scripts/run_repository_db_integration.py and archive its output.",
    )
    parser.add_argument(
        "--include-backup-restore-drill",
        action="store_true",
        help="Also run scripts/run_backup_restore_drill.py and archive its output.",
    )
    parser.add_argument(
        "--include-runtime-observability-snapshot",
        action="store_true",
        help="Also run scripts/run_runtime_observability_snapshot.py and archive its output. Requires a running stack.",
    )
    parser.add_argument(
        "--include-frontend-e2e",
        action="store_true",
        help="Also run frontend Playwright browser regressions via npm run test:e2e and archive the console output.",
    )
    parser.add_argument(
        "--include-retention-preview",
        action="store_true",
        help="Also run scripts/retention_maintenance.py in preview mode and archive its output.",
    )
    parser.add_argument(
        "--include-capacity-estimate",
        action="store_true",
        help="Also run scripts/run_capacity_smoke.py --estimate-only and archive its output.",
    )
    parser.add_argument(
        "--skip-migration-status",
        action="store_true",
        help="Skip the default migration-status step. Useful when collecting frontend-only evidence without a reachable database.",
    )
    parser.add_argument(
        "--continue-on-failure",
        action="store_true",
        help="Continue collecting later evidence steps even after one step fails. The command still exits non-zero.",
    )
    return parser.parse_args()


def selected_steps(
    *,
    include_runtime: bool,
    include_runtime_precheck: bool,
    include_db_integration: bool,
    include_backup_restore_drill: bool,
    include_runtime_observability_snapshot: bool,
    include_retention_preview: bool,
    include_capacity_estimate: bool,
    include_frontend_e2e: bool = False,
    skip_migration_status: bool = False,
) -> list[EvidenceStep]:
    steps = []
    if not skip_migration_status:
        steps.append(
            EvidenceStep(
                "migration-status",
                [tool_path("go"), "run", "./cmd/migrate", "-action=status"],
                ROOT / "backend",
                "01-migrate-status.txt",
            )
        )
    steps.append(
        EvidenceStep(
            "preflight",
            [sys.executable, "scripts/preflight_check.py"],
            ROOT,
            "02-preflight.txt",
        )
    )
    if include_frontend_e2e:
        steps.append(
            EvidenceStep(
                "frontend-e2e",
                [tool_path("npm"), "run", "test:e2e"],
                ROOT / "frontend",
                "03-frontend-e2e.txt",
            )
        )
    if include_runtime_precheck:
        steps.append(
            EvidenceStep(
                "runtime-precheck",
                [sys.executable, "scripts/runtime_precheck.py"],
                ROOT,
                "04-runtime-precheck.txt" if include_frontend_e2e else "03-runtime-precheck.txt",
            )
        )
    if include_runtime:
        steps.append(
            EvidenceStep(
                "smoke-check",
                [sys.executable, "scripts/smoke_check.py"],
                ROOT,
                evidence_filename(
                    include_runtime=include_runtime,
                    include_runtime_precheck=include_runtime_precheck,
                    include_db_integration=include_db_integration,
                    include_backup_restore_drill=include_backup_restore_drill,
                    include_runtime_observability_snapshot=include_runtime_observability_snapshot,
                    include_retention_preview=include_retention_preview,
                    include_capacity_estimate=include_capacity_estimate,
                    include_frontend_e2e=include_frontend_e2e,
                    step_name="smoke-check",
                ),
            )
        )
    if include_db_integration:
        steps.append(
            EvidenceStep(
                "db-integration",
                [sys.executable, "scripts/run_repository_db_integration.py"],
                ROOT,
                evidence_filename(
                    include_runtime=include_runtime,
                    include_runtime_precheck=include_runtime_precheck,
                    include_db_integration=True,
                    include_backup_restore_drill=include_backup_restore_drill,
                    include_runtime_observability_snapshot=include_runtime_observability_snapshot,
                    include_retention_preview=include_retention_preview,
                    include_capacity_estimate=include_capacity_estimate,
                    include_frontend_e2e=include_frontend_e2e,
                    step_name="db-integration",
                ),
            )
        )
    if include_backup_restore_drill:
        steps.append(
            EvidenceStep(
                "backup-restore-drill",
                [sys.executable, "scripts/run_backup_restore_drill.py"],
                ROOT,
                evidence_filename(
                    include_runtime=include_runtime,
                    include_runtime_precheck=include_runtime_precheck,
                    include_db_integration=include_db_integration,
                    include_backup_restore_drill=True,
                    include_runtime_observability_snapshot=include_runtime_observability_snapshot,
                    include_retention_preview=include_retention_preview,
                    include_capacity_estimate=include_capacity_estimate,
                    include_frontend_e2e=include_frontend_e2e,
                    step_name="backup-restore-drill",
                ),
            )
        )
    if include_runtime_observability_snapshot:
        steps.append(
            EvidenceStep(
                "runtime-observability-snapshot",
                [sys.executable, "scripts/run_runtime_observability_snapshot.py"],
                ROOT,
                evidence_filename(
                    include_runtime=include_runtime,
                    include_runtime_precheck=include_runtime_precheck,
                    include_db_integration=include_db_integration,
                    include_backup_restore_drill=include_backup_restore_drill,
                    include_runtime_observability_snapshot=True,
                    include_retention_preview=include_retention_preview,
                    include_capacity_estimate=include_capacity_estimate,
                    include_frontend_e2e=include_frontend_e2e,
                    step_name="runtime-observability-snapshot",
                ),
            )
        )
    if include_retention_preview:
        steps.append(
            EvidenceStep(
                "retention-preview",
                [sys.executable, "scripts/retention_maintenance.py"],
                ROOT,
                evidence_filename(
                    include_runtime=include_runtime,
                    include_runtime_precheck=include_runtime_precheck,
                    include_db_integration=include_db_integration,
                    include_backup_restore_drill=include_backup_restore_drill,
                    include_runtime_observability_snapshot=include_runtime_observability_snapshot,
                    include_retention_preview=True,
                    include_capacity_estimate=include_capacity_estimate,
                    include_frontend_e2e=include_frontend_e2e,
                    step_name="retention-preview",
                ),
            )
        )
    if include_capacity_estimate:
        steps.append(
            EvidenceStep(
                "capacity-estimate",
                [
                    sys.executable,
                    "scripts/run_capacity_smoke.py",
                    "--estimate-only",
                    "--track-counts",
                    "5,20,100",
                    "--ticks",
                    "6",
                    "--duration-seconds",
                    "30",
                    "--action-every-ticks",
                    "2",
                ],
                ROOT,
                evidence_filename(
                    include_runtime=include_runtime,
                    include_runtime_precheck=include_runtime_precheck,
                    include_db_integration=include_db_integration,
                    include_backup_restore_drill=include_backup_restore_drill,
                    include_runtime_observability_snapshot=include_runtime_observability_snapshot,
                    include_retention_preview=include_retention_preview,
                    include_capacity_estimate=True,
                    include_frontend_e2e=include_frontend_e2e,
                    step_name="capacity-estimate",
                ),
            )
        )
    return steps


def evidence_filename(
    *,
    include_runtime: bool,
    include_runtime_precheck: bool,
    include_db_integration: bool,
    include_backup_restore_drill: bool,
    include_runtime_observability_snapshot: bool,
    include_retention_preview: bool,
    include_capacity_estimate: bool,
    include_frontend_e2e: bool = False,
    step_name: str,
) -> str:
    index = 3
    if step_name == "frontend-e2e" and include_frontend_e2e:
        return f"{index:02d}-frontend-e2e.txt"
    if include_frontend_e2e:
        index += 1
    if step_name == "runtime-precheck" and include_runtime_precheck:
        return f"{index:02d}-runtime-precheck.txt"
    if include_runtime_precheck:
        index += 1
    if step_name == "smoke-check" and include_runtime:
        return f"{index:02d}-smoke-check.txt"
    if include_runtime:
        index += 1
    if step_name == "db-integration":
        return f"{index:02d}-db-integration.txt"
    if include_db_integration:
        index += 1
    if step_name == "backup-restore-drill" and include_backup_restore_drill:
        return f"{index:02d}-backup-restore-drill.txt"
    if include_backup_restore_drill:
        index += 1
    if step_name == "runtime-observability-snapshot" and include_runtime_observability_snapshot:
        return f"{index:02d}-runtime-observability-snapshot.txt"
    if include_runtime_observability_snapshot:
        index += 1
    if step_name == "retention-preview" and include_retention_preview:
        return f"{index:02d}-retention-preview.txt"
    if include_retention_preview:
        index += 1
    if step_name == "capacity-estimate" and include_capacity_estimate:
        return f"{index:02d}-capacity-estimate.txt"
    raise ValueError(f"unsupported evidence step name: {step_name}")


def run_step(step: EvidenceStep, output_dir: Path) -> StepResult:
    output_path = output_dir / step.output_file
    try:
        completed = subprocess.run(
            step.command,
            cwd=step.cwd,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            env=os.environ.copy(),
        )
    except FileNotFoundError as exc:
        missing = exc.filename or step.command[0]
        completed = subprocess.CompletedProcess(
            args=step.command,
            returncode=127,
            stdout="",
            stderr=f"[FAIL] {step.name} could not start command: {missing}\n",
        )
    output_path.write_text(render_step_output(step, completed), encoding="utf-8")
    print(f"[{'OK' if completed.returncode == 0 else 'FAIL'}] {step.name} -> {output_path.name}")
    return StepResult(
        name=step.name,
        command=step.command,
        cwd=str(step.cwd),
        outputFile=step.output_file,
        exitCode=completed.returncode,
    )


def render_step_output(step: EvidenceStep, completed: subprocess.CompletedProcess[str]) -> str:
    lines = [
        f"name: {step.name}",
        f"cwd: {step.cwd}",
        f"command: {format_command(step.command)}",
        f"exit_code: {completed.returncode}",
        "",
        "stdout:",
        completed.stdout or "",
        "",
        "stderr:",
        completed.stderr or "",
    ]
    return "\n".join(lines).rstrip() + "\n"


def write_manifest(output_dir: Path, manifest: dict[str, object]) -> None:
    (output_dir / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")


def resolve_output_dir(raw: str | None) -> Path:
    if raw:
        return Path(raw).resolve()
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return DEFAULT_OUTPUT_ROOT / timestamp


def format_command(command: list[str]) -> str:
    return " ".join(quote_part(part) for part in command)


def quote_part(part: str) -> str:
    if not part or any(ch.isspace() for ch in part):
        return f'"{part}"'
    return part


def tool_path(name: str) -> str:
    return shutil.which(name) or name


if __name__ == "__main__":
    sys.exit(main())
