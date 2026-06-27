import importlib.util
import os
import sys
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "runtime_precheck.py"
SPEC = importlib.util.spec_from_file_location("runtime_precheck_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def completed(stdout: str = "", stderr: str = "", returncode: int = 0):
    return MODULE.subprocess.CompletedProcess(args=["test"], returncode=returncode, stdout=stdout, stderr=stderr)


class RuntimePrecheckScriptTest(unittest.TestCase):
    def test_runtime_ports_uses_defaults_when_env_is_unset(self) -> None:
        with mock.patch.dict(
            os.environ,
            {
                "SHIPSYSTEM_BACKEND_URL": "",
                "SHIPSYSTEM_ANALYTICS_URL": "",
                "SHIPSYSTEM_FRONTEND_URL": "",
            },
            clear=False,
        ):
            with mock.patch.object(MODULE, "configured_runtime_url", side_effect=lambda name: MODULE.DEFAULT_RUNTIME_URLS[name]):
                ports = MODULE.runtime_ports()

        self.assertEqual((3000, 8080, 8090), ports)

    def test_runtime_ports_uses_env_overrides_instead_of_default_ports(self) -> None:
        with mock.patch.dict(
            os.environ,
            {
                "SHIPSYSTEM_BACKEND_URL": "http://127.0.0.1:18080",
                "SHIPSYSTEM_ANALYTICS_URL": "http://127.0.0.1:18090",
                "SHIPSYSTEM_FRONTEND_URL": "http://127.0.0.1:13000",
            },
            clear=False,
        ):
            ports = MODULE.runtime_ports()

        self.assertEqual((13000, 18080, 18090), ports)

    def test_check_smoke_urls_reports_effective_runtime_urls(self) -> None:
        with mock.patch.dict(
            os.environ,
            {
                "SHIPSYSTEM_BACKEND_URL": "http://127.0.0.1:18080",
                "SHIPSYSTEM_ANALYTICS_URL": "http://127.0.0.1:18090",
                "SHIPSYSTEM_FRONTEND_URL": "http://127.0.0.1:13000",
            },
            clear=False,
        ):
            result = MODULE.check_smoke_urls()

        self.assertTrue(result.ok)
        self.assertEqual(
            "backend=http://127.0.0.1:18080, analytics=http://127.0.0.1:18090, frontend=http://127.0.0.1:13000",
            result.detail,
        )

    def test_compose_published_ports_reads_rendered_compose_json(self) -> None:
        compose_json = """
        {
          "services": {
            "postgres": {"ports": [{"published": "5432"}]},
            "backend": {"ports": [{"published": "8080"}]},
            "frontend": {"ports": [{"published": "3000"}]},
            "analytics": {"ports": [{"published": "8090"}]}
          }
        }
        """
        with mock.patch.object(MODULE, "run", return_value=completed(stdout=compose_json)):
            ports = MODULE.compose_published_ports()

        self.assertEqual((3000, 5432, 8080, 8090), ports)

    def test_checked_ports_merges_runtime_targets_and_compose_published_ports(self) -> None:
        with mock.patch.object(MODULE, "runtime_ports", return_value=(4173, 8080, 8090)):
            with mock.patch.object(MODULE, "compose_published_ports", return_value=(3000, 5432, 8080, 8090)):
                ports = MODULE.checked_ports()

        self.assertEqual((3000, 4173, 5432, 8080, 8090), ports)

    def test_port_from_url_handles_invalid_values(self) -> None:
        self.assertEqual(0, MODULE.port_from_url(""))
        self.assertEqual(0, MODULE.port_from_url("not a url"))
        self.assertEqual(3000, MODULE.port_from_url("http://localhost:3000/path"))

    def test_windows_listening_pid_parses_netstat_output(self) -> None:
        with mock.patch.object(MODULE, "run", return_value=completed(stdout="  TCP    0.0.0.0:3000   0.0.0.0:0   LISTENING   5052\r\n")):
            pid = MODULE.windows_listening_pid(3000)
        self.assertEqual(5052, pid)

    def test_describe_busy_port_expands_pid_and_process_name_on_windows(self) -> None:
        with mock.patch.object(MODULE.os, "name", "nt"):
            with mock.patch.object(MODULE, "windows_listening_pid", return_value=5052):
                with mock.patch.object(MODULE, "windows_process_name", return_value="java"):
                    self.assertEqual("3000(pid=5052, process=java)", MODULE.describe_busy_port(3000))

    def test_describe_busy_port_falls_back_when_pid_missing(self) -> None:
        with mock.patch.object(MODULE.os, "name", "nt"):
            with mock.patch.object(MODULE, "windows_listening_pid", return_value=0):
                self.assertEqual("3000", MODULE.describe_busy_port(3000))

    def test_check_docker_service_reports_windows_status(self) -> None:
        with mock.patch.object(MODULE.os, "name", "nt"):
            with mock.patch.object(MODULE, "run", return_value=completed(stdout="Running\n")):
                result = MODULE.check_docker_service()
        self.assertTrue(result.ok)
        self.assertEqual("Running", result.detail)

    def test_check_docker_service_explains_stopped_status(self) -> None:
        with mock.patch.object(MODULE.os, "name", "nt"):
            with mock.patch.object(MODULE, "run", return_value=completed(stdout="Stopped\n")):
                result = MODULE.check_docker_service()
        self.assertFalse(result.ok)
        self.assertIn("installed but not running", result.detail)

    def test_check_docker_service_is_not_applicable_off_windows(self) -> None:
        with mock.patch.object(MODULE.os, "name", "posix"):
            result = MODULE.check_docker_service()
        self.assertTrue(result.ok)
        self.assertEqual("not applicable", result.detail)

    def test_check_docker_daemon_explains_missing_linux_engine_pipe(self) -> None:
        with mock.patch.object(
            MODULE,
            "run",
            return_value=completed(
                stdout=(
                    "failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine; "
                    "check if the path is correct and if the daemon is running: open //./pipe/dockerDesktopLinuxEngine: "
                    "The system cannot find the file specified."
                ),
                returncode=1,
            ),
        ):
            result = MODULE.check_docker_daemon()
        self.assertFalse(result.ok)
        self.assertIn("pipe is missing", result.detail)

    def test_check_docker_daemon_explains_access_denied(self) -> None:
        with mock.patch.object(
            MODULE,
            "run",
            return_value=completed(
                stdout="error during connect: open //./pipe/docker_engine: Access is denied.",
                returncode=1,
            ),
        ):
            result = MODULE.check_docker_daemon()
        self.assertFalse(result.ok)
        self.assertIn("access denied to Docker daemon pipe", result.detail)

    def test_check_ports_lists_busy_ports_with_process_names(self) -> None:
        with mock.patch.object(MODULE, "checked_ports", return_value=(3000, 8080, 5432)):
            with mock.patch.object(MODULE, "port_is_busy", side_effect=lambda port: port == 3000):
                with mock.patch.object(MODULE, "describe_busy_port", side_effect=lambda port: f"{port}(pid=1, process=test)"):
                    result = MODULE.check_ports()
        self.assertFalse(result.ok)
        self.assertEqual("busy: 3000(pid=1, process=test)", result.detail)

    def test_compose_override_hint_is_returned_for_busy_ports(self) -> None:
        hint = MODULE.compose_override_hint([MODULE.CheckResult("runtime ports", False, "busy: 3000(pid=1, process=java)")])

        self.assertIn("generate_compose_local_override.py", hint)
        self.assertIn("compose.smoke.override.yml", hint)

    def test_compose_override_hint_is_empty_without_port_conflicts(self) -> None:
        hint = MODULE.compose_override_hint([MODULE.CheckResult("docker daemon", False, "permission denied")])

        self.assertEqual("", hint)

    def test_runtime_failure_hints_report_service_and_pipe_actions(self) -> None:
        hints = MODULE.runtime_failure_hints(
            [
                MODULE.CheckResult("docker service", False, "Stopped (Docker Desktop Service is installed but not running)"),
                MODULE.CheckResult("docker daemon", False, "open //./pipe/dockerDesktopLinuxEngine: The system cannot find the file specified. (Docker Desktop Linux engine pipe is missing)"),
            ]
        )

        self.assertEqual(2, len(hints))
        self.assertIn("elevated shell", hints[0])
        self.assertIn("pipe is missing", hints[1].lower())

    def test_normalize_runtime_results_downgrades_service_failure_when_daemon_is_ready(self) -> None:
        normalized = MODULE.normalize_runtime_results(
            [
                MODULE.CheckResult("docker service", False, "Stopped (Docker Desktop Service is installed but not running)"),
                MODULE.CheckResult("docker daemon", True, "29.3.0"),
            ]
        )

        self.assertEqual(2, len(normalized))
        self.assertTrue(normalized[0].ok)
        self.assertIn("daemon is reachable", normalized[0].detail)

    def test_short_output_prefers_stdout_and_truncates(self) -> None:
        text = "x" * 300
        output = MODULE.short_output(completed(stdout=text))
        self.assertEqual(263, len(output))
        self.assertTrue(output.endswith("..."))


if __name__ == "__main__":
    unittest.main()
