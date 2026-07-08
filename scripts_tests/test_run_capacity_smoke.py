import importlib.util
import io
import sys
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "run_capacity_smoke.py"
SPEC = importlib.util.spec_from_file_location("run_capacity_smoke_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class RunCapacitySmokeTest(unittest.TestCase):
    def test_parse_track_counts_requires_values(self) -> None:
        self.assertEqual([5, 20, 100], MODULE.parse_track_counts("5,20,100"))
        with self.assertRaises(SystemExit):
            MODULE.parse_track_counts(" , ")

    def test_build_capacity_profile_counts_snapshots_targets_and_events(self) -> None:
        profile = MODULE.build_capacity_profile(tracks=20, ticks=6, action_every_ticks=2)

        self.assertEqual(6, profile.snapshots_per_run)
        self.assertEqual(20, profile.radar_targets_per_snapshot)
        self.assertEqual(120, profile.radar_targets_per_run)
        self.assertEqual(3, profile.battle_events_per_run)

    def test_estimate_daily_growth_uses_duration_to_scale(self) -> None:
        profile = MODULE.build_capacity_profile(tracks=5, ticks=6, action_every_ticks=3)
        estimate = MODULE.estimate_daily_growth(profile, duration_seconds=30)

        self.assertEqual(2880, estimate.runs_per_day)
        self.assertEqual(17280, estimate.snapshots_per_day)
        self.assertEqual(86400, estimate.radar_targets_per_day)
        self.assertEqual(5760, estimate.battle_events_per_day)

    def test_build_radar_report_payload_matches_current_contract_shape(self) -> None:
        payload = MODULE.build_radar_report_payload("battle-1", tracks=3, tick=2, action_every_ticks=2)

        self.assertEqual("battle-1", payload["sessionId"])
        self.assertEqual(3, len(payload["targets"]))
        self.assertEqual(3, len(payload["state"]["units"]))
        self.assertEqual(1, len(payload["state"]["projectiles"]))
        self.assertEqual(1, len(payload["state"]["events"]))
        self.assertEqual("WEAPON_FIRED", payload["state"]["events"][0]["type"])

    def test_build_radar_report_payload_skips_event_when_tick_is_not_triggered(self) -> None:
        payload = MODULE.build_radar_report_payload("battle-1", tracks=3, tick=1, action_every_ticks=2)

        self.assertEqual([], payload["state"]["events"])

    def test_fetch_items_requires_items_array(self) -> None:
        client = mock.Mock()
        client.request.return_value = (200, {"items": [{"tick": 1}]})

        items = MODULE.fetch_items(client, "/api/v1/battle/sessions/test/timeline")

        self.assertEqual([{"tick": 1}], items)

    def test_print_estimates_emits_tabular_output(self) -> None:
        estimate = MODULE.DailyEstimate(
            tracks=5,
            ticks=6,
            seconds_per_run=30,
            runs_per_day=2880,
            snapshots_per_run=6,
            radar_targets_per_run=30,
            battle_events_per_run=3,
            snapshots_per_day=17280,
            radar_targets_per_day=86400,
            battle_events_per_day=8640,
        )
        stream = io.StringIO()
        with mock.patch.object(sys, "stdout", stream):
            MODULE.print_estimates([estimate])

        output = stream.getvalue()
        self.assertIn("mode=estimate", output)
        self.assertIn("tracks", output)
        self.assertIn("86400", output)

    def test_run_live_capacity_probe_aggregates_observed_counts(self) -> None:
        client = mock.Mock()
        client.request.side_effect = [
            (201, {"session": {"sessionId": "battle-live"}}),
            (202, None),
            (202, None),
            (202, None),
            (200, {"items": [{"tick": 1}, {"tick": 2}, {"tick": 3}]}),
            (200, {"items": [
                {"id": 1, "radarTargets": [{"targetId": "a"}, {"targetId": "b"}]},
                {"id": 2, "radarTargets": [{"targetId": "a"}]},
                {"id": 3, "radarTargets": []},
            ]}),
            (200, {"firedCount": 2, "hitCount": 1, "destroyedCount": 0, "keyEvents": [{}, {}]}),
            (200, None),
        ]
        with mock.patch.object(MODULE.time, "sleep"):
            result = MODULE.run_live_capacity_probe(client, tracks=2, ticks=3, action_every_ticks=2, duration_seconds=30)

        self.assertEqual("battle-live", result["session_id"])
        self.assertEqual(3, result["observed_snapshots"])
        self.assertEqual(3, result["observed_timeline_frames"])
        self.assertEqual(3, result["observed_radar_targets"])
        self.assertEqual(2, result["observed_report_fired_count"])
        self.assertEqual(2, result["observed_report_key_events"])


if __name__ == "__main__":
    unittest.main()