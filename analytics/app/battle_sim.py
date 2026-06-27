from __future__ import annotations

import math
import random
from dataclasses import dataclass, field
from datetime import datetime, timezone


KNOT_TO_KMH = 1.852
EARTH_KM_PER_DEGREE = 111.0


@dataclass
class TacticalUnit:
    unit_id: str
    ship_id: int
    name: str
    side: str
    longitude: float
    latitude: float
    course: float
    speed_knots: float
    hp: float = 100
    max_hp: float = 100
    radar_range_km: float = 32
    weapon_range_km: float = 20
    cooldown_seconds: float = 6
    cooldown_remaining: float = 0
    status: str = "active"

    def to_payload(self) -> dict[str, object]:
        return {
            "unitId": self.unit_id,
            "shipId": self.ship_id,
            "name": self.name,
            "side": self.side,
            "hp": round(self.hp, 1),
            "maxHp": self.max_hp,
            "radarRangeKm": self.radar_range_km,
            "weaponRangeKm": self.weapon_range_km,
            "cooldownSeconds": self.cooldown_seconds,
            "longitude": round(self.longitude, 6),
            "latitude": round(self.latitude, 6),
            "course": round(self.course % 360, 2),
            "speedKnots": round(self.speed_knots, 2),
            "status": self.status,
        }


@dataclass
class Projectile:
    projectile_id: str
    source_unit_id: str
    target_unit_id: str
    side: str
    longitude: float
    latitude: float
    speed_kmh: float = 1600
    status: str = "flying"

    def to_payload(self) -> dict[str, object]:
        return {
            "projectileId": self.projectile_id,
            "sourceUnitId": self.source_unit_id,
            "targetUnitId": self.target_unit_id,
            "side": self.side,
            "longitude": round(self.longitude, 6),
            "latitude": round(self.latitude, 6),
            "speedKmH": round(self.speed_kmh, 1),
            "status": self.status,
        }


@dataclass
class BattleEvent:
    event_id: str
    type: str
    severity: str
    message: str
    source_unit_id: str = ""
    target_unit_id: str = ""
    longitude: float | None = None
    latitude: float | None = None
    occurred_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))

    def to_payload(self) -> dict[str, object]:
        payload: dict[str, object] = {
            "eventId": self.event_id,
            "type": self.type,
            "severity": self.severity,
            "message": self.message,
            "sourceUnitId": self.source_unit_id,
            "targetUnitId": self.target_unit_id,
            "occurredAt": self.occurred_at.isoformat(),
        }
        if self.longitude is not None:
            payload["longitude"] = round(self.longitude, 6)
        if self.latitude is not None:
            payload["latitude"] = round(self.latitude, 6)
        return payload


