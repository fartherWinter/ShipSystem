#!/usr/bin/env python3
"""Static checks for analytics -> Go callback payload contract drift."""

from __future__ import annotations

import importlib.util
import re
import sys
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
GO_SERVICES = ROOT / "backend" / "internal" / "services" / "services.go"
BATTLE_SIM = ROOT / "analytics" / "app" / "battle_sim.py"

GO_PAYLOAD_STRUCTS = {
    "RadarReportPayload": {"sessionId", "radarId", "scanTime", "targets", "state"},
    "RadarTargetPayload": {"targetId", "side", "longitude", "latitude", "course", "speedKnots", "confidence", "detected"},
    "BattleStatePayload": {"sessionId", "status", "units", "projectiles", "events", "updatedAt"},
    "BattleUnitPayload": {
        "unitId",
        "shipId",
        "name",
        "side",
        "hp",
        "maxHp",
        "radarRangeKm",
        "weaponRangeKm",
        "cooldownSeconds",
        "longitude",
        "latitude",
        "course",
        "speedKnots",
        "status",
    },
    "BattleProjectilePayload": {"projectileId", "sourceUnitId", "targetUnitId", "side", "longitude", "latitude", "speedKmH", "status"},
    "BattleEventPayload": {
        "eventId",
        "type",
        "severity",
        "message",
        "sourceUnitId",
        "targetUnitId",
        "longitude",
        "latitude",
        "occurredAt",
    },
}
SAMPLE_REQUIRED_PATHS = {
    "RadarReportPayload": (),
    "RadarTargetPayload": ("targets", 0),
    "BattleStatePayload": ("state",),
    "BattleUnitPayload": ("state", "units", 0),
    "BattleProjectilePayload": ("state", "projectiles", 0),
    "BattleEventPayload": ("state", "events", 0),
}
OPTIONAL_SAMPLE_FIELDS = {
    "BattleEventPayload": {"longitude", "latitude"},
}


def main() -> int:
    failures: list[str] = []
    go_structs = go_json_tags()
    for name, expected in GO_PAYLOAD_STRUCTS.items():
        actual = go_structs.get(name, set())
        failures.extend(compare_fields(f"Go {name}", expected, actual))

    sample = battle_simulation_sample()
    for name, path in SAMPLE_REQUIRED_PATHS.items():
        value = nested_value(sample, path)
        if not isinstance(value, dict):
            failures.append(f"analytics sample for {name} must be an object at {format_path(path)}")
            continue
        allowed = GO_PAYLOAD_STRUCTS[name]
        required = allowed - OPTIONAL_SAMPLE_FIELDS.get(name, set())
        actual = set(value)
        failures.extend(compare_fields(f"analytics sample {name}", required, actual, allowed=allowed))

    if failures:
        for failure in failures:
            print(f"[FAIL] {failure}")
        return 1

    print("callback contract checks passed")
    return 0


def go_json_tags() -> dict[str, set[str]]:
    text = GO_SERVICES.read_text(encoding="utf-8")
    structs: dict[str, set[str]] = {}
    for match in re.finditer(r"type\s+(\w+Payload)\s+struct\s*\{(?P<body>.*?)\n\}", text, flags=re.S):
        name = match.group(1)
        tags = set(re.findall(r"`json:\"([^\",]+)", match.group("body")))
        structs[name] = tags
    return structs


def battle_simulation_sample() -> dict[str, Any]:
    module_name = "_shipsystem_contract_battle_sim"
    spec = importlib.util.spec_from_file_location(module_name, BATTLE_SIM)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {BATTLE_SIM}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[module_name] = module
    spec.loader.exec_module(module)
    sim = module.BattleSimulation("contract-session", scenario_code="close-quarter-barrage", seed=2)
    for unit in sim.units:
        if unit.side == "red":
            unit.longitude = sim.origin_longitude + 0.01
            unit.latitude = sim.origin_latitude
        else:
            unit.longitude = sim.origin_longitude - 0.01
            unit.latitude = sim.origin_latitude
        unit.cooldown_seconds = 0.1
    payload = sim.step(1.2)
    if not isinstance(payload, dict):
        raise RuntimeError("BattleSimulation.step must return a dict")
    return payload


def nested_value(value: Any, path: tuple[Any, ...]) -> Any:
    current = value
    for part in path:
        if isinstance(part, int):
            if not isinstance(current, list) or len(current) <= part:
                return None
            current = current[part]
            continue
        if not isinstance(current, dict):
            return None
        current = current.get(part)
    return current


def compare_fields(label: str, expected: set[str], actual: set[str], allowed: set[str] | None = None) -> list[str]:
    failures: list[str] = []
    allowed = expected if allowed is None else allowed
    missing = expected - actual
    extra = actual - allowed
    if missing:
        failures.append(f"{label} missing fields: {', '.join(sorted(missing))}")
    if extra:
        failures.append(f"{label} has unsupported fields: {', '.join(sorted(extra))}")
    return failures


def format_path(path: tuple[Any, ...]) -> str:
    if not path:
        return "<root>"
    return ".".join(str(item) for item in path)


if __name__ == "__main__":
    sys.exit(main())
