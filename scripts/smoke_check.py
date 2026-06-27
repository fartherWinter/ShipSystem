#!/usr/bin/env python3
"""Local smoke checks for a running ShipSystem stack."""

from __future__ import annotations

import json
import os
import base64
import hashlib
import socket
import ssl
import struct
import sys
import time
import uuid
import urllib.error
import urllib.parse
import urllib.request
from http.cookiejar import CookieJar
from typing import Any


BACKEND_BASE_URL = os.getenv("SHIPSYSTEM_BACKEND_URL", "http://localhost:8080").rstrip("/")
ANALYTICS_BASE_URL = os.getenv("SHIPSYSTEM_ANALYTICS_URL", "http://localhost:8090").rstrip("/")
FRONTEND_BASE_URL = os.getenv("SHIPSYSTEM_FRONTEND_URL", "http://localhost:3000").rstrip("/")
WS_BASE_URL = os.getenv("SHIPSYSTEM_WS_URL", "").strip()
USERNAME = os.getenv("SHIPSYSTEM_USERNAME", "admin")
PASSWORD = os.getenv("SHIPSYSTEM_PASSWORD", "Admin123!")
REQUEST_ID_HEADER = "X-Request-ID"
AUTH_COOKIE_NAME = "shipsystem_token"
REQUIRED_BATTLE_WS_EVENTS = {
    "radar_scan_updated",
    "projectile_updated",
    "battle_event_created",
    "battle_state_updated",
}


class SmokeHTTPError(RuntimeError):
    def __init__(self, method: str, url: str, status: int, request_id: str, message: str, body: Any) -> None:
        self.method = method
        self.url = url
        self.status = status
        self.request_id = request_id
        self.message = message
        self.body = body
        super().__init__(self._format())

    def _format(self) -> str:
        parts = [f"{self.method} {self.url} returned HTTP {self.status}"]
        if self.message:
            parts.append(f"message={self.message}")
        if self.request_id:
            parts.append(f"requestId={self.request_id}")
        body_summary = summarize_body(self.body)
        if body_summary:
            parts.append(f"body={body_summary}")
        return "; ".join(parts)


class SmokeClient:
    def __init__(self) -> None:
        self.cookie_jar = CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.cookie_jar))
        self.token = ""
        self.last_response_headers: dict[str, list[str]] = {}

    def request(
        self,
        method: str,
        url: str,
        payload: dict[str, Any] | None = None,
        *,
        require_request_id: bool = True,
        accept: str = "application/json",
    ) -> tuple[int, Any]:
        data = None
        request_id = f"smoke-{uuid.uuid4().hex}"
        headers = {"Accept": accept, REQUEST_ID_HEADER: request_id}
        if payload is not None:
            data = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"

        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with self.opener.open(req, timeout=8) as response:
                self.capture_response_headers(response.headers)
                body = decode_response_body(response)
                response_request_id = response.headers.get(REQUEST_ID_HEADER, "")
                if require_request_id and response_request_id != request_id:
                    raise RuntimeError(
                        f"{method} {url} did not echo {REQUEST_ID_HEADER}: "
                        f"expected {request_id}, got {response_request_id or '<missing>'}"
                    )
                return response.status, body
        except urllib.error.HTTPError as exc:
            self.capture_response_headers(exc.headers)
            body = decode_response_body(exc)
            response_request_id = exc.headers.get(REQUEST_ID_HEADER, request_id)
            if isinstance(body, dict):
                response_request_id = str(body.get("requestId") or response_request_id)
                message = str(body.get("message") or exc.reason)
            else:
                message = str(exc.reason)
            raise SmokeHTTPError(method, url, exc.code, response_request_id, message, body) from exc

    def login(self) -> None:
        status, data = self.request(
            "POST",
            f"{BACKEND_BASE_URL}/api/v1/auth/login",
            {"username": USERNAME, "password": PASSWORD},
        )
        if status != 200 or not isinstance(data, dict) or not data.get("token"):
            raise RuntimeError("login did not return a token")
        self.token = str(data["token"])
        self.require_auth_cookie()

    def auth_cookie(self):
        now = time.time()
        for cookie in self.cookie_jar:
            if cookie.name == AUTH_COOKIE_NAME and cookie.value and not cookie.is_expired(now):
                return cookie
        return None

    def response_headers(self, name: str) -> list[str]:
        return self.last_response_headers.get(name.lower(), [])

    def capture_response_headers(self, headers) -> None:
        captured: dict[str, list[str]] = {}
        for name in headers.keys():
            values = headers.get_all(name, []) if hasattr(headers, "get_all") else [headers.get(name)]
            captured[name.lower()] = [value for value in values if value is not None]
        self.last_response_headers = captured

    def require_auth_cookie(self) -> None:
        cookie = self.auth_cookie()
        if cookie is None or not cookie.value:
            raise RuntimeError(f"login did not set {AUTH_COOKIE_NAME} cookie")
        if cookie.path != "/":
            raise RuntimeError(f"{AUTH_COOKIE_NAME} cookie path must be /")
        if not cookie.has_nonstandard_attr("HttpOnly"):
            raise RuntimeError(f"{AUTH_COOKIE_NAME} cookie must be HttpOnly")

    def logout(self) -> None:
        status, _ = self.request("POST", f"{BACKEND_BASE_URL}/api/v1/auth/logout")
        expect_status("logout", status, 204)
        cookie_headers = [
            value
            for value in self.response_headers("Set-Cookie")
            if value.lower().split(";", 1)[0].startswith(f"{AUTH_COOKIE_NAME.lower()}=")
        ]
        if not cookie_headers:
            raise RuntimeError(f"logout did not return Set-Cookie for {AUTH_COOKIE_NAME}")
        normalized_headers = "; ".join(cookie_headers).lower()
        if "max-age=-1" not in normalized_headers and "max-age=0" not in normalized_headers:
            raise RuntimeError(f"logout did not expire {AUTH_COOKIE_NAME} cookie")
        if self.auth_cookie() is not None:
            raise RuntimeError(f"logout did not clear {AUTH_COOKIE_NAME} cookie from client jar")
        self.token = ""


