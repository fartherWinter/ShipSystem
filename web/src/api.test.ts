import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ApiRequestError,
  clearApiToken,
  getNearestSnapshot,
  getRunReport,
  listSnapshots,
  reportCsvUrl,
  reportJsonUrl,
  setApiToken,
  toWsUrl
} from "./api";

afterEach(() => {
  vi.restoreAllMocks();
  clearApiToken();
});

describe("api client helpers", () => {
  it("builds websocket urls from http api bases", () => {
    expect(toWsUrl("http://localhost:8080", "run-1")).toBe("ws://localhost:8080/ws/runs/run-1");
    expect(toWsUrl("https://example.test/api", "run-2")).toBe("wss://example.test/api/ws/runs/run-2");
  });

  it("builds csv report urls with token query fallback", () => {
    expect(reportCsvUrl("http://localhost:8080", "run-1")).toBe("http://localhost:8080/api/runs/run-1/report?format=csv");
    expect(reportJsonUrl("http://localhost:8080", "run-1")).toBe("http://localhost:8080/api/runs/run-1/report");

    setApiToken("secret");

    expect(reportCsvUrl("https://example.test/api", "run-2")).toBe(
      "https://example.test/api/api/runs/run-2/report?format=csv&access_token=secret"
    );
    expect(reportJsonUrl("https://example.test/api", "run-2")).toBe(
      "https://example.test/api/api/runs/run-2/report?access_token=secret"
    );
  });

  it("carries structured error details", () => {
    const err = new ApiRequestError(400, "validation_failed", "validation failed", ["tick_hz must be between 1 and 60"]);

    expect(err.status).toBe(400);
    expect(err.code).toBe("validation_failed");
    expect(err.details).toHaveLength(1);
  });

  it("calls snapshot and report endpoints", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      text: async () => "[]"
    } as Response);

    await listSnapshots("run-1", 25);

    expect(fetchMock).toHaveBeenCalledWith("http://localhost:8080/api/runs/run-1/snapshots?limit=25", expect.any(Object));

    fetchMock.mockResolvedValueOnce({
      ok: true,
      text: async () => "[]"
    } as Response);
    await listSnapshots("run-1", { from: "2026-06-07T00:00:00Z", to: "2026-06-07T00:05:00Z", limit: 50 });

    expect(fetchMock).toHaveBeenLastCalledWith(
      "http://localhost:8080/api/runs/run-1/snapshots?limit=50&from=2026-06-07T00%3A00%3A00Z&to=2026-06-07T00%3A05%3A00Z",
      expect.any(Object)
    );

    fetchMock.mockResolvedValueOnce({
      ok: true,
      text: async () => "{}"
    } as Response);
    await getNearestSnapshot("run-1", "2026-06-07T00:00:00Z");

    expect(fetchMock).toHaveBeenLastCalledWith(
      "http://localhost:8080/api/runs/run-1/snapshots/nearest?at=2026-06-07T00%3A00%3A00Z",
      expect.any(Object)
    );

    fetchMock.mockResolvedValueOnce({
      ok: true,
      text: async () => "{}"
    } as Response);
    await getRunReport("run-1");

    expect(fetchMock).toHaveBeenLastCalledWith("http://localhost:8080/api/runs/run-1/report", expect.any(Object));
  });
});