class BattleSimulation:
    def __init__(
        self,
        session_id: str,
        scenario_code: str = "open-water-duel",
        origin_longitude: float = 121.49,
        origin_latitude: float = 31.23,
        seed: int = 7,
    ) -> None:
        self.session_id = session_id
        self.scenario_code = scenario_code or "open-water-duel"
        self.origin_longitude = origin_longitude
        self.origin_latitude = origin_latitude
        self.rng = random.Random(seed)
        self.units = self._build_units()
        self.projectiles: list[Projectile] = []
        self.event_seq = 0
        self.projectile_seq = 0
        self.status = "running"

    def step(self, delta_seconds: float) -> dict[str, object]:
        now = datetime.now(timezone.utc)
        events: list[BattleEvent] = []
        self._move_units(delta_seconds)
        self._tick_cooldowns(delta_seconds)
        detections = self._scan()
        events.extend(self._fire(detections, now))
        events.extend(self._advance_projectiles(delta_seconds, now))
        if self._side_destroyed("blue"):
            self.status = "red_victory"
            events.append(self._event("BATTLE_ENDED", "CRITICAL", "Red force wins the radar barrage duel.", occurred_at=now))
        elif self._side_destroyed("red"):
            self.status = "blue_victory"
            events.append(self._event("BATTLE_ENDED", "CRITICAL", "Blue force wins the radar barrage duel.", occurred_at=now))
        radar_targets = [
            {
                "targetId": detection["target"].unit_id,
                "side": detection["target"].side,
                "longitude": round(detection["target"].longitude, 6),
                "latitude": round(detection["target"].latitude, 6),
                "course": round(detection["target"].course % 360, 2),
                "speedKnots": round(detection["target"].speed_knots, 2),
                "confidence": detection["confidence"],
                "detected": detection["detected"],
            }
            for detection in detections
        ]
        return {
            "sessionId": self.session_id,
            "radarId": "SIM-RADAR-01",
            "scanTime": now.isoformat(),
            "targets": radar_targets,
            "state": {
                "sessionId": self.session_id,
                "status": self.status,
                "units": [unit.to_payload() for unit in self.units],
                "projectiles": [item.to_payload() for item in self.projectiles],
                "events": [event.to_payload() for event in events],
                "updatedAt": now.isoformat(),
            },
        }

    def _build_units(self) -> list[TacticalUnit]:
        close = self.scenario_code == "close-quarter-barrage"
        radar_range = 22 if close else 32
        weapon_range = 15 if close else 20
        cooldown = 3.8 if close else 5.5
        red_count = 3 if close else 2
        units = [
            TacticalUnit("blue-1", 1, "Blue Destroyer 01", "blue", self.origin_longitude - 0.07, self.origin_latitude - 0.025, 68, 18, radar_range_km=radar_range, weapon_range_km=weapon_range, cooldown_seconds=cooldown),
            TacticalUnit("blue-2", 2, "Blue Frigate 02", "blue", self.origin_longitude - 0.06, self.origin_latitude + 0.035, 92, 16, radar_range_km=radar_range, weapon_range_km=weapon_range, cooldown_seconds=cooldown + 0.8),
        ]
        for index in range(red_count):
            units.append(
                TacticalUnit(
                    f"red-{index + 1}",
                    100 + index,
                    f"Red Fast Attack {index + 1:02d}",
                    "red",
                    self.origin_longitude + 0.075 + index * 0.015,
                    self.origin_latitude - 0.03 + index * 0.032,
                    258 - index * 12,
                    21 + index,
                    hp=82,
                    max_hp=82,
                    radar_range_km=radar_range * 0.9,
                    weapon_range_km=weapon_range * 0.9,
                    cooldown_seconds=max(3.2, cooldown - 0.5),
                )
            )
        return units

    def _move_units(self, delta_seconds: float) -> None:
        for unit in self.units:
            if unit.status != "active":
                continue
            distance_km = unit.speed_knots * KNOT_TO_KMH * delta_seconds / 3600
            unit.longitude, unit.latitude = move_point(unit.longitude, unit.latitude, unit.course, distance_km)
            if distance_km > 0:
                unit.course = self._steer_toward_enemy(unit)

    def _steer_toward_enemy(self, unit: TacticalUnit) -> float:
        enemies = [item for item in self.units if item.side != unit.side and item.status == "active"]
        if not enemies:
            return unit.course
        target = min(enemies, key=lambda enemy: distance_km(unit.longitude, unit.latitude, enemy.longitude, enemy.latitude))
        bearing = bearing_degrees(unit.longitude, unit.latitude, target.longitude, target.latitude)
        return (unit.course * 0.82 + bearing * 0.18) % 360

    def _tick_cooldowns(self, delta_seconds: float) -> None:
        for unit in self.units:
            unit.cooldown_remaining = max(0, unit.cooldown_remaining - delta_seconds)

    def _scan(self) -> list[dict[str, object]]:
        detections: list[dict[str, object]] = []
        for source in self.units:
            if source.status != "active":
                continue
            for target in self.units:
                if source.side == target.side or target.status != "active":
                    continue
                distance = distance_km(source.longitude, source.latitude, target.longitude, target.latitude)
                if distance > source.radar_range_km:
                    continue
                probability = max(0.45, 0.98 - distance / max(source.radar_range_km, 1) * 0.55)
                detected = self.rng.random() <= probability
                detections.append({"source": source, "target": target, "detected": detected, "confidence": round(probability if detected else probability * 0.45, 2)})
        return detections

    def _fire(self, detections: list[dict[str, object]], now: datetime) -> list[BattleEvent]:
        events: list[BattleEvent] = []
        for detection in detections:
            if not detection["detected"]:
                continue
            source = detection["source"]
            target = detection["target"]
            assert isinstance(source, TacticalUnit)
            assert isinstance(target, TacticalUnit)
            if source.cooldown_remaining > 0:
                continue
            distance = distance_km(source.longitude, source.latitude, target.longitude, target.latitude)
            if distance > source.weapon_range_km:
                continue
            source.cooldown_remaining = source.cooldown_seconds
            self.projectile_seq += 1
            projectile = Projectile(
                projectile_id=f"{self.session_id}-p{self.projectile_seq}",
                source_unit_id=source.unit_id,
                target_unit_id=target.unit_id,
                side=source.side,
                longitude=source.longitude,
                latitude=source.latitude,
            )
            self.projectiles.append(projectile)
            events.append(
                self._event(
                    "WEAPON_FIRED",
                    "INFO",
                    f"{source.name} fires a barrage salvo at {target.name}.",
                    source.unit_id,
                    target.unit_id,
                    source.longitude,
                    source.latitude,
                    now,
                )
            )
        return events

    def _advance_projectiles(self, delta_seconds: float, now: datetime) -> list[BattleEvent]:
        events: list[BattleEvent] = []
        active: list[Projectile] = []
        for projectile in self.projectiles:
            if projectile.status != "flying":
                continue
            target = self._unit(projectile.target_unit_id)
            if target is None or target.status != "active":
                projectile.status = "expired"
                active.append(projectile)
                continue
            travel_km = projectile.speed_kmh * delta_seconds / 3600
            remaining_km = distance_km(projectile.longitude, projectile.latitude, target.longitude, target.latitude)
            if remaining_km <= max(0.35, travel_km):
                projectile.longitude = target.longitude
                projectile.latitude = target.latitude
                projectile.status = "hit"
                damage = 18 + self.rng.random() * 18
                target.hp = max(0, target.hp - damage)
                events.append(
                    self._event(
                        "PROJECTILE_HIT",
                        "WARN",
                        f"{projectile.source_unit_id} hits {target.name}, damage {damage:.1f}, hp {target.hp:.1f}.",
                        projectile.source_unit_id,
                        target.unit_id,
                        target.longitude,
                        target.latitude,
                        now,
                    )
                )
                if target.hp <= 0 and target.status != "destroyed":
                    target.status = "destroyed"
                    target.speed_knots = 0
                    events.append(
                        self._event(
                            "UNIT_DESTROYED",
                            "CRITICAL",
                            f"{target.name} is disabled by concentrated barrage fire.",
                            projectile.source_unit_id,
                            target.unit_id,
                            target.longitude,
                            target.latitude,
                            now,
                        )
                    )
                active.append(projectile)
                continue
            bearing = bearing_degrees(projectile.longitude, projectile.latitude, target.longitude, target.latitude)
            projectile.longitude, projectile.latitude = move_point(projectile.longitude, projectile.latitude, bearing, travel_km)
            active.append(projectile)
        self.projectiles = active[-80:]
        return events

    def _unit(self, unit_id: str) -> TacticalUnit | None:
        for unit in self.units:
            if unit.unit_id == unit_id:
                return unit
        return None

    def _side_destroyed(self, side: str) -> bool:
        return all(unit.status != "active" for unit in self.units if unit.side == side)

    def _event(
        self,
        event_type: str,
        severity: str,
        message: str,
        source_unit_id: str = "",
        target_unit_id: str = "",
        longitude: float | None = None,
        latitude: float | None = None,
        occurred_at: datetime | None = None,
    ) -> BattleEvent:
        self.event_seq += 1
        return BattleEvent(
            event_id=f"{self.session_id}-e{self.event_seq}",
            type=event_type,
            severity=severity,
            message=message,
            source_unit_id=source_unit_id,
            target_unit_id=target_unit_id,
            longitude=longitude,
            latitude=latitude,
            occurred_at=occurred_at or datetime.now(timezone.utc),
        )


