#!/usr/bin/env python3
"""Precheck local runtime prerequisites before starting ShipSystem smoke tests."""

from __future__ import annotations

import json
import os
import shutil
import socket
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_RUNTIME_URLS = {
    "SHIPSYSTEM_BACKEND_URL": "http://localhost:8080",
    "SHIPSYSTEM_ANALYTICS_URL": "http://localhost:8090",
    "SHIPSYSTEM_FRONTEND_URL": "http://localhost:3000",
}


@dataclass(frozen=True)
class CheckResult:
    name: str
    ok: bool
    detail: str = ""


def main() -> int:
    ports = checked_ports()
    results = [
        check_tool("docker"),
        check_tool("docker compose", ["docker", "compose", "version"]),
        check_docker_config(),
        check_docker_service(),
        check_docker_daemon(),
        check_compose_config(),
        check_ports(ports),
        check_smoke_urls(),
    ]
    results = normalize_runtime_results(results)

    failures = 0
    for result in results:
        if result.ok:
            print(f"[ OK ] {result.name}{format_detail(result.detail)}")
            continue
        failures += 1
        print(f"[FAIL] {result.name}{format_detail(result.detail)}")

    if failures:
        print("")
        print("Runtime smoke prerequisites are not ready.")
        port_list = "/".join(str(port) for port in ports) if ports else "required runtime"
        print(f"Start Docker Desktop/daemon, ensure the current user can access docker_engine, and free ports {port_list}.")
        for hint in runtime_failure_hints(results):
            print(hint)
        hint = compose_override_hint(results)
        if hint:
            print(hint)
        return 1

    print("")
    print("Runtime smoke prerequisites look ready.")
    return 0


def check_tool(name: str, command: list[str] | None = None) -> CheckResult:
    executable = name.split()[0]
    if shutil.which(executable) is None:
        return CheckResult(name, False, f"{executable} not found in PATH")
    if command is None:
        command = [executable, "--version"]
    completed = run(command)
    return CheckResult(name, completed.returncode == 0, short_output(completed))


def check_docker_config() -> CheckResult:
    config_dir = docker_config_dir()
    try:
        config_dir.mkdir(parents=True, exist_ok=True)
        probe = config_dir / ".write-test"
        probe.write_text("ok", encoding="utf-8")
        probe.unlink(missing_ok=True)
    except OSError as exc:
        return CheckResult("docker config", False, f"{config_dir}: {exc}")
    return CheckResult("docker config", True, str(config_dir))


def check_docker_service() -> CheckResult:
    if os.name != "nt":
        return CheckResult("docker service", True, "not applicable")
    completed = run(
        [
            "powershell",
            "-NoProfile",
            "-Command",
            "(Get-Service -Name com.docker.service | Select-Object -ExpandProperty Status)",
        ]
    )
    if completed.returncode != 0:
        return CheckResult("docker service", False, short_output(completed))
    status = (completed.stdout or "").strip()
    return CheckResult("docker service", status.lower() == "running", explain_docker_service_status(status))


def check_docker_daemon() -> CheckResult:
    completed = run(["docker", "version", "--format", "{{.Server.Version}}"])
    detail = short_output(completed)
    if completed.returncode == 0:
        return CheckResult("docker daemon", True, detail)
    return CheckResult("docker daemon", False, explain_docker_daemon_failure(detail))


def check_compose_config() -> CheckResult:
    completed = run(["docker", "compose", "config", "--quiet"])
    return CheckResult("docker compose config", completed.returncode == 0, short_output(completed))


def check_ports(ports: tuple[int, ...] | None = None) -> CheckResult:
    if ports is None:
        ports = checked_ports()
    busy = [describe_busy_port(port) for port in ports if port_is_busy(port)]
    if busy:
        return CheckResult("runtime ports", False, "busy: " + ", ".join(busy))
    return CheckResult("runtime ports", True, "free: " + ", ".join(str(port) for port in ports))


def check_smoke_urls() -> CheckResult:
    backend = configured_runtime_url("SHIPSYSTEM_BACKEND_URL")
    analytics = configured_runtime_url("SHIPSYSTEM_ANALYTICS_URL")
    frontend = configured_runtime_url("SHIPSYSTEM_FRONTEND_URL")
    return CheckResult("smoke target URLs", True, f"backend={backend}, analytics={analytics}, frontend={frontend}")


def runtime_ports() -> tuple[int, ...]:
    ports = set()
    for env_name in DEFAULT_RUNTIME_URLS:
        port = port_from_url(configured_runtime_url(env_name))
        if port:
            ports.add(port)
    return tuple(sorted(ports))


def checked_ports() -> tuple[int, ...]:
    return tuple(sorted(set(runtime_ports()) | set(compose_published_ports())))


def configured_runtime_url(env_name: str) -> str:
    return os.getenv(env_name, DEFAULT_RUNTIME_URLS[env_name])


def port_from_url(value: str) -> int:
    if not value:
        return 0
    try:
        from urllib.parse import urlsplit

        return urlsplit(value).port or 0
    except ValueError:
        return 0


def port_is_busy(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.3)
        return sock.connect_ex(("127.0.0.1", port)) == 0


