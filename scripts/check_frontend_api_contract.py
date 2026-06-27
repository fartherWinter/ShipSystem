#!/usr/bin/env python3
"""Static checks that frontend API calls are covered by docs/openapi.yaml."""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
FRONTEND_CLIENT = ROOT / "frontend" / "src" / "api" / "client.ts"
OPENAPI = ROOT / "docs" / "openapi.yaml"
REST_METHODS = {"GET", "POST", "PUT", "DELETE", "PATCH"}


def main() -> int:
    frontend_routes = parse_frontend_routes()
    openapi_routes = parse_openapi_routes()
    failures: list[str] = []

    missing = frontend_routes - openapi_routes
    if missing:
        failures.append("frontend API calls missing from OpenAPI: " + ", ".join(format_route(route) for route in sorted(missing)))

    if failures:
        for failure in failures:
            print(f"[FAIL] {failure}")
        return 1

    print("frontend api contract checks passed")
    return 0


def parse_frontend_routes() -> set[tuple[str, str]]:
    text = FRONTEND_CLIENT.read_text(encoding="utf-8")
    routes: set[tuple[str, str]] = set()
    for start in find_request_call_starts(text):
        path, end_index = extract_first_argument(text, start)
        method = extract_request_method(text, end_index)
        routes.add((method.lower(), normalize_frontend_path(path)))
    return routes


def find_request_call_starts(text: str) -> list[int]:
    starts: list[int] = []
    index = 0
    while True:
        index = text.find("request", index)
        if index == -1:
            break
        prefix = text[max(0, index - 24) : index].strip()
        if prefix.endswith("function") or prefix.endswith("async function"):
            index += len("request")
            continue
        next_char_index = index + len("request")
        if next_char_index < len(text) and text[next_char_index] == "<":
            open_paren = text.find("(", next_char_index)
        else:
            open_paren = next_char_index
        if open_paren < len(text) and text[open_paren] == "(":
            starts.append(open_paren + 1)
        index = next_char_index
    return starts


def extract_first_argument(text: str, start: int) -> tuple[str, int]:
    index = start
    while index < len(text) and text[index].isspace():
        index += 1
    if index >= len(text) or text[index] not in {"'", "`"}:
        raise RuntimeError("frontend request path must be a string literal or template literal")
    quote = text[index]
    index += 1
    value_chars: list[str] = []
    brace_depth = 0
    while index < len(text):
        char = text[index]
        if char == "\\":
            value_chars.append(char)
            if index + 1 < len(text):
                value_chars.append(text[index + 1])
            index += 2
            continue
        if quote == "`" and char == "$" and index + 1 < len(text) and text[index + 1] == "{":
            brace_depth += 1
            value_chars.append("${")
            index += 2
            continue
        if brace_depth:
            if char == "{":
                brace_depth += 1
            elif char == "}":
                brace_depth -= 1
            value_chars.append(char)
            index += 1
            continue
        if char == quote:
            return "".join(value_chars), index + 1
        value_chars.append(char)
        index += 1
    raise RuntimeError("unterminated frontend request path")


def extract_request_method(text: str, first_arg_end: int) -> str:
    cursor = first_arg_end
    while cursor < len(text) and text[cursor].isspace():
        cursor += 1
    if cursor >= len(text) or text[cursor] != ",":
        return "GET"
    close_index = find_call_close(text, cursor)
    options_text = text[cursor:close_index]
    match = re.search(r"method:\s*'([A-Z]+)'", options_text)
    return match.group(1) if match else "GET"


def find_call_close(text: str, start: int) -> int:
    depth = 1
    index = start
    quote = ""
    while index < len(text):
        char = text[index]
        if quote:
            if char == "\\":
                index += 2
                continue
            if char == quote:
                quote = ""
            index += 1
            continue
        if char in {"'", '"', "`"}:
            quote = char
        elif char == "(":
            depth += 1
        elif char == ")":
            depth -= 1
            if depth == 0:
                return index
        index += 1
    return len(text)


def normalize_frontend_path(path: str) -> str:
    path = path.split("?", 1)[0]
    path = re.sub(r"\$\{query\s.*$", "", path)
    path = re.sub(r"\$\{encodeURIComponent\(([^)]+)\)\}", lambda match: "{" + normalize_param(match.group(1)) + "}", path)
    path = re.sub(r"\$\{([^}]+)\}", lambda match: "{" + normalize_param(match.group(1)) + "}", path)
    return path


def normalize_param(value: str) -> str:
    value = value.strip()
    if value in {"id", "shipId"}:
        return "id"
    if value == "sessionId":
        return "sessionId"
    return value


def parse_openapi_routes() -> set[tuple[str, str]]:
    routes: set[tuple[str, str]] = set()
    current_path = ""
    for raw_line in OPENAPI.read_text(encoding="utf-8").splitlines():
        if raw_line.startswith("  /") and raw_line.rstrip().endswith(":"):
            current_path = raw_line.strip()[:-1]
            continue
        if current_path and raw_line.startswith("    "):
            key = raw_line.strip().split(":", 1)[0]
            if key.upper() in REST_METHODS:
                routes.add((key.lower(), current_path))
    return routes


def format_route(route: tuple[str, str]) -> str:
    method, path = route
    return f"{method.upper()} {path}"


if __name__ == "__main__":
    sys.exit(main())
