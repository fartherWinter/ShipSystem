import importlib.util
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "generate_compose_local_override.py"
SPEC = importlib.util.spec_from_file_location("generate_compose_local_override_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class GenerateComposeLocalOverrideTest(unittest.TestCase):
    def test_select_port_plan_keeps_defaults_when_free(self) -> None:
        plan = MODULE.select_port_plan(is_free=lambda port: True)

        self.assertEqual(
            MODULE.PortPlan(postgres=5432, backend=8080, analytics=8090, frontend=3000),
            plan,
        )

    def test_select_port_plan_bumps_only_busy_ports(self) -> None:
        busy = {5432, 3000}
        plan = MODULE.select_port_plan(is_free=lambda port: port not in busy)

        self.assertEqual(5433, plan.postgres)
        self.assertEqual(8080, plan.backend)
        self.assertEqual(8090, plan.analytics)
        self.assertEqual(3001, plan.frontend)

    def test_build_override_yaml_contains_selected_ports(self) -> None:
        yaml_text = MODULE.build_override_yaml(
            MODULE.PortPlan(postgres=15432, backend=18080, analytics=18090, frontend=13000)
        )

        self.assertIn('"15432:5432"', yaml_text)
        self.assertIn('"18080:8080"', yaml_text)
        self.assertIn('"18090:8090"', yaml_text)
        self.assertIn('"13000:8080"', yaml_text)


if __name__ == "__main__":
    unittest.main()
