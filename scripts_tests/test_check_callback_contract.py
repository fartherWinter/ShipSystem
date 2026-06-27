import importlib.util
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "check_callback_contract.py"
SPEC = importlib.util.spec_from_file_location("check_callback_contract_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class CallbackContractScriptTest(unittest.TestCase):
    def test_nested_value_handles_dicts_lists_and_missing_path(self) -> None:
        value = {"state": {"units": [{"unitId": "u-1"}]}}

        self.assertEqual({"unitId": "u-1"}, MODULE.nested_value(value, ("state", "units", 0)))
        self.assertIsNone(MODULE.nested_value(value, ("state", "units", 1)))
        self.assertIsNone(MODULE.nested_value(value, ("state", "events", 0)))

    def test_compare_fields_reports_missing_and_extra(self) -> None:
        failures = MODULE.compare_fields("sample", {"a", "b"}, {"b", "c"})

        self.assertEqual(
            ["sample missing fields: a", "sample has unsupported fields: c"],
            failures,
        )

    def test_go_json_tags_reads_current_payload_structs(self) -> None:
        structs = MODULE.go_json_tags()

        self.assertEqual(MODULE.GO_PAYLOAD_STRUCTS["RadarReportPayload"], structs["RadarReportPayload"])
        self.assertIn("projectileId", structs["BattleProjectilePayload"])
        self.assertIn("occurredAt", structs["BattleEventPayload"])

    def test_battle_simulation_sample_matches_expected_shape(self) -> None:
        payload = MODULE.battle_simulation_sample()

        self.assertIsInstance(payload, dict)
        self.assertEqual("contract-session", payload["sessionId"])
        self.assertIn("state", payload)
        self.assertIsInstance(payload["targets"], list)
        self.assertIn("units", payload["state"])
        self.assertIn("projectiles", payload["state"])
        self.assertIn("events", payload["state"])


if __name__ == "__main__":
    unittest.main()
