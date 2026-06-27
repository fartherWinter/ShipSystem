#!/usr/bin/env python3
"""Static checks that docs/rbac_matrix.yaml matches Go route middleware."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
HANDLERS = ROOT / "backend" / "internal" / "handlers" / "handlers.go"
FRONTEND_APP = ROOT / "frontend" / "src" / "App.tsx"
RBAC_MATRIX = ROOT / "docs" / "rbac_matrix.yaml"
ROLES = {"super_admin", "admin", "dispatcher", "viewer", "analytics_service"}
ROLE_CONSTANTS = {
    "middleware.RoleSuperAdmin": "super_admin",
    "middleware.RoleAdmin": "admin",
    "middleware.RoleDispatcher": "dispatcher",
    "middleware.RoleViewer": "viewer",
    "middleware.RoleAnalytics": "analytics_service",
}
REST_METHODS = {"GET", "POST", "PUT", "DELETE", "PATCH"}


def main() -> int:
    handler_routes = parse_handler_rbac()
    frontend_routes = parse_frontend_rbac()
    matrix = parse_matrix()
    matrix_routes = matrix_section(matrix, "rest")
    matrix_frontend = matrix_section(matrix, "frontend")
    failures: list[str] = []

    matrix_route_map = {
        (str(item["method"]).lower(), str(item["path"])): set(item["roles"])
        for item in matrix_routes
    }
    missing = set(handler_routes) - set(matrix_route_map)
    extra = set(matrix_route_map) - set(handler_routes)
    if missing:
        failures.append("RBAC matrix missing routes: " + ", ".join(format_route(route) for route in sorted(missing)))
    if extra:
        failures.append("RBAC matrix has routes not registered in handlers.go: " + ", ".join(format_route(route) for route in sorted(extra)))

    for route, actual_roles in sorted(handler_routes.items()):
        expected_roles = matrix_route_map.get(route)
        if expected_roles is not None and expected_roles != actual_roles:
            failures.append(
                f"RBAC roles mismatch for {format_route(route)}: "
                f"matrix={format_roles(expected_roles)} handlers={format_roles(actual_roles)}"
            )

    matrix_frontend_map = {
        str(item["path"]): set(item["roles"])
        for item in matrix_frontend
    }
    frontend_missing = set(frontend_routes) - set(matrix_frontend_map)
    frontend_extra = set(matrix_frontend_map) - set(frontend_routes)
    if frontend_missing:
        failures.append("RBAC matrix missing frontend pages: " + ", ".join(sorted(frontend_missing)))
    if frontend_extra:
        failures.append("RBAC matrix has frontend pages not registered in App.tsx: " + ", ".join(sorted(frontend_extra)))
    for path, actual_roles in sorted(frontend_routes.items()):
        expected_roles = matrix_frontend_map.get(path)
        if expected_roles is not None and expected_roles != actual_roles:
            failures.append(
                f"RBAC frontend roles mismatch for {path}: "
                f"matrix={format_roles(expected_roles)} app={format_roles(actual_roles)}"
            )

    matrix_roles = set(matrix.get("roles", []))
    if matrix_roles != ROLES:
        failures.append(f"RBAC matrix roles must be {format_roles(ROLES)}, got {format_roles(matrix_roles)}")

    for item in matrix_routes + matrix_frontend:
        roles = set(item.get("roles", []))
        unknown = roles - ROLES
        if unknown:
            failures.append(f"RBAC matrix contains unknown roles for {item.get('path')}: {format_roles(unknown)}")

    if failures:
        for failure in failures:
            print(f"[FAIL] {failure}")
        return 1

    print("rbac matrix checks passed")
    return 0


def parse_handler_rbac() -> dict[tuple[str, str], set[str]]:
    text = HANDLERS.read_text(encoding="utf-8")
    aliases = parse_role_aliases(text)
    routes: dict[tuple[str, str], set[str]] = {}
    route_pattern = re.compile(
        r"\br\.(GET|POST|PUT|DELETE|PATCH)\(\s*\"([^\"]+)\"\s*,\s*([A-Za-z_][A-Za-z0-9_]*)\s*,",
    )
    for method, path, alias in route_pattern.findall(text):
        if alias not in aliases:
            raise RuntimeError(f"unknown RBAC middleware alias {alias} for {method} {path}")
        routes[(method.lower(), gin_to_openapi_path(path))] = aliases[alias]
    return routes


def parse_role_aliases(text: str) -> dict[str, set[str]]:
    aliases: dict[str, set[str]] = {}
    for name, args in re.findall(r"(\w+)\s*:=\s*middleware\.RequireRoles\(([^)]*)\)", text):
        roles = {ROLE_CONSTANTS[arg.strip()] for arg in args.split(",") if arg.strip()}
        if not roles:
            roles = {"super_admin"}
        aliases[name] = roles
    return aliases


def parse_frontend_rbac() -> dict[str, set[str]]:
    text = FRONTEND_APP.read_text(encoding="utf-8")
    routes_match = re.search(r"const routes = \[(?P<body>.*?)\n\];", text, flags=re.S)
    if not routes_match:
        raise RuntimeError("cannot locate frontend routes array in App.tsx")
    routes_text = routes_match.group("body")
    pattern = re.compile(r"path:\s*'([^']+)'.*?roles:\s*\[([^\]]+)\]", flags=re.S)
    routes: dict[str, set[str]] = {}
    for path, roles_text in pattern.findall(routes_text):
        roles = {item.strip().strip("'") for item in roles_text.split(",") if item.strip()}
        routes[path] = roles
    return routes


def parse_matrix() -> dict[str, Any]:
    result: dict[str, Any] = {}
    current_section = ""
    current_item: dict[str, Any] | None = None
    for raw_line in RBAC_MATRIX.read_text(encoding="utf-8").splitlines():
        line = raw_line.rstrip()
        if not line or line.lstrip().startswith("#"):
            continue
        if not raw_line.startswith(" "):
            key, _, value = line.partition(":")
            key = key.strip()
            if value.strip():
                result[key] = parse_scalar(value.strip())
                current_section = ""
            else:
                result[key] = []
                current_section = key
            current_item = None
            continue
        if current_section == "roles" and line.strip().startswith("- "):
            result[current_section].append(line.strip()[2:].strip())
            continue
        if current_section in {"rest", "frontend"}:
            stripped = line.strip()
            if stripped.startswith("- "):
                current_item = {}
                result[current_section].append(current_item)
                stripped = stripped[2:].strip()
                if stripped:
                    key, _, value = stripped.partition(":")
                    current_item[key.strip()] = parse_scalar(value.strip())
                continue
            if current_item is None:
                raise RuntimeError(f"invalid RBAC matrix line: {line}")
            key, _, value = stripped.partition(":")
            current_item[key.strip()] = parse_scalar(value.strip())
    return result


def parse_scalar(value: str) -> Any:
    value = value.strip()
    if value.startswith("[") and value.endswith("]"):
        inner = value[1:-1].strip()
        if not inner:
            return []
        return [item.strip() for item in inner.split(",")]
    return value


def matrix_section(matrix: dict[str, Any], name: str) -> list[dict[str, Any]]:
    value = matrix.get(name)
    if not isinstance(value, list):
        raise RuntimeError(f"docs/rbac_matrix.yaml must contain list section {name}")
    return value


def gin_to_openapi_path(path: str) -> str:
    return re.sub(r":([A-Za-z_][A-Za-z0-9_]*)", r"{\1}", path)


def format_route(route: tuple[str, str]) -> str:
    method, path = route
    return f"{method.upper()} {path}"


def format_roles(roles: set[str]) -> str:
    return "[" + ", ".join(sorted(roles)) + "]"


if __name__ == "__main__":
    sys.exit(main())
