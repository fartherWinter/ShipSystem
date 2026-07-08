import importlib.util
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "generate_frontend_api_types.py"
SPEC = importlib.util.spec_from_file_location("generate_frontend_api_types_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class GenerateFrontendAPITypesScriptTest(unittest.TestCase):
    def test_type_for_schema_handles_nullable_allof(self) -> None:
        rendered = MODULE.type_for_schema(
            {
                "allOf": [{"$ref": "#/components/schemas/ErrorResponse"}],
                "nullable": True,
            }
        )

        self.assertEqual("ErrorResponse | null", rendered)

    def test_type_for_schema_handles_additional_properties(self) -> None:
        rendered = MODULE.type_for_schema(
            {
                "type": "object",
                "properties": {"status": {"type": "string"}},
                "additionalProperties": True,
                "required": ["status"],
            }
        )

        self.assertIn("status: string;", rendered)
        self.assertIn("[key: string]: unknown;", rendered)

    def test_render_types_reads_current_contract(self) -> None:
        output = MODULE.render_types(MODULE.parse_openapi_schemas())

        self.assertIn("export type LoginResponse =", output)
        self.assertIn("export type AnalyticsProxyResponse = {", output)
        self.assertIn("upstreamStatus: number;", output)
        self.assertIn("parentId?: number | null;", output)
        self.assertIn("alarm?: Alarm | null;", output)


if __name__ == "__main__":
    unittest.main()
