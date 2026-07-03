#!/usr/bin/env python3
"""Run the local ShipSystem release gate checks in a stable order."""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class Check:
    name: str
    command: list[str]
    cwd: Path
    env: dict[str, str] | None = None
    hint: str = ""
    capture_output: bool = False


RUNTIME_SMOKE_CHECK = Check(
    "runtime smoke",
    [sys.executable, "scripts/smoke_check.py"],
    ROOT,
)


CAPACITY_ESTIMATE_CHECK = Check(
    "capacity estimate",
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
)


def main() -> int:
    args = parse_args()
    checks = selected_checks(args)
    print(f"ShipSystem preflight: {len(checks)} checks", flush=True)

    started_at = time.monotonic()
    passed: list[tuple[str, float]] = []
    for check in checks:
        elapsed = run_check(check)
        passed.append((check.name, elapsed))

    total_elapsed = time.monotonic() - started_at
    print("")
    print("Preflight passed")
    for name, elapsed in passed:
        print(f"- {name}: {elapsed:.1f}s")
    print(f"Total: {total_elapsed:.1f}s")
    if any(check.name == RUNTIME_SMOKE_CHECK.name for check in checks):
        print("Runtime smoke checks were included.")
    if any(check.name == "repository DB integration tests" for check in checks):
        print("Repository DB integration checks were included.")
    if any(check.name == "frontend e2e" for check in checks):
        print("Frontend E2E checks were included.")
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run ShipSystem local release gate checks.")
    parser.add_argument(
        "--only",
        action="append",
        choices=(
            "go",
            "analytics",
            "script-unit",
            "frontend",
            "compose",
            "compose-override",
            "event-contract",
            "callback-contract",
            "openapi-contract",
            "frontend-api-contract",
            "rbac-matrix",
            "runtime-precheck",
            "runtime-observability-snapshot",
            "db-integration",
            "backup-restore-drill",
            "frontend-e2e",
            "retention-preview",
            "capacity-estimate",
        ),
        help="Run only selected checks. Can be passed multiple times.",
    )
    parser.add_argument(
        "--include-runtime-smoke",
        action="store_true",
        help="Run runtime prerequisite checks followed by smoke_check.py. Requires a running stack.",
    )
    parser.add_argument(
        "--include-db-integration",
        action="store_true",
        help="Run repository DB integration tests. Requires SHIPSYSTEM_REPOSITORY_TEST_DSN to point at a disposable PostgreSQL database.",
    )
    parser.add_argument(
        "--include-runtime-observability-snapshot",
        action="store_true",
        help="Run a read-only runtime observability snapshot against a running local stack.",
    )
    parser.add_argument(
        "--include-backup-restore-drill",
        action="store_true",
        help="Run an isolated PostgreSQL backup/restore drill against a temporary PostGIS container.",
    )
    parser.add_argument(
        "--include-retention-preview",
        action="store_true",
        help="Run a preview-only retention maintenance check against DATABASE_DSN. Requires psql and a reachable database.",
    )
    parser.add_argument(
        "--include-compose-override",
        action="store_true",
        help="Generate a local docker compose override with free host ports. Does not start the stack.",
    )
    parser.add_argument(
        "--include-capacity-estimate",
        action="store_true",
        help="Run the estimate-only battle replay capacity projection used to size retention thresholds.",
    )
    parser.add_argument(
        "--include-frontend-e2e",
        action="store_true",
        help="Run the mocked Playwright browser regression suite in frontend/ via npm run test:e2e.",
    )
    return parser.parse_args()


