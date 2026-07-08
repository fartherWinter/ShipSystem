import importlib.util
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "check_frontend_api_contract.py"
SPEC = importlib.util.spec_from_file_location("check_frontend_api_contract_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class FrontendAPIContractScriptTest(unittest.TestCase):
    def test_find_request_call_starts_skips_helper_function_definition(self) -> None:
        text = """
async function request<T>(path: string, options: RequestInit = {}) {
  return fetch(path, options);
}

const api = {
  ships: () => request('/ships'),
  deleteShip: (id: number) => request(`/ships/${id}`, { method: 'DELETE' }),
};
"""

        starts = MODULE.find_request_call_starts(text)

        self.assertEqual(2, len(starts))

    def test_extract_first_argument_supports_template_literals(self) -> None:
        text = "request(`/battle/sessions/${encodeURIComponent(sessionId)}/snapshots${query ? `?${query}` : ''}`, { method: 'GET' })"

        path, end_index = MODULE.extract_first_argument(text, text.index("(") + 1)

        self.assertEqual("/battle/sessions/${encodeURIComponent(sessionId)}/snapshots${query ? `?${query}` : ''}", path)
        self.assertGreater(end_index, 0)

    def test_normalize_frontend_path_rewrites_route_params_and_query_suffix(self) -> None:
        self.assertEqual(
            "/battle/sessions/{sessionId}/snapshots",
            MODULE.normalize_frontend_path("/battle/sessions/${encodeURIComponent(sessionId)}/snapshots${query ? `?${query}` : ''}"),
        )
        self.assertEqual("/ships/{id}/locations", MODULE.normalize_frontend_path("/ships/${shipId}/locations"))

    def test_parse_openapi_routes_reads_current_contract(self) -> None:
        routes = MODULE.parse_openapi_routes()

        self.assertIn(("post", "/auth/login"), routes)
        self.assertIn(("put", "/dispatch-events/{id}/status"), routes)
        self.assertIn(("post", "/analytics/simulate/battle/stop"), routes)

    def test_parse_frontend_routes_reads_current_client(self) -> None:
        routes = MODULE.parse_frontend_routes()

        self.assertIn(("get", "/ships"), routes)
        self.assertIn(("delete", "/ships/{id}"), routes)
        self.assertIn(("get", "/battle/sessions/{sessionId}/snapshots"), routes)
        self.assertIn(("post", "/analytics/simulate/battle/start"), routes)


if __name__ == "__main__":
    unittest.main()
