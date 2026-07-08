import unittest

from app.battle_sim import BattleSimulation, distance_km


class BattleSimulationTest(unittest.TestCase):
    def test_distance_uses_kilometers(self) -> None:
        self.assertAlmostEqual(distance_km(121.49, 31.23, 121.50, 31.23), 0.95, delta=0.15)

    def test_step_reports_units_and_radar_targets(self) -> None:
        sim = BattleSimulation("test-session", seed=1)
        payload = sim.step(1.2)

        self.assertEqual(payload["sessionId"], "test-session")
        self.assertGreaterEqual(len(payload["targets"]), 1)
        state = payload["state"]
        self.assertEqual(state["sessionId"], "test-session")
        self.assertGreaterEqual(len(state["units"]), 4)

    def test_weapons_create_projectiles_when_in_range(self) -> None:
        sim = BattleSimulation("close-session", scenario_code="close-quarter-barrage", seed=2)
        for unit in sim.units:
            if unit.side == "red":
                unit.longitude = sim.origin_longitude + 0.01
                unit.latitude = sim.origin_latitude
            else:
                unit.longitude = sim.origin_longitude - 0.01
                unit.latitude = sim.origin_latitude

        payload = sim.step(1.2)

        projectiles = payload["state"]["projectiles"]
        events = payload["state"]["events"]
        self.assertTrue(projectiles)
        self.assertTrue(any(event["type"] == "WEAPON_FIRED" for event in events))

    def test_projectile_hit_reduces_hp(self) -> None:
        sim = BattleSimulation("hit-session", seed=3)
        sim.rng.random = lambda: 0.0
        target = sim.units[2]
        for unit in sim.units:
            unit.cooldown_seconds = 0.1
            unit.longitude = sim.origin_longitude
            unit.latitude = sim.origin_latitude
        starting_hp = target.hp

        for _ in range(6):
            sim.step(1.2)

        self.assertLess(target.hp, starting_hp)


if __name__ == "__main__":
    unittest.main()
