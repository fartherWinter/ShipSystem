#!/usr/bin/env python3
"""Static checks for ShipSystem WebSocket event contract drift."""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
BACKEND_EVENT_SOURCES = (
    ROOT / "backend" / "internal" / "services" / "services.go",
    ROOT / "backend" / "internal" / "ws" / "hub.go",
)
FRONTEND_TYPES = ROOT / "frontend" / "src" / "types.ts"
FRONTEND_APP = ROOT / "frontend" / "src" / "App.tsx"
README = ROOT / "README.md"
SMOKE_CHECK = ROOT / "scripts" / "smoke_check.py"

EXPECTED_WS_EVENTS = {
    "ship_location_updated",
    "alarm_created",
    "dispatch_event_updated",
    "radar_scan_updated",
    "projectile_updated",
    "battle_event_created",
    "battle_state_updated",
    "heartbeat",
}
FRONTEND_HANDLED_EVENTS = EXPECTED_WS_EVENTS - {"heartbeat"}
SMOKE_REQUIRED_EVENTS = {
    "radar_scan_updated",
    "projectile_updated",
    "battle_event_created",
    "battle_state_updated",
}


def main() -> int:
    failures: list[str] = []
    backend_events = backend_event_types()
    frontend_union_events = frontend_ws_union_types()
    frontend_handled_events = frontend_handled_types()
    readme_text = README.read_text(encoding="utf-8")
    smoke_text = SMOKE_CHECK.read_text(encoding="utf-8")

    failures.extend(compare_event_set("backend ws events", EXPECTED_WS_EVENTS, backend_events))
    failures.extend(compare_event_set("frontend WsMessage union", EXPECTED_WS_EVENTS, frontend_union_events))
    failures.extend(compare_event_set("frontend App handlers", FRONTEND_HANDLED_EVENTS, frontend_handled_events))

    for event_type in sorted(EXPECTED_WS_EVENTS):
        if event_type not in readme_text:
            failures.append(f"README WebSocket section must document {event_type}")

    for event_type in sorted(SMOKE_REQUIRED_EVENTS):
        if event_type not in smoke_text:
            failures.append(f"scripts/smoke_check.py must observe runtime event {event_type}")

    if failures:
        for failure in failures:
            print(f"[FAIL] {failure}")
        return 1

    print("event contract checks passed")
    return 0


def backend_event_types() -> set[str]:
    events: set[str] = set()
    for path in BACKEND_EVENT_SOURCES:
        text = path.read_text(encoding="utf-8")
        events.update(re.findall(r"Event\s*\{\s*Type:\s*\"([^\"]+)\"", text))
        events.update(re.findall(r"ws\.Event\s*\{\s*Type:\s*\"([^\"]+)\"", text))
    return events


def frontend_ws_union_types() -> set[str]:
    text = FRONTEND_TYPES.read_text(encoding="utf-8")
    match = re.search(r"export type WsMessage =(?P<body>.*?)(?:\n\n|$)", text, flags=re.S)
    if not match:
        return set()
    return set(re.findall(r"type:\s*'([^']+)'", match.group("body")))


def frontend_handled_types() -> set[str]:
    text = FRONTEND_APP.read_text(encoding="utf-8")
    return set(re.findall(r"data\.type\s*===\s*'([^']+)'", text))


def compare_event_set(label: str, expected: set[str], actual: set[str]) -> list[str]:
    failures: list[str] = []
    missing = expected - actual
    extra = actual - expected
    if missing:
        failures.append(f"{label} missing: {', '.join(sorted(missing))}")
    if extra:
        failures.append(f"{label} has undocumented events: {', '.join(sorted(extra))}")
    return failures


if __name__ == "__main__":
    sys.exit(main())
