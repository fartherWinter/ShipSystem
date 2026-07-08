#!/usr/bin/env python3
"""Capture a read-only runtime observability snapshot for a running ShipSystem stack."""

from __future__ import annotations

import json
import os
import time
import urllib.error
import urllib.request
import uuid
from typing import Any


BACKEND_BASE_URL = os.getenv("SHIPSYSTEM_BACKEND_URL", "http://localhost:8080").rstrip("/")
ANALYTICS_BASE_URL = os.getenv("SHIPSYSTEM_ANALYTICS_URL", "http://localhost:8090").rstrip("/")
FRONTEND_BASE_URL = os.getenv("SHIPSYSTEM_FRONTEND_URL", "http://localhost:3000").rstrip("/")
USERNAME = os.getenv("SHIPSYSTEM_USERNAME", "admin")
PASSWORD = os.getenv("SHIPSYSTEM_PASSWORD", "Admin123!")
REQUEST_ID_HEADER = "X-Request-ID"


class SnapshotClient:
    def request(
        self,
        method: str,
        url: str,
        *,
        payload: dict[str, Any] | None = None,
        token: str = "",
        accept: str = "application/json",
    ) -> dict[str, Any]:
        body = None
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
        request_id = f"snapshot-{uuid.uuid4().hex}"
        headers = {REQUEST_ID_HEADER: request_id, "Accept": accept}
        if payload is not None:
            headers["Content-Type"] = "application/json"
        if token:
            headers["Authorization"] = f"Bearer {token}"

        request = urllib.request.Request(url, data=body, headers=headers, method=method)
        started_at = time.perf_counter()
        try:
            with urllib.request.urlopen(request, timeout=8) as response:
                elapsed_ms = round((time.perf_counter() - started_at) * 1000, 1)
                response_body = decode_body(response.read(), response.headers.get("Content-Type", ""))
                echoed_request_id = response.headers.get(REQUEST_ID_HEADER, "")
                return {
                    "ok": True,
                    "status": response.status,
                    "latencyMs": elapsed_ms,
                    "requestId": echoed_request_id,
                    "expectedRequestId": request_id,
                    "body": response_body,
                }
        except urllib.error.HTTPError as exc:
            elapsed_ms = round((time.perf_counter() - started_at) * 1000, 1)
            response_body = decode_body(exc.read(), exc.headers.get("Content-Type", ""))
            return {
                "ok": False,
                "status": exc.code,
                "latencyMs": elapsed_ms,
                "requestId": exc.headers.get(REQUEST_ID_HEADER, ""),
                "expectedRequestId": request_id,
                "body": response_body,
            }

    def login(self) -> dict[str, Any]:
        return self.request(
            "POST",
            f"{BACKEND_BASE_URL}/api/v1/auth/login",
            payload={"username": USERNAME, "password": PASSWORD},
        )


def main() -> int:
    client = SnapshotClient()
    login = client.login()
    if not login["ok"] or not isinstance(login["body"], dict) or not login["body"].get("token"):
        raise SystemExit(f"[FAIL] backend login failed for runtime snapshot: {summarize_result(login)}")
    token = str(login["body"]["token"])

    snapshot = {
        "generatedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "backendHealth": client.request("GET", f"{BACKEND_BASE_URL}/health"),
        "backendReady": client.request("GET", f"{BACKEND_BASE_URL}/ready"),
        "analyticsHealth": client.request("GET", f"{ANALYTICS_BASE_URL}/health"),
        "frontendIndex": client.request("GET", f"{FRONTEND_BASE_URL}/", accept="text/html,*/*"),
        "analyticsProxyStatus": client.request("GET", f"{BACKEND_BASE_URL}/api/v1/analytics/simulate/status", token=token),
    }
    validate_snapshot(snapshot)
    print(json.dumps(snapshot, ensure_ascii=False, indent=2))
    return 0


def decode_body(raw: bytes, content_type: str) -> Any:
    if not raw:
        return None
    text = raw.decode("utf-8", errors="replace")
    if "application/json" in content_type.lower():
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            return text
    return text


def validate_snapshot(snapshot: dict[str, Any]) -> None:
    required = (
        ("backendHealth", 200),
        ("backendReady", 200),
        ("analyticsHealth", 200),
        ("frontendIndex", 200),
        ("analyticsProxyStatus", 200),
    )
    request_id_required = {"backendHealth", "backendReady", "analyticsHealth", "analyticsProxyStatus"}
    for key, expected_status in required:
        result = snapshot[key]
        if not result["ok"] or result["status"] != expected_status:
            raise SystemExit(f"[FAIL] {key} did not return HTTP {expected_status}: {summarize_result(result)}")
        if key in request_id_required and result["requestId"] != result["expectedRequestId"]:
            raise SystemExit(
                f"[FAIL] {key} did not echo {REQUEST_ID_HEADER}: expected {result['expectedRequestId']}, got {result['requestId'] or '<missing>'}"
            )
        if result["latencyMs"] < 0:
            raise SystemExit(f"[FAIL] {key} returned invalid latency: {result['latencyMs']}")

    analytics_body = snapshot["analyticsProxyStatus"]["body"]
    if not isinstance(analytics_body, dict):
        raise SystemExit("[FAIL] analyticsProxyStatus did not return a JSON object")
    if "delivery" not in analytics_body:
        raise SystemExit("[FAIL] analyticsProxyStatus did not include delivery metrics")
    delivery = analytics_body["delivery"]
    if not isinstance(delivery, dict):
        raise SystemExit("[FAIL] analytics delivery metrics must be a JSON object")
    for key in ("successCount", "failureCount", "retryCount", "droppedCount"):
        if key not in delivery:
            raise SystemExit(f"[FAIL] analytics delivery metrics missing {key}")


def summarize_result(result: dict[str, Any]) -> str:
    return json.dumps(
        {
            "status": result.get("status"),
            "requestId": result.get("requestId"),
            "expectedRequestId": result.get("expectedRequestId"),
            "latencyMs": result.get("latencyMs"),
            "body": result.get("body"),
        },
        ensure_ascii=False,
        separators=(",", ":"),
    )


if __name__ == "__main__":
    raise SystemExit(main())
