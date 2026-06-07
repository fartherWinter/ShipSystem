import { useMemo, useState } from "react";
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Clock,
  Database,
  Download,
  FileJson,
  Filter,
  LogIn,
  LogOut,
  Pause,
  Play,
  Radio,
  RefreshCw,
  Route,
  Shield,
  SkipBack,
  SkipForward,
  Square,
  StepBack,
  StepForward,
  Waves,
  Zap
} from "lucide-react";
import type { FrontendAuthMode } from "./config";
import type { ConnectionState, Run, RunReport, ScenarioSummary, SimEvent, Snapshot, SnapshotFrame, Track, TrainingAction } from "./types";
import { trainingActions } from "./types";

type ThreatFilter = "all" | "high" | "medium" | "low";
type SeverityFilter = "all" | "info" | "warning" | "error";

export type ReplaySpeed = 0.5 | 1 | 2 | 4;
export type ReportExportFormat = "json" | "csv";

type ControlSidebarProps = {
  runs: Run[];
  scenarios: ScenarioSummary[];
  selectedScenarioID: string;
  run: Run | null;
  snapshot: Snapshot | null;
  snapshotFrames: SnapshotFrame[];
  replayFrame: SnapshotFrame | null;
  replayIndex: number;
  replayPlaying: boolean;
  replaySpeed: ReplaySpeed;
  report: RunReport | null;
  tracks: Track[];
  visibleTrackCount: number;
  threatFilter: ThreatFilter;
  authRequired: boolean;
  authMode: FrontendAuthMode;
  tokenInput: string;
  connectionState: ConnectionState;
  events: SimEvent[];
  replayLoading: boolean;
  replayError: string;
  error: string;
  busy: boolean;
  onCreateRun: () => void;
  onCommand: (command: "start" | "pause" | "stop") => void;
  onAction: (type: TrainingAction) => void;
  onSelectRun: (run: Run) => void;
  onSelectScenario: (scenarioID: string) => void;
  onScenarioFile: (file: File) => void;
  onThreatFilter: (filter: ThreatFilter) => void;
  onReplayIndex: (index: number) => void;
  onReplayStep: (delta: number) => void;
  onReplayBoundary: (boundary: "start" | "end") => void;
  onReplayWindow: (direction: "previous" | "next") => void;
  onReplayRetry: () => void;
  onReplayPlayToggle: () => void;
  onReplaySpeed: (speed: ReplaySpeed) => void;
  onJumpToEvent: (event: SimEvent) => void;
  onLiveView: () => void;
  onExportReport: (format: ReportExportFormat) => void;
  onTokenInput: (token: string) => void;
  onLogin: () => void;
  onLogout: () => void;
};