def selected_checks(args: argparse.Namespace) -> list[Check]:
    checks = {
        "go": Check("go tests", [tool_path("go"), "test", "./..."], ROOT / "backend"),
        "analytics": Check(
            "analytics tests",
            [tool_path("uv"), "run", "--with-requirements", "requirements.txt", "python", "-m", "unittest", "discover", "-s", "tests"],
            ROOT / "analytics",
            {
                "UV_CACHE_DIR": str(ROOT / ".uv-cache"),
                "UV_PYTHON_INSTALL_DIR": str(ROOT / ".uv-python"),
            },
        ),
        "script-unit": Check(
            "script unit tests",
            [sys.executable, "-m", "unittest", "discover", "-s", "scripts_tests"],
            ROOT,
        ),
        "frontend": Check(
            "frontend build",
            [tool_path("npm"), "run", "build"],
            ROOT / "frontend",
            hint=(
                "On Windows sandboxed shells, Python subprocess may trigger Node EPERM while starting npm. "
                "If that happens, run `cd frontend && npm run build` directly and keep its output with the preflight evidence."
            ),
            capture_output=os.name == "nt",
        ),
        "frontend-e2e": Check(
            "frontend e2e",
            [tool_path("npm"), "run", "test:e2e"],
            ROOT / "frontend",
            hint=(
                "On Windows sandboxed shells, Python subprocess may trigger Node EPERM while starting npm. "
                "If that happens, run `cd frontend && npm run test:e2e` directly and keep its output with the preflight evidence."
            ),
            capture_output=os.name == "nt",
        ),
        "compose": Check("compose baseline", [sys.executable, "scripts/check_compose_config.py"], ROOT),
        "compose-override": Check(
            "compose override",
            [sys.executable, "scripts/generate_compose_local_override.py"],
            ROOT,
        ),
        "event-contract": Check("event contract", [sys.executable, "scripts/check_event_contract.py"], ROOT),
        "callback-contract": Check("callback contract", [sys.executable, "scripts/check_callback_contract.py"], ROOT),
        "openapi-contract": Check("openapi contract", [sys.executable, "scripts/check_openapi_contract.py"], ROOT),
        "frontend-api-contract": Check("frontend api contract", [sys.executable, "scripts/check_frontend_api_contract.py"], ROOT),
        "rbac-matrix": Check("rbac matrix", [sys.executable, "scripts/check_rbac_matrix.py"], ROOT),
        "runtime-precheck": Check("runtime smoke precheck", [sys.executable, "scripts/runtime_precheck.py"], ROOT),
        "runtime-observability-snapshot": Check(
            "runtime observability snapshot",
            [sys.executable, "scripts/run_runtime_observability_snapshot.py"],
            ROOT,
        ),
        "backup-restore-drill": Check(
            "backup restore drill",
            [sys.executable, "scripts/run_backup_restore_drill.py"],
            ROOT,
        ),
        "retention-preview": Check("retention preview", [sys.executable, "scripts/retention_maintenance.py"], ROOT),
        "capacity-estimate": CAPACITY_ESTIMATE_CHECK,
    }
    include_db_integration = flag_enabled(args, "include_db_integration") or (args.only is not None and "db-integration" in args.only)
    if include_db_integration:
        checks["db-integration"] = repository_integration_check()
    include_runtime_observability_snapshot = flag_enabled(args, "include_runtime_observability_snapshot") or (
        args.only is not None and "runtime-observability-snapshot" in args.only
    )
    include_backup_restore_drill = flag_enabled(args, "include_backup_restore_drill") or (
        args.only is not None and "backup-restore-drill" in args.only
    )
    include_compose_override = flag_enabled(args, "include_compose_override") or (
        args.only is not None and "compose-override" in args.only
    )
    include_retention_preview = flag_enabled(args, "include_retention_preview") or (
        args.only is not None and "retention-preview" in args.only
    )
    include_capacity_estimate = flag_enabled(args, "include_capacity_estimate") or (
        args.only is not None and "capacity-estimate" in args.only
    )
    include_frontend_e2e = flag_enabled(args, "include_frontend_e2e") or (
        args.only is not None and "frontend-e2e" in args.only
    )
    extra_checks: list[Check] = []
    if flag_enabled(args, "include_runtime_smoke"):
        extra_checks.extend([checks["runtime-precheck"], RUNTIME_SMOKE_CHECK])
    if include_runtime_observability_snapshot:
        extra_checks.append(checks["runtime-observability-snapshot"])
    if include_db_integration:
        extra_checks.append(checks["db-integration"])
    if include_backup_restore_drill:
        extra_checks.append(checks["backup-restore-drill"])
    if include_compose_override:
        extra_checks.append(checks["compose-override"])
    if include_frontend_e2e:
        extra_checks.append(checks["frontend-e2e"])
    if include_retention_preview:
        extra_checks.append(checks["retention-preview"])
    if include_capacity_estimate:
        extra_checks.append(checks["capacity-estimate"])
    if not args.only:
        selected = [
            checks["go"],
            checks["analytics"],
            checks["script-unit"],
            checks["compose"],
            checks["event-contract"],
            checks["callback-contract"],
            checks["openapi-contract"],
            checks["frontend-api-contract"],
            checks["rbac-matrix"],
            checks["frontend"],
        ]
    else:
        selected = [checks[name] for name in args.only]

    selected_names = {check.name for check in selected}
    for check in extra_checks:
        if check.name in selected_names:
            continue
        selected.append(check)
        selected_names.add(check.name)
    return selected


def repository_integration_check() -> Check:
    dsn = os.environ.get("SHIPSYSTEM_REPOSITORY_TEST_DSN", "")
    if not dsn:
        raise SystemExit("[FAIL] repository DB integration checks require SHIPSYSTEM_REPOSITORY_TEST_DSN")
    return Check(
        "repository DB integration tests",
        [tool_path("go"), "test", "./internal/repositories", "-run", "Test.*Concurrent", "-count=1"],
        ROOT / "backend",
        {"SHIPSYSTEM_REPOSITORY_TEST_DSN": dsn},
    )


def run_check(check: Check) -> float:
    print("", flush=True)
    print(f"==> {check.name}", flush=True)
    print(f"cwd: {check.cwd}", flush=True)
    print(f"cmd: {format_command(check.command)}", flush=True)
    started_at = time.monotonic()
    env = os.environ.copy()
    if check.env:
        env.update(check.env)
    try:
        completed = subprocess.run(
            check.command,
            cwd=check.cwd,
            check=False,
            env=env,
            capture_output=check.capture_output,
            text=check.capture_output,
            encoding="utf-8" if check.capture_output else None,
        )
    except FileNotFoundError as exc:
        if check.hint:
            print(f"[HINT] {check.hint}")
        missing = exc.filename or check.command[0]
        raise SystemExit(f"[FAIL] {check.name} could not start command: {missing}") from exc
    elapsed = time.monotonic() - started_at
    if check.capture_output:
        relay_completed_output(completed)
    if completed.returncode != 0:
        if check.hint:
            print(f"[HINT] {check.hint}")
        if check.name in {"frontend build", "frontend e2e"} and is_known_windows_node_realpath_eperm(completed):
            direct_command = "npm run build" if check.name == "frontend build" else "npm run test:e2e"
            raise SystemExit(
                f"[FAIL] {check.name} hit the known Windows/Codex Node realpath sandbox limitation; "
                f"run `cd frontend && {direct_command}` directly and keep its output as the {check.name} evidence"
            )
        raise SystemExit(f"[FAIL] {check.name} failed after {elapsed:.1f}s with exit code {completed.returncode}")
    print(f"[ OK ] {check.name} ({elapsed:.1f}s)")
    return elapsed


def format_command(command: list[str]) -> str:
    return " ".join(quote_part(part) for part in command)


def quote_part(part: str) -> str:
    if not part or any(ch.isspace() for ch in part):
        return f'"{part}"'
    return part


def tool_path(name: str) -> str:
    return shutil.which(name) or name


def flag_enabled(args: argparse.Namespace, name: str) -> bool:
    return bool(getattr(args, name, False))


def relay_completed_output(completed: subprocess.CompletedProcess[str]) -> None:
    if completed.stdout:
        write_console_text(completed.stdout)
    if completed.stderr:
        write_console_text(completed.stderr, stream=sys.stderr)


def is_known_windows_node_realpath_eperm(completed: subprocess.CompletedProcess[str]) -> bool:
    output = "\n".join(part for part in (completed.stdout, completed.stderr) if part)
    lowered = output.lower()
    return "realpathsync" in lowered and "eperm: operation not permitted, lstat" in lowered and "c:\\users\\chenn" in lowered


def write_console_text(text: str, *, stream=None) -> None:
    target = stream or sys.stdout
    ending = "" if text.endswith("\n") else "\n"
    try:
        print(text, end=ending, file=target)
    except UnicodeEncodeError:
        encoding = getattr(target, "encoding", None) or "utf-8"
        safe = text.encode(encoding, errors="replace").decode(encoding, errors="replace")
        print(safe, end=ending, file=target)


if __name__ == "__main__":
    sys.exit(main())
