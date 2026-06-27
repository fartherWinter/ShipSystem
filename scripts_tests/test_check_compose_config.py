import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "check_compose_config.py"
SPEC = importlib.util.spec_from_file_location("check_compose_config_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def completed(stdout: str = "", stderr: str = "", returncode: int = 0):
    return MODULE.subprocess.CompletedProcess(args=["docker", "compose"], returncode=returncode, stdout=stdout, stderr=stderr)


class ComposeConfigScriptTest(unittest.TestCase):
    def test_service_environment_supports_mapping_and_list(self) -> None:
        services = {
            "backend": {
                "environment": {
                    "GO_API_TOKEN": "a",
                    "ANALYTICS_HTTP_TIMEOUT": "10s",
                    "REQUEST_BODY_LIMIT_BYTES": "1048576",
                }
            },
            "analytics": {
                "environment": [
                    "GO_API_TOKEN=b",
                    "ANALYTICS_ADMIN_TOKEN=c",
                    "HTTP_RETRY_ATTEMPTS=3",
                    "HTTP_TIMEOUT_SECONDS=10",
                    "HTTP_RETRY_BACKOFF_SECONDS=0.2",
                    "HTTP_RETRY_BACKOFF_MAX_SECONDS=2",
                ]
            },
        }

        self.assertEqual(
            {
                "GO_API_TOKEN": "a",
                "ANALYTICS_HTTP_TIMEOUT": "10s",
                "REQUEST_BODY_LIMIT_BYTES": "1048576",
            },
            MODULE.service_environment(services, "backend"),
        )
        self.assertEqual(
            {
                "GO_API_TOKEN": "b",
                "ANALYTICS_ADMIN_TOKEN": "c",
                "HTTP_RETRY_ATTEMPTS": "3",
                "HTTP_TIMEOUT_SECONDS": "10",
                "HTTP_RETRY_BACKOFF_SECONDS": "0.2",
                "HTTP_RETRY_BACKOFF_MAX_SECONDS": "2",
            },
            MODULE.service_environment(services, "analytics"),
        )

    def test_check_required_env_vars_reports_missing_keys(self) -> None:
        failures = MODULE.check_required_env_vars(
            {"GO_API_TOKEN": "a"},
            {
                "GO_API_TOKEN": "missing go token",
                "REQUEST_BODY_LIMIT_BYTES": "missing request body limit",
            },
        )
        self.assertEqual(["missing request body limit"], failures)

    def test_env_keys_and_check_env_example(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            env_file = Path(tmpdir) / ".env.example"
            env_file.write_text(
                "\n".join(
                    [
                        "# comment",
                        "GO_API_TOKEN=",
                        "ANALYTICS_HTTP_TIMEOUT=10s",
                        "HTTP_TIMEOUT_SECONDS=10",
                    ]
                ),
                encoding="utf-8",
            )

            keys = MODULE.env_keys(env_file)
            failures = MODULE.check_env_example(
                env_file,
                ["GO_API_TOKEN", "ANALYTICS_HTTP_TIMEOUT", "HTTP_RETRY_ATTEMPTS"],
            )

        self.assertEqual(
            {"GO_API_TOKEN", "ANALYTICS_HTTP_TIMEOUT", "HTTP_TIMEOUT_SECONDS"},
            keys,
        )
        self.assertEqual([".env.example must declare HTTP_RETRY_ATTEMPTS"], failures)

    def test_service_depends_on_healthy_handles_dict_and_list(self) -> None:
        self.assertTrue(MODULE.service_depends_on_healthy({"backend": {"condition": "service_healthy"}}, "backend"))
        self.assertTrue(MODULE.service_depends_on_healthy(["backend", "postgres"], "backend"))
        self.assertFalse(MODULE.service_depends_on_healthy({"backend": {"condition": "service_started"}}, "backend"))
        self.assertFalse(MODULE.service_depends_on_healthy(None, "backend"))

    def test_image_reference_requires_pinned_non_latest_tag(self) -> None:
        self.assertEqual("3.22", MODULE.image_tag("alpine:3.22"))
        self.assertIsNone(MODULE.image_tag("alpine"))
        self.assertEqual([], MODULE.check_image_reference("nginx:1.29-alpine", "frontend"))
        self.assertEqual(
            ["frontend must not use the floating latest tag"],
            MODULE.check_image_reference("nginx:latest", "frontend"),
        )
        self.assertEqual(
            ["frontend must pin an explicit image tag or digest"],
            MODULE.check_image_reference("nginx", "frontend"),
        )

    def test_dockerfile_instructions_and_runtime_user_parse_multistage_files(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            dockerfile = Path(tmpdir) / "Dockerfile"
            dockerfile.write_text(
                "\n".join(
                    [
                        "FROM node:20-alpine AS builder",
                        "RUN echo build \\",
                        "  && echo done",
                        "FROM nginx:1.29-alpine",
                        "COPY nginx.conf /etc/nginx/nginx.conf",
                        "USER nginx",
                    ]
                ),
                encoding="utf-8",
            )

            instructions = MODULE.dockerfile_instructions(dockerfile)
            user = MODULE.dockerfile_runtime_user(dockerfile)

        self.assertEqual(
            [
                ("FROM", "node:20-alpine AS builder"),
                ("RUN", "echo build && echo done"),
                ("FROM", "nginx:1.29-alpine"),
                ("COPY", "nginx.conf /etc/nginx/nginx.conf"),
                ("USER", "nginx"),
            ],
            instructions,
        )
        self.assertEqual("nginx", user)

    def test_check_frontend_nginx_and_dockerignore_read_current_files(self) -> None:
        self.assertEqual([], MODULE.check_frontend_nginx(MODULE.FRONTEND_NGINX_CONF))
        self.assertEqual([], MODULE.check_root_dockerignore(MODULE.ROOT_DOCKERIGNORE))

    def test_load_compose_parses_json_and_rejects_errors(self) -> None:
        payload = {"services": {"backend": {"image": "shipsystem/backend:1.0.0"}}}
        with mock.patch.object(MODULE.subprocess, "run", return_value=completed(stdout=json.dumps(payload))):
            loaded = MODULE.load_compose()
        self.assertEqual(payload, loaded)

        with mock.patch.object(MODULE.subprocess, "run", return_value=completed(stderr="boom", returncode=1)):
            with self.assertRaises(RuntimeError) as ctx:
                MODULE.load_compose()
        self.assertIn("docker compose config failed", str(ctx.exception))

    def test_check_compose_image_reference_uses_service_image(self) -> None:
        failures = MODULE.check_compose_image_reference("frontend", {"image": "nginx:latest"})
        self.assertEqual(["services.frontend.image must not use the floating latest tag"], failures)


if __name__ == "__main__":
    unittest.main()
