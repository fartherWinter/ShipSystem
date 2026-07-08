import importlib.util
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "check_rbac_matrix.py"
SPEC = importlib.util.spec_from_file_location("check_rbac_matrix_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class RBACMatrixScriptTest(unittest.TestCase):
    def test_parse_scalar_handles_inline_role_lists(self) -> None:
        self.assertEqual(["admin", "dispatcher"], MODULE.parse_scalar("[admin, dispatcher]"))
        self.assertEqual([], MODULE.parse_scalar("[]"))
        self.assertEqual("GET", MODULE.parse_scalar("GET"))

    def test_gin_to_openapi_path_rewrites_params(self) -> None:
        self.assertEqual("/ships/{id}/locations", MODULE.gin_to_openapi_path("/ships/:id/locations"))
        self.assertEqual(
            "/battle/sessions/{sessionId}/timeline",
            MODULE.gin_to_openapi_path("/battle/sessions/:sessionId/timeline"),
        )

    def test_parse_handler_rbac_reads_current_routes(self) -> None:
        routes = MODULE.parse_handler_rbac()

        self.assertEqual({"admin"}, routes[("delete", "/ships/{id}")])
        self.assertEqual({"admin", "dispatcher", "analytics_service"}, routes[("post", "/radar/reports")])
        self.assertEqual({"super_admin"}, routes[("get", "/rbac/users")])

    def test_parse_frontend_rbac_reads_current_pages(self) -> None:
        routes = MODULE.parse_frontend_rbac()

        self.assertEqual({"super_admin", "admin", "dispatcher", "viewer"}, routes["/dashboard"])
        self.assertEqual({"super_admin", "admin", "dispatcher"}, routes["/dispatch"])
        self.assertEqual({"super_admin"}, routes["/rbac"])

    def test_parse_matrix_reads_current_role_matrix(self) -> None:
        matrix = MODULE.parse_matrix()

        self.assertEqual(["super_admin", "admin", "dispatcher", "viewer", "analytics_service"], matrix["roles"])
        rest_routes = MODULE.matrix_section(matrix, "rest")
        frontend_routes = MODULE.matrix_section(matrix, "frontend")
        self.assertTrue(any(item["path"] == "/radar/reports" and item["roles"] == ["admin", "dispatcher", "analytics_service"] for item in rest_routes))
        self.assertTrue(any(item["path"] == "/rbac" and item["roles"] == ["super_admin"] for item in frontend_routes))


if __name__ == "__main__":
    unittest.main()
