import importlib.util
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "check_event_contract.py"
SPEC = importlib.util.spec_from_file_location("check_event_contract_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class EventContractScriptTest(unittest.TestCase):
    def test_smoke_required_events_cover_runtime_battle_pushes(self) -> None:
        self.assertEqual(
            {"radar_scan_updated", "projectile_updated", "battle_event_created", "battle_state_updated"},
            MODULE.SMOKE_REQUIRED_EVENTS,
        )

    def test_compare_event_set_reports_missing_and_extra(self) -> None:
        failures = MODULE.compare_event_set("test", {"a", "b"}, {"b", "c"})

        self.assertEqual(
            ["test missing: a", "test has undocumented events: c"],
            failures,
        )

    def test_backend_event_types_reads_current_sources(self) -> None:
        events = MODULE.backend_event_types()

        self.assertEqual(MODULE.EXPECTED_WS_EVENTS, events)

    def test_frontend_ws_union_types_reads_current_types(self) -> None:
        events = MODULE.frontend_ws_union_types()

        self.assertEqual(MODULE.EXPECTED_WS_EVENTS, events)

    def test_frontend_handled_types_reads_current_app(self) -> None:
        events = MODULE.frontend_handled_types()

        self.assertEqual(MODULE.FRONTEND_HANDLED_EVENTS, events)


if __name__ == "__main__":
    unittest.main()
