import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ApiRequestError,
  clearApiToken,
  createWebSocketTicket,
  downloadRunReport,
  getNearestSnapshot,
  getRunReport,
  listSnapshots,
  reportDownloadPath,
  reportFilename,
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

    setApiToken("secret");

    expect(toWsUrl("https://example.test/api", "run-2", "short-ticket")).toBe(
      "wss://example.test/api/ws/runs/run-2?ticket=short-ticket"
    );
    expect(toWsUrl("https://example.test/api", "run-2")).not.toContain("access_token");
  });

  it("builds report download paths without token query fallback", () => {
    expect(reportDownloadPath("run-1", "csv")).toBe("/api/runs/run-1/report?format=csv");
    expect(reportDownloadPath("run-1", "json")).toBe("/api/runs/run-1/report");
    expect(reportFilename("run-1", "csv")).toBe("run-run-1-report.csv");

    setApiToken("secret");

    expect(reportDownloadPath("run-2", "csv")).not.toContain("access_token");
  });

  it("uses headers for report downloads and websocket ticket requests", async () => {
    setApiToken("secret");
    const blob = new Blob(["summary"]);
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: true,
      blob: async () => blob,
      text: async () => ""
    } as Response);

    await expect(downloadRunReport("run-1", "csv")).resolves.toBe(blob);

    expect(fetchMock).toHaveBeenCalledWith("http://localhost:8080/api/runs/run-1/report?format=csv", expect.any(Object));
    const downloadInit = fetchMock.mock.calls[0][1] as RequestInit;
    expect(new Headers(downloadInit.headers).get("Authorization")).toBe("Bearer secret");
    expect(fetchMock.mock.calls[0][0]).not.toContain("access_token");

    fetchMock.mockResolvedValueOnce({
      ok: true,
      text: async () => JSON.stringify({ ticket: "short-ticket", expires_at: "2026-06-08T00:00:30Z" })
    } as Response);

    await expect(createWebSocketTicket("run-1")).resolves.toEqual({
      ticket: "short-ticket",
      expires_at: "2026-06-08T00:00:30Z"
    });

    expect(fetchMock).toHaveBeenLastCalledWith("http://localhost:8080/api/runs/run-1/ws-ticket", expect.any(Object));
    const ticketInit = fetchMock.mock.calls[1][1] as RequestInit;
    expect(ticketInit.method).toBe("POST");
    expect(new Headers(ticketInit.headers).get("Authorization")).toBe("Bearer secret");
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
