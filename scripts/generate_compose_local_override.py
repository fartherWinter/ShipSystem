#!/usr/bin/env python3
"""Generate a local docker compose override with conflict-free published ports."""

from __future__ import annotations

import argparse
import socket
from dataclasses import dataclass
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_OUTPUT = ROOT / ".docker-codex" / "compose.smoke.override.yml"
DEFAULT_PORTS = {
    "postgres": 5432,
    "backend": 8080,
    "analytics": 8090,
    "frontend": 3000,
}
CONTAINER_PORTS = {
    "postgres": 5432,
    "backend": 8080,
    "analytics": 8090,
    "frontend": 8080,
}


@dataclass(frozen=True)
class PortPlan:
    postgres: int
    backend: int
    analytics: int
    frontend: int


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="Override file path to write")
    args = parser.parse_args()

    plan = select_port_plan()
    output = Path(args.output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(build_override_yaml(plan), encoding="utf-8")

    print(f"override={output}")
    print(f"backend_url=http://127.0.0.1:{plan.backend}")
    print(f"analytics_url=http://127.0.0.1:{plan.analytics}")
    print(f"frontend_url=http://127.0.0.1:{plan.frontend}")
    print(f"ws_url=ws://127.0.0.1:{plan.backend}/ws/monitor")
    print(f"postgres_published={plan.postgres}")
    return 0


def select_port_plan(is_free= None) -> PortPlan:
    if is_free is None:
        is_free = port_is_free
    return PortPlan(
        postgres=choose_port(DEFAULT_PORTS["postgres"], is_free=is_free),
        backend=choose_port(DEFAULT_PORTS["backend"], is_free=is_free),
        analytics=choose_port(DEFAULT_PORTS["analytics"], is_free=is_free),
        frontend=choose_port(DEFAULT_PORTS["frontend"], is_free=is_free),
    )


def choose_port(preferred: int, *, is_free, search_window: int = 50) -> int:
    if preferred > 0 and is_free(preferred):
        return preferred
    for offset in range(1, search_window + 1):
        candidate = preferred + offset
        if is_free(candidate):
            return candidate
    raise RuntimeError(f"no free port found near {preferred}")


def port_is_free(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.2)
        return sock.connect_ex(("127.0.0.1", port)) != 0


def build_override_yaml(plan: PortPlan) -> str:
    return (
        "services:\n"
        "  postgres:\n"
        "    ports:\n"
        f"      - \"{plan.postgres}:{CONTAINER_PORTS['postgres']}\"\n"
        "  backend:\n"
        "    ports:\n"
        f"      - \"{plan.backend}:{CONTAINER_PORTS['backend']}\"\n"
        "  analytics:\n"
        "    ports:\n"
        f"      - \"{plan.analytics}:{CONTAINER_PORTS['analytics']}\"\n"
        "  frontend:\n"
        "    ports:\n"
        f"      - \"{plan.frontend}:{CONTAINER_PORTS['frontend']}\"\n"
    )


if __name__ == "__main__":
    raise SystemExit(main())