def describe_busy_port(port: int) -> str:
    if os.name != "nt":
        return str(port)
    pid = windows_listening_pid(port)
    if not pid:
        return str(port)
    process_name = windows_process_name(pid)
    if process_name:
        return f"{port}(pid={pid}, process={process_name})"
    return f"{port}(pid={pid})"


def windows_listening_pid(port: int) -> int:
    completed = run(["cmd", "/c", f"netstat -ano -p tcp | findstr LISTENING | findstr :{port}"])
    if completed.returncode != 0:
        return 0
    for line in (completed.stdout or "").splitlines():
        parts = line.split()
        if len(parts) >= 5:
            try:
                return int(parts[-1])
            except ValueError:
                continue
    return 0


def windows_process_name(pid: int) -> str:
    completed = run(
        [
            "powershell",
            "-NoProfile",
            "-Command",
            f"(Get-Process -Id {pid} | Select-Object -ExpandProperty ProcessName)",
        ]
    )
    if completed.returncode != 0:
        return ""
    return (completed.stdout or "").strip()


def compose_published_ports() -> tuple[int, ...]:
    completed = run(["docker", "compose", "config", "--format", "json"])
    if completed.returncode != 0 or not (completed.stdout or "").strip():
        return ()
    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError:
        return ()
    services = payload.get("services", {})
    ports: set[int] = set()
    for service in services.values():
        for port_spec in service.get("ports", []):
            published = port_spec.get("published")
            try:
                if published is not None:
                    ports.add(int(published))
            except (TypeError, ValueError):
                continue
    return tuple(sorted(ports))


def run(command: list[str]) -> subprocess.CompletedProcess[str]:
    env = os.environ.copy()
    env.setdefault("DOCKER_CONFIG", str(docker_config_dir()))
    return subprocess.run(
        command,
        cwd=ROOT,
        env=env,
        check=False,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )


def docker_config_dir() -> Path:
    return ROOT / ".docker-codex"


def compose_override_hint(results: list[CheckResult]) -> str:
    for result in results:
        if result.name == "runtime ports" and not result.ok and result.detail.startswith("busy:"):
            command = format_command([sys.executable, "scripts/generate_compose_local_override.py"])
            return (
                "Port conflicts detected. Generate a local override with free host ports via "
                f"`{command}` and start docker compose with `.docker-codex/compose.smoke.override.yml`."
            )
    return ""


def runtime_failure_hints(results: list[CheckResult]) -> list[str]:
    hints: list[str] = []
    by_name = {result.name: result for result in results}

    service_result = by_name.get("docker service")
    if service_result and not service_result.ok:
        service_detail = service_result.detail.lower()
        if service_detail.startswith("stopped"):
            hints.append(
                "Docker Desktop Service is installed but stopped. Start Docker Desktop or start `com.docker.service` from an elevated shell."
            )

    daemon_result = by_name.get("docker daemon")
    if daemon_result and not daemon_result.ok:
        daemon_detail = daemon_result.detail.lower()
        if "access denied" in daemon_detail or "permission denied" in daemon_detail:
            hints.append(
                "The current user cannot access the Docker daemon pipe. Start Docker Desktop elevated or grant this user Docker daemon access, then reopen the shell."
            )
        elif "pipe is missing" in daemon_detail:
            hints.append(
                "The Docker Desktop Linux engine pipe is missing. Wait for Docker Desktop to finish starting, or restart Docker Desktop."
            )

    return hints


def normalize_runtime_results(results: list[CheckResult]) -> list[CheckResult]:
    by_name = {result.name: result for result in results}
    service_result = by_name.get("docker service")
    daemon_result = by_name.get("docker daemon")
    if service_result and daemon_result and not service_result.ok and daemon_result.ok:
        replacement = CheckResult(
            service_result.name,
            True,
            service_result.detail + "; docker daemon is reachable from this shell",
        )
        normalized: list[CheckResult] = []
        for result in results:
            if result.name == replacement.name:
                normalized.append(replacement)
            else:
                normalized.append(result)
        return normalized
    return results


def format_command(command: list[str]) -> str:
    return " ".join(quote_part(part) for part in command)


def quote_part(part: str) -> str:
    if not part or any(ch.isspace() for ch in part):
        return f'"{part}"'
    return part


def short_output(completed: subprocess.CompletedProcess[str]) -> str:
    output = (completed.stdout or completed.stderr or "").strip()
    if len(output) > 260:
        output = output[:260] + "..."
    return output


def explain_docker_service_status(status: str) -> str:
    normalized = status.strip()
    if normalized.lower() == "stopped":
        return "Stopped (Docker Desktop Service is installed but not running)"
    return normalized or "unknown"


def explain_docker_daemon_failure(detail: str) -> str:
    normalized = detail.strip()
    lower = normalized.lower()
    if "access is denied" in lower or "permission denied" in lower:
        return normalized + " (access denied to Docker daemon pipe)"
    if "dockerdesktoplinuxengine" in lower and "system cannot find the file specified" in lower:
        return normalized + " (Docker Desktop Linux engine pipe is missing)"
    if "docker_engine" in lower and "system cannot find the file specified" in lower:
        return normalized + " (Docker daemon pipe is missing)"
    return normalized


def format_detail(detail: str) -> str:
    return f": {detail}" if detail else ""


if __name__ == "__main__":
    sys.exit(main())
