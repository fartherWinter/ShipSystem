#!/usr/bin/env python3
"""Estimate or smoke-test ShipSystem battle replay capacity growth."""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Any


BACKEND_BASE_URL = os.getenv("SHIPSYSTEM_BACKEND_URL", "http://localhost:8080").rstrip("/")
DEFAULT_TRACK_COUNTS = (5, 20, 100)
DEFAULT_DURATION_SECONDS = 30
DEFAULT_TICKS = 6
REQUEST_TIMEOUT_SECONDS = 8


@dataclass(frozen=True)
class CapacityProfile:
    tracks: int
    ticks: int
    action_every_ticks: int
    snapshots_per_run: int
    radar_targets_per_snapshot: int
    radar_targets_per_run: int
    battle_events_per_run: int


@dataclass(frozen=True)
class DailyEstimate:
    tracks: int
    ticks: int
    seconds_per_run: int
    runs_per_day: int
    snapshots_per_run: int
    radar_targets_per_run: int
    battle_events_per_run: int
    snapshots_per_day: int
    radar_targets_per_day: int
    battle_events_per_day: int


class JsonHttpClient:
    def __init__(self, base_url: str) -> None:
        self.base_url = base_url.rstrip("/")

    def request(self, method: str, path: str, payload: dict[str, Any] | None = None) -> tuple[int, Any]:
        url = f"{self.base_url}{path}"
        data = None
        headers = {"Accept": "application/json"}
        if payload is not None:
            data = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        request = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
                return response.status, decode_response_body(response)
        except urllib.error.HTTPError as exc:
            body = decode_response_body(exc)
            detail = summarize_body(body) or exc.reason
            raise RuntimeError(f"{method} {path} returned HTTP {exc.code}: {detail}") from exc
        except urllib.error.URLError as exc:
            raise RuntimeError(f"{method} {path} failed: {exc.reason}") from exc


def main() -> int:
    args = parse_args()
    track_counts = parse_track_counts(args.track_counts)
    estimates = [estimate_daily_growth(build_capacity_profile(tracks, args.ticks, args.action_every_ticks), args.duration_seconds) for tracks in track_counts]

    if args.estimate_only:
        print_estimates(estimates)
        return 0

    client = JsonHttpClient(args.backend_base_url)
    results = []
    for tracks in track_counts:
        result = run_live_capacity_probe(
            client,
            tracks=tracks,
            ticks=args.ticks,
            action_every_ticks=args.action_every_ticks,
            duration_seconds=args.duration_seconds,
        )
        results.append(result)

    print_live_results(results)
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--backend-base-url", default=BACKEND_BASE_URL, help="Backend base URL, defaults to SHIPSYSTEM_BACKEND_URL or http://localhost:8080")
    parser.add_argument("--track-counts", default=",".join(str(value) for value in DEFAULT_TRACK_COUNTS), help="Comma-separated target counts to estimate or simulate")
    parser.add_argument("--duration-seconds", type=positive_int, default=DEFAULT_DURATION_SECONDS, help="Representative run duration used for daily estimates")
    parser.add_argument("--ticks", type=positive_int, default=DEFAULT_TICKS, help="Synthetic radar report ticks to submit per live probe")
    parser.add_argument("--action-every-ticks", type=positive_int, default=2, help="Inject one WEAPON_FIRED event every N ticks during live probes and estimates")
    parser.add_argument("--estimate-only", action="store_true", help="Only print daily growth estimates, do not call the running stack")
    return parser.parse_args()


