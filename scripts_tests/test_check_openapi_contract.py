import importlib.util
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "check_openapi_contract.py"
SPEC = importlib.util.spec_from_file_location("check_openapi_contract_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class OpenAPIContractScriptTest(unittest.TestCase):
    def test_parse_yaml_like_handles_inline_objects_and_quoted_status_keys(self) -> None:
        data = MODULE.parse_yaml_like(
            """
paths:
  /auth/login:
    post:
      summary: Login
      responses:
        "200": { description: Login succeeded }
        "400": { $ref: "#/components/responses/BadRequest" }
"""
        )

        responses = data["paths"]["/auth/login"]["post"]["responses"]
        self.assertEqual({"200", "400"}, set(responses))
        self.assertEqual("Login succeeded", responses["200"]["description"])
        self.assertEqual("#/components/responses/BadRequest", responses["400"]["$ref"])

    def test_parse_yaml_like_handles_multiline_scalars_and_nested_lists(self) -> None:
        data = MODULE.parse_yaml_like(
            """
info:
  description: |
    first line
    second line
paths:
  /ships:
    get:
      x-roles: [admin, dispatcher, viewer]
      parameters:
        - $ref: "#/components/parameters/Page"
        - $ref: "#/components/parameters/Size"
"""
        )

        self.assertEqual("first line\nsecond line", data["info"]["description"])
        operation = data["paths"]["/ships"]["get"]
        self.assertEqual(["admin", "dispatcher", "viewer"], operation["x-roles"])
        self.assertEqual(
            {"Page", "Size"},
            MODULE.parse_parameter_refs(operation),
        )

    def test_parse_openapi_operations_reads_current_openapi_doc(self) -> None:
        operations = MODULE.parse_openapi_operations()

        radar_report = operations[("post", "/radar/reports")]
        self.assertEqual(["admin", "dispatcher", "analytics_service"], radar_report["x-roles"])
        self.assertTrue(radar_report["requestBody"]["required"])
        self.assertIn("202", radar_report["responses"])

        logout = operations[("post", "/auth/logout")]
        self.assertEqual([], logout["x-roles"])
        self.assertIn("204", logout["responses"])

    def test_parse_matrix_roles_reads_current_role_set(self) -> None:
        roles = MODULE.parse_matrix_roles()
        self.assertEqual(
            {"super_admin", "admin", "dispatcher", "viewer", "analytics_service"},
            roles,
        )

    def test_split_inline_respects_nested_structures(self) -> None:
        parts = MODULE.split_inline('a, { nested: [1, 2, "x,y"] }, "z,w"')
        self.assertEqual(
            ["a", '{ nested: [1, 2, "x,y"] }', '"z,w"'],
            parts,
        )


if __name__ == "__main__":
    unittest.main()
