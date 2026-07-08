import asyncio
import logging
import unittest
from unittest import mock

import httpx
from fastapi import FastAPI, Request
from fastapi.testclient import TestClient

from app import main


class AnalyticsAPITest(unittest.TestCase):
    def setUp(self) -> None:
        self.original_token = main.ANALYTICS_ADMIN_TOKEN
        main.ANALYTICS_ADMIN_TOKEN = "test-token"
        self.original_retry_attempts = main.HTTP_RETRY_ATTEMPTS
        self.original_http_timeout_seconds = main.HTTP_TIMEOUT_SECONDS
        self.original_http_retry_backoff_seconds = main.HTTP_RETRY_BACKOFF_SECONDS
        self.original_http_retry_backoff_max_seconds = main.HTTP_RETRY_BACKOFF_MAX_SECONDS
        main.delivery_metrics = main.DeliveryMetrics()
        self.client = TestClient(main.app)

    def tearDown(self) -> None:
        main.ANALYTICS_ADMIN_TOKEN = self.original_token
        main.HTTP_RETRY_ATTEMPTS = self.original_retry_attempts
        main.HTTP_TIMEOUT_SECONDS = self.original_http_timeout_seconds
        main.HTTP_RETRY_BACKOFF_SECONDS = self.original_http_retry_backoff_seconds
        main.HTTP_RETRY_BACKOFF_MAX_SECONDS = self.original_http_retry_backoff_max_seconds

    def test_simulation_status_requires_admin_token(self) -> None:
        response = self.client.get("/simulate/status", headers={"X-Request-ID": "trace-unauthorized"})

        self.assertEqual(response.status_code, 401)
        self.assertEqual(response.headers.get("X-Request-ID"), "trace-unauthorized")
        body = response.json()
        self.assertEqual(body["requestId"], "trace-unauthorized")
        self.assertEqual(body["message"], "analytics admin token is required")

    def test_simulation_status_accepts_analytics_token_header(self) -> None:
        response = self.client.get(
            "/simulate/status",
            headers={"X-Analytics-Token": "test-token", "X-Request-ID": "trace-status"},
        )

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.headers.get("X-Request-ID"), "trace-status")
        body = response.json()
        self.assertIn("running", body)
        self.assertIn("battleSessions", body)
        self.assertIn("delivery", body)

    def test_request_id_middleware_generates_missing_id(self) -> None:
        response = self.client.get("/health")

        self.assertEqual(response.status_code, 200)
        request_id = response.headers.get("X-Request-ID")
        self.assertIsNotNone(request_id)
        self.assertEqual(len(request_id), 32)

    def test_request_id_middleware_replaces_invalid_id(self) -> None:
        response = self.client.get("/health", headers={"X-Request-ID": "bad id"})

        self.assertEqual(response.status_code, 200)
        request_id = response.headers.get("X-Request-ID")
        self.assertIsNotNone(request_id)
        self.assertNotEqual(request_id, "bad id")
        self.assertEqual(len(request_id), 32)

    def test_unhandled_exception_response_includes_request_id(self) -> None:
        app = FastAPI()
        app.middleware("http")(main.request_id_middleware)
        app.exception_handler(Exception)(main.unhandled_exception_handler)

        @app.get("/boom")
        async def boom(request: Request) -> None:
            raise RuntimeError("boom")

        client = TestClient(app, raise_server_exceptions=False)
        original_logger = main.logger
        try:
            main.logger = logging.getLogger("shipsystem.analytics.test")
            main.logger.addHandler(logging.NullHandler())
            main.logger.propagate = False
            response = client.get("/boom", headers={"X-Request-ID": "trace-boom"})
        finally:
            main.logger = original_logger

        self.assertEqual(response.status_code, 500)
        self.assertEqual(response.headers.get("X-Request-ID"), "trace-boom")
        body = response.json()
        self.assertEqual(body["message"], "internal server error")
        self.assertEqual(body["requestId"], "trace-boom")

    def test_admin_token_can_use_authorization_bearer(self) -> None:
        response = self.client.get("/simulate/status", headers={"Authorization": "Bearer test-token"})

        self.assertEqual(response.status_code, 200)

    def test_analyze_location_rejects_invalid_longitude(self) -> None:
        response = self.client.post(
            "/analyze/location",
            json={
                "shipId": 1,
                "longitude": 181,
                "latitude": 31.23,
                "speedKnots": 10,
                "course": 90,
            },
        )

        self.assertEqual(response.status_code, 422)
        body = response.json()
        self.assertEqual(body["message"], "request validation failed")
        self.assertIn("requestId", body)
        self.assertIn("detail", body)

    def test_analyze_location_rejects_negative_speed(self) -> None:
        response = self.client.post(
            "/analyze/location",
            json={
                "shipId": 1,
                "longitude": 121.49,
                "latitude": 31.23,
                "speedKnots": -0.1,
                "course": 90,
            },
        )

        self.assertEqual(response.status_code, 422)

    def test_start_simulation_rejects_empty_ship_ids(self) -> None:
        response = self.client.post(
            "/simulate/start",
            headers={"X-Analytics-Token": "test-token"},
            json={"shipIds": []},
        )

        self.assertEqual(response.status_code, 422)

    def test_start_simulation_rejects_nonpositive_ship_id(self) -> None:
        response = self.client.post(
            "/simulate/start",
            headers={"X-Analytics-Token": "test-token"},
            json={"shipIds": [1, 0]},
        )

        self.assertEqual(response.status_code, 422)

    def test_start_battle_rejects_blank_session_id(self) -> None:
        response = self.client.post(
            "/simulate/battle/start",
            headers={"X-Analytics-Token": "test-token"},
            json={
                "sessionId": "   ",
                "scenarioCode": "open-water-duel",
                "originLongitude": 121.49,
                "originLatitude": 31.23,
            },
        )

        self.assertEqual(response.status_code, 422)

    def test_start_battle_rejects_unknown_scenario_code(self) -> None:
        response = self.client.post(
            "/simulate/battle/start",
            headers={"X-Analytics-Token": "test-token"},
            json={
                "sessionId": "battle-test",
                "scenarioCode": "missing-scenario",
                "originLongitude": 121.49,
                "originLatitude": 31.23,
            },
        )

        self.assertEqual(response.status_code, 422)

    def test_start_battle_rejects_invalid_origin_latitude(self) -> None:
        response = self.client.post(
            "/simulate/battle/start",
            headers={"X-Analytics-Token": "test-token"},
            json={
                "sessionId": "battle-test",
                "scenarioCode": "open-water-duel",
                "originLongitude": 121.49,
                "originLatitude": 91,
            },
        )

        self.assertEqual(response.status_code, 422)

    def test_stop_battle_rejects_blank_session_id(self) -> None:
        response = self.client.post(
            "/simulate/battle/stop",
            headers={"X-Analytics-Token": "test-token"},
            json={"sessionId": "   "},
        )

        self.assertEqual(response.status_code, 422)

    def test_task_running_handles_none(self) -> None:
        self.assertFalse(main.task_running(None))

    def test_callback_headers_forward_request_id_and_service_token(self) -> None:
        original_go_token = main.GO_API_TOKEN
        try:
            main.GO_API_TOKEN = "go-callback-token"

            headers = main.callback_headers("Bearer browser-token", "trace-callback")
        finally:
            main.GO_API_TOKEN = original_go_token

        self.assertEqual(headers["Authorization"], "Bearer go-callback-token")
        self.assertEqual(headers["X-Request-ID"], "trace-callback")

    def test_callback_headers_falls_back_to_browser_authorization_when_go_token_missing(self) -> None:
        original_go_token = main.GO_API_TOKEN
        try:
            main.GO_API_TOKEN = ""
            headers = main.callback_headers("Bearer browser-token", "trace-browser")
        finally:
            main.GO_API_TOKEN = original_go_token

        self.assertEqual(headers["Authorization"], "Bearer browser-token")
        self.assertEqual(headers["X-Request-ID"], "trace-browser")

    def test_delivery_metrics_records_success_and_status_exposes_counts(self) -> None:
        main.delivery_metrics.record(main.DeliveryResult(success=True, attempts=2, status_code=201))

        response = self.client.get("/simulate/status", headers={"X-Analytics-Token": "test-token"})

        self.assertEqual(response.status_code, 200)
        delivery = response.json()["delivery"]
        self.assertEqual(delivery["successCount"], 1)
        self.assertEqual(delivery["retryCount"], 1)
        self.assertEqual(delivery["failureCount"], 0)
        self.assertIsNotNone(delivery["lastSuccessAt"])

    def test_post_with_retry_does_not_retry_non_retryable_status(self) -> None:
        async def run() -> main.DeliveryResult:
            calls = 0

            async def handler(request: httpx.Request) -> httpx.Response:
                nonlocal calls
                calls += 1
                return httpx.Response(400, text="bad payload")

            async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
                main.HTTP_RETRY_ATTEMPTS = 3
                result = await main.post_with_retry(
                    client,
                    "http://test.local/locations",
                    json={"longitude": 121.49},
                    headers={},
                    context="test 400",
                )
            self.assertEqual(calls, 1)
            return result

        result = asyncio.run(run())

        self.assertFalse(result.success)
        self.assertEqual(result.attempts, 1)
        self.assertEqual(result.status_code, 400)
        self.assertIn("bad payload", result.error)

    def test_post_with_retry_retries_retryable_status_and_metrics_record_drop(self) -> None:
        async def run() -> main.DeliveryResult:
            calls = 0

            async def handler(request: httpx.Request) -> httpx.Response:
                nonlocal calls
                calls += 1
                return httpx.Response(503, text="temporarily unavailable")

            async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
                main.HTTP_RETRY_ATTEMPTS = 3
                result = await main.post_with_retry(
                    client,
                    "http://test.local/radar/reports",
                    json={"sessionId": "battle-test"},
                    headers={},
                    context="test 503",
                )
            self.assertEqual(calls, 3)
            return result

        result = asyncio.run(run())
        main.delivery_metrics.record(result)

        self.assertFalse(result.success)
        self.assertEqual(result.attempts, 3)
        self.assertEqual(result.status_code, 503)
        self.assertEqual(main.delivery_metrics.failure_count, 1)
        self.assertEqual(main.delivery_metrics.retry_count, 2)
        self.assertEqual(main.delivery_metrics.dropped_count, 1)

    def test_post_with_retry_uses_configured_backoff_sequence(self) -> None:
        async def run() -> tuple[main.DeliveryResult, list[float]]:
            calls = 0
            delays: list[float] = []

            async def handler(request: httpx.Request) -> httpx.Response:
                nonlocal calls
                calls += 1
                return httpx.Response(503, text="temporarily unavailable")

            async def fake_sleep(delay: float) -> None:
                delays.append(delay)

            async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
                main.HTTP_RETRY_ATTEMPTS = 4
                main.HTTP_RETRY_BACKOFF_SECONDS = 0.5
                main.HTTP_RETRY_BACKOFF_MAX_SECONDS = 0.9
                with mock.patch("app.main.asyncio.sleep", new=fake_sleep):
                    result = await main.post_with_retry(
                        client,
                        "http://test.local/radar/reports",
                        json={"sessionId": "battle-test"},
                        headers={},
                        context="test backoff",
                    )
            self.assertEqual(calls, 4)
            return result, delays

        result, delays = asyncio.run(run())

        self.assertFalse(result.success)
        self.assertEqual(result.attempts, 4)
        self.assertEqual(delays, [0.5, 0.9, 0.9])

    def test_post_with_retry_retries_http_error_and_succeeds(self) -> None:
        async def run() -> main.DeliveryResult:
            calls = 0

            async def handler(request: httpx.Request) -> httpx.Response:
                nonlocal calls
                calls += 1
                if calls < 3:
                    raise httpx.ReadTimeout("temporary timeout", request=request)
                return httpx.Response(202, json={"accepted": True})

            async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
                main.HTTP_RETRY_ATTEMPTS = 4
                result = await main.post_with_retry(
                    client,
                    "http://test.local/radar/reports",
                    json={"sessionId": "battle-test"},
                    headers={"X-Request-ID": "trace-retry"},
                    context="test timeout recovery",
                )
            self.assertEqual(calls, 3)
            return result

        result = asyncio.run(run())

        self.assertTrue(result.success)
        self.assertEqual(result.attempts, 3)
        self.assertEqual(result.status_code, 202)
        self.assertEqual(result.error, "")

    def test_callback_http_client_uses_configured_timeout(self) -> None:
        main.HTTP_TIMEOUT_SECONDS = 7.5

        client = main.callback_http_client()
        try:
            self.assertEqual(client.timeout.connect, 7.5)
            self.assertEqual(client.timeout.read, 7.5)
            self.assertEqual(client.timeout.write, 7.5)
            self.assertEqual(client.timeout.pool, 7.5)
        finally:
            asyncio.run(client.aclose())

    def test_retry_backoff_seconds_caps_at_max(self) -> None:
        main.HTTP_RETRY_BACKOFF_SECONDS = 0.4
        main.HTTP_RETRY_BACKOFF_MAX_SECONDS = 1.0

        self.assertEqual(main.retry_backoff_seconds(1), 0.4)
        self.assertEqual(main.retry_backoff_seconds(2), 0.8)
        self.assertEqual(main.retry_backoff_seconds(3), 1.0)

    def test_response_error_summary_truncates_long_body(self) -> None:
        response = httpx.Response(502, text="x" * (main.MAX_ERROR_SUMMARY_LENGTH + 50))

        summary = main.response_error_summary(response)

        self.assertTrue(summary.endswith("..."))
        self.assertEqual(len(summary), main.MAX_ERROR_SUMMARY_LENGTH + 3)

    def test_run_battle_simulator_forwards_request_id_and_service_token(self) -> None:
        class FakeSimulation:
            def __init__(self) -> None:
                self.session_id = "battle-test"
                self.status = "running"
                self.calls = 0

            def step(self, interval_seconds: float) -> dict:
                self.calls += 1
                self.status = "stopped"
                return {
                    "sessionId": self.session_id,
                    "radarId": "SIM-RADAR-01",
                    "targets": [],
                    "state": {"sessionId": self.session_id, "status": "stopped", "events": []},
                }

        captured: dict[str, object] = {}
        original_go_token = main.GO_API_TOKEN
        original_post_with_retry = main.post_with_retry

        async def fake_post_with_retry(client, url, *, json, headers, context):
            captured["url"] = url
            captured["json"] = json
            captured["headers"] = headers
            captured["context"] = context
            return main.DeliveryResult(success=True, attempts=1, status_code=202)

        try:
            main.GO_API_TOKEN = "go-callback-token"
            main.post_with_retry = fake_post_with_retry

            asyncio.run(
                main.run_battle_simulator(
                    FakeSimulation(),
                    "Bearer browser-token",
                    "trace-battle",
                )
            )
        finally:
            main.GO_API_TOKEN = original_go_token
            main.post_with_retry = original_post_with_retry

        self.assertEqual(captured["url"], f"{main.GO_API_BASE}/radar/reports")
        self.assertEqual(captured["context"], "battle radar session_id=battle-test")
        self.assertEqual(captured["headers"]["Authorization"], "Bearer go-callback-token")
        self.assertEqual(captured["headers"]["X-Request-ID"], "trace-battle")
        self.assertEqual(captured["json"]["sessionId"], "battle-test")

    def test_development_runtime_config_allows_empty_tokens(self) -> None:
        main.validate_runtime_config("development", "", "")

    def test_runtime_config_rejects_negative_http_timeout(self) -> None:
        original_timeout = main.HTTP_TIMEOUT_SECONDS
        try:
            main.HTTP_TIMEOUT_SECONDS = -1
            with self.assertRaisesRegex(ValueError, "HTTP_TIMEOUT_SECONDS"):
                main.validate_runtime_config("development", "", "")
        finally:
            main.HTTP_TIMEOUT_SECONDS = original_timeout

    def test_production_runtime_config_requires_go_api_token(self) -> None:
        with self.assertRaisesRegex(RuntimeError, "GO_API_TOKEN"):
            main.validate_runtime_config("production", "strong-analytics-admin-token", "")

    def test_production_runtime_config_accepts_strong_tokens(self) -> None:
        main.validate_runtime_config(
            "production",
            "strong-analytics-admin-token",
            "strong-go-api-callback-token",
        )


if __name__ == "__main__":
    unittest.main()