def positive_int(value: str) -> int:
    try:
        parsed = int(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be an integer") from exc
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be greater than 0")
    return parsed


def parse_track_counts(value: str) -> list[int]:
    items = []
    for part in value.split(","):
        text = part.strip()
        if not text:
            continue
        items.append(positive_int(text))
    if not items:
        raise SystemExit("[FAIL] track counts must not be empty")
    return items


def build_capacity_profile(tracks: int, ticks: int, action_every_ticks: int) -> CapacityProfile:
    snapshots_per_run = ticks
    radar_targets_per_snapshot = tracks
    radar_targets_per_run = snapshots_per_run * radar_targets_per_snapshot
    event_ticks = count_event_ticks(ticks, action_every_ticks)
    battle_events_per_run = event_ticks
    return CapacityProfile(
        tracks=tracks,
        ticks=ticks,
        action_every_ticks=action_every_ticks,
        snapshots_per_run=snapshots_per_run,
        radar_targets_per_snapshot=radar_targets_per_snapshot,
        radar_targets_per_run=radar_targets_per_run,
        battle_events_per_run=battle_events_per_run,
    )


def count_event_ticks(ticks: int, action_every_ticks: int) -> int:
    return sum(1 for tick in range(1, ticks + 1) if tick % action_every_ticks == 0)


def estimate_daily_growth(profile: CapacityProfile, duration_seconds: int) -> DailyEstimate:
    runs_per_day = max(1, 86400 // duration_seconds)
    return DailyEstimate(
        tracks=profile.tracks,
        ticks=profile.ticks,
        seconds_per_run=duration_seconds,
        runs_per_day=runs_per_day,
        snapshots_per_run=profile.snapshots_per_run,
        radar_targets_per_run=profile.radar_targets_per_run,
        battle_events_per_run=profile.battle_events_per_run,
        snapshots_per_day=profile.snapshots_per_run * runs_per_day,
        radar_targets_per_day=profile.radar_targets_per_run * runs_per_day,
        battle_events_per_day=profile.battle_events_per_run * runs_per_day,
    )


def print_estimates(estimates: list[DailyEstimate]) -> None:
    print("mode=estimate")
    print("tracks\tticks\tseconds_per_run\truns_per_day\tsnapshots_per_run\tradar_targets_per_run\tbattle_events_per_run\tsnapshots_per_day\tradar_targets_per_day\tbattle_events_per_day")
    for item in estimates:
        print(
            "\t".join(
                [
                    str(item.tracks),
                    str(item.ticks),
                    str(item.seconds_per_run),
                    str(item.runs_per_day),
                    str(item.snapshots_per_run),
                    str(item.radar_targets_per_run),
                    str(item.battle_events_per_run),
                    str(item.snapshots_per_day),
                    str(item.radar_targets_per_day),
                    str(item.battle_events_per_day),
                ]
            )
        )


def run_live_capacity_probe(
    client: JsonHttpClient,
    *,
    tracks: int,
    ticks: int,
    action_every_ticks: int,
    duration_seconds: int,
) -> dict[str, Any]:
    status, data = client.request("POST", "/api/v1/battle/sessions", {"scenarioCode": "open-water-duel"})
    if status != 201:
        raise RuntimeError(f"create battle session returned HTTP {status}")
    body = require_dict(data, "create battle session")
    session = require_dict(body.get("session"), "create battle session.session")
    session_id = str(session.get("sessionId") or "")
    if not session_id:
        raise RuntimeError("battle session did not return sessionId")

    try:
        for tick in range(1, ticks + 1):
            payload = build_radar_report_payload(session_id, tracks=tracks, tick=tick, action_every_ticks=action_every_ticks)
            status, _ = client.request("POST", "/api/v1/radar/reports", payload)
            if status != 202:
                raise RuntimeError(f"post radar report returned HTTP {status}")
            time.sleep(0.05)

        timeline = fetch_items(client, f"/api/v1/battle/sessions/{session_id}/timeline")
        snapshots = fetch_items(client, f"/api/v1/battle/sessions/{session_id}/snapshots")
        report_status, report_data = client.request("GET", f"/api/v1/battle/sessions/{session_id}/report")
        if report_status != 200:
            raise RuntimeError(f"battle report returned HTTP {report_status}")
        report = require_dict(report_data, "battle report")

        observed_event_count = int(report.get("firedCount") or 0) + int(report.get("hitCount") or 0) + int(report.get("destroyedCount") or 0)
        observed_radar_targets = sum(len(require_dict(item, "snapshot").get("radarTargets") or []) for item in snapshots)
        observed_snapshot_count = len(snapshots)
        observed_timeline_frames = len(timeline)

        estimate = estimate_daily_growth(build_capacity_profile(tracks, ticks, action_every_ticks), duration_seconds)
        return {
            "tracks": tracks,
            "session_id": session_id,
            "ticks": ticks,
            "observed_snapshots": observed_snapshot_count,
            "observed_timeline_frames": observed_timeline_frames,
            "observed_radar_targets": observed_radar_targets,
            "observed_report_fired_count": int(report.get("firedCount") or 0),
            "observed_report_hit_count": int(report.get("hitCount") or 0),
            "observed_report_destroyed_count": int(report.get("destroyedCount") or 0),
            "observed_report_key_events": len(report.get("keyEvents") or []),
            "estimated_snapshots_per_day": estimate.snapshots_per_day,
            "estimated_radar_targets_per_day": estimate.radar_targets_per_day,
            "estimated_battle_events_per_day": estimate.battle_events_per_day,
            "estimated_total_report_events": observed_event_count,
        }
    finally:
        client.request("POST", f"/api/v1/battle/sessions/{session_id}/stop")


def fetch_items(client: JsonHttpClient, path: str) -> list[dict[str, Any]]:
    status, data = client.request("GET", path)
    if status != 200:
        raise RuntimeError(f"GET {path} returned HTTP {status}")
    body = require_dict(data, path)
    items = body.get("items")
    if not isinstance(items, list):
        raise RuntimeError(f"{path} did not return an items array")
    return [require_dict(item, path) for item in items]


def build_radar_report_payload(session_id: str, *, tracks: int, tick: int, action_every_ticks: int) -> dict[str, Any]:
    updated_at = format_timestamp(tick)
    targets = []
    units = []
    for index in range(tracks):
        side = "blue" if index % 2 == 0 else "red"
        unit_id = f"{side}-{index + 1}"
        lon = 121.30 + index * 0.01
        lat = 31.10 + index * 0.005
        targets.append(
            {
                "targetId": unit_id,
                "side": side,
                "longitude": lon,
                "latitude": lat,
                "course": float((90 + index * 7) % 360),
                "speedKnots": 18.0 + (index % 6),
                "confidence": 0.75,
                "detected": True,
            }
        )
        units.append(
            {
                "unitId": unit_id,
                "shipId": index + 1,
                "name": f"{side.title()} Unit {index + 1}",
                "side": side,
                "hp": 100.0 - float(index % 10),
                "maxHp": 100.0,
                "radarRangeKm": 32.0,
                "weaponRangeKm": 20.0,
                "cooldownSeconds": 5.0,
                "longitude": lon,
                "latitude": lat,
                "course": float((90 + index * 7) % 360),
                "speedKnots": 18.0 + (index % 6),
                "status": "active",
            }
        )

    projectiles = []
    if tracks >= 2:
        projectiles.append(
            {
                "projectileId": f"{session_id}-p{tick}",
                "sourceUnitId": units[0]["unitId"],
                "targetUnitId": units[1]["unitId"],
                "side": units[0]["side"],
                "longitude": 121.35 + tick * 0.001,
                "latitude": 31.15 + tick * 0.001,
                "speedKmH": 1500.0,
                "status": "flying",
            }
        )

    events = []
    if tick % action_every_ticks == 0 and tracks >= 2:
        events.append(
            {
                "eventId": f"{session_id}-e{tick}",
                "type": "WEAPON_FIRED",
                "severity": "INFO",
                "message": f"Synthetic capacity tick {tick}",
                "sourceUnitId": units[0]["unitId"],
                "targetUnitId": units[1]["unitId"],
                "longitude": units[0]["longitude"],
                "latitude": units[0]["latitude"],
            }
        )

    return {
        "sessionId": session_id,
        "radarId": "CAPACITY-SMOKE",
        "scanTime": updated_at,
        "targets": targets,
        "state": {
            "sessionId": session_id,
            "status": "running",
            "units": units,
            "projectiles": projectiles,
            "events": events,
            "updatedAt": updated_at,
        },
    }


def format_timestamp(tick: int) -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(1_717_900_000 + tick))


def require_dict(value: Any, name: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise RuntimeError(f"{name} did not return a JSON object")
    return value


def decode_response_body(response) -> Any:
    raw = response.read()
    if not raw:
        return None
    content_type = response.headers.get("Content-Type", "")
    text = raw.decode("utf-8", errors="replace")
    if "application/json" in content_type:
        return json.loads(text)
    return text


def summarize_body(body: Any) -> str:
    if body is None:
        return ""
    if isinstance(body, dict):
        summary = json.dumps(body, ensure_ascii=False, separators=(",", ":"))
    else:
        summary = str(body).strip()
    if len(summary) > 300:
        return summary[:300] + "..."
    return summary


def print_live_results(results: list[dict[str, Any]]) -> None:
    print("mode=live")
    print(
        "tracks\tsession_id\tticks\tobserved_snapshots\tobserved_timeline_frames\tobserved_radar_targets\t"
        "observed_report_fired_count\tobserved_report_hit_count\tobserved_report_destroyed_count\t"
        "observed_report_key_events\testimated_snapshots_per_day\testimated_radar_targets_per_day\testimated_battle_events_per_day"
    )
    for item in results:
        print(
            "\t".join(
                [
                    str(item["tracks"]),
                    str(item["session_id"]),
                    str(item["ticks"]),
                    str(item["observed_snapshots"]),
                    str(item["observed_timeline_frames"]),
                    str(item["observed_radar_targets"]),
                    str(item["observed_report_fired_count"]),
                    str(item["observed_report_hit_count"]),
                    str(item["observed_report_destroyed_count"]),
                    str(item["observed_report_key_events"]),
                    str(item["estimated_snapshots_per_day"]),
                    str(item["estimated_radar_targets_per_day"]),
                    str(item["estimated_battle_events_per_day"]),
                ]
            )
        )


if __name__ == "__main__":
    sys.exit(main())