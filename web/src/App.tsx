import { useEffect, useMemo, useRef, useState } from "react";
import { X } from "lucide-react";
import { ControlSidebar, type ReportExportFormat, type ReplaySpeed } from "./ControlSidebar";
import { SimulationMap } from "./SimulationMap";
import {
  ApiRequestError,
  apiBase,
  commandRun,
  createWebSocketTicket,
  createRun,
  clearApiToken,
  downloadRunReport,
  getNearestSnapshot,
  getRunReport,
  listEvents,
  listRuns,
  listScenarios,
  listSnapshots,
  listTrackPoints,
  listTracks,
  listZones,
  reportFilename,
  setApiToken,
  submitTrainingAction,
  toWsUrl
} from "./api";
import type {
  ConnectionState,
  Run,
  RunReport,
  Scenario,
  ScenarioSummary,
  SimEvent,
  Snapshot,
  SnapshotFrame,
  Track,
  TrackPoint,
  TrainingAction,
  Vec3,
  Zone
} from "./types";

const defaultCenter: Vec3 = { lon: 121.5, lat: 31.2, alt_m: 0 };
const snapshotWindowMS = 2 * 60 * 1000;
type ThreatFilter = "all" | "high" | "medium" | "low";

export function App() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [scenarios, setScenarios] = useState<ScenarioSummary[]>([]);
  const [selectedScenarioID, setSelectedScenarioID] = useState("");
  const [run, setRun] = useState<Run | null>(null);
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [snapshotFrames, setSnapshotFrames] = useState<SnapshotFrame[]>([]);
  const [replayFrame, setReplayFrame] = useState<SnapshotFrame | null>(null);
  const [replayIndex, setReplayIndex] = useState(0);
  const [replayPlaying, setReplayPlaying] = useState(false);
  const [replaySpeed, setReplaySpeed] = useState<ReplaySpeed>(1);
  const [report, setReport] = useState<RunReport | null>(null);
  const [tracks, setTracks] = useState<Track[]>([]);
  const [zones, setZones] = useState<Zone[]>([]);
  const [events, setEvents] = useState<SimEvent[]>([]);
  const [trackPoints, setTrackPoints] = useState<TrackPoint[]>([]);
  const [selectedTrack, setSelectedTrack] = useState<Track | null>(null);
  const [threatFilter, setThreatFilter] = useState<ThreatFilter>("all");
  const [connectionState, setConnectionState] = useState<ConnectionState>("idle");
  const [error, setError] = useState("");
  const [authRequired, setAuthRequired] = useState(false);
  const [tokenInput, setTokenInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [streamAttempt, setStreamAttempt] = useState(0);
  const replayActiveRef = useRef(false);

  const displaySnapshot = replayFrame ? snapshotFromFrame(replayFrame) : snapshot;
  const allTracks = replayFrame?.tracks ?? snapshot?.tracks ?? tracks;
  const visibleTracks = allTracks.filter((track) => threatFilter === "all" || track.threat_level === threatFilter);
  const center = run?.scenario?.ownship ?? defaultCenter;

  useEffect(() => {
    bootstrap().catch(handleError);
  }, []);

  useEffect(() => {
    replayActiveRef.current = replayFrame !== null;
  }, [replayFrame]);

  useEffect(() => {
    if (!run) return;
    const runID = run.id;
    let closed = false;
    let retryTimer: number | undefined;
    let socket: WebSocket | undefined;
    setConnectionState("connecting");

    async function connect() {
      try {
        const wsTicket = await createWebSocketTicket(runID);
        if (closed) return;
        socket = new WebSocket(toWsUrl(apiBase, runID, wsTicket.ticket));
        socket.onmessage = (event) => {
          if (closed) return;
          const message = JSON.parse(event.data);
          if (message.type === "snapshot") {
            const nextSnapshot = message.payload as Snapshot;
            setSnapshot(nextSnapshot);
            setTracks(nextSnapshot.tracks);
            if (nextSnapshot.events.length > 0) {
              setEvents((items) => mergeEvents(nextSnapshot.events, items));
            }
            if (!replayActiveRef.current) {
              setConnectionState("live");
            }
            setError("");
          }
        };
        socket.onerror = () => {
          if (!closed) {
            setConnectionState("error");
            setError("Live stream is unavailable; replay data remains available.");
          }
        };
        socket.onclose = () => {
          if (!closed) {
            setConnectionState("replay");
            const delay = Math.min(1000 * 2 ** streamAttempt, 15000);
            retryTimer = window.setTimeout(() => setStreamAttempt((attempt) => attempt + 1), delay);
          }
        };
      } catch (err) {
        if (!closed) {
          setConnectionState("error");
          handleError(err);
          if (!(err instanceof ApiRequestError && err.status === 401)) {
            const delay = Math.min(1000 * 2 ** streamAttempt, 15000);
            retryTimer = window.setTimeout(() => setStreamAttempt((attempt) => attempt + 1), delay);
          }
        }
      }
    }

    void connect();
    return () => {
      closed = true;
      if (retryTimer) window.clearTimeout(retryTimer);
      socket?.close();
    };
  }, [run?.id, streamAttempt]);

  useEffect(() => {
    if (!replayPlaying || snapshotFrames.length === 0) return;
    const interval = window.setInterval(() => {
      setReplayIndex((current) => {
        const nextIndex = Math.min(current + 1, snapshotFrames.length - 1);
        const nextFrame = snapshotFrames[nextIndex] ?? null;
        setReplayFrame(nextFrame);
        setConnectionState(nextFrame ? "replay" : "idle");
        if (nextIndex >= snapshotFrames.length - 1) {
          setReplayPlaying(false);
        }
        return nextIndex;
      });
    }, Math.max(120, 700 / replaySpeed));
    return () => window.clearInterval(interval);
  }, [replayPlaying, replaySpeed, snapshotFrames]);

  async function bootstrap() {
    const [nextScenarios, nextRuns] = await Promise.all([listScenarios(), listRuns()]);
    setScenarios(nextScenarios);
    setRuns(nextRuns);
    if (!selectedScenarioID && nextScenarios[0]) {
      setSelectedScenarioID(nextScenarios[0].id);
    }
    setAuthRequired(false);
  }

  async function refreshRuns() {
    const nextRuns = await listRuns();
    setRuns(nextRuns);
  }

  async function loadRunData(nextRun: Run) {
    setRun(nextRun);
    setSnapshot(null);
    setSnapshotFrames([]);
    setReplayFrame(null);
    setReplayIndex(0);
    setReplayPlaying(false);
    setReport(null);
    setSelectedTrack(null);
    setTrackPoints([]);
    setStreamAttempt(0);
    setError("");
    const [nextZones, nextTracks, eventPage, nextReport] = await Promise.all([
      listZones(nextRun.id),
      listTracks(nextRun.id),
      listEvents(nextRun.id, 50),
      getRunReport(nextRun.id)
    ]);
    const nextFrames = await loadSnapshotWindow(nextRun.id, nextReport, nextReport.snapshot_range?.to);
    setZones(nextZones);
    setTracks(nextTracks);
    setSnapshotFrames(nextFrames);
    setReport(nextReport);
    setEvents(mergeEvents(eventPage.items, nextReport.events ?? []));
    const latestIndex = Math.max(0, nextFrames.length - 1);
    const shouldShowReplay = nextFrames.length > 0 && nextReport.run.status !== "running";
    setReplayIndex(latestIndex);
    setReplayFrame(shouldShowReplay ? nextFrames[latestIndex] : null);
    setConnectionState(shouldShowReplay || nextTracks.length > 0 ? "replay" : "connecting");
  }

  async function handleCreateRun() {
    await withBusy(async () => {
      const nextRun = await createRun(selectedScenarioID ? { scenario_id: selectedScenarioID } : {});
      await loadRunData(nextRun);
      await refreshRuns();
      prependLocalEvent(`Created run ${shortID(nextRun.id)}`, nextRun.id);
    });
  }

  async function handleScenarioFile(file: File) {
    await withBusy(async () => {
      const scenario = JSON.parse(await file.text()) as Scenario;
      const nextRun = await createRun({ scenario });
      await loadRunData(nextRun);
      await refreshRuns();
      prependLocalEvent(`Created run ${shortID(nextRun.id)} from uploaded scenario`, nextRun.id);
    });
  }

  async function handleCommand(command: "start" | "pause" | "stop") {
    if (!run) return;
    await withBusy(async () => {
      const nextRun = await commandRun(run.id, command);
      setRun(nextRun);
      if (command === "start") {
        setReplayFrame(null);
        setReplayPlaying(false);
        setConnectionState("connecting");
      } else {
        await refreshReplayData(run.id);
      }
      await refreshRuns();
      prependLocalEvent(`${command} requested`, run.id);
    });
  }

  async function handleAction(type: TrainingAction) {
    if (!run) return;
    await withBusy(async () => {
      const event = await submitTrainingAction(run.id, type);
      await refreshReplayData(run.id);
      setEvents((items) => mergeEvents([event], items));
    });
  }

  async function handleSelectRun(nextRun: Run) {
    await withBusy(async () => loadRunData(nextRun));
  }

  async function handleSelectTrack(track: Track | null) {
    setSelectedTrack(track);
    if (!run || !track) {
      setTrackPoints([]);
      return;
    }
    try {
      setTrackPoints(await listTrackPoints(run.id, track.id, 200));
    } catch (err) {
      handleError(err);
    }
  }

  async function refreshReplayData(runID: string) {
    const nextReport = await getRunReport(runID);
    const anchor = replayActiveRef.current ? replayFrame?.sampled_at : nextReport.snapshot_range?.to;
    const nextFrames = await loadSnapshotWindow(runID, nextReport, anchor);
    setSnapshotFrames(nextFrames);
    setReport(nextReport);
    setRun(nextReport.run);
    setEvents((items) => mergeEvents(nextReport.events ?? [], items));
    if (replayActiveRef.current) {
      const nextIndex = Math.min(replayIndex, Math.max(0, nextFrames.length - 1));
      setReplayIndex(nextIndex);
      setReplayFrame(nextFrames[nextIndex] ?? null);
    }
  }

  function handleReplayIndex(index: number) {
    const nextIndex = Math.max(0, Math.min(index, snapshotFrames.length - 1));
    const frame = snapshotFrames[nextIndex];
    setReplayIndex(nextIndex);
    setReplayFrame(frame ?? null);
    setReplayPlaying(false);
    if (frame) {
      setConnectionState("replay");
    }
  }

  function handleReplayStep(delta: number) {
    handleReplayIndex(replayIndex + delta);
  }

  function handleReplayPlayToggle() {
    if (snapshotFrames.length === 0) return;
    if (!replayFrame) {
      handleReplayIndex(0);
    }
    setReplayPlaying((playing) => !playing);
    setConnectionState("replay");
  }

  async function handleReplayBoundary(boundary: "start" | "end") {
    if (!run || !report?.snapshot_range) return;
    await withBusy(async () => {
      const anchor = boundary === "start" ? report.snapshot_range?.from : report.snapshot_range?.to;
      const frames = await loadSnapshotWindow(run.id, report, anchor);
      const index = boundary === "start" ? 0 : Math.max(0, frames.length - 1);
      setSnapshotFrames(frames);
      setReplayIndex(index);
      setReplayFrame(frames[index] ?? null);
      setReplayPlaying(false);
      setConnectionState(frames.length > 0 ? "replay" : "idle");
    });
  }

  async function handleJumpToEvent(event: SimEvent) {
    if (!run || !report || report.replay_mode === "legacy") return;
    await withBusy(async () => {
      const nearest = await getNearestSnapshot(run.id, event.occurred_at);
      const windowFrames = await loadSnapshotWindow(run.id, report, nearest.sampled_at);
      const frames = mergeSnapshotFrames(windowFrames, [nearest]);
      const nextIndex = Math.max(0, frames.findIndex((frame) => frame.sampled_at === nearest.sampled_at && frame.tick === nearest.tick));
      setSnapshotFrames(frames);
      setReplayIndex(nextIndex);
      setReplayFrame(frames[nextIndex] ?? nearest);
      setReplayPlaying(false);
      setConnectionState("replay");
    });
  }

  function handleLiveView() {
    setReplayFrame(null);
    setReplayPlaying(false);
    setConnectionState(snapshot ? "live" : "connecting");
  }

  async function handleExportReport(format: ReportExportFormat) {
    if (!run) return;
    await withBusy(async () => {
      const blob = await downloadRunReport(run.id, format);
      saveBlob(blob, reportFilename(run.id, format));
    });
  }

  async function withBusy(task: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await task();
    } catch (err) {
      handleError(err);
    } finally {
      setBusy(false);
    }
  }

  async function handleLogin() {
    if (tokenInput.trim()) {
      setApiToken(tokenInput);
    }
    await withBusy(async () => {
      await bootstrap();
    });
  }

  function handleLogout() {
    clearApiToken();
    setAuthRequired(true);
    setRun(null);
    setRuns([]);
    setScenarios([]);
    setSnapshot(null);
    setSnapshotFrames([]);
    setReplayFrame(null);
    setReplayIndex(0);
    setReplayPlaying(false);
    setTracks([]);
    setReport(null);
    setEvents([]);
    setTokenInput("");
    setConnectionState("idle");
  }

  function handleError(err: unknown) {
    if (err instanceof ApiRequestError && err.status === 401) {
      setAuthRequired(true);
      setError("Sign in with an access token to use this deployment.");
      return;
    }
    if (err instanceof ApiRequestError && (err.status === 403 || err.status === 404)) {
      setError("No access to this resource, or it no longer exists.");
      return;
    }
    setError(err instanceof Error ? err.message : "Request failed.");
  }

  function prependLocalEvent(message: string, runID = run?.id ?? "") {
    setEvents((items) => mergeEvents([localEvent(message, runID)], items));
  }

  const selectedTrackID = selectedTrack?.id;
  const selectedLiveTrack = useMemo(
    () => visibleTracks.find((track) => track.id === selectedTrackID) ?? selectedTrack,
    [selectedTrack, selectedTrackID, visibleTracks]
  );

  return (
    <main className="app">
      <ControlSidebar
        runs={runs}
        scenarios={scenarios}
        selectedScenarioID={selectedScenarioID}
        run={run}
        snapshot={displaySnapshot}
        snapshotFrames={snapshotFrames}
        replayFrame={replayFrame}
        replayIndex={replayIndex}
        replayPlaying={replayPlaying}
        replaySpeed={replaySpeed}
        report={report}
        tracks={allTracks}
        visibleTrackCount={visibleTracks.length}
        threatFilter={threatFilter}
        authRequired={authRequired}
        tokenInput={tokenInput}
        connectionState={connectionState}
        events={events}
        error={error}
        busy={busy}
        onCreateRun={handleCreateRun}
        onCommand={handleCommand}
        onAction={handleAction}
        onSelectRun={handleSelectRun}
        onSelectScenario={setSelectedScenarioID}
        onScenarioFile={handleScenarioFile}
        onThreatFilter={setThreatFilter}
        onReplayIndex={handleReplayIndex}
        onReplayStep={handleReplayStep}
        onReplayBoundary={handleReplayBoundary}
        onReplayPlayToggle={handleReplayPlayToggle}
        onReplaySpeed={setReplaySpeed}
        onJumpToEvent={handleJumpToEvent}
        onLiveView={handleLiveView}
        onExportReport={handleExportReport}
        onTokenInput={setTokenInput}
        onLogin={handleLogin}
        onLogout={handleLogout}
      />

      <section className="mapShell">
        <SimulationMap
          center={center}
          tracks={visibleTracks}
          trackPoints={trackPoints}
          zones={zones}
          selectedTrackID={selectedTrackID}
          onSelectTrack={handleSelectTrack}
        />
        <TrackList tracks={visibleTracks} selectedTrackID={selectedTrackID} onSelectTrack={handleSelectTrack} />
        <TrackDetail track={selectedLiveTrack} points={trackPoints} onClose={() => handleSelectTrack(null)} />
      </section>
    </main>
  );
}

