#!/usr/bin/env python3
"""Collect ShipSystem release evidence into a timestamped artifact directory."""

from __future__ import annotations

import argparse
import json
import os
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
    )
    manifest: dict[str, object] = {
        "generatedAt": datetime.now(timezone.utc).isoformat(),
        "outputDir": str(output_dir),
        "includeRuntime": args.include_runtime,
        "includeRuntimePrecheck": args.include_runtime_precheck,
        "includeDbIntegration": args.include_db_integration,
        "includeBackupRestoreDrill": args.include_backup_restore_drill,
        "includeRuntimeObservabilitySnapshot": args.include_runtime_observability_snapshot,
        "steps": [],
    }

    for step in steps:
        result = run_step(step, output_dir)
        manifest["steps"].append(asdict(result))
        if result.exitCode != 0:
            write_manifest(output_dir, manifest)
            print(f"[FAIL] {step.name} failed; see {output_dir}")
            return 1

    write_manifest(output_dir, manifest)
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
    return parser.parse_args()


def selected_steps(
    *,
    include_runtime: bool,
    include_runtime_precheck: bool,
    include_db_integration: bool,
    include_backup_restore_drill: bool,
    include_runtime_observability_snapshot: bool,
) -> list[EvidenceStep]:
    steps = [
        EvidenceStep(
            "migration-status",
            ["go", "run", "./cmd/migrate", "-action=status"],
            ROOT / "backend",
            "01-migrate-status.txt",
        ),
        EvidenceStep(
            "preflight",
            [sys.executable, "scripts/preflight_check.py"],
            ROOT,
            "02-preflight.txt",
        ),
    ]
    if include_runtime_precheck:
        steps.append(
            EvidenceStep(
                "runtime-precheck",
                [sys.executable, "scripts/runtime_precheck.py"],
                ROOT,
                "03-runtime-precheck.txt",
            )
        )
    if include_runtime:
        steps.append(
            EvidenceStep(
                "smoke-check",
                [sys.executable, "scripts/smoke_check.py"],
                ROOT,
                "04-smoke-check.txt" if include_runtime_precheck else "03-smoke-check.txt",
            )
        )
    if include_db_integration:
        steps.append(
            EvidenceStep(
                "db-integration",
                [sys.executable, "scripts/run_repository_db_integration.py"],
                ROOT,
                "05-db-integration.txt" if include_runtime and include_runtime_precheck else (
                    "04-db-integration.txt" if include_runtime or include_runtime_precheck else "03-db-integration.txt"
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
                    step_name="runtime-observability-snapshot",
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
    step_name: str,
) -> str:
    index = 3
    if include_runtime_precheck:
        index += 1
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
    raise ValueError(f"unsupported evidence step name: {step_name}")


def run_step(step: EvidenceStep, output_dir: Path) -> StepResult:
    output_path = output_dir / step.output_file
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


if __name__ == "__main__":
    sys.exit(main())