export function ControlSidebar({
  runs,
  scenarios,
  selectedScenarioID,
  run,
  snapshot,
  snapshotFrames,
  replayFrame,
  replayIndex,
  replayPlaying,
  replaySpeed,
  report,
  tracks,
  visibleTrackCount,
  threatFilter,
  authRequired,
  authMode,
  tokenInput,
  connectionState,
  events,
  replayLoading,
  replayError,
  error,
  busy,
  onCreateRun,
  onCommand,
  onAction,
  onSelectRun,
  onSelectScenario,
  onScenarioFile,
  onThreatFilter,
  onReplayIndex,
  onReplayStep,
  onReplayBoundary,
  onReplayWindow,
  onReplayRetry,
  onReplayPlayToggle,
  onReplaySpeed,
  onJumpToEvent,
  onLiveView,
  onExportReport,
  onTokenInput,
  onLogin,
  onLogout
}: ControlSidebarProps) {
  const [actionFilter, setActionFilter] = useState("all");
  const [severityFilter, setSeverityFilter] = useState<SeverityFilter>("all");
  const [fromFilter, setFromFilter] = useState("");
  const [toFilter, setToFilter] = useState("");
  const highThreats = tracks.filter((track) => track.threat_level === "high").length;
  const statusText = snapshot?.status ?? run?.status ?? "idle";
  const started = run?.started_at ? new Date(run.started_at).getTime() : 0;
  const current = snapshot?.time ? new Date(snapshot.time).getTime() : Date.now();
  const elapsedSeconds = started ? Math.max(0, Math.round((current - started) / 1000)) : 0;
  const canReplay = snapshotFrames.length > 0;
  const activeReplayFrame = replayFrame ?? snapshotFrames[replayIndex] ?? null;
  const replayTime = activeReplayFrame ? formatTime(activeReplayFrame.sampled_at) : "No frames";
  const frameLabel = canReplay ? `${Math.min(replayIndex + 1, snapshotFrames.length)} / ${snapshotFrames.length}` : "0 / 0";
  const replayRange = report?.snapshot_range;
  const replayWindowStart = snapshotFrames[0]?.sampled_at;
  const replayWindowEnd = snapshotFrames[snapshotFrames.length - 1]?.sampled_at;
  const rangeLabel = replayRange ? `${formatShortTime(replayRange.from)} - ${formatShortTime(replayRange.to)}` : "No snapshot range";
  const windowLabel =
    replayWindowStart && replayWindowEnd ? `${formatShortTime(replayWindowStart)} - ${formatShortTime(replayWindowEnd)}` : "No window loaded";
  const reportEvents = report?.events?.length ? report.events : events;
  const actionOptions = useMemo(() => uniqueActions(reportEvents), [reportEvents]);
  const filteredEvents = useMemo(
    () => filterEvents(reportEvents, actionFilter, severityFilter, fromFilter, toFilter),
    [reportEvents, actionFilter, severityFilter, fromFilter, toFilter]
  );
  const replayDisabled = !canReplay || busy || replayLoading;
  const legacyReplay = report?.replay_mode === "legacy";
  const replayProgress = replayRange ? timelineProgress(replayRange.from, replayRange.to, activeReplayFrame?.sampled_at) : 0;
  const canPagePrevious = canPageWindow(replayRange?.from, replayWindowStart, "previous") && !replayDisabled && !legacyReplay;
  const canPageNext = canPageWindow(replayRange?.to, replayWindowEnd, "next") && !replayDisabled && !legacyReplay;

  return (
    <aside className="sidebar">
      <div className="brand">
        <Waves size={24} />
        <div>
          <h1>ShipSim</h1>
          <p>Training simulation only</p>
        </div>
      </div>

      <section className="statusStrip" data-state={connectionState}>
        <Activity size={16} />
        <span>{connectionLabel(connectionState)}</span>
      </section>

      {authRequired && authMode === "token" ? (
        <section className="authPanel">
          <h2>Access</h2>
          <input
            type="password"
            value={tokenInput}
            onChange={(event) => onTokenInput(event.target.value)}
            placeholder="Access token"
            autoComplete="current-password"
          />
          <div className="toolbar">
            <button onClick={onLogin} disabled={busy || tokenInput.trim() === ""}>
              <LogIn size={16} /> Sign in
            </button>
            <button onClick={onLogout} disabled={busy}>
              <LogOut size={16} /> Clear
            </button>
          </div>
        </section>
      ) : null}

      {authRequired && authMode !== "token" ? (
        <section className="authPanel">
          <h2>Access</h2>
          <p className="emptyLine">{authMode === "proxy" ? "Authenticated proxy session required" : "Access denied"}</p>
          <div className="toolbar">
            <button onClick={onLogin} disabled={busy}>
              <RefreshCw size={16} /> Retry
            </button>
            <button onClick={onLogout} disabled={busy}>
              <LogOut size={16} /> Clear
            </button>
          </div>
        </section>
      ) : null}

      <section className="panel">
        <Metric label="Status" value={statusText} />
        <Metric label="Tracks" value={tracks.length.toString()} />
        <Metric label="Visible" value={visibleTrackCount.toString()} />
        <Metric label="High" value={highThreats.toString()} />
        <Metric label="Tick" value={(snapshot?.tick ?? 0).toString()} />
        <Metric label="Hz" value={(snapshot?.snapshot_hz ?? run?.scenario?.snapshot_hz ?? 0).toString()} />
        <Metric label="Elapsed" value={`${elapsedSeconds}s`} />
        <Metric label="Frame" value={frameLabel} />
        <Metric label="Report" value={report ? `v${report.version}` : "none"} />
      </section>

      <section className="selectorPanel">
        <h2>Scenario</h2>
        <select value={selectedScenarioID} onChange={(event) => onSelectScenario(event.target.value)} disabled={busy || scenarios.length === 0}>
          {scenarios.map((scenario) => (
            <option key={scenario.id} value={scenario.id}>
              {scenario.name} v{scenario.version ?? 1}
            </option>
          ))}
        </select>
        <input
          type="file"
          accept="application/json,.json"
          disabled={busy || authRequired}
          onChange={(event) => {
            const file = event.target.files?.[0];
            if (file) onScenarioFile(file);
            event.currentTarget.value = "";
          }}
        />
      </section>

      <section className="filterPanel" aria-label="Threat filter">
        {(["all", "high", "medium", "low"] as ThreatFilter[]).map((filter) => (
          <button key={filter} data-active={filter === threatFilter} onClick={() => onThreatFilter(filter)}>
            {filter}
          </button>
        ))}
      </section>

      <section className="replayPanel">
        <div className="sectionHeader">
          <h2>Replay</h2>
          <button onClick={onLiveView} disabled={!run || busy || !snapshot}>
            <Activity size={16} /> Live
          </button>
        </div>
        <div className="replayMeta">
          <Clock size={16} />
          <span>{canReplay ? `${replayTime} - windowed replay` : "Legacy history only"}</span>
        </div>
        <div className="replayTimeline">
          <div>
            <span>Range</span>
            <b>{rangeLabel}</b>
          </div>
          <div>
            <span>Window</span>
            <b>{windowLabel}</b>
          </div>
          <progress max="100" value={replayProgress} aria-label="Replay timeline progress" />
        </div>
        {replayLoading ? <p className="replayStatus">Loading replay window</p> : null}
        {replayError ? (
          <div className="replayError">
            <span>{replayError}</span>
            <button onClick={onReplayRetry} disabled={busy || replayLoading}>
              <RefreshCw size={14} /> Retry
            </button>
          </div>
        ) : null}
        <div className="replayButtons">
          <button onClick={() => onReplayBoundary("start")} disabled={replayDisabled} aria-label="Jump to start">
            <SkipBack size={16} />
          </button>
          <button onClick={() => onReplayStep(-1)} disabled={replayDisabled || replayIndex <= 0} aria-label="Previous frame">
            <StepBack size={16} />
          </button>
          <button className="playButton" onClick={onReplayPlayToggle} disabled={replayDisabled} aria-label={replayPlaying ? "Pause replay" : "Play replay"}>
            {replayPlaying ? <Pause size={16} /> : <Play size={16} />}
            {replayPlaying ? "Pause" : "Play"}
          </button>
          <button onClick={() => onReplayStep(1)} disabled={replayDisabled || replayIndex >= snapshotFrames.length - 1} aria-label="Next frame">
            <StepForward size={16} />
          </button>
          <button onClick={() => onReplayBoundary("end")} disabled={replayDisabled} aria-label="Jump to end">
            <SkipForward size={16} />
          </button>
        </div>
        <div className="replayWindowButtons">
          <button onClick={() => onReplayWindow("previous")} disabled={!canPagePrevious}>
            <ChevronLeft size={16} /> Previous window
          </button>
          <button onClick={() => onReplayWindow("next")} disabled={!canPageNext}>
            Next window <ChevronRight size={16} />
          </button>
        </div>
        <div className="replaySpeed">
          <span>Speed</span>
          <select value={replaySpeed} onChange={(event) => onReplaySpeed(Number(event.target.value) as ReplaySpeed)} disabled={replayDisabled}>
            {([0.5, 1, 2, 4] as ReplaySpeed[]).map((speed) => (
              <option key={speed} value={speed}>
                {speed}x
              </option>
            ))}
          </select>
        </div>
        <input
          type="range"
          min="0"
          max={Math.max(0, snapshotFrames.length - 1)}
          value={canReplay ? replayIndex : 0}
          disabled={replayDisabled}
          onChange={(event) => onReplayIndex(Number(event.target.value))}
        />
        {filteredEvents.length > 0 ? (
          <div className="eventMarkers" aria-label="Event markers">
            {filteredEvents.slice(0, 16).map((event) => (
              <button
                key={event.id}
                title={formatEvent(event)}
                disabled={busy || legacyReplay || !canReplay}
                onClick={() => onJumpToEvent(event)}
                aria-label={`Jump to ${eventAction(event)} at ${formatTime(event.occurred_at)}`}
              />
            ))}
          </div>
        ) : null}
        {legacyReplay ? <p className="emptyLine">Snapshot replay is unavailable for this run. Historical lines and events remain available.</p> : null}
      </section>

      <section className="toolbar">
        <button onClick={onCreateRun} disabled={busy || authRequired}>
          <Radio size={16} /> New Run
        </button>
        <button onClick={() => onCommand("start")} disabled={!run || busy || authRequired || run.status === "stopped"}>
          <Play size={16} /> Start
        </button>
        <button onClick={() => onCommand("pause")} disabled={!run || busy || authRequired}>
          <Pause size={16} /> Pause
        </button>
        <button onClick={() => onCommand("stop")} disabled={!run || busy || authRequired}>
          <Square size={16} /> Stop
        </button>
      </section>

      <section className="toolbar">
        {trainingActions.map((action) => (
          <button key={action.type} onClick={() => onAction(action.type)} disabled={!run || busy || authRequired}>
            {actionIcon(action.type)}
            {action.label}
          </button>
        ))}
      </section>

      {error ? <p className="errorLine">{error}</p> : null}

      <section className="notice">
        <Shield size={18} />
        <p>No real weapon, fire-control, radar, or electronic-warfare interface is implemented.</p>
      </section>

      <section className="reportPanel">
        <div className="sectionHeader">
          <h2>Report</h2>
          <div className="exportButtons">
            <button onClick={() => onExportReport("json")} disabled={!run || !report || busy} aria-label="Export report JSON">
              <FileJson size={16} /> JSON
            </button>
            <button onClick={() => onExportReport("csv")} disabled={!run || !report || busy} aria-label="Export report CSV">
              <Download size={16} /> CSV
            </button>
          </div>
        </div>
        {report ? (
          <>
            <div className="reportGrid">
              <Metric label="Mode" value={report.replay_mode} />
              <Metric label="Duration" value={`${report.duration_seconds}s`} />
              <Metric label="Tracks" value={report.track_count.toString()} />
              <Metric label="High max" value={report.threat_summary.high_watermark.toString()} />
              <Metric label="Frames" value={(report.snapshot_coverage?.count ?? report.snapshot_range?.count ?? 0).toString()} />
              <Metric label="Avg ms" value={formatInterval(report.snapshot_coverage?.average_interval_ms)} />
            </div>
            <div className="reportFilters">
              <label>
                <span>
                  <Filter size={13} /> Action
                </span>
                <select value={actionFilter} onChange={(event) => setActionFilter(event.target.value)}>
                  <option value="all">All</option>
                  {actionOptions.map((action) => (
                    <option key={action} value={action}>
                      {action}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Level</span>
                <select value={severityFilter} onChange={(event) => setSeverityFilter(event.target.value as SeverityFilter)}>
                  {(["all", "info", "warning", "error"] as SeverityFilter[]).map((level) => (
                    <option key={level} value={level}>
                      {level}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>From</span>
                <input type="datetime-local" value={fromFilter} onChange={(event) => setFromFilter(event.target.value)} />
              </label>
              <label>
                <span>To</span>
                <input type="datetime-local" value={toFilter} onChange={(event) => setToFilter(event.target.value)} />
              </label>
            </div>
            <div className="actionStats">
              {report.event_audit.action_stats.length === 0 ? (
                <p className="emptyLine">No actions yet</p>
              ) : (
                report.event_audit.action_stats.map((stat) => (
                  <span key={stat.type}>
                    {stat.type}: <b>{stat.count}</b>
                  </span>
                ))
              )}
            </div>
            <div className="auditSummary">
              <span>Events {report.event_audit.event_count}</span>
              <span>Actors {report.event_audit.actor_stats.length}</span>
              <span>First {formatTime(report.event_audit.first_action_at)}</span>
              <span>Last {formatTime(report.event_audit.last_action_at)}</span>
            </div>
            <div className="auditList">
              {filteredEvents.length === 0 ? (
                <p className="emptyLine">No events match the filters</p>
              ) : (
                filteredEvents.slice(0, 8).map((event) => (
                  <button key={event.id} onClick={() => onJumpToEvent(event)} disabled={busy || legacyReplay || !canReplay}>
                    <span>{formatTime(event.occurred_at)}</span>
                    <strong>{eventAction(event)}</strong>
                    <b data-severity={eventSeverity(event)}>{eventSeverity(event)}</b>
                  </button>
                ))
              )}
            </div>
          </>
        ) : (
          <p className="emptyLine">No report loaded</p>
        )}
      </section>

      <section className="runHistory">
        <h2>Recent Runs</h2>
        <div className="runList">
          {runs.map((item) => (
            <button key={item.id} className="runOption" data-active={item.id === run?.id} onClick={() => onSelectRun(item)}>
              <Database size={14} />
              <span>{item.name}</span>
              <b>{item.restored_from_store ? "restored" : item.status}</b>
            </button>
          ))}
        </div>
      </section>

      <section className="events">
        <h2>Events</h2>
        {events.length === 0 ? (
          <p className="emptyLine">No events yet</p>
        ) : (
          events.slice(0, 12).map((event) => (
            <button key={event.id} className="eventRow" onClick={() => onJumpToEvent(event)} disabled={busy || legacyReplay || !canReplay}>
              <span>{formatTime(event.occurred_at)}</span>
              <strong>{formatEvent(event)}</strong>
            </button>
          ))
        )}
      </section>
    </aside>
  );
}

function filterEvents(events: SimEvent[], action: string, severity: SeverityFilter, from: string, to: string) {
  const fromMS = localDateTimeToMS(from);
  const toMS = localDateTimeToMS(to);
  return events.filter((event) => {
    const eventMS = new Date(event.occurred_at).getTime();
    if (action !== "all" && eventAction(event) !== action) return false;
    if (severity !== "all" && eventSeverity(event) !== severity) return false;
    if (fromMS !== null && eventMS < fromMS) return false;
    if (toMS !== null && eventMS > toMS) return false;
    return true;
  });
}

function uniqueActions(events: SimEvent[]) {
  return Array.from(new Set(events.map(eventAction))).filter(Boolean).sort();
}

function eventAction(event: SimEvent) {
  const action = event.payload?.action;
  return typeof action === "string" && action ? action : event.type;
}

function eventSeverity(event: SimEvent): SeverityFilter {
  const severity = event.payload?.severity;
  if (severity === "warning" || severity === "error") return severity;
  return "info";
}

function formatEvent(event: SimEvent) {
  const action = eventAction(event);
  const result = event.payload?.result;
  return typeof result === "string" && result ? `${action}: ${result}` : action;
}

function localDateTimeToMS(value: string) {
  if (!value) return null;
  const ms = new Date(value).getTime();
  return Number.isFinite(ms) ? ms : null;
}

function formatTime(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleTimeString();
}

function formatShortTime(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function formatInterval(value?: number) {
  if (value === undefined || value === null || !Number.isFinite(value)) return "-";
  return Math.round(value).toString();
}

function timelineProgress(from: string, to: string, current?: string) {
  if (!current) return 0;
  const fromMS = new Date(from).getTime();
  const toMS = new Date(to).getTime();
  const currentMS = new Date(current).getTime();
  if (!Number.isFinite(fromMS) || !Number.isFinite(toMS) || !Number.isFinite(currentMS) || toMS <= fromMS) return 0;
  return Math.max(0, Math.min(100, ((currentMS - fromMS) / (toMS - fromMS)) * 100));
}

function canPageWindow(boundary?: string, edge?: string, direction?: "previous" | "next") {
  if (!boundary || !edge || !direction) return false;
  const boundaryMS = new Date(boundary).getTime();
  const edgeMS = new Date(edge).getTime();
  if (!Number.isFinite(boundaryMS) || !Number.isFinite(edgeMS)) return false;
  return direction === "previous" ? edgeMS > boundaryMS : edgeMS < boundaryMS;
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function connectionLabel(state: ConnectionState) {
  switch (state) {
    case "connecting":
      return "Connecting";
    case "live":
      return "Live stream";
    case "replay":
      return "Replay data";
    case "error":
      return "Connection issue";
    default:
      return "Idle";
  }
}

function actionIcon(type: TrainingAction) {
  if (type === "maneuver") return <Route size={16} />;
  if (type === "decoy") return <Shield size={16} />;
  return <Zap size={16} />;
}
