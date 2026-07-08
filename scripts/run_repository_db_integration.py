#!/usr/bin/env python3
"""Run repository PostgreSQL integration tests against an isolated temporary PostGIS container."""

from __future__ import annotations

import argparse
import os
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
TEMP_DIR = ROOT / ".docker-codex"
DEFAULT_PROJECT = "shipsystem-repo-test"
DEFAULT_DB_NAME = "shipsystem_repo_test"
DEFAULT_DB_USER = "shipsystem"
DEFAULT_DB_PASSWORD = "shipsystem"
DEFAULT_HOST_PORT = 15432
CONTAINER_PORT = 5432
READY_TIMEOUT_SECONDS = 60


def main() -> int:
    args = parse_args()
    ensure_tool("docker")
    ensure_tool("go")

    host_port = choose_port(args.port)
    compose_file = Path(args.compose_file).resolve()
    compose_file.parent.mkdir(parents=True, exist_ok=True)
    project = args.project
    dsn = build_dsn(host_port, args.db_name, args.db_user, args.db_password)

    compose_file.write_text(
        build_compose_yaml(
            host_port=host_port,
            db_name=args.db_name,
            db_user=args.db_user,
            db_password=args.db_password,
        ),
        encoding="utf-8",
    )

    print(f"compose_file={compose_file}")
    print(f"project={project}")
    print(f"dsn={dsn}")

    keep_db = args.keep_database
    try:
        run_compose(project, compose_file, ["up", "-d", "postgres-test"])
        wait_for_postgres(project, compose_file, args.db_user, args.db_name, timeout_seconds=args.ready_timeout)

        env = os.environ.copy()
        env["SHIPSYSTEM_REPOSITORY_TEST_DSN"] = dsn
        command = [tool_path("go"), "test", "./internal/repositories", "-run", "Test.*Concurrent|TestCreateLocationRejectsSoftDeletedShip|TestCreateBattleEventsReturnsOnlyInsertedEvents", "-count=1"]
        completed = subprocess.run(command, cwd=ROOT / "backend", env=env, check=False)
        if completed.returncode != 0:
            raise SystemExit(completed.returncode)
        print("repository integration tests passed")
        return 0
    finally:
        if not keep_db:
            down_result = run_compose(project, compose_file, ["down", "-v"], check=False)
            if down_result.returncode != 0:
                print("[WARN] failed to clean up temporary repository test database", file=sys.stderr)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", default=DEFAULT_PROJECT, help="Docker Compose project name")
    parser.add_argument("--port", type=int, default=DEFAULT_HOST_PORT, help="Preferred published PostgreSQL port")
    parser.add_argument("--db-name", default=DEFAULT_DB_NAME, help="Temporary database name")
    parser.add_argument("--db-user", default=DEFAULT_DB_USER, help="Temporary database user")
    parser.add_argument("--db-password", default=DEFAULT_DB_PASSWORD, help="Temporary database password")
    parser.add_argument(
        "--compose-file",
        default=str(TEMP_DIR / "docker-compose.repo-test.yml"),
        help="Path to the generated temporary compose file",
    )
    parser.add_argument("--keep-database", action="store_true", help="Keep the temporary database container and volume after tests")
    parser.add_argument("--ready-timeout", type=int, default=READY_TIMEOUT_SECONDS, help="Seconds to wait for postgres readiness")
    return parser.parse_args()


def ensure_tool(name: str) -> None:
    if shutil.which(name) is None:
        raise SystemExit(f"[FAIL] required tool not found in PATH: {name}")


def choose_port(preferred: int, search_window: int = 50) -> int:
    if preferred > 0 and port_is_free(preferred):
        return preferred
    for offset in range(1, search_window + 1):
        candidate = preferred + offset
        if port_is_free(candidate):
            return candidate
    raise SystemExit(f"[FAIL] no free port found near {preferred}")


def port_is_free(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.2)
        return sock.connect_ex(("127.0.0.1", port)) != 0


def build_compose_yaml(*, host_port: int, db_name: str, db_user: str, db_password: str) -> str:
    return (
        "services:\n"
        "  postgres-test:\n"
        "    image: postgis/postgis:17-3.5\n"
        "    environment:\n"
        f"      POSTGRES_DB: {db_name}\n"
        f"      POSTGRES_USER: {db_user}\n"
        f"      POSTGRES_PASSWORD: {db_password}\n"
        "      TZ: Asia/Shanghai\n"
        "    ports:\n"
        f"      - \"{host_port}:{CONTAINER_PORT}\"\n"
        "    volumes:\n"
        "      - repo_test_postgres_data:/var/lib/postgresql/data\n"
        "      - ./backend/migrations:/docker-entrypoint-initdb.d:ro\n"
        "    healthcheck:\n"
        f"      test: [\"CMD-SHELL\", \"pg_isready -U {db_user} -d {db_name}\"]\n"
        "      interval: 5s\n"
        "      timeout: 5s\n"
        "      retries: 12\n"
        "volumes:\n"
        "  repo_test_postgres_data:\n"
    )


def build_dsn(host_port: int, db_name: str, db_user: str, db_password: str) -> str:
    return (
        f"host=127.0.0.1 user={db_user} password={db_password} dbname={db_name} "
        f"port={host_port} sslmode=disable TimeZone=Asia/Shanghai"
    )


def tool_path(name: str) -> str:
    return shutil.which(name) or name


def compose_base_command(project: str, compose_file: Path) -> list[str]:
    return [tool_path("docker"), "compose", "-p", project, "-f", str(compose_file)]


def run_compose(project: str, compose_file: Path, args: list[str], *, check: bool = True) -> subprocess.CompletedProcess[str]:
    command = compose_base_command(project, compose_file) + args
    completed = subprocess.run(command, cwd=ROOT, check=False, text=True, encoding="utf-8", errors="replace")
    if check and completed.returncode != 0:
        raise SystemExit(completed.returncode)
    return completed


def wait_for_postgres(project: str, compose_file: Path, db_user: str, db_name: str, *, timeout_seconds: int) -> None:
    deadline = time.monotonic() + timeout_seconds
    command = compose_base_command(project, compose_file) + ["exec", "-T", "postgres-test", "pg_isready", "-U", db_user, "-d", db_name]
    while time.monotonic() < deadline:
        completed = subprocess.run(command, cwd=ROOT, check=False, capture_output=True, text=True, encoding="utf-8", errors="replace")
        if completed.returncode == 0:
            return
        time.sleep(1)
    raise SystemExit("[FAIL] temporary PostGIS test database did not become ready in time")


if __name__ == "__main__":
    raise SystemExit(main())