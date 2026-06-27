import importlib.util
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts" / "run_runtime_observability_snapshot.py"
SPEC = importlib.util.spec_from_file_location("run_runtime_observability_snapshot_for_tests", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class RuntimeObservabilitySnapshotTest(unittest.TestCase):
    def test_decode_body_handles_json_text_and_empty(self) -> None:
        self.assertEqual({"ok": True}, MODULE.decode_body(b'{"ok": true}', "application/json"))
        self.assertEqual("plain text", MODULE.decode_body(b"plain text", "text/plain"))
        self.assertIsNone(MODULE.decode_body(b"", "application/json"))

    def test_validate_snapshot_accepts_expected_payload(self) -> None:
        snapshot = {
            "backendHealth": {"ok": True, "status": 200, "requestId": "a", "expectedRequestId": "a", "latencyMs": 10, "body": {"status": "ok"}},
            "backendReady": {"ok": True, "status": 200, "requestId": "b", "expectedRequestId": "b", "latencyMs": 12, "body": {"status": "ready"}},
            "analyticsHealth": {"ok": True, "status": 200, "requestId": "c", "expectedRequestId": "c", "latencyMs": 14, "body": {"status": "ok"}},
            "frontendIndex": {"ok": True, "status": 200, "requestId": "d", "expectedRequestId": "d", "latencyMs": 16, "body": "<html></html>"},
            "analyticsProxyStatus": {
                "ok": True,
                "status": 200,
                "requestId": "e",
                "expectedRequestId": "e",
                "latencyMs": 18,
                "body": {
                    "running": False,
                    "delivery": {
                        "successCount": 1,
                        "failureCount": 0,
                        "retryCount": 0,
                        "droppedCount": 0,
                    },
                },
            },
        }

        MODULE.validate_snapshot(snapshot)

    def test_validate_snapshot_rejects_missing_delivery_metrics(self) -> None:
        snapshot = {
            "backendHealth": {"ok": True, "status": 200, "requestId": "a", "expectedRequestId": "a", "latencyMs": 1, "body": {"status": "ok"}},
            "backendReady": {"ok": True, "status": 200, "requestId": "b", "expectedRequestId": "b", "latencyMs": 1, "body": {"status": "ready"}},
            "analyticsHealth": {"ok": True, "status": 200, "requestId": "c", "expectedRequestId": "c", "latencyMs": 1, "body": {"status": "ok"}},
            "frontendIndex": {"ok": True, "status": 200, "requestId": "d", "expectedRequestId": "d", "latencyMs": 1, "body": "<html></html>"},
            "analyticsProxyStatus": {"ok": True, "status": 200, "requestId": "e", "expectedRequestId": "e", "latencyMs": 1, "body": {"running": False}},
        }

        with self.assertRaises(SystemExit) as ctx:
            MODULE.validate_snapshot(snapshot)

        self.assertIn("delivery metrics", str(ctx.exception))

    def test_validate_snapshot_allows_frontend_without_request_id_echo(self) -> None:
        snapshot = {
            "backendHealth": {"ok": True, "status": 200, "requestId": "a", "expectedRequestId": "a", "latencyMs": 1, "body": {"status": "ok"}},
            "backendReady": {"ok": True, "status": 200, "requestId": "b", "expectedRequestId": "b", "latencyMs": 1, "body": {"status": "ready"}},
            "analyticsHealth": {"ok": True, "status": 200, "requestId": "c", "expectedRequestId": "c", "latencyMs": 1, "body": {"status": "ok"}},
            "frontendIndex": {"ok": True, "status": 200, "requestId": "", "expectedRequestId": "frontend-trace", "latencyMs": 1, "body": "<html></html>"},
            "analyticsProxyStatus": {
                "ok": True,
                "status": 200,
                "requestId": "e",
                "expectedRequestId": "e",
                "latencyMs": 1,
                "body": {
                    "running": False,
                    "delivery": {
                        "successCount": 0,
                        "failureCount": 0,
                        "retryCount": 0,
                        "droppedCount": 0,
                    },
                },
            },
        }

        MODULE.validate_snapshot(snapshot)

    def test_summarize_result_is_compact_json(self) -> None:
        text = MODULE.summarize_result({"status": 200, "requestId": "trace", "expectedRequestId": "trace", "latencyMs": 12.3, "body": {"ok": True}})

        self.assertIn('"status":200', text)
        self.assertIn('"requestId":"trace"', text)


if __name__ == "__main__":
    unittest.main()