class SmokeWebSocket:
    def __init__(self, url: str, cookie_header: str) -> None:
        self.url = url
        self.cookie_header = cookie_header
        self.sock: socket.socket | ssl.SSLSocket | None = None
        self.buffer = b""

    def connect(self) -> None:
        parts = urllib.parse.urlsplit(self.url)
        if parts.scheme not in {"ws", "wss"}:
            raise RuntimeError(f"unsupported websocket scheme: {parts.scheme}")
        if not parts.hostname:
            raise RuntimeError("websocket URL must include a host")

        port = parts.port or (443 if parts.scheme == "wss" else 80)
        raw_sock = socket.create_connection((parts.hostname, port), timeout=8)
        if parts.scheme == "wss":
            context = ssl.create_default_context()
            self.sock = context.wrap_socket(raw_sock, server_hostname=parts.hostname)
        else:
            self.sock = raw_sock
        self.sock.settimeout(8)

        key = base64.b64encode(os.urandom(16)).decode("ascii")
        path = parts.path or "/"
        if parts.query:
            path = f"{path}?{parts.query}"
        headers = [
            f"GET {path} HTTP/1.1",
            f"Host: {parts.netloc}",
            "Upgrade: websocket",
            "Connection: Upgrade",
            f"Sec-WebSocket-Key: {key}",
            "Sec-WebSocket-Version: 13",
        ]
        if self.cookie_header:
            headers.append(f"Cookie: {self.cookie_header}")
        request = "\r\n".join(headers) + "\r\n\r\n"
        self.sock.sendall(request.encode("ascii"))

        response = self._read_http_header()
        lines = response.split("\r\n")
        if not lines or " 101 " not in lines[0]:
            raise RuntimeError(f"websocket handshake failed: {lines[0] if lines else '<empty>'}")
        response_headers = {}
        for line in lines[1:]:
            if ":" not in line:
                continue
            name, value = line.split(":", 1)
            response_headers[name.strip().lower()] = value.strip()
        expected_accept = websocket_accept(key)
        if response_headers.get("sec-websocket-accept") != expected_accept:
            raise RuntimeError("websocket handshake returned invalid Sec-WebSocket-Accept")

    def read_json_until(self, event_types: set[str], timeout: float = 8) -> dict[str, Any]:
        return self.read_json_matching(lambda event: event.get("type") in event_types, timeout)

    def read_json_matching(self, predicate, timeout: float = 8) -> dict[str, Any]:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if not self.sock:
                raise RuntimeError("websocket is not connected")
            self.sock.settimeout(max(0.1, deadline - time.monotonic()))
            try:
                text = self.read_text()
            except socket.timeout:
                break
            try:
                data = json.loads(text)
            except json.JSONDecodeError:
                continue
            if isinstance(data, dict) and predicate(data):
                return data
        raise RuntimeError(f"websocket did not receive a matching event within {timeout:.1f}s")

    def read_text(self) -> str:
        chunks: list[bytes] = []
        while True:
            header = self._read_exact(2)
            first, second = header[0], header[1]
            fin = bool(first & 0x80)
            opcode = first & 0x0F
            masked = bool(second & 0x80)
            length = second & 0x7F
            if length == 126:
                length = struct.unpack("!H", self._read_exact(2))[0]
            elif length == 127:
                length = struct.unpack("!Q", self._read_exact(8))[0]
            mask_key = self._read_exact(4) if masked else b""
            payload = self._read_exact(length) if length else b""
            if masked:
                payload = bytes(byte ^ mask_key[index % 4] for index, byte in enumerate(payload))

            if opcode == 0x8:
                raise RuntimeError("websocket closed before expected event")
            if opcode == 0x9:
                self._send_frame(0xA, payload)
                continue
            if opcode == 0xA:
                continue
            if opcode not in {0x0, 0x1}:
                continue

            chunks.append(payload)
            if fin:
                return b"".join(chunks).decode("utf-8")

    def close(self) -> None:
        if not self.sock:
            return
        try:
            self._send_frame(0x8, b"")
        except OSError:
            pass
        try:
            self.sock.close()
        finally:
            self.sock = None

    def _read_http_header(self) -> str:
        while b"\r\n\r\n" not in self.buffer:
            self.buffer += self._recv(4096)
        raw_header, self.buffer = self.buffer.split(b"\r\n\r\n", 1)
        return raw_header.decode("iso-8859-1")

    def _read_exact(self, size: int) -> bytes:
        while len(self.buffer) < size:
            self.buffer += self._recv(max(4096, size - len(self.buffer)))
        data, self.buffer = self.buffer[:size], self.buffer[size:]
        return data

    def _recv(self, size: int) -> bytes:
        if not self.sock:
            raise RuntimeError("websocket is not connected")
        chunk = self.sock.recv(size)
        if not chunk:
            raise RuntimeError("websocket connection closed")
        return chunk

    def _send_frame(self, opcode: int, payload: bytes) -> None:
        if not self.sock:
            return
        mask_key = os.urandom(4)
        first = 0x80 | opcode
        length = len(payload)
        if length <= 125:
            header = bytes([first, 0x80 | length])
        elif length <= 0xFFFF:
            header = bytes([first, 0x80 | 126]) + struct.pack("!H", length)
        else:
            header = bytes([first, 0x80 | 127]) + struct.pack("!Q", length)
        masked = bytes(byte ^ mask_key[index % 4] for index, byte in enumerate(payload))
        self.sock.sendall(header + mask_key + masked)