async function loadSnapshotWindow(runID: string, report: RunReport, anchor?: string) {
  if (!report.snapshot_range) {
    return [];
  }
  const rangeStart = new Date(report.snapshot_range.from).getTime();
  const rangeEnd = new Date(report.snapshot_range.to).getTime();
  const anchorMS = anchor ? new Date(anchor).getTime() : rangeEnd;
  const center = Number.isFinite(anchorMS) ? anchorMS : rangeEnd;
  const from = new Date(Math.max(rangeStart, center - snapshotWindowMS)).toISOString();
  const to = new Date(Math.min(rangeEnd, center + snapshotWindowMS)).toISOString();
  return listSnapshots(runID, { from, to, limit: 500 });
}

function mergeEvents(incoming: SimEvent[], existing: SimEvent[]) {
  const seen = new Set<string>();
  return [...incoming, ...existing]
    .filter((event) => {
      const key = event.id || `${event.run_id}-${event.occurred_at}-${event.type}-${JSON.stringify(event.payload)}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    })
    .sort((a, b) => new Date(b.occurred_at).getTime() - new Date(a.occurred_at).getTime())
    .slice(0, 80);
}

function mergeSnapshotFrames(primary: SnapshotFrame[], extra: SnapshotFrame[]) {
  const seen = new Set<string>();
  return [...primary, ...extra]
    .filter((frame) => {
      const key = `${frame.sampled_at}-${frame.tick}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    })
    .sort((a, b) => {
      const timeDiff = new Date(a.sampled_at).getTime() - new Date(b.sampled_at).getTime();
      return timeDiff || a.tick - b.tick;
    });
}

