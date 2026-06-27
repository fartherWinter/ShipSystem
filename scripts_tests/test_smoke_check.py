import ast
import importlib.util
import re
import sys
import time
import unittest
from email.message import Message
from http.cookiejar import Cookie
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "smoke_check.py"
SPEC = importlib.util.spec_from_file_location("smoke_check_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class FakeResponse:
    def __init__(self, body: bytes, content_type: str) -> None:
        self._body = body
        self.headers = {"Content-Type": content_type}

    def read(self) -> bytes:
        return self._body


class FakeWebSocket:
    def __init__(self, events: list[dict]) -> None:
        self._events = list(events)

    def read_json_matching(self, predicate, timeout: float = 8) -> dict:
        while self._events:
            event = self._events.pop(0)
            if predicate(event):
                return event
        raise RuntimeError(f"no matching event within {timeout:.1f}s")


def make_cookie(name: str, value: str, *, path: str = "/", httponly: bool = True) -> Cookie:
    rest = {"HttpOnly": None} if httponly else {}
    return Cookie(
        version=0,
        name=name,
        value=value,
        port=None,
        port_specified=False,
        domain="localhost",
        domain_specified=False,
        domain_initial_dot=False,
        path=path,
        path_specified=True,
        secure=False,
        expires=int(time.time()) + 3600,
        discard=False,
        comment=None,
        comment_url=None,
        rest=rest,
        rfc2109=False,
    )


class SmokeCheckScriptTest(unittest.TestCase):
    def test_smoke_main_still_declares_required_check_sequence(self) -> None:
        source = MODULE_PATH.read_text(encoding="utf-8")
        tree = ast.parse(source, filename=str(MODULE_PATH))
        check_names: list[str] = []
        for node in ast.walk(tree):
            if not isinstance(node, ast.Assign):
                continue
            if len(node.targets) != 1 or not isinstance(node.targets[0], ast.Name) or node.targets[0].id != "checks":
                continue
            self.assertIsInstance(node.value, ast.List, "smoke_check.py checks must be a list literal")
            for element in node.value.elts:
                self.assertIsInstance(element, ast.Tuple, "each smoke check must be a tuple")
                self.assertGreaterEqual(len(element.elts), 2, "each smoke check tuple must contain name and callable")
                name = element.elts[0]
                self.assertIsInstance(name, ast.Constant, "smoke check name must be a string literal")
                self.assertIsInstance(name.value, str, "smoke check name must be a string literal")
                check_names.append(name.value)
            break
        self.assertTrue(check_names, "smoke_check.py must declare checks = [...]")

        expected = [
            "backend health",
            "backend readiness",
            "analytics health",
            "frontend index",
            "login",
            "ships list",
            "alarms list",
            "battle scenarios",
            "analytics proxy status",
            "create battle session",
            "websocket connect",
            "post radar report",
            "websocket radar event",
            "battle state",
            "battle timeline",
            "battle snapshots",
            "battle report",
            "logout clears auth cookie",
        ]
        self.assertEqual(expected, check_names)

    def test_smoke_script_still_targets_required_runtime_endpoints(self) -> None:
        source = MODULE_PATH.read_text(encoding="utf-8")

        for expected in [
            "/health",
            "/ready",
            "/api/v1/auth/login",
            "/api/v1/auth/logout",
            "/api/v1/ships",
            "/api/v1/alarms",
            "/api/v1/battle/scenarios",
            "/api/v1/battle/sessions",
            "/api/v1/radar/reports",
            "/api/v1/analytics/simulate/status",
            "/state",
            "/timeline",
            "/snapshots",
            "/report",
        ]:
            self.assertIn(expected, source)
        self.assertIn('accept="text/html,*/*"', source)

    def test_websocket_url_and_accept_helpers(self) -> None:
        self.assertEqual("ws://localhost:8080/ws/monitor", MODULE.websocket_url_from_backend("http://localhost:8080"))
        self.assertEqual("wss://example.com/ws/monitor", MODULE.websocket_url_from_backend("https://example.com/api"))
        self.assertEqual(
            "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=",
            MODULE.websocket_accept("dGhlIHNhbXBsZSBub25jZQ=="),
        )

    def test_decode_response_body_handles_json_text_and_empty_payload(self) -> None:
        self.assertEqual({"ok": True}, MODULE.decode_response_body(FakeResponse(b'{"ok": true}', "application/json")))
        self.assertEqual("plain text", MODULE.decode_response_body(FakeResponse(b"plain text", "text/plain")))
        self.assertIsNone(MODULE.decode_response_body(FakeResponse(b"", "application/json")))

    def test_summarize_body_truncates_and_formats_dicts(self) -> None:
        self.assertEqual('{"message":"ok"}', MODULE.summarize_body({"message": "ok"}))
        self.assertEqual("", MODULE.summarize_body(None))
        summary = MODULE.summarize_body("x" * 400)
        self.assertEqual(303, len(summary))
        self.assertTrue(summary.endswith("..."))

    def test_cookie_header_joins_cookie_values(self) -> None:
        cookie_jar = MODULE.CookieJar()
        cookie_jar.set_cookie(make_cookie("shipsystem_token", "abc"))
        cookie_jar.set_cookie(make_cookie("session", "xyz"))

        header = MODULE.cookie_header(cookie_jar)

        self.assertEqual({"shipsystem_token=abc", "session=xyz"}, set(header.split("; ")))

    def test_capture_response_headers_and_require_auth_cookie(self) -> None:
        client = MODULE.SmokeClient()
        headers = Message()
        headers.add_header("Set-Cookie", "shipsystem_token=abc; Path=/; HttpOnly")
        headers.add_header("Set-Cookie", "session=xyz; Path=/")
        client.capture_response_headers(headers)

        self.assertEqual(
            ["shipsystem_token=abc; Path=/; HttpOnly", "session=xyz; Path=/"],
            client.response_headers("Set-Cookie"),
        )

        client.cookie_jar.set_cookie(make_cookie(MODULE.AUTH_COOKIE_NAME, "abc"))
        client.require_auth_cookie()

    def test_require_auth_cookie_rejects_missing_flags(self) -> None:
        client = MODULE.SmokeClient()
        client.cookie_jar.set_cookie(make_cookie(MODULE.AUTH_COOKIE_NAME, "abc", path="/api"))
        with self.assertRaises(RuntimeError) as wrong_path:
            client.require_auth_cookie()
        self.assertIn("cookie path must be /", str(wrong_path.exception))

        client = MODULE.SmokeClient()
        client.cookie_jar.set_cookie(make_cookie(MODULE.AUTH_COOKIE_NAME, "abc", httponly=False))
        with self.assertRaises(RuntimeError) as missing_flag:
            client.require_auth_cookie()
        self.assertIn("cookie must be HttpOnly", str(missing_flag.exception))

    def test_expect_status_and_require_dict_raise_with_clear_messages(self) -> None:
        MODULE.expect_status("health", 200)
        self.assertEqual({"ok": True}, MODULE.require_dict({"ok": True}, "payload"))

        with self.assertRaises(RuntimeError) as bad_status:
            MODULE.expect_status("health", 500)
        self.assertIn("health returned HTTP 500, expected 200", str(bad_status.exception))

        with self.assertRaises(RuntimeError) as bad_dict:
            MODULE.require_dict([], "payload")
        self.assertIn("payload did not return a JSON object", str(bad_dict.exception))

    def test_smoke_http_error_formats_request_id_and_body_summary(self) -> None:
        err = MODULE.SmokeHTTPError(
            "GET",
            "http://localhost:8080/api/v1/ships",
            500,
            "req-123",
            "failed",
            {"message": "bad", "requestId": "req-123"},
        )

        text = str(err)
        self.assertIn("GET http://localhost:8080/api/v1/ships returned HTTP 500", text)
        self.assertIn("message=failed", text)
        self.assertIn("requestId=req-123", text)
        self.assertIn('body={"message":"bad","requestId":"req-123"}', text)

    def test_collect_required_websocket_events_waits_for_full_event_set(self) -> None:
        session_id = "battle-test"
        ws_client = FakeWebSocket(
            [
                {"type": "heartbeat", "data": {"time": "2026-06-10T12:00:00Z"}},
                {"type": "radar_scan_updated", "data": {"sessionId": session_id, "targets": [1]}},
                {"type": "projectile_updated", "data": {"sessionId": session_id, "projectileId": "p-1"}},
                {"type": "battle_event_created", "data": {"sessionId": session_id, "eventId": "e-1"}},
                {"type": "battle_state_updated", "data": {"sessionId": session_id, "radarTargets": [1]}},
            ]
        )

        events = MODULE.collect_required_websocket_events(
            ws_client,
            session_id,
            set(MODULE.REQUIRED_BATTLE_WS_EVENTS),
            timeout=1,
        )

        self.assertEqual(set(MODULE.REQUIRED_BATTLE_WS_EVENTS), set(events))

    def test_collect_required_websocket_events_reports_missing_types(self) -> None:
        ws_client = FakeWebSocket(
            [
                {"type": "radar_scan_updated", "data": {"sessionId": "battle-test", "targets": [1]}},
            ]
        )

        with self.assertRaises(RuntimeError) as ctx:
            MODULE.collect_required_websocket_events(
                ws_client,
                "battle-test",
                set(MODULE.REQUIRED_BATTLE_WS_EVENTS),
                timeout=0.1,
            )

        text = str(ctx.exception)
        self.assertIn("battle_event_created", text)
        self.assertIn("battle_state_updated", text)
        self.assertIn("projectile_updated", text)


if __name__ == "__main__":
    unittest.main()