def require_dict(data: Any, name: str) -> dict[str, Any]:
    if not isinstance(data, dict):
        raise RuntimeError(f"{name} did not return a JSON object")
    return data


def expect_status(name: str, status: int, expected: int = 200) -> None:
    if status != expected:
        raise RuntimeError(f"{name} returned HTTP {status}, expected {expected}")


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


def websocket_url_from_backend(base_url: str) -> str:
    parts = urllib.parse.urlsplit(base_url)
    scheme = "wss" if parts.scheme == "https" else "ws"
    return urllib.parse.urlunsplit((scheme, parts.netloc, "/ws/monitor", "", ""))


def websocket_accept(key: str) -> str:
    digest = hashlib.sha1((key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").encode("ascii")).digest()
    return base64.b64encode(digest).decode("ascii")


def cookie_header(cookie_jar: CookieJar) -> str:
    return "; ".join(f"{cookie.name}={cookie.value}" for cookie in cookie_jar)


def collect_required_websocket_events(
    ws_client: SmokeWebSocket,
    session_id: str,
    event_types: set[str],
    *,
    timeout: float = 8,
) -> dict[str, dict[str, Any]]:
    pending = set(event_types)
    collected: dict[str, dict[str, Any]] = {}
    deadline = time.monotonic() + timeout
    while pending:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            break
        try:
            event = ws_client.read_json_matching(
                lambda item: item.get("type") in pending
                and isinstance(item.get("data"), dict)
                and item["data"].get("sessionId") == session_id,
                timeout=remaining,
            )
        except RuntimeError as exc:
            if "matching event" not in str(exc):
                raise
            break
        event_type = str(event.get("type") or "")
        if not event_type:
            continue
        collected[event_type] = event
        pending.discard(event_type)
    if pending:
        raise RuntimeError(f"websocket did not receive required runtime events: {', '.join(sorted(pending))}")
    return collected


def run_check(name: str, func) -> bool:
    try:
        func()
    except (urllib.error.URLError, urllib.error.HTTPError, OSError, TimeoutError, RuntimeError, json.JSONDecodeError) as exc:
        print(f"[FAIL] {name}: {exc}")
        return False
    print(f"[ OK ] {name}")
    return True


def main() -> int:
    client = SmokeClient()
    battle_session_id = ""
    ws_client: SmokeWebSocket | None = None

    def connect_websocket() -> None:
        nonlocal ws_client
        client.require_auth_cookie()
        header = cookie_header(client.cookie_jar)
        if not header:
            raise RuntimeError("login cookie is required for websocket smoke check")
        ws_url = WS_BASE_URL or websocket_url_from_backend(BACKEND_BASE_URL)
        ws_client = SmokeWebSocket(ws_url, header)
        ws_client.connect()

    def create_battle_session() -> None:
        nonlocal battle_session_id
        status, data = client.request(
            "POST",
            f"{BACKEND_BASE_URL}/api/v1/battle/sessions",
            {"scenarioCode": "open-water-duel"},
        )
        expect_status("create battle session", status, 201)
        body = require_dict(data, "create battle session")
        session = require_dict(body.get("session"), "create battle session.session")
        battle_session_id = str(session.get("sessionId") or "")
        if not battle_session_id:
            raise RuntimeError("create battle session did not return sessionId")

    def post_radar_report() -> None:
        if not battle_session_id:
            raise RuntimeError("battle session was not created")
        status, _ = client.request(
            "POST",
            f"{BACKEND_BASE_URL}/api/v1/radar/reports",
            {
                "sessionId": battle_session_id,
                "radarId": "SMOKE-RADAR-01",
                "targets": [
                    {
                        "targetId": "red-1",
                        "side": "red",
                        "longitude": 121.51,
                        "latitude": 31.24,
                        "course": 270,
                        "speedKnots": 20.5,
                        "confidence": 0.9,
                        "detected": True,
                    }
                ],
                "state": {
                    "sessionId": battle_session_id,
                    "status": "running",
                    "units": [
                        {
                            "unitId": "blue-1",
                            "shipId": 1,
                            "name": "Blue Smoke Unit",
                            "side": "blue",
                            "hp": 100,
                            "maxHp": 100,
                            "radarRangeKm": 32,
                            "weaponRangeKm": 20,
                            "cooldownSeconds": 5,
                            "longitude": 121.49,
                            "latitude": 31.23,
                            "course": 90,
                            "speedKnots": 18,
                            "status": "active",
                        },
                        {
                            "unitId": "red-1",
                            "shipId": 100,
                            "name": "Red Smoke Unit",
                            "side": "red",
                            "hp": 82,
                            "maxHp": 82,
                            "radarRangeKm": 28,
                            "weaponRangeKm": 18,
                            "cooldownSeconds": 4,
                            "longitude": 121.51,
                            "latitude": 31.24,
                            "course": 270,
                            "speedKnots": 20.5,
                            "status": "active",
                        },
                    ],
                    "projectiles": [
                        {
                            "projectileId": f"{battle_session_id}-smoke-p1",
                            "sourceUnitId": "blue-1",
                            "targetUnitId": "red-1",
                            "side": "blue",
                            "longitude": 121.5,
                            "latitude": 31.235,
                            "speedKmH": 1600,
                            "status": "flying",
                        }
                    ],
                    "events": [
                        {
                            "eventId": f"{battle_session_id}-smoke-e1",
                            "type": "WEAPON_FIRED",
                            "severity": "INFO",
                            "message": "Smoke check radar report accepted.",
                            "sourceUnitId": "blue-1",
                            "targetUnitId": "red-1",
                            "longitude": 121.49,
                            "latitude": 31.23,
                        }
                    ],
                },
            },
        )
        expect_status("post radar report", status, 202)

    def websocket_radar_event() -> None:
        if not ws_client:
            raise RuntimeError("websocket was not connected")
        if not battle_session_id:
            raise RuntimeError("battle session was not created")
        events = collect_required_websocket_events(ws_client, battle_session_id, REQUIRED_BATTLE_WS_EVENTS)

        radar_scan = require_dict(events["radar_scan_updated"].get("data"), "websocket radar_scan_updated.data")
        if not radar_scan.get("targets"):
            raise RuntimeError("websocket radar_scan_updated did not include targets")

        projectile = require_dict(events["projectile_updated"].get("data"), "websocket projectile_updated.data")
        if not projectile.get("projectileId"):
            raise RuntimeError("websocket projectile_updated did not include projectileId")

        battle_event = require_dict(events["battle_event_created"].get("data"), "websocket battle_event_created.data")
        if not battle_event.get("eventId"):
            raise RuntimeError("websocket battle_event_created did not include eventId")

        battle_state = require_dict(events["battle_state_updated"].get("data"), "websocket battle_state_updated.data")
        if not battle_state.get("radarTargets"):
            raise RuntimeError("websocket battle_state_updated did not include radarTargets")

    def battle_state() -> None:
        if not battle_session_id:
            raise RuntimeError("battle session was not created")
        status, data = client.request("GET", f"{BACKEND_BASE_URL}/api/v1/battle/sessions/{battle_session_id}/state")
        expect_status("battle state", status)
        body = require_dict(data, "battle state")
        if not body.get("units") or not body.get("radarTargets"):
            raise RuntimeError("battle state did not include units and radarTargets")

    def battle_timeline() -> None:
        if not battle_session_id:
            raise RuntimeError("battle session was not created")
        status, data = client.request("GET", f"{BACKEND_BASE_URL}/api/v1/battle/sessions/{battle_session_id}/timeline")
        expect_status("battle timeline", status)
        body = require_dict(data, "battle timeline")
        if not body.get("items"):
            raise RuntimeError("battle timeline did not include items")

    def battle_snapshots() -> None:
        if not battle_session_id:
            raise RuntimeError("battle session was not created")
        status, data = client.request("GET", f"{BACKEND_BASE_URL}/api/v1/battle/sessions/{battle_session_id}/snapshots")
        expect_status("battle snapshots", status)
        body = require_dict(data, "battle snapshots")
        if not body.get("items"):
            raise RuntimeError("battle snapshots did not include items")

    def battle_report() -> None:
        if not battle_session_id:
            raise RuntimeError("battle session was not created")
        status, data = client.request("GET", f"{BACKEND_BASE_URL}/api/v1/battle/sessions/{battle_session_id}/report")
        expect_status("battle report", status)
        body = require_dict(data, "battle report")
        if "firedCount" not in body:
            raise RuntimeError("battle report did not include firedCount")

    checks = [
        ("backend health", lambda: expect_status("backend health", client.request("GET", f"{BACKEND_BASE_URL}/health")[0])),
        ("backend readiness", lambda: expect_status("backend readiness", client.request("GET", f"{BACKEND_BASE_URL}/ready")[0])),
        ("analytics health", lambda: expect_status("analytics health", client.request("GET", f"{ANALYTICS_BASE_URL}/health")[0])),
        (
            "frontend index",
            lambda: expect_status(
                "frontend index",
                client.request("GET", FRONTEND_BASE_URL + "/", require_request_id=False, accept="text/html,*/*")[0],
            ),
        ),
        ("login", client.login),
        ("ships list", lambda: expect_status("ships list", client.request("GET", f"{BACKEND_BASE_URL}/api/v1/ships")[0])),
        ("alarms list", lambda: expect_status("alarms list", client.request("GET", f"{BACKEND_BASE_URL}/api/v1/alarms")[0])),
        (
            "battle scenarios",
            lambda: expect_status("battle scenarios", client.request("GET", f"{BACKEND_BASE_URL}/api/v1/battle/scenarios")[0]),
        ),
        (
            "analytics proxy status",
            lambda: expect_status(
                "analytics proxy status",
                client.request("GET", f"{BACKEND_BASE_URL}/api/v1/analytics/simulate/status")[0],
            ),
        ),
        ("create battle session", create_battle_session),
        ("websocket connect", connect_websocket),
        ("post radar report", post_radar_report),
        ("websocket radar event", websocket_radar_event),
        ("battle state", battle_state),
        ("battle timeline", battle_timeline),
        ("battle snapshots", battle_snapshots),
        ("battle report", battle_report),
        ("logout clears auth cookie", client.logout),
    ]

    passed = 0
    try:
        for name, check in checks:
            if run_check(name, check):
                passed += 1
    finally:
        if ws_client:
            ws_client.close()

    total = len(checks)
    print(f"{passed}/{total} smoke checks passed")
    return 0 if passed == total else 1


if __name__ == "__main__":
    sys.exit(main())