def distance_km(lon1: float, lat1: float, lon2: float, lat2: float) -> float:
    radius_km = 6371.0
    d_lat = math.radians(lat2 - lat1)
    d_lon = math.radians(lon2 - lon1)
    a = math.sin(d_lat / 2) ** 2 + math.cos(math.radians(lat1)) * math.cos(math.radians(lat2)) * math.sin(d_lon / 2) ** 2
    return radius_km * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))


def bearing_degrees(lon1: float, lat1: float, lon2: float, lat2: float) -> float:
    y = math.sin(math.radians(lon2 - lon1)) * math.cos(math.radians(lat2))
    x = math.cos(math.radians(lat1)) * math.sin(math.radians(lat2)) - math.sin(math.radians(lat1)) * math.cos(math.radians(lat2)) * math.cos(math.radians(lon2 - lon1))
    return (math.degrees(math.atan2(y, x)) + 360) % 360


def move_point(longitude: float, latitude: float, course: float, distance: float) -> tuple[float, float]:
    course_rad = math.radians(course)
    north_km = math.cos(course_rad) * distance
    east_km = math.sin(course_rad) * distance
    new_latitude = latitude + north_km / EARTH_KM_PER_DEGREE
    lon_scale = max(0.2, math.cos(math.radians(latitude)))
    new_longitude = longitude + east_km / (EARTH_KM_PER_DEGREE * lon_scale)
    return new_longitude, new_latitude
