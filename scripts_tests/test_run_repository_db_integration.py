import importlib.util
import sys
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "run_repository_db_integration.py"
SPEC = importlib.util.spec_from_file_location("run_repository_db_integration_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class RunRepositoryDbIntegrationTest(unittest.TestCase):
    def test_build_dsn_uses_expected_local_values(self) -> None:
        dsn = MODULE.build_dsn(15432, "shipsystem_repo_test", "shipsystem", "shipsystem")

        self.assertIn("host=127.0.0.1", dsn)
        self.assertIn("port=15432", dsn)
        self.assertIn("dbname=shipsystem_repo_test", dsn)
        self.assertIn("user=shipsystem", dsn)

    def test_build_compose_yaml_contains_postgis_service_and_bindings(self) -> None:
        yaml_text = MODULE.build_compose_yaml(
            host_port=15432,
            db_name="shipsystem_repo_test",
            db_user="shipsystem",
            db_password="shipsystem",
        )

        self.assertIn("postgres-test", yaml_text)
        self.assertIn("postgis/postgis:17-3.5", yaml_text)
        self.assertIn('"15432:5432"', yaml_text)
        self.assertIn("./backend/migrations:/docker-entrypoint-initdb.d:ro", yaml_text)
        self.assertIn("repo_test_postgres_data", yaml_text)

    def test_choose_port_prefers_requested_port_when_free(self) -> None:
        with mock.patch.object(MODULE, "port_is_free", side_effect=lambda port: True):
            self.assertEqual(15432, MODULE.choose_port(15432))

    def test_choose_port_bumps_to_next_free_port(self) -> None:
        busy = {15432, 15433}
        with mock.patch.object(MODULE, "port_is_free", side_effect=lambda port: port not in busy):
            self.assertEqual(15434, MODULE.choose_port(15432))

    def test_compose_base_command_uses_project_and_file(self) -> None:
        compose_file = Path(r"C:\temp\repo-test.yml")
        with mock.patch.object(MODULE, "tool_path", return_value="docker"):
            command = MODULE.compose_base_command("shipsystem-repo-test", compose_file)

        self.assertEqual(["docker", "compose", "-p", "shipsystem-repo-test", "-f", str(compose_file)], command)

    def test_wait_for_postgres_retries_until_ready(self) -> None:
        responses = [
            MODULE.subprocess.CompletedProcess(args=["docker"], returncode=1, stdout="", stderr="not ready"),
            MODULE.subprocess.CompletedProcess(args=["docker"], returncode=0, stdout="ready", stderr=""),
        ]

        with mock.patch.object(MODULE.subprocess, "run", side_effect=responses) as run_mock:
            with mock.patch.object(MODULE.time, "sleep") as sleep_mock:
                MODULE.wait_for_postgres("proj", Path("compose.yml"), "shipsystem", "shipsystem_repo_test", timeout_seconds=5)

        self.assertEqual(2, run_mock.call_count)
        sleep_mock.assert_called_once_with(1)

    def test_wait_for_postgres_fails_after_timeout(self) -> None:
        with mock.patch.object(
            MODULE.subprocess,
            "run",
            return_value=MODULE.subprocess.CompletedProcess(args=["docker"], returncode=1, stdout="", stderr="not ready"),
        ):
            with mock.patch.object(MODULE.time, "monotonic", side_effect=[0, 10]):
                with self.assertRaises(SystemExit) as ctx:
                    MODULE.wait_for_postgres("proj", Path("compose.yml"), "shipsystem", "shipsystem_repo_test", timeout_seconds=5)

        self.assertIn("did not become ready", str(ctx.exception))

    def test_parse_args_accepts_custom_values(self) -> None:
        with mock.patch.object(
            sys,
            "argv",
            [
                "run_repository_db_integration.py",
                "--project",
                "repo-it",
                "--port",
                "25432",
                "--db-name",
                "testdb",
                "--db-user",
                "tester",
                "--db-password",
                "secret",
                "--keep-database",
            ],
        ):
            args = MODULE.parse_args()

        self.assertEqual("repo-it", args.project)
        self.assertEqual(25432, args.port)
        self.assertEqual("testdb", args.db_name)
        self.assertEqual("tester", args.db_user)
        self.assertEqual("secret", args.db_password)
        self.assertTrue(args.keep_database)


if __name__ == "__main__":
    unittest.main()