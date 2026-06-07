import type {
  EventPage,
  Run,
  RunReport,
  Scenario,
  ScenarioSummary,
  SimEvent,
  SnapshotFrame,
  Track,
  TrackPoint,
  TrainingAction,
  Zone
} from "./types";

export const apiBase = import.meta.env.VITE_API_BASE ?? "http://localhost:8080";
const tokenStorageKey = "ship-sim-api-token";
let apiToken = loadStoredToken();

type JsonBody = unknown;

export class ApiRequestError extends Error {
  code: string;
  details: string[];
  status: number;

  constructor(status: number, code: string, message: string, details: string[] = []) {
    super(message);
    this.name = "ApiRequestError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (apiToken && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${apiToken}`);
  }
  const res = await fetch(`${apiBase}${path}`, { ...init, headers });
  const text = await res.text();
  const body = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const error = body?.error;
    throw new ApiRequestError(
      res.status,
      error?.code ?? "request_failed",
      error?.message ?? res.statusText,
      error?.details ?? []
    );
  }
  return body as T;
}

function loadStoredToken() {
  try {
    return globalThis.localStorage?.getItem(tokenStorageKey) ?? "";
  } catch {
    return "";
  }
}

export function setApiToken(token: string) {
  apiToken = token.trim();
  try {
    if (apiToken) {
      globalThis.localStorage?.setItem(tokenStorageKey, apiToken);
    } else {
      globalThis.localStorage?.removeItem(tokenStorageKey);
    }
  } catch {
    // Ignore storage failures; the in-memory token still works for this tab.
  }
}

export function clearApiToken() {
  setApiToken("");
}

export function hasApiToken() {
  return apiToken !== "";
}

export function getApiToken() {
  return apiToken;
}

function jsonInit(method: string, body?: JsonBody): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body)
  };
}

export function listRuns(limit = 20): Promise<Run[]> {
  return apiFetch<Run[]>(`/api/runs?limit=${limit}`);
}

export type CreateRunInput = {
  name?: string;
  scenario_id?: string;
  scenario?: Scenario;
};

export function createRun(input: CreateRunInput = {}): Promise<Run> {
  return apiFetch<Run>("/api/runs", jsonInit("POST", input));
}

export function commandRun(runID: string, command: "start" | "pause" | "stop"): Promise<Run> {
  return apiFetch<Run>(`/api/runs/${runID}/${command}`, { method: "POST" });
}

export function submitTrainingAction(runID: string, type: TrainingAction): Promise<SimEvent> {
  return apiFetch<SimEvent>(`/api/runs/${runID}/actions`, jsonInit("POST", { type }));
}

export function listTracks(runID: string): Promise<Track[]> {
  return apiFetch<Track[]>(`/api/runs/${runID}/tracks`);
}

export function listZones(runID: string): Promise<Zone[]> {
  return apiFetch<Zone[]>(`/api/runs/${runID}/zones`);
}

export function listScenarios(): Promise<ScenarioSummary[]> {
  return apiFetch<ScenarioSummary[]>("/api/scenarios");
}

export function getScenario(id: string): Promise<Scenario> {
  return apiFetch<Scenario>(`/api/scenarios/${encodeURIComponent(id)}`);
}

export function listEvents(runID: string, limit = 20, cursor = ""): Promise<EventPage> {
  const query = new URLSearchParams({ limit: String(limit) });
  if (cursor) query.set("cursor", cursor);
  return apiFetch<EventPage>(`/api/runs/${runID}/events?${query.toString()}`);
}

export function listTrackPoints(runID: string, trackID: string, limit = 200): Promise<TrackPoint[]> {
  const query = new URLSearchParams({ limit: String(limit) });
  if (trackID) query.set("track_id", trackID);
  return apiFetch<TrackPoint[]>(`/api/runs/${runID}/track-points?${query.toString()}`);
}

export type SnapshotListOptions = {
  from?: string;
  to?: string;
  limit?: number;
};

export function listSnapshots(runID: string, options: number | SnapshotListOptions = 500): Promise<SnapshotFrame[]> {
  const opts = typeof options === "number" ? { limit: options } : options;
  const query = new URLSearchParams({ limit: String(opts.limit ?? 500) });
  if (opts.from) query.set("from", opts.from);
  if (opts.to) query.set("to", opts.to);
  return apiFetch<SnapshotFrame[]>(`/api/runs/${runID}/snapshots?${query.toString()}`);
}

export function getNearestSnapshot(runID: string, at: string): Promise<SnapshotFrame> {
  const query = new URLSearchParams();
  if (at) query.set("at", at);
  return apiFetch<SnapshotFrame>(`/api/runs/${runID}/snapshots/nearest?${query.toString()}`);
}

export function getRunReport(runID: string): Promise<RunReport> {
  return apiFetch<RunReport>(`/api/runs/${runID}/report`);
}

export type WebSocketTicket = {
  ticket: string;
  expires_at: string;
};

export function createWebSocketTicket(runID: string): Promise<WebSocketTicket> {
  return apiFetch<WebSocketTicket>(`/api/runs/${runID}/ws-ticket`, { method: "POST" });
}

export function reportDownloadPath(runID: string, format: "csv" | "json"): string {
  const query = new URLSearchParams();
  if (format === "csv") {
    query.set("format", "csv");
  }
  const suffix = query.toString();
  return `/api/runs/${runID}/report${suffix ? `?${suffix}` : ""}`;
}

export async function downloadRunReport(runID: string, format: "csv" | "json"): Promise<Blob> {
  const headers = new Headers();
  if (apiToken) {
    headers.set("Authorization", `Bearer ${apiToken}`);
  }
  const res = await fetch(`${apiBase}${reportDownloadPath(runID, format)}`, { headers });
  if (!res.ok) {
    const text = await res.text();
    const body = text ? JSON.parse(text) : null;
    const error = body?.error;
    throw new ApiRequestError(
      res.status,
      error?.code ?? "request_failed",
      error?.message ?? res.statusText,
      error?.details ?? []
    );
  }
  return res.blob();
}

export function reportFilename(runID: string, format: "csv" | "json"): string {
  return `run-${runID}-report.${format}`;
}

export function toWsUrl(base: string, runID: string, ticket?: string): string {
  const url = new URL(`${base.replace(/^http/, "ws")}/ws/runs/${runID}`);
  if (ticket) {
    url.searchParams.set("ticket", ticket);
  }
  return url.toString();
}
