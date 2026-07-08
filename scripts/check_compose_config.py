#!/usr/bin/env python3
"""Static checks for the local Docker Compose production baseline."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
COMPOSE_FILE = ROOT / "docker-compose.yml"
ENV_EXAMPLE_FILE = ROOT / ".env.example"
DOCKERFILES = {
    "backend": ROOT / "backend" / "Dockerfile",
    "analytics": ROOT / "analytics" / "Dockerfile",
    "frontend": ROOT / "frontend" / "Dockerfile",
}
FRONTEND_NGINX_CONF = ROOT / "frontend" / "nginx.conf"
ROOT_DOCKERIGNORE = ROOT / ".dockerignore"
REQUIRED_SERVICES = {"postgres", "backend", "analytics", "frontend"}
REQUIRED_FRONTEND_HEADERS = {
    "X-Content-Type-Options": "nosniff",
    "X-Frame-Options": "DENY",
    "Referrer-Policy": "strict-origin-when-cross-origin",
    "Permissions-Policy": "geolocation=(), microphone=(), camera=()",
    "Content-Security-Policy": "default-src 'self'",
}
REQUIRED_DOCKERIGNORE_PATTERNS = {
    ".git",
    ".codegraph",
    ".uv-cache",
    ".uv-python",
    ".docker-codex",
    ".env",
    ".env.*",
    "!.env.example",
    "**/__pycache__/",
    "**/*.py[cod]",
    "frontend/node_modules/",
    "frontend/dist/",
    "frontend/*.tsbuildinfo",
    "*.log",
}
REQUIRED_BACKEND_ENV_VARS = {
    "GO_API_TOKEN": "backend must receive GO_API_TOKEN for analytics callback auth",
    "ANALYTICS_ADMIN_TOKEN": "backend must receive ANALYTICS_ADMIN_TOKEN for analytics admin proxy",
    "ANALYTICS_HTTP_TIMEOUT": "backend must receive ANALYTICS_HTTP_TIMEOUT for analytics upstream timeout control",
    "REQUEST_BODY_LIMIT_BYTES": "backend must receive REQUEST_BODY_LIMIT_BYTES for request size enforcement",
    "HTTP_READ_TIMEOUT": "backend must receive HTTP_READ_TIMEOUT for server read timeout control",
    "HTTP_READ_HEADER_TIMEOUT": "backend must receive HTTP_READ_HEADER_TIMEOUT for header timeout control",
    "HTTP_WRITE_TIMEOUT": "backend must receive HTTP_WRITE_TIMEOUT for write timeout control",
    "HTTP_IDLE_TIMEOUT": "backend must receive HTTP_IDLE_TIMEOUT for idle timeout control",
    "HTTP_SHUTDOWN_TIMEOUT": "backend must receive HTTP_SHUTDOWN_TIMEOUT for graceful shutdown control",
}
REQUIRED_ANALYTICS_ENV_VARS = {
    "GO_API_TOKEN": "analytics must receive GO_API_TOKEN for callback auth",
    "ANALYTICS_ADMIN_TOKEN": "analytics must receive ANALYTICS_ADMIN_TOKEN for admin endpoints",
    "HTTP_RETRY_ATTEMPTS": "analytics must receive HTTP_RETRY_ATTEMPTS for callback retry control",
    "HTTP_TIMEOUT_SECONDS": "analytics must receive HTTP_TIMEOUT_SECONDS for callback timeout control",
    "HTTP_RETRY_BACKOFF_SECONDS": "analytics must receive HTTP_RETRY_BACKOFF_SECONDS for callback retry backoff control",
    "HTTP_RETRY_BACKOFF_MAX_SECONDS": "analytics must receive HTTP_RETRY_BACKOFF_MAX_SECONDS for callback retry backoff cap",
}
REQUIRED_ENV_EXAMPLE_VARS = sorted(set(REQUIRED_BACKEND_ENV_VARS) | set(REQUIRED_ANALYTICS_ENV_VARS))


def main() -> int:
    compose = load_compose()
    services = require_mapping(compose.get("services"), "services")
    failures: list[str] = []

    missing = REQUIRED_SERVICES - set(services)
    if missing:
        failures.append(f"missing services: {', '.join(sorted(missing))}")

    for name in sorted(REQUIRED_SERVICES & set(services)):
        service = require_mapping(services[name], f"services.{name}")
        if "healthcheck" not in service:
            failures.append(f"services.{name} must define a healthcheck")
        failures.extend(check_compose_image_reference(name, service))

    backend_env = service_environment(services, "backend")
    analytics_env = service_environment(services, "analytics")
    failures.extend(check_required_env_vars(backend_env, REQUIRED_BACKEND_ENV_VARS))
    failures.extend(check_required_env_vars(analytics_env, REQUIRED_ANALYTICS_ENV_VARS))

    frontend = require_mapping(services.get("frontend"), "services.frontend")
    frontend_depends = frontend.get("depends_on")
    if not service_depends_on_healthy(frontend_depends, "backend"):
        failures.append("frontend must depend on a healthy backend")

    analytics = require_mapping(services.get("analytics"), "services.analytics")
    analytics_depends = analytics.get("depends_on")
    if not service_depends_on_healthy(analytics_depends, "backend"):
        failures.append("analytics must depend on a healthy backend")

    backend = require_mapping(services.get("backend"), "services.backend")
    backend_depends = backend.get("depends_on")
    if not service_depends_on_healthy(backend_depends, "postgres"):
        failures.append("backend must depend on a healthy postgres")

    for service_name, dockerfile in DOCKERFILES.items():
        user = dockerfile_runtime_user(dockerfile)
        if user is None:
            failures.append(f"{dockerfile.relative_to(ROOT)} must define USER in the final runtime stage")
        elif user in {"", "0", "root"}:
            failures.append(f"{dockerfile.relative_to(ROOT)} final runtime USER must not be root")
        failures.extend(check_dockerfile_base_images(dockerfile))

    failures.extend(check_frontend_nginx(FRONTEND_NGINX_CONF))
    failures.extend(check_root_dockerignore(ROOT_DOCKERIGNORE))
    failures.extend(check_env_example(ENV_EXAMPLE_FILE, REQUIRED_ENV_EXAMPLE_VARS))

    if failures:
        for failure in failures:
            print(f"[FAIL] {failure}")
        return 1

    print("compose config baseline checks passed")
    return 0


def load_compose() -> dict[str, Any]:
    completed = subprocess.run(
        ["docker", "compose", "-f", str(COMPOSE_FILE), "config", "--format", "json"],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
        encoding="utf-8",
    )
    if completed.returncode != 0:
        raise RuntimeError(f"docker compose config failed: {completed.stderr.strip() or completed.stdout.strip()}")
    try:
        data = json.loads(completed.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"docker compose config did not return valid JSON: {exc}") from exc
    return require_mapping(data, "docker compose config")


def require_mapping(value: Any, name: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise RuntimeError(f"{name} must be a mapping")
    return value


def service_environment(services: dict[str, Any], service_name: str) -> dict[str, Any]:
    service = require_mapping(services.get(service_name), f"services.{service_name}")
    env = service.get("environment", {})
    if isinstance(env, dict):
        return env
    if isinstance(env, list):
        result: dict[str, str] = {}
        for item in env:
            if not isinstance(item, str) or "=" not in item:
                continue
            key, value = item.split("=", 1)
            result[key] = value
        return result
    raise RuntimeError(f"services.{service_name}.environment must be a mapping or list")


def service_depends_on_healthy(depends_on: Any, dependency: str) -> bool:
    if isinstance(depends_on, dict):
        value = depends_on.get(dependency)
        if isinstance(value, dict):
            return value.get("condition") == "service_healthy"
        return value is not None
    if isinstance(depends_on, list):
        return dependency in depends_on
    return False


def dockerfile_runtime_user(path: Path) -> str | None:
    instructions = dockerfile_instructions(path)
    user: str | None = None
    for instruction, argument in instructions:
        if instruction == "FROM":
            user = None
        elif instruction == "USER":
            user = argument.split()[0].strip()
    return user


def check_compose_image_reference(service_name: str, service: dict[str, Any]) -> list[str]:
    image = str(service.get("image") or "").strip()
    if not image:
        return []
    return check_image_reference(image, f"services.{service_name}.image")


def check_dockerfile_base_images(path: Path) -> list[str]:
    failures: list[str] = []
    for instruction, argument in dockerfile_instructions(path):
        if instruction != "FROM":
            continue
        image = argument.split()[0].strip()
        failures.extend(check_image_reference(image, f"{path.relative_to(ROOT)} FROM {image}"))
    return failures


def check_image_reference(reference: str, label: str) -> list[str]:
    if reference == "scratch":
        return []
    if not reference:
        return [f"{label} must not be empty"]
    tag = image_tag(reference)
    if tag is None and "@" not in reference:
        return [f"{label} must pin an explicit image tag or digest"]
    if tag == "latest":
        return [f"{label} must not use the floating latest tag"]
    return []


def image_tag(reference: str) -> str | None:
    image = reference.split("@", 1)[0]
    last_slash = image.rfind("/")
    last_colon = image.rfind(":")
    if last_colon > last_slash:
        return image[last_colon + 1 :]
    return None


def dockerfile_instructions(path: Path) -> list[tuple[str, str]]:
    lines = path.read_text(encoding="utf-8").splitlines()
    instructions: list[tuple[str, str]] = []
    current = ""
    for raw_line in lines:
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if current:
            current += line
        else:
            current = line
        if current.endswith("\\"):
            current = current[:-1].rstrip() + " "
            continue
        parts = current.split(maxsplit=1)
        if parts:
            instruction = parts[0].upper()
            argument = parts[1].strip() if len(parts) > 1 else ""
            instructions.append((instruction, argument))
        current = ""
    if current:
        parts = current.split(maxsplit=1)
        instruction = parts[0].upper()
        argument = parts[1].strip() if len(parts) > 1 else ""
        instructions.append((instruction, argument))
    return instructions


def check_frontend_nginx(path: Path) -> list[str]:
    text = path.read_text(encoding="utf-8")
    failures: list[str] = []
    for header, expected in REQUIRED_FRONTEND_HEADERS.items():
        if not has_nginx_header(text, header, expected):
            failures.append(f"{path.relative_to(ROOT)} must set {header} including {expected!r}")
    for required in [
        "proxy_set_header X-Forwarded-Proto $scheme;",
        "proxy_set_header Upgrade $http_upgrade;",
        'proxy_set_header Connection "upgrade";',
    ]:
        if required not in text:
            failures.append(f"{path.relative_to(ROOT)} must include {required}")
    return failures


def check_root_dockerignore(path: Path) -> list[str]:
    if not path.exists():
        return [".dockerignore is required to keep secrets, VCS metadata, and local build output out of image contexts"]
    patterns = dockerignore_patterns(path)
    failures: list[str] = []
    for pattern in sorted(REQUIRED_DOCKERIGNORE_PATTERNS):
        if pattern not in patterns:
            failures.append(f".dockerignore must include {pattern}")
    return failures


def dockerignore_patterns(path: Path) -> set[str]:
    patterns: set[str] = set()
    for line in path.read_text(encoding="utf-8").splitlines():
        value = line.strip()
        if not value or value.startswith("#"):
            continue
        patterns.add(value)
    return patterns


def has_nginx_header(text: str, header: str, expected: str) -> bool:
    needle = f"add_header {header}"
    return any(needle in line and expected in line and "always" in line for line in text.splitlines())


def check_env_example(path: Path, required: list[str]) -> list[str]:
    if not path.exists():
        return [".env.example is required"]
    declared = env_keys(path)
    return [f".env.example must declare {key}" for key in required if key not in declared]


def env_keys(path: Path) -> set[str]:
    keys: set[str] = set()
    for line in path.read_text(encoding="utf-8").splitlines():
        value = line.strip()
        if not value or value.startswith("#") or "=" not in value:
            continue
        key, _ = value.split("=", 1)
        key = key.strip()
        if key:
            keys.add(key)
    return keys


def check_required_env_vars(environment: dict[str, Any], required: dict[str, str]) -> list[str]:
    failures: list[str] = []
    for key, message in required.items():
        if key not in environment:
            failures.append(message)
    return failures


if __name__ == "__main__":
    try:
        sys.exit(main())
    except RuntimeError as exc:
        print(f"[FAIL] {exc}")
        sys.exit(1)
