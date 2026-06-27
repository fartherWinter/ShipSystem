#!/usr/bin/env python3
"""Static checks that docs/openapi.yaml matches Go REST contracts."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
HANDLERS = ROOT / "backend" / "internal" / "handlers" / "handlers.go"
MAIN = ROOT / "backend" / "cmd" / "api" / "main.go"
OPENAPI = ROOT / "docs" / "openapi.yaml"
RBAC_MATRIX = ROOT / "docs" / "rbac_matrix.yaml"
REST_METHODS = {"GET", "POST", "PUT", "DELETE", "PATCH"}
AUTH_ROUTES = {("post", "/auth/login"), ("post", "/auth/logout")}
ROLE_CONSTANTS = {
    "middleware.RoleSuperAdmin": "super_admin",
    "middleware.RoleAdmin": "admin",
    "middleware.RoleDispatcher": "dispatcher",
    "middleware.RoleViewer": "viewer",
    "middleware.RoleAnalytics": "analytics_service",
}
MUTATING_METHODS = {"post", "put", "patch", "delete"}
BODY_REQUIRED_ROUTES = {
    ("post", "/auth/login"),
    ("post", "/ships"),
    ("put", "/ships/{id}"),
    ("post", "/ships/{id}/locations"),
    ("post", "/dispatch-events"),
    ("put", "/dispatch-events/{id}/status"),
    ("post", "/radar/reports"),
    ("post", "/analytics/simulate/battle/start"),
    ("post", "/analytics/simulate/battle/stop"),
}
BODY_OPTIONAL_ROUTES = {
    ("post", "/battle/sessions"),
    ("post", "/analytics/simulate/start"),
}
SUCCESS_STATUS = {
    ("post", "/auth/login"): "200",
    ("post", "/auth/logout"): "204",
    ("get", "/ships"): "200",
    ("post", "/ships"): "201",
    ("get", "/ships/{id}"): "200",
    ("put", "/ships/{id}"): "200",
    ("delete", "/ships/{id}"): "204",
    ("post", "/ships/{id}/locations"): "201",
    ("get", "/ships/{id}/tracks"): "200",
    ("get", "/alarms"): "200",
    ("put", "/alarms/{id}/ack"): "200",
    ("get", "/dispatch-events"): "200",
    ("post", "/dispatch-events"): "201",
    ("put", "/dispatch-events/{id}/status"): "200",
    ("get", "/battle/scenarios"): "200",
    ("get", "/battle/sessions"): "200",
    ("post", "/battle/sessions"): "201",
    ("get", "/battle/sessions/{sessionId}/timeline"): "200",
    ("get", "/battle/sessions/{sessionId}/snapshots"): "200",
    ("get", "/battle/sessions/{sessionId}/report"): "200",
    ("get", "/battle/sessions/{sessionId}/state"): "200",
    ("post", "/battle/sessions/{sessionId}/stop"): "200",
    ("post", "/radar/reports"): "202",
    ("get", "/analytics/simulate/status"): "200",
    ("post", "/analytics/simulate/start"): "200",
    ("post", "/analytics/simulate/stop"): "200",
    ("post", "/analytics/simulate/battle/start"): "200",
    ("post", "/analytics/simulate/battle/stop"): "200",
    ("get", "/rbac/users"): "200",
    ("get", "/rbac/roles"): "200",
    ("get", "/rbac/menus"): "200",
}
NOT_FOUND_ROUTES = {
    ("get", "/ships/{id}"),
    ("put", "/ships/{id}"),
    ("post", "/ships/{id}/locations"),
    ("get", "/ships/{id}/tracks"),
    ("put", "/alarms/{id}/ack"),
    ("put", "/dispatch-events/{id}/status"),
    ("get", "/battle/sessions/{sessionId}/timeline"),
    ("get", "/battle/sessions/{sessionId}/snapshots"),
    ("get", "/battle/sessions/{sessionId}/report"),
    ("get", "/battle/sessions/{sessionId}/state"),
    ("post", "/battle/sessions/{sessionId}/stop"),
    ("post", "/radar/reports"),
}
INTERNAL_ERROR_ROUTES = {
    ("get", "/ships"),
    ("post", "/ships"),
    ("get", "/ships/{id}"),
    ("put", "/ships/{id}"),
    ("delete", "/ships/{id}"),
    ("post", "/ships/{id}/locations"),
    ("get", "/ships/{id}/tracks"),
    ("get", "/alarms"),
    ("put", "/alarms/{id}/ack"),
    ("get", "/dispatch-events"),
    ("post", "/dispatch-events"),
    ("put", "/dispatch-events/{id}/status"),
    ("get", "/battle/sessions"),
    ("post", "/battle/sessions"),
    ("get", "/battle/sessions/{sessionId}/timeline"),
    ("get", "/battle/sessions/{sessionId}/snapshots"),
    ("get", "/battle/sessions/{sessionId}/report"),
    ("get", "/battle/sessions/{sessionId}/state"),
    ("post", "/battle/sessions/{sessionId}/stop"),
    ("post", "/radar/reports"),
    ("get", "/rbac/users"),
    ("get", "/rbac/roles"),
    ("get", "/rbac/menus"),
}
ANALYTICS_ROUTES = {
    ("get", "/analytics/simulate/status"),
    ("post", "/analytics/simulate/start"),
    ("post", "/analytics/simulate/stop"),
    ("post", "/analytics/simulate/battle/start"),
    ("post", "/analytics/simulate/battle/stop"),
}
PARAMETER_ROUTES = {
    ("get", "/ships"): {"Page", "Size", "Keyword"},
    ("get", "/ships/{id}"): {"NumericId"},
    ("put", "/ships/{id}"): {"NumericId"},
    ("delete", "/ships/{id}"): {"NumericId"},
    ("post", "/ships/{id}/locations"): {"NumericId"},
    ("get", "/ships/{id}/tracks"): {"NumericId", "StartTime", "EndTime"},
    ("get", "/alarms"): {"Page", "Size", "AlarmStatus"},
    ("put", "/alarms/{id}/ack"): {"NumericId"},
    ("get", "/dispatch-events"): {"Page", "Size", "DispatchStatus"},
    ("put", "/dispatch-events/{id}/status"): {"NumericId"},
    ("get", "/battle/sessions"): {"Page", "Size"},
    ("get", "/battle/sessions/{sessionId}/timeline"): {"SessionId"},
    ("get", "/battle/sessions/{sessionId}/snapshots"): {"SessionId", "FromTick", "ToTick"},
    ("get", "/battle/sessions/{sessionId}/report"): {"SessionId"},
    ("get", "/battle/sessions/{sessionId}/state"): {"SessionId"},
    ("post", "/battle/sessions/{sessionId}/stop"): {"SessionId"},
}


def main() -> int:
    handler_routes = parse_handler_rbac()
    matrix_roles = parse_matrix_roles()
    operations = parse_openapi_operations()
    failures: list[str] = []

    openapi_routes = set(operations)
    missing = set(handler_routes) | AUTH_ROUTES
    missing -= openapi_routes
    extra = openapi_routes - set(handler_routes) - AUTH_ROUTES
    if missing:
        failures.append("OpenAPI missing routes: " + ", ".join(format_route(route) for route in sorted(missing)))
    if extra:
        failures.append("OpenAPI has routes not registered in handlers.go: " + ", ".join(format_route(route) for route in sorted(extra)))

    text = OPENAPI.read_text(encoding="utf-8")
    for token in (
        "openapi: 3.0.3",
        "cookieAuth:",
        "bearerAuth:",
        "shipsystem_token",
        "ErrorResponse:",
        "AnalyticsErrorResponse:",
        "x-roles:",
    ):
        if token not in text:
            failures.append(f"OpenAPI must include {token}")

    for route, operation in sorted(operations.items()):
        method, path = route
        responses = operation.get("responses", {})
        success_status = SUCCESS_STATUS.get(route)
        if success_status and success_status not in responses:
            failures.append(f"OpenAPI {format_route(route)} must declare success response {success_status}")

        declared_roles = set(operation.get("x-roles", []))
        if route in AUTH_ROUTES:
            if declared_roles:
                failures.append(f"OpenAPI {format_route(route)} must keep x-roles empty for public auth route")
            security = operation.get("security")
            if security != []:
                failures.append(f"OpenAPI {format_route(route)} must explicitly declare security: []")
        else:
            actual_roles = handler_routes.get(route)
            if actual_roles is None:
                continue
            if declared_roles != actual_roles:
                failures.append(
                    f"OpenAPI x-roles mismatch for {format_route(route)}: "
                    f"openapi={format_roles(declared_roles)} handlers={format_roles(actual_roles)}"
                )
            if "security" in operation and operation.get("security") == []:
                failures.append(f"OpenAPI {format_route(route)} must not opt out of security")
            if not declared_roles:
                failures.append(f"OpenAPI {format_route(route)} must declare x-roles")

        unknown_roles = declared_roles - matrix_roles
        if unknown_roles:
            failures.append(f"OpenAPI {format_route(route)} contains unknown x-roles {format_roles(unknown_roles)}")

        request_body = operation.get("requestBody")
        if route in BODY_REQUIRED_ROUTES:
            if not isinstance(request_body, dict):
                failures.append(f"OpenAPI {format_route(route)} must declare requestBody")
            elif request_body.get("required") is not True:
                failures.append(f"OpenAPI {format_route(route)} requestBody must set required: true")
        elif route in BODY_OPTIONAL_ROUTES:
            if request_body is not None and not isinstance(request_body, dict):
                failures.append(f"OpenAPI {format_route(route)} requestBody must be an object when present")
        elif request_body is not None:
            failures.append(f"OpenAPI {format_route(route)} must not declare requestBody")

        expected_parameters = PARAMETER_ROUTES.get(route, set())
        actual_parameters = parse_parameter_refs(operation)
        if actual_parameters != expected_parameters:
            failures.append(
                f"OpenAPI parameters mismatch for {format_route(route)}: "
                f"openapi={format_names(actual_parameters)} expected={format_names(expected_parameters)}"
            )

        if route not in AUTH_ROUTES:
            for code in ("401", "403"):
                if code not in responses:
                    failures.append(f"OpenAPI {format_route(route)} must declare {code} response")
        if route in NOT_FOUND_ROUTES and "404" not in responses:
            failures.append(f"OpenAPI {format_route(route)} must declare 404 response")
        if route in INTERNAL_ERROR_ROUTES and "500" not in responses:
            failures.append(f"OpenAPI {format_route(route)} must declare 500 response")
        if route in ANALYTICS_ROUTES and "502" not in responses:
            failures.append(f"OpenAPI {format_route(route)} must declare 502 response")

        if method in MUTATING_METHODS and route not in {("post", "/auth/logout")}:
            if not any(code in responses for code in ("400", "404", "502")):
                failures.append(f"OpenAPI {format_route(route)} should declare a client or upstream error response")

    if failures:
        for failure in failures:
            print(f"[FAIL] {failure}")
        return 1

    print("openapi contract checks passed")
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


def parse_openapi_operations() -> dict[tuple[str, str], dict[str, Any]]:
    operations: dict[tuple[str, str], dict[str, Any]] = {}
    data = parse_yaml_like(OPENAPI.read_text(encoding="utf-8"))
    paths = data.get("paths")
    if not isinstance(paths, dict):
        raise RuntimeError("docs/openapi.yaml must contain object section paths")
    for path, path_item in paths.items():
        if not isinstance(path_item, dict):
            continue
        for method, operation in path_item.items():
            if str(method).upper() not in REST_METHODS:
                continue
            if not isinstance(operation, dict):
                raise RuntimeError(f"OpenAPI operation {method.upper()} {path} must be a mapping")
            operations[(str(method).lower(), str(path))] = operation
    return operations


def parse_matrix_roles() -> set[str]:
    matrix = parse_yaml_like(RBAC_MATRIX.read_text(encoding="utf-8"))
    roles = matrix.get("roles")
    if not isinstance(roles, list):
        raise RuntimeError("docs/rbac_matrix.yaml must contain list section roles")
    return {str(item) for item in roles}


def parse_parameter_refs(operation: dict[str, Any]) -> set[str]:
    refs: set[str] = set()
    parameters = operation.get("parameters", [])
    if not isinstance(parameters, list):
        return refs
    for parameter in parameters:
        if not isinstance(parameter, dict):
            continue
        ref = parameter.get("$ref")
        if not isinstance(ref, str):
            continue
        refs.add(ref.rsplit("/", 1)[-1])
    return refs


def parse_yaml_like(text: str) -> Any:
    lines = [strip_comment(line.rstrip("\n")) for line in text.splitlines()]
    return parse_block(lines, 0, 0)[0]


def strip_comment(line: str) -> str:
    if "#" not in line:
        return line.rstrip()
    in_single = False
    in_double = False
    for index, ch in enumerate(line):
        if ch == "'" and not in_double:
            in_single = not in_single
        elif ch == '"' and not in_single:
            in_double = not in_double
        elif ch == "#" and not in_single and not in_double:
            return line[:index].rstrip()
    return line.rstrip()


def parse_block(lines: list[str], start: int, indent: int) -> tuple[Any, int]:
    index = skip_blank(lines, start)
    if index >= len(lines):
        return {}, index
    current = lines[index]
    if indentation(current) < indent:
        return {}, index
    stripped = current[indent:]
    if stripped.startswith("- "):
        return parse_list(lines, index, indent)
    return parse_dict(lines, index, indent)


def parse_dict(lines: list[str], start: int, indent: int) -> tuple[dict[str, Any], int]:
    result: dict[str, Any] = {}
    index = start
    while index < len(lines):
        line = lines[index]
        if not line.strip():
            index += 1
            continue
        line_indent = indentation(line)
        if line_indent < indent:
            break
        if line_indent > indent:
            raise RuntimeError(f"invalid indentation in YAML near: {line}")
        stripped = line[indent:]
        if stripped.startswith("- "):
            break
        key, has_value, raw_value = split_key_value(stripped)
        if not has_value:
            raise RuntimeError(f"invalid mapping line in YAML: {line}")
        key = normalize_key(key)
        if raw_value == "":
            child, next_index = parse_block(lines, index + 1, indent + 2)
            result[key] = child
            index = next_index
            continue
        if raw_value in {"|", ">"}:
            scalar, next_index = parse_multiline_scalar(lines, index + 1, indent + 2)
            result[key] = scalar
            index = next_index
            continue
        result[key] = parse_scalar(raw_value)
        index += 1
    return result, index


def parse_list(lines: list[str], start: int, indent: int) -> tuple[list[Any], int]:
    result: list[Any] = []
    index = start
    while index < len(lines):
        line = lines[index]
        if not line.strip():
            index += 1
            continue
        line_indent = indentation(line)
        if line_indent < indent:
            break
        if line_indent != indent:
            raise RuntimeError(f"invalid list indentation in YAML near: {line}")
        stripped = line[indent:]
        if not stripped.startswith("- "):
            break
        item_text = stripped[2:].strip()
        if item_text == "":
            child, next_index = parse_block(lines, index + 1, indent + 2)
            result.append(child)
            index = next_index
            continue
        if ":" in item_text and not item_text.startswith("{") and not item_text.startswith("["):
            key, has_value, raw_value = split_key_value(item_text)
            if not has_value:
                raise RuntimeError(f"invalid list mapping line in YAML: {line}")
            key = normalize_key(key)
            item: dict[str, Any] = {}
            if raw_value == "":
                child, next_index = parse_block(lines, index + 1, indent + 4)
                item[key] = child
                index = next_index
            elif raw_value in {"|", ">"}:
                scalar, next_index = parse_multiline_scalar(lines, index + 1, indent + 4)
                item[key] = scalar
                index = next_index
            else:
                item[key] = parse_scalar(raw_value)
                index += 1
            if index < len(lines):
                nested_indent = indent + 2
                nested, next_index = parse_optional_dict(lines, index, nested_indent)
                if nested:
                    item.update(nested)
                    index = next_index
            result.append(item)
            continue
        result.append(parse_scalar(item_text))
        index += 1
    return result, index


def parse_optional_dict(lines: list[str], start: int, indent: int) -> tuple[dict[str, Any], int]:
    result: dict[str, Any] = {}
    index = start
    while index < len(lines):
        line = lines[index]
        if not line.strip():
            index += 1
            continue
        line_indent = indentation(line)
        if line_indent < indent:
            break
        if line_indent != indent:
            break
        stripped = line[indent:]
        if stripped.startswith("- "):
            break
        key, has_value, raw_value = split_key_value(stripped)
        if not has_value:
            break
        key = normalize_key(key)
        if raw_value == "":
            child, next_index = parse_block(lines, index + 1, indent + 2)
            result[key] = child
            index = next_index
            continue
        if raw_value in {"|", ">"}:
            scalar, next_index = parse_multiline_scalar(lines, index + 1, indent + 2)
            result[key] = scalar
            index = next_index
            continue
        result[key] = parse_scalar(raw_value)
        index += 1
    return result, index


def parse_multiline_scalar(lines: list[str], start: int, indent: int) -> tuple[str, int]:
    chunks: list[str] = []
    index = start
    while index < len(lines):
        line = lines[index]
        if not line.strip():
            chunks.append("")
            index += 1
            continue
        line_indent = indentation(line)
        if line_indent < indent:
            break
        chunks.append(line[indent:])
        index += 1
    return "\n".join(chunks).rstrip(), index


def split_key_value(text: str) -> tuple[str, bool, str]:
    in_single = False
    in_double = False
    depth_brace = 0
    depth_bracket = 0
    for index, ch in enumerate(text):
        if ch == "'" and not in_double:
            in_single = not in_single
        elif ch == '"' and not in_single:
            in_double = not in_double
        elif ch == "{" and not in_single and not in_double:
            depth_brace += 1
        elif ch == "}" and not in_single and not in_double and depth_brace > 0:
            depth_brace -= 1
        elif ch == "[" and not in_single and not in_double:
            depth_bracket += 1
        elif ch == "]" and not in_single and not in_double and depth_bracket > 0:
            depth_bracket -= 1
        elif ch == ":" and not in_single and not in_double and depth_brace == 0 and depth_bracket == 0:
            key = text[:index].strip()
            value = text[index + 1 :].strip()
            return key, True, value
    return text.strip(), False, ""


def parse_scalar(value: str) -> Any:
    value = value.strip()
    if value == "":
        return ""
    if value == "true":
        return True
    if value == "false":
        return False
    if value == "null":
        return None
    if value.startswith('"') and value.endswith('"'):
        return value[1:-1]
    if value.startswith("'") and value.endswith("'"):
        return value[1:-1]
    if value.startswith("[") and value.endswith("]"):
        return parse_inline_list(value)
    if value.startswith("{") and value.endswith("}"):
        return parse_inline_dict(value)
    if re.fullmatch(r"-?\d+", value):
        try:
            return int(value)
        except ValueError:
            return value
    if re.fullmatch(r"-?\d+\.\d+", value):
        try:
            return float(value)
        except ValueError:
            return value
    return value


def normalize_key(key: str) -> str:
    key = key.strip()
    if len(key) >= 2 and ((key[0] == '"' and key[-1] == '"') or (key[0] == "'" and key[-1] == "'")):
        return key[1:-1]
    return key


def parse_inline_list(value: str) -> list[Any]:
    inner = value[1:-1].strip()
    if not inner:
        return []
    return [parse_scalar(part.strip()) for part in split_inline(inner)]


def parse_inline_dict(value: str) -> dict[str, Any]:
    inner = value[1:-1].strip()
    if not inner:
        return {}
    result: dict[str, Any] = {}
    for part in split_inline(inner):
        key, has_value, raw_value = split_key_value(part.strip())
        if not has_value:
            raise RuntimeError(f"invalid inline mapping entry: {part}")
        result[normalize_key(key)] = parse_scalar(raw_value)
    return result


def split_inline(text: str) -> list[str]:
    parts: list[str] = []
    current: list[str] = []
    in_single = False
    in_double = False
    depth_brace = 0
    depth_bracket = 0
    for ch in text:
        if ch == "'" and not in_double:
            in_single = not in_single
        elif ch == '"' and not in_single:
            in_double = not in_double
        elif ch == "{" and not in_single and not in_double:
            depth_brace += 1
        elif ch == "}" and not in_single and not in_double and depth_brace > 0:
            depth_brace -= 1
        elif ch == "[" and not in_single and not in_double:
            depth_bracket += 1
        elif ch == "]" and not in_single and not in_double and depth_bracket > 0:
            depth_bracket -= 1
        if ch == "," and not in_single and not in_double and depth_brace == 0 and depth_bracket == 0:
            parts.append("".join(current).strip())
            current = []
            continue
        current.append(ch)
    if current:
        parts.append("".join(current).strip())
    return parts


def skip_blank(lines: list[str], index: int) -> int:
    while index < len(lines) and not lines[index].strip():
        index += 1
    return index


def indentation(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def gin_to_openapi_path(path: str) -> str:
    return re.sub(r":([A-Za-z_][A-Za-z0-9_]*)", r"{\1}", path)


def format_route(route: tuple[str, str]) -> str:
    method, path = route
    return f"{method.upper()} {path}"


def format_roles(roles: set[str]) -> str:
    return "[" + ", ".join(sorted(roles)) + "]"


def format_names(items: set[str]) -> str:
    return "[" + ", ".join(sorted(items)) + "]"


if __name__ == "__main__":
    sys.exit(main())