function localEvent(message: string, runID: string): SimEvent {
  const occurredAt = new Date().toISOString();
  return {
    id: `ui-${occurredAt}-${message}`,
    run_id: runID,
    occurred_at: occurredAt,
    type: "ui_event",
    payload: { result: message, severity: "info" }
  };
}

function TrackList({
  tracks,
  selectedTrackID,
  onSelectTrack
}: {
  tracks: Track[];
  selectedTrackID?: string;
  onSelectTrack: (track: Track) => void;
}) {
  return (
    <div className="trackList">
      {tracks.slice(0, 10).map((track) => (
        <button className="track" key={track.id} data-active={track.id === selectedTrackID} onClick={() => onSelectTrack(track)}>
          <strong>{track.track_no}</strong>
          <span>{track.kind}</span>
          <b data-threat={track.threat_level}>{track.threat_level}</b>
        </button>
      ))}
    </div>
  );
}

function TrackDetail({ track, points, onClose }: { track: Track | null; points: TrackPoint[]; onClose: () => void }) {
  if (!track) return null;
  return (
    <aside className="trackDetail">
      <button className="iconButton" onClick={onClose} aria-label="Close track detail">
        <X size={16} />
      </button>
      <h2>{track.track_no}</h2>
      <dl>
        <div>
          <dt>Kind</dt>
          <dd>{track.kind}</dd>
        </div>
        <div>
          <dt>Status</dt>
          <dd>{track.status}</dd>
        </div>
        <div>
          <dt>Threat</dt>
          <dd>{track.threat_level}</dd>
        </div>
        <div>
          <dt>Confidence</dt>
          <dd>{Math.round(track.confidence * 100)}%</dd>
        </div>
        <div>
          <dt>Position</dt>
          <dd>
            {track.position.lon.toFixed(4)}, {track.position.lat.toFixed(4)}
          </dd>
        </div>
        <div>
          <dt>History</dt>
          <dd>{points.length} points</dd>
        </div>
      </dl>
    </aside>
  );
}

function snapshotFromFrame(frame: SnapshotFrame): Snapshot {
  return {
    run_id: frame.run_id,
    status: frame.status,
    tick: frame.tick,
    time: frame.sampled_at,
    tracks: frame.tracks,
    contacts: frame.contacts,
    events: [],
    notice: frame.notice,
    snapshot_hz: frame.snapshot_hz
  };
}

function shortID(id: string) {
  return id.slice(0, 8);
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.rel = "noopener";
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
