import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  Bookmark,
  BookmarkPlus,
  BookOpen,
  ChevronLeft,
  ChevronRight,
  CheckCircle,
  Clock,
  Copy,
  Database,
  Download,
  Eraser,
  FileJson,
  Filter,
  FileText,
  LogIn,
  LogOut,
  MapPin,
  Pause,
  Play,
  Radio,
  RefreshCw,
  Route,
  Save,
  Shield,
  SkipBack,
  SkipForward,
  Square,
  StepBack,
  StepForward,
  Tags,
  Trash2,
  Undo2,
  Waves,
  Zap
} from "lucide-react";
import type { FrontendAuthMode } from "./config";
import type {
  AssessmentRules,
  CapacityTrendSample,
  ConnectionState,
  CourseTemplate,
  MetricsResponse,
  ReplayBookmark,
  Run,
  RunMetadata,
  RunReport,
  ScenarioEditorMapMode,
  ScenarioEditorTemplate,
  ScenarioSummary,
  SessionResponse,
  SimEvent,
  Snapshot,
  SnapshotFrame,
  Track,
  TrainingAction,
  Zone
} from "./types";
import { trainingActions } from "./types";

type ThreatFilter = "all" | "high" | "medium" | "low";
type SeverityFilter = "all" | "info" | "warning" | "error";

export type ReplaySpeed = 0.5 | 1 | 2 | 4;
export type ReportExportFormat = "json" | "csv" | "html" | "pdf";

type AssessmentRuleDraft = {
  name: string;
  actionTarget: string;
  replayTarget: string;
  actionWeight: string;
  replayWeight: string;
  contextWeight: string;
};

type ControlSidebarProps = {
  runs: Run[];
  courseTemplates: CourseTemplate[];
  selectedCourseTemplateID: string;
  courseTemplateStatus: string;
  courseTemplateError: string;
  scenarios: ScenarioSummary[];
  selectedScenarioID: string;
  scenarioEditorText: string;
  scenarioEditorStatus: string;
  scenarioEditorError: string;
  scenarioEditorLoading: boolean;
  scenarioEditorValid: boolean;
  scenarioEditorGuidance: string[];
  scenarioEditorDiff: string;
  scenarioEditorMapMode: ScenarioEditorMapMode;
  scenarioEditorZoneDraftCount: number;
  selectedScenarioEditorZoneID: string;
  selectedScenarioEditorVertexIndex: number;
  run: Run | null;
  snapshot: Snapshot | null;
  snapshotFrames: SnapshotFrame[];
  replayFrame: SnapshotFrame | null;
  replayIndex: number;
  replayPlaying: boolean;
  replaySpeed: ReplaySpeed;
  replayAnchorStatus: string;
  reportExportStatus: string;
  replayBookmarks: ReplayBookmark[];
  report: RunReport | null;
  metrics: MetricsResponse | null;
  capacityTrend: CapacityTrendSample[];
  session: SessionResponse | null;
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
  onSelectCourseTemplate: (templateID: string) => void;
  onApplyCourseTemplate: () => void;
  onCommand: (command: "start" | "pause" | "stop") => void;
  onAction: (type: TrainingAction) => void;
  onSelectRun: (run: Run) => void;
  onSelectScenario: (scenarioID: string) => void;
  onScenarioFile: (file: File) => void;
  onCopyScenario: (name: string) => void;
  onSetScenarioEnabled: (enabled: boolean) => void;
  onScenarioEditorText: (text: string) => void;
  onLoadScenarioEditor: () => void;
  onSaveScenarioEditor: () => void;
  onScenarioEditorMapMode: (mode: ScenarioEditorMapMode) => void;
  onScenarioTemplate: (template: ScenarioEditorTemplate) => void;
  onScenarioAssessmentRules: (rules: AssessmentRules | null) => void;
  onScenarioEditorZoneSelect: (zoneID: string) => void;
  onScenarioEditorVertexSelect: (index: number) => void;
  onScenarioEditorZoneDelete: () => void;
  onScenarioZoneDraftUndo: () => void;
  onScenarioZoneDraftClear: () => void;
  onScenarioZoneDraftFinish: () => void;
  onSaveRunMetadata: (metadata: RunMetadata) => void;
  onAddAnnotation: (eventID: string, note: string) => void;
  onThreatFilter: (filter: ThreatFilter) => void;
  onReplayIndex: (index: number) => void;
  onReplayStep: (delta: number) => void;
  onReplayBoundary: (boundary: "start" | "end") => void;
  onReplayWindow: (direction: "previous" | "next") => void;
  onReplayRetry: () => void;
  onReplayPlayToggle: () => void;
  onReplaySpeed: (speed: ReplaySpeed) => void;
  onCopyReplayAnchor: () => void;
  onSaveReplayBookmark: () => void;
  onOpenReplayBookmark: (bookmark: ReplayBookmark) => void;
  onDeleteReplayBookmark: (id: string) => void;
  onJumpToEvent: (event: SimEvent) => void;
  onLiveView: () => void;
  onExportReport: (format: ReportExportFormat) => void;
  onTokenInput: (token: string) => void;
  onLogin: () => void;
  onLogout: () => void;
};

export function ControlSidebar({
  runs,
  courseTemplates,
  selectedCourseTemplateID,
  courseTemplateStatus,
  courseTemplateError,
  scenarios,
  selectedScenarioID,
  scenarioEditorText,
  scenarioEditorStatus,
  scenarioEditorError,
  scenarioEditorLoading,
  scenarioEditorValid,
  scenarioEditorGuidance,
  scenarioEditorDiff,
  scenarioEditorMapMode,
  scenarioEditorZoneDraftCount,
  selectedScenarioEditorZoneID,
  selectedScenarioEditorVertexIndex,
  run,
  snapshot,
  snapshotFrames,
  replayFrame,
  replayIndex,
  replayPlaying,
  replaySpeed,
  replayAnchorStatus,
  reportExportStatus,
  replayBookmarks,
  report,
  metrics,
  capacityTrend,
  session,
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
  onSelectCourseTemplate,
  onApplyCourseTemplate,
  onCommand,
  onAction,
  onSelectRun,
  onSelectScenario,
  onScenarioFile,
  onCopyScenario,
  onSetScenarioEnabled,
  onScenarioEditorText,
  onLoadScenarioEditor,
  onSaveScenarioEditor,
  onScenarioEditorMapMode,
  onScenarioTemplate,
  onScenarioAssessmentRules,
  onScenarioEditorZoneSelect,
  onScenarioEditorVertexSelect,
  onScenarioEditorZoneDelete,
  onScenarioZoneDraftUndo,
  onScenarioZoneDraftClear,
  onScenarioZoneDraftFinish,
  onSaveRunMetadata,
  onAddAnnotation,
  onThreatFilter,
  onReplayIndex,
  onReplayStep,
  onReplayBoundary,
  onReplayWindow,
  onReplayRetry,
  onReplayPlayToggle,
  onReplaySpeed,
  onCopyReplayAnchor,
  onSaveReplayBookmark,
  onOpenReplayBookmark,
  onDeleteReplayBookmark,
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
  const [scenarioCopyName, setScenarioCopyName] = useState("");
  const [tagInput, setTagInput] = useState("");
  const [traineeInput, setTraineeInput] = useState("");
  const [instructorNotes, setInstructorNotes] = useState("");
  const [archived, setArchived] = useState(false);
  const [annotationEventID, setAnnotationEventID] = useState("");
  const [annotationNote, setAnnotationNote] = useState("");
  const [assessmentRuleDraft, setAssessmentRuleDraft] = useState<AssessmentRuleDraft>(() => assessmentRuleDraftFromText(""));
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
  const reportActionStats = report?.event_audit.action_stats ?? [];
  const reportActorStats = report?.event_audit.actor_stats ?? [];
  const reportAnnotations = report?.annotations ?? [];
  const actionOptions = useMemo(() => uniqueActions(reportEvents), [reportEvents]);
  const filteredEvents = useMemo(
    () => filterEvents(reportEvents, actionFilter, severityFilter, fromFilter, toFilter),
    [reportEvents, actionFilter, severityFilter, fromFilter, toFilter]
  );
  const replayDisabled = !canReplay || busy || replayLoading;
  const legacyReplay = report?.replay_mode === "legacy";
  const replayProgress = replayRange ? timelineProgress(replayRange.from, replayRange.to, activeReplayFrame?.sampled_at) : 0;
  const replayInsights = buildReplayInsights(report, snapshotFrames, reportEvents);
  const canPagePrevious = canPageWindow(replayRange?.from, replayWindowStart, "previous") && !replayDisabled && !legacyReplay;
  const canPageNext = canPageWindow(replayRange?.to, replayWindowEnd, "next") && !replayDisabled && !legacyReplay;
  const canAnchorReplay = Boolean(run && (activeReplayFrame || snapshot));
  const selectedCourseTemplate = courseTemplates.find((template) => template.id === selectedCourseTemplateID);
  const selectedCourseChecklist = selectedCourseTemplate?.review_checklist ?? [];
  const selectedScenario = scenarios.find((scenario) => scenario.id === selectedScenarioID);
  const scenarioDisabled = selectedScenario?.enabled === false;
  const scenarioEditable = selectedScenario?.source === "database";
  const scenarioMapDisabled = busy || scenarioEditorLoading || !scenarioEditorValid || scenarioEditorText.trim() === "";
  const scenarioGeometryZones = scenarioZonesFromText(scenarioEditorText);
  const selectedScenarioGeometryZone = scenarioGeometryZones.find((zone) => zone.id === selectedScenarioEditorZoneID) ?? scenarioGeometryZones[0] ?? null;
  const selectedScenarioGeometryZoneID = selectedScenarioGeometryZone?.id ?? "";
  const scenarioGeometryVertices = selectedScenarioGeometryZone?.polygon ?? [];
  const assessmentRuleSummary = assessmentRuleDraftSummary(assessmentRuleDraft);
  const assessmentRuleApplyDisabled = scenarioMapDisabled || !assessmentRuleDraftValid(assessmentRuleDraft);
  const capacitySummary = buildCapacitySummary(metrics, runs);
  const capacityTrendSummary = buildCapacityTrendSummary(capacityTrend);

  useEffect(() => {
    setTagInput((run?.tags ?? []).join(", "));
    setTraineeInput((run?.trainees ?? []).join(", "));
    setInstructorNotes(run?.instructor_notes ?? "");
    setArchived(Boolean(run?.archived_at));
  }, [run?.id, run?.tags, run?.trainees, run?.instructor_notes, run?.archived_at]);

  useEffect(() => {
    if (!annotationEventID && reportEvents[0]?.id) {
      setAnnotationEventID(reportEvents[0].id);
    }
  }, [annotationEventID, reportEvents]);

  useEffect(() => {
    setAssessmentRuleDraft(assessmentRuleDraftFromText(scenarioEditorText));
  }, [scenarioEditorText]);

  useEffect(() => {
    if (!selectedScenarioEditorZoneID && scenarioGeometryZones[0]?.id) {
      onScenarioEditorZoneSelect(scenarioGeometryZones[0].id);
    }
  }, [onScenarioEditorZoneSelect, scenarioGeometryZones, selectedScenarioEditorZoneID]);

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

      {!authRequired && session ? (
        <section className="authPanel" aria-label="Access summary">
          <div className="sectionHeader">
            <h2>Access</h2>
            <Shield size={16} />
          </div>
          <div className="accessSummary">
            <span>{session.auth_mode === "off" ? "local" : session.auth_mode}</span>
            <b>{session.role}</b>
            {session.role_source ? <span>{session.role_source}</span> : null}
          </div>
          <p className="emptyLine">{session.user_id || "local session"}</p>
          <div className="permissionList" aria-label="Session permissions">
            {sessionPermissionLabels(session).map((item) => (
              <span key={item.label} data-enabled={item.enabled}>
                {item.label}
              </span>
            ))}
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

      <section className="coursePanel">
        <div className="sectionHeader">
          <h2>Course</h2>
          <BookOpen size={16} />
        </div>
        <select
          value={selectedCourseTemplateID}
          onChange={(event) => onSelectCourseTemplate(event.target.value)}
          disabled={busy || courseTemplates.length === 0}
        >
          {courseTemplates.length === 0 ? (
            <option value="">No course templates</option>
          ) : (
            courseTemplates.map((template) => (
              <option key={template.id} value={template.id}>
                {template.name}{template.enabled === false ? " (disabled)" : ""}
              </option>
            ))
          )}
        </select>
        <div className="scenarioMeta">
          <span>{selectedCourseTemplate?.source ?? "none"}</span>
          <b data-enabled={selectedCourseTemplate?.enabled !== false}>{selectedCourseTemplate?.enabled === false ? "Disabled" : "Ready"}</b>
        </div>
        {selectedCourseTemplate ? (
          <div className="courseTemplateDetail" aria-label="Course template detail">
            <span>Scenario {selectedCourseTemplate.scenario?.name ?? "Untitled scenario"}</span>
            <span>{courseMetadataSummary(selectedCourseTemplate)}</span>
            <span>{courseAssessmentSummary(selectedCourseTemplate)}</span>
            {selectedCourseChecklist.slice(0, 3).map((item) => (
              <span key={item.id}>{item.label}</span>
            ))}
          </div>
        ) : (
          <p className="emptyLine">No course template loaded</p>
        )}
        <button onClick={onApplyCourseTemplate} disabled={busy || authRequired || !selectedCourseTemplate || selectedCourseTemplate.enabled === false}>
          <BookOpen size={16} /> Create Scenario
        </button>
        {courseTemplateStatus ? <b className="statusLine" data-state="ok">{courseTemplateStatus}</b> : null}
        {courseTemplateError ? <b className="statusLine" data-state="error">{courseTemplateError}</b> : null}
      </section>

      <section className="selectorPanel">
        <h2>Scenario</h2>
        <select value={selectedScenarioID} onChange={(event) => onSelectScenario(event.target.value)} disabled={busy || scenarios.length === 0}>
          {scenarios.map((scenario) => (
            <option key={scenario.id} value={scenario.id}>
              {scenario.name} v{scenario.version ?? 1}{scenario.enabled === false ? " (disabled)" : ""}
            </option>
          ))}
        </select>
        <div className="scenarioMeta">
          <span>{selectedScenario?.source ?? "unknown"}</span>
          <b data-enabled={selectedScenario?.enabled !== false}>{selectedScenario?.enabled === false ? "Disabled" : "Enabled"}</b>
        </div>
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
        <div className="scenarioActions">
          <input
            value={scenarioCopyName}
            onChange={(event) => setScenarioCopyName(event.target.value)}
            placeholder="Copy name"
            disabled={busy || !selectedScenario}
          />
          <button onClick={() => onCopyScenario(scenarioCopyName)} disabled={busy || !selectedScenario}>
            <FileText size={16} /> Copy
          </button>
          <button
            onClick={() => onSetScenarioEnabled(selectedScenario?.enabled === false)}
            disabled={busy || !selectedScenario || selectedScenario.source !== "database"}
          >
            {selectedScenario?.enabled === false ? "Enable" : "Disable"}
          </button>
        </div>
        <div className="scenarioEditor">
          <div className="sectionHeader">
            <h3>Scenario JSON</h3>
            <div className="editorButtons">
              <button onClick={onLoadScenarioEditor} disabled={busy || scenarioEditorLoading || !selectedScenario}>
                <FileJson size={16} /> Load
              </button>
              <button
                onClick={onSaveScenarioEditor}
                disabled={busy || scenarioEditorLoading || !scenarioEditable || !scenarioEditorValid || scenarioEditorText.trim() === ""}
                aria-label="Save scenario JSON"
              >
                <Save size={16} /> Save
              </button>
            </div>
          </div>
          <textarea
            className="scenarioJson"
            value={scenarioEditorText}
            onChange={(event) => onScenarioEditorText(event.target.value)}
            placeholder="Load scenario JSON"
            spellCheck={false}
            disabled={busy || scenarioEditorLoading || !selectedScenario}
          />
          <div className="scenarioMapTools" aria-label="Scenario map editor">
            <div className="scenarioMapModes">
              {(["off", "sensor", "zone", "vertex"] as ScenarioEditorMapMode[]).map((mode) => (
                <button
                  key={mode}
                  onClick={() => onScenarioEditorMapMode(mode)}
                  disabled={mode !== "off" && scenarioMapDisabled}
                  data-active={scenarioEditorMapMode === mode}
                  aria-label={`Scenario map mode ${mode}`}
                >
                  {mode === "sensor" ? <MapPin size={15} /> : mode === "zone" || mode === "vertex" ? <Route size={15} /> : <Activity size={15} />}
                  {mode}
                </button>
              ))}
            </div>
            <div className="scenarioGeometryEditor" aria-label="Scenario geometry editor">
              <select
                value={selectedScenarioGeometryZoneID}
                onChange={(event) => onScenarioEditorZoneSelect(event.target.value)}
                disabled={scenarioMapDisabled || scenarioGeometryZones.length === 0}
                aria-label="Scenario geometry zone"
              >
                {scenarioGeometryZones.length === 0 ? (
                  <option value="">No zones</option>
                ) : (
                  scenarioGeometryZones.map((zone) => (
                    <option key={zone.id} value={zone.id}>
                      {zone.name || zone.id}
                    </option>
                  ))
                )}
              </select>
              <select
                value={Math.min(selectedScenarioEditorVertexIndex, Math.max(0, scenarioGeometryVertices.length - 1))}
                onChange={(event) => onScenarioEditorVertexSelect(Number(event.target.value))}
                disabled={scenarioMapDisabled || scenarioGeometryVertices.length === 0}
                aria-label="Scenario geometry vertex"
              >
                {scenarioGeometryVertices.length === 0 ? (
                  <option value={0}>No vertices</option>
                ) : (
                  scenarioGeometryVertices.map((point, index) => (
                    <option key={`${point.lon}-${point.lat}-${index}`} value={index}>
                      Vertex {index + 1}
                    </option>
                  ))
                )}
              </select>
              <button onClick={onScenarioEditorZoneDelete} disabled={scenarioMapDisabled || !selectedScenarioGeometryZoneID} aria-label="Delete scenario zone">
                <Trash2 size={15} /> Delete
              </button>
            </div>
            <div className="scenarioTemplates" aria-label="Scenario visual templates">
              <button onClick={() => onScenarioTemplate("review_sensor")} disabled={scenarioMapDisabled} aria-label="Add simulated sensor template">
                <Radio size={15} /> Sensor
              </button>
              <button onClick={() => onScenarioTemplate("exercise_boundary")} disabled={scenarioMapDisabled} aria-label="Add exercise boundary template">
                <Square size={15} /> Boundary
              </button>
              <button onClick={() => onScenarioTemplate("focus_zone")} disabled={scenarioMapDisabled} aria-label="Add focus zone template">
                <Route size={15} /> Focus
              </button>
            </div>
            <div className="assessmentRuleForm" aria-label="Assessment rule editor">
              <input
                value={assessmentRuleDraft.name}
                onChange={(event) => setAssessmentRuleDraft((draft) => ({ ...draft, name: event.target.value }))}
                placeholder="Rule name"
                disabled={scenarioMapDisabled}
              />
              <input
                type="number"
                min="1"
                max="1000"
                value={assessmentRuleDraft.actionTarget}
                onChange={(event) => setAssessmentRuleDraft((draft) => ({ ...draft, actionTarget: event.target.value }))}
                aria-label="Assessment action target"
                disabled={scenarioMapDisabled}
              />
              <input
                type="number"
                min="1"
                max="1000000"
                value={assessmentRuleDraft.replayTarget}
                onChange={(event) => setAssessmentRuleDraft((draft) => ({ ...draft, replayTarget: event.target.value }))}
                aria-label="Assessment replay target"
                disabled={scenarioMapDisabled}
              />
              <input
                type="number"
                min="0"
                max="100"
                value={assessmentRuleDraft.actionWeight}
                onChange={(event) => setAssessmentRuleDraft((draft) => ({ ...draft, actionWeight: event.target.value }))}
                aria-label="Assessment action weight"
                disabled={scenarioMapDisabled}
              />
              <input
                type="number"
                min="0"
                max="100"
                value={assessmentRuleDraft.replayWeight}
                onChange={(event) => setAssessmentRuleDraft((draft) => ({ ...draft, replayWeight: event.target.value }))}
                aria-label="Assessment replay weight"
                disabled={scenarioMapDisabled}
              />
              <input
                type="number"
                min="0"
                max="100"
                value={assessmentRuleDraft.contextWeight}
                onChange={(event) => setAssessmentRuleDraft((draft) => ({ ...draft, contextWeight: event.target.value }))}
                aria-label="Assessment context weight"
                disabled={scenarioMapDisabled}
              />
              <span data-state={assessmentRuleSummary.valid ? "ok" : "warning"}>{assessmentRuleSummary.label}</span>
              <button
                onClick={() => onScenarioAssessmentRules(assessmentRulesFromDraft(assessmentRuleDraft))}
                disabled={assessmentRuleApplyDisabled}
                aria-label="Apply assessment rules"
              >
                <CheckCircle size={15} /> Apply
              </button>
              <button onClick={() => onScenarioAssessmentRules(null)} disabled={scenarioMapDisabled} aria-label="Clear assessment rules">
                <Eraser size={15} /> Clear
              </button>
            </div>
            <div className="scenarioZoneDraft">
              <span>Draft {scenarioEditorZoneDraftCount}</span>
              <button onClick={onScenarioZoneDraftFinish} disabled={scenarioMapDisabled || scenarioEditorZoneDraftCount < 3} aria-label="Finish scenario zone">
                <CheckCircle size={15} />
              </button>
              <button onClick={onScenarioZoneDraftUndo} disabled={scenarioEditorZoneDraftCount === 0 || busy} aria-label="Undo scenario zone point">
                <Undo2 size={15} />
              </button>
              <button onClick={onScenarioZoneDraftClear} disabled={scenarioEditorZoneDraftCount === 0 || busy} aria-label="Clear scenario zone draft">
                <Eraser size={15} />
              </button>
            </div>
          </div>
          <div className="scenarioEditorMeta">
            <span>{scenarioEditable ? "Database scenario editable" : "Copy built-in or file scenarios before editing"}</span>
            <span data-state={scenarioEditorValid ? "ok" : "warning"}>{scenarioEditorGuidance[0] ?? "Load or paste scenario JSON before saving."}</span>
            {scenarioEditorGuidance.slice(1, 4).map((item) => (
              <span key={item} data-state="warning">
                {item}
              </span>
            ))}
            <span>{scenarioEditorDiff}</span>
            {scenarioEditorStatus ? <b data-state="ok">{scenarioEditorStatus}</b> : null}
            {scenarioEditorError ? <b data-state="error">{scenarioEditorError}</b> : null}
          </div>
        </div>
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
          <div className="replayInsights" aria-label="Replay quality summary">
            <span data-state={replayInsights.coverageState}>{replayInsights.coverageLabel}</span>
            <span data-state={replayInsights.gapState}>{replayInsights.gapLabel}</span>
            <span>{replayInsights.eventDensityLabel}</span>
          </div>
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
        <div className="replayReviewActions">
          <button onClick={onCopyReplayAnchor} disabled={!canAnchorReplay || busy || replayLoading} aria-label="Copy replay anchor">
            <Copy size={16} /> Copy Anchor
          </button>
          <button onClick={onSaveReplayBookmark} disabled={!canAnchorReplay || busy || replayLoading} aria-label="Save replay bookmark">
            <BookmarkPlus size={16} /> Bookmark
          </button>
        </div>
        {replayAnchorStatus ? <p className="replayAnchorStatus">{replayAnchorStatus}</p> : null}
        {replayBookmarks.length > 0 ? (
          <div className="replayBookmarkList" aria-label="Replay bookmarks">
            {replayBookmarks.slice(0, 6).map((bookmark) => (
              <div key={bookmark.id}>
                <button onClick={() => onOpenReplayBookmark(bookmark)} disabled={busy || replayLoading} title={formatBookmark(bookmark)}>
                  <Bookmark size={14} />
                  <span>{bookmark.label}</span>
                </button>
                <button onClick={() => onDeleteReplayBookmark(bookmark.id)} disabled={busy} aria-label={`Delete bookmark ${bookmark.label}`}>
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        ) : null}
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
                data-severity={eventSeverity(event)}
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
        <button onClick={onCreateRun} disabled={busy || authRequired || scenarioDisabled}>
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

      <section className="metadataPanel">
        <div className="sectionHeader">
          <h2>Training Record</h2>
          <button
            onClick={() =>
              onSaveRunMetadata({
                tags: splitList(tagInput),
                trainees: splitList(traineeInput),
                instructor_notes: instructorNotes,
                archived
              })
            }
            disabled={!run || busy || authRequired}
          >
            <Tags size={16} /> Save
          </button>
        </div>
        <input value={tagInput} onChange={(event) => setTagInput(event.target.value)} placeholder="Tags" disabled={!run || busy} />
        <input value={traineeInput} onChange={(event) => setTraineeInput(event.target.value)} placeholder="Trainees" disabled={!run || busy} />
        <textarea value={instructorNotes} onChange={(event) => setInstructorNotes(event.target.value)} placeholder="Instructor notes" disabled={!run || busy} />
        <label className="checkRow">
          <input type="checkbox" checked={archived} onChange={(event) => setArchived(event.target.checked)} disabled={!run || busy} />
          <span>Archived</span>
        </label>
      </section>

      <section className="notice">
        <Shield size={18} />
        <p>No real weapon, fire-control, radar, or electronic-warfare interface is implemented.</p>
      </section>

      <section className="capacityPanel">
        <div className="sectionHeader">
          <h2>Capacity</h2>
          <Database size={16} />
        </div>
        <div className="reportGrid">
          <Metric label="Frames" value={capacitySummary.frames} />
          <Metric label="Events" value={capacitySummary.events} />
          <Metric label="Points" value={capacitySummary.points} />
          <Metric label="Contacts" value={capacitySummary.contacts} />
        </div>
        <div className="capacityLimits">
          <span>Snapshot pressure {capacitySummary.snapshotPressure}</span>
          <span>Event pressure {capacitySummary.eventPressure}</span>
          <span>Point pressure {capacitySummary.pointPressure}</span>
        </div>
        <div className="capacityLimits">
          <span>Snapshots/run {capacitySummary.snapshotLimit}</span>
          <span>Events/run {capacitySummary.eventLimit}</span>
          <span>Points/run {capacitySummary.pointLimit}</span>
        </div>
        <div className="capacityLatency">
          <span>DB {capacitySummary.db}</span>
          <span>Write fail {capacitySummary.failures}</span>
          <span>Last {capacitySummary.writeLast}</span>
          <span>Avg {capacitySummary.writeAvg}</span>
          <span>Max {capacitySummary.writeMax}</span>
        </div>
        <div className="capacityDbPlan" aria-label="Database planning">
          <span>Tables {capacitySummary.dbTable}</span>
          <span>Indexes {capacitySummary.dbIndex}</span>
          <span>Total {capacitySummary.dbTotal}</span>
          <strong data-state={capacitySummary.dbIndexState}>{capacitySummary.dbIndexRatio}</strong>
        </div>
        {capacityTrendSummary ? (
          <div className="capacityTrend" aria-label="Capacity trend">
            <span>{capacityTrendSummary.windowLabel}</span>
            <span>{capacityTrendSummary.frameGrowth}</span>
            <span>{capacityTrendSummary.eventGrowth}</span>
            <span>{capacityTrendSummary.pointGrowth}</span>
            <span>{capacityTrendSummary.dbGrowth}</span>
            <span>{capacityTrendSummary.latencyTrend}</span>
            <strong data-state={capacityTrendSummary.archiveState}>{capacityTrendSummary.archiveGuidance}</strong>
          </div>
        ) : null}
        {capacitySummary.runRows.length > 0 ? (
          <div className="capacityRunList" aria-label="Capacity by run">
            {capacitySummary.runRows.map((row) => (
              <div key={row.id}>
                <span>{row.name}</span>
                <b>S {row.frames}</b>
                <b>E {row.events}</b>
                <b>P {row.points}</b>
                <b>C {row.contacts}</b>
                <strong>{row.pressure}</strong>
              </div>
            ))}
          </div>
        ) : null}
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
            <button onClick={() => onExportReport("html")} disabled={!run || !report || busy} aria-label="Export report HTML">
              <FileText size={16} /> HTML
            </button>
            <button onClick={() => onExportReport("pdf")} disabled={!run || !report || busy} aria-label="Export report PDF">
              <Download size={16} /> PDF
            </button>
          </div>
        </div>
        {reportExportStatus ? <p className="replayAnchorStatus">{reportExportStatus}</p> : null}
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
              {reportActionStats.length === 0 ? (
                <p className="emptyLine">No actions yet</p>
              ) : (
                reportActionStats.map((stat) => (
                  <span key={stat.type}>
                    {stat.type}: <b>{stat.count}</b>
                  </span>
                ))
              )}
            </div>
            <div className="assessmentSummary">
              <span>Assessment {report.assessment.label}</span>
              <b>{report.assessment.score}</b>
            </div>
            <div className="auditSummary">
              <span>Events {report.event_audit.event_count}</span>
              <span>Actors {reportActorStats.length}</span>
              <span>First {formatTime(report.event_audit.first_action_at)}</span>
              <span>Last {formatTime(report.event_audit.last_action_at)}</span>
            </div>
            <div className="annotationPanel">
              <select value={annotationEventID} onChange={(event) => setAnnotationEventID(event.target.value)} disabled={busy || reportEvents.length === 0}>
                {reportEvents.slice(0, 20).map((event) => (
                  <option key={event.id} value={event.id}>
                    {formatShortTime(event.occurred_at)} {eventAction(event)}
                  </option>
                ))}
              </select>
              <textarea value={annotationNote} onChange={(event) => setAnnotationNote(event.target.value)} placeholder="Event annotation" disabled={busy || !run} />
              <button
                onClick={() => {
                  onAddAnnotation(annotationEventID, annotationNote);
                  setAnnotationNote("");
                }}
                disabled={!run || busy || annotationNote.trim() === ""}
              >
                Add Annotation
              </button>
            </div>
            {reportAnnotations.length > 0 ? (
              <div className="annotationList">
                {reportAnnotations.slice(0, 4).map((annotation) => (
                  <span key={annotation.id}>
                    {formatShortTime(annotation.created_at)} {annotation.note}
                  </span>
                ))}
              </div>
            ) : null}
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

function sessionPermissionLabels(session: SessionResponse) {
  const permissions = session.permissions ?? {};
  return [
    { label: "View", enabled: Boolean(permissions.view_runs) },
    { label: "Run ops", enabled: Boolean(permissions.create_runs && permissions.control_runs) },
    { label: "Training actions", enabled: Boolean(permissions.submit_training_actions) },
    { label: "Scenarios", enabled: Boolean(permissions.manage_scenarios) },
    { label: "Retention", enabled: Boolean(permissions.manage_retention) }
  ];
}

function courseMetadataSummary(template: CourseTemplate) {
  const metadataKeys = Object.keys(template.expected_metadata ?? {});
  const checklistCount = (template.review_checklist ?? []).length;
  const metadataLabel = metadataKeys.length > 0 ? metadataKeys.slice(0, 3).join(", ") : "no metadata";
  const suffix = metadataKeys.length > 3 ? ` +${metadataKeys.length - 3}` : "";
  return `Metadata ${metadataLabel}${suffix} | Checklist ${checklistCount}`;
}

function courseAssessmentSummary(template: CourseTemplate) {
  const rules = template.scenario?.assessment_rules;
  if (!rules) {
    return `Rules profile ${template.scenario?.assessment_profile ?? "standard"}`;
  }
  return `Rules ${assessmentRuleName(rules)} | Actions ${rules.action_target} | Replay ${rules.replay_target} | Weights ${rules.action_weight}/${rules.replay_weight}/${rules.context_weight}`;
}

function assessmentRuleName(rules: AssessmentRules) {
  return rules.name?.trim() || "custom_record";
}

function assessmentRuleDraftFromText(text: string): AssessmentRuleDraft {
  const scenario = parseScenarioDraft(text);
  const rules = scenario?.assessment_rules ?? assessmentPresetRules(scenario?.assessment_profile);
  return {
    name: rules.name ?? "",
    actionTarget: String(rules.action_target),
    replayTarget: String(rules.replay_target),
    actionWeight: String(rules.action_weight),
    replayWeight: String(rules.replay_weight),
    contextWeight: String(rules.context_weight)
  };
}

function parseScenarioDraft(text: string) {
  try {
    return JSON.parse(text) as { assessment_profile?: string; assessment_rules?: AssessmentRules };
  } catch {
    return null;
  }
}

function scenarioZonesFromText(text: string): Zone[] {
  try {
    const scenario = JSON.parse(text) as { zones?: Zone[] };
    return Array.isArray(scenario.zones) ? scenario.zones : [];
  } catch {
    return [];
  }
}

function assessmentPresetRules(profile?: string): AssessmentRules {
  switch (profile) {
    case "quick_review":
      return { name: "quick_review_record", action_target: 3, replay_target: 8, action_weight: 30, replay_weight: 30, context_weight: 40 };
    case "extended_review":
      return { name: "extended_review_record", action_target: 10, replay_target: 60, action_weight: 40, replay_weight: 35, context_weight: 25 };
    default:
      return { name: "standard_record", action_target: 6, replay_target: 20, action_weight: 34, replay_weight: 33, context_weight: 33 };
  }
}

function assessmentRulesFromDraft(draft: AssessmentRuleDraft): AssessmentRules {
  return {
    name: draft.name.trim() || undefined,
    action_target: numberFromDraft(draft.actionTarget),
    replay_target: numberFromDraft(draft.replayTarget),
    action_weight: numberFromDraft(draft.actionWeight),
    replay_weight: numberFromDraft(draft.replayWeight),
    context_weight: numberFromDraft(draft.contextWeight)
  };
}

function assessmentRuleDraftSummary(draft: AssessmentRuleDraft) {
  const values = assessmentRulesFromDraft(draft);
  const total = values.action_weight + values.replay_weight + values.context_weight;
  const valid = assessmentRuleDraftValid(draft);
  return {
    valid,
    label: `Weights ${total}/100`
  };
}

function assessmentRuleDraftValid(draft: AssessmentRuleDraft) {
  const values = assessmentRulesFromDraft(draft);
  return (
    inRange(values.action_target, 1, 1000) &&
    inRange(values.replay_target, 1, 1_000_000) &&
    inRange(values.action_weight, 0, 100) &&
    inRange(values.replay_weight, 0, 100) &&
    inRange(values.context_weight, 0, 100) &&
    values.action_weight + values.replay_weight + values.context_weight === 100
  );
}

function numberFromDraft(value: string) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? Math.round(parsed) : Number.NaN;
}

function inRange(value: number, min: number, max: number) {
  return Number.isFinite(value) && value >= min && value <= max;
}

function buildReplayInsights(report: RunReport | null, frames: SnapshotFrame[], events: SimEvent[]) {
  const coverage = report?.snapshot_coverage;
  const range = report?.snapshot_range;
  if (!coverage || !range || report?.replay_mode === "legacy") {
    return {
      coverageLabel: "Coverage unavailable",
      coverageState: "empty",
      gapLabel: "Gaps unavailable",
      gapState: "empty",
      eventDensityLabel: `Events ${events.length}`
    };
  }
  const rangeMS = timeDiffMS(range.from, range.to);
  const expectedFrames =
    coverage.average_interval_ms > 0 && rangeMS > 0 ? Math.max(1, Math.floor(rangeMS / coverage.average_interval_ms) + 1) : coverage.count;
  const coveragePercent = expectedFrames > 0 ? Math.min(100, Math.round((coverage.count / expectedFrames) * 100)) : 0;
  const expectedGapMS = coverage.average_interval_ms || (frames[0]?.snapshot_hz ? 1000 / frames[0].snapshot_hz : 0);
  const largestGapMS = largestWindowGapMS(frames);
  const gapWarning = expectedGapMS > 0 && largestGapMS > expectedGapMS * 3;
  const minutes = rangeMS > 0 ? rangeMS / 60000 : 0;
  const eventRate = minutes > 0 ? events.length / minutes : events.length;

  return {
    coverageLabel: `Coverage ${coveragePercent}%`,
    coverageState: coveragePercent >= 90 ? "ok" : "warning",
    gapLabel: frames.length < 2 ? "Window gaps -" : `Max gap ${formatInterval(largestGapMS)}ms`,
    gapState: gapWarning ? "warning" : "ok",
    eventDensityLabel: `Events ${eventRate.toFixed(eventRate < 10 ? 1 : 0)}/min`
  };
}

function buildCapacitySummary(metrics: MetricsResponse | null, runs: Run[]) {
  const frames = metricNumber(metrics, "snapshot_frames");
  const events = metricNumber(metrics, "event_count");
  const points = metricNumber(metrics, "track_point_count");
  const contacts = metricNumber(metrics, "contact_count");
  const snapshotPressure = metricNumber(metrics, "snapshot_capacity_pressure");
  const eventPressure = metricNumber(metrics, "event_capacity_pressure");
  const pointPressure = metricNumber(metrics, "track_point_capacity_pressure");
  const failures = metricNumber(metrics, "snapshot_write_failures");
  const dbReady = metricBoolean(metrics, "db_ready");
  const dbTableBytes = metricNumber(metrics, "db_table_bytes") ?? 0;
  const dbIndexBytes = metricNumber(metrics, "db_index_bytes") ?? 0;
  const dbTotalBytes = metricNumber(metrics, "db_total_bytes") ?? 0;
  const dbIndexPercent = dbTableBytes > 0 ? Math.round((dbIndexBytes / dbTableBytes) * 100) : 0;
  return {
    frames: formatMetric(frames),
    events: formatMetric(events),
    points: formatMetric(points),
    contacts: formatMetric(contacts),
    snapshotPressure: formatPressure(snapshotPressure),
    eventPressure: formatPressure(eventPressure),
    pointPressure: formatPressure(pointPressure),
    failures: formatMetric(failures),
    db: dbReady === null ? "-" : dbReady ? "ready" : "down",
    snapshotLimit: formatLimit(metricNumber(metrics, "max_snapshots_per_run")),
    eventLimit: formatLimit(metricNumber(metrics, "max_events_per_run")),
    pointLimit: formatLimit(metricNumber(metrics, "max_track_points_per_run")),
    writeLast: formatMilliseconds(metricNumber(metrics, "snapshot_write_last_ms")),
    writeAvg: formatMilliseconds(metricNumber(metrics, "snapshot_write_avg_ms")),
    writeMax: formatMilliseconds(metricNumber(metrics, "snapshot_write_max_ms")),
    dbTable: formatBytes(dbTableBytes),
    dbIndex: formatBytes(dbIndexBytes),
    dbTotal: formatBytes(dbTotalBytes),
    dbIndexRatio: dbTableBytes > 0 ? `Index/table ${dbIndexPercent}%` : "Index/table -",
    dbIndexState: dbIndexPercent >= 80 ? "warning" : "ok",
    runRows: capacityRunRows(metrics, runs)
  };
}

function buildCapacityTrendSummary(samples: CapacityTrendSample[]) {
  if (samples.length === 0) return null;
  const sorted = [...samples].sort((a, b) => new Date(a.sampled_at).getTime() - new Date(b.sampled_at).getTime());
  const first = sorted[0];
  const last = sorted[sorted.length - 1];
  const windowMS = timeDiffMS(first.sampled_at, last.sampled_at);
  const maxPressure = Math.max(last.snapshot_capacity_pressure, last.event_capacity_pressure, last.track_point_capacity_pressure);
  const growingFast = delta(first.track_point_count, last.track_point_count) > 0 && sorted.length >= 3;
  const archiveState = maxPressure >= 0.8 ? "error" : maxPressure >= 0.6 || growingFast ? "warning" : "ok";
  return {
    windowLabel: `Trend ${sorted.length} sample${sorted.length === 1 ? "" : "s"} / ${formatTrendWindow(windowMS)}`,
    frameGrowth: `Frames ${formatSigned(delta(first.snapshot_frames, last.snapshot_frames))}`,
    eventGrowth: `Events ${formatSigned(delta(first.event_count, last.event_count))}`,
    pointGrowth: `Points ${formatSigned(delta(first.track_point_count, last.track_point_count))}`,
    dbGrowth: `DB ${formatBytes(last.db_total_bytes)} (${formatSignedBytes(delta(first.db_total_bytes, last.db_total_bytes))})`,
    latencyTrend: `Write avg ${formatMilliseconds(first.snapshot_write_avg_ms)} -> ${formatMilliseconds(last.snapshot_write_avg_ms)}`,
    archiveState,
    archiveGuidance: archiveGuidance(archiveState, maxPressure)
  };
}

function archiveGuidance(state: string, pressure: number) {
  if (state === "error") return `Archive soon ${Math.round(pressure * 100)}%`;
  if (state === "warning") return pressure > 0 ? `Watch growth ${Math.round(pressure * 100)}%` : "Watch growth";
  return "Archive steady";
}

function delta(first: number, last: number) {
  return Math.round(last - first);
}

function formatSigned(value: number) {
  return `${value >= 0 ? "+" : ""}${value.toLocaleString()}`;
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let scaled = value;
  let index = 0;
  while (scaled >= 1024 && index < units.length - 1) {
    scaled /= 1024;
    index += 1;
  }
  const digits = scaled >= 10 || index === 0 ? 0 : 1;
  return `${scaled.toFixed(digits)} ${units[index]}`;
}

function formatSignedBytes(value: number) {
  const sign = value >= 0 ? "+" : "-";
  return `${sign}${formatBytes(Math.abs(value))}`;
}

function formatTrendWindow(ms: number) {
  if (ms <= 0) return "now";
  const minutes = Math.round(ms / 60000);
  if (minutes < 60) return `${Math.max(1, minutes)}m`;
  const hours = Math.round(minutes / 60);
  return `${hours}h`;
}

function capacityRunRows(metrics: MetricsResponse | null, runs: Run[]) {
  const framesByRun = metricNumberRecord(metrics, "snapshot_frames_by_run");
  const pressureByRun = metricNumberRecord(metrics, "snapshot_capacity_pressure_by_run");
  const eventsByRun = metricNumberRecord(metrics, "event_count_by_run");
  const eventPressureByRun = metricNumberRecord(metrics, "event_capacity_pressure_by_run");
  const pointsByRun = metricNumberRecord(metrics, "track_point_count_by_run");
  const pointPressureByRun = metricNumberRecord(metrics, "track_point_capacity_pressure_by_run");
  const contactsByRun = metricNumberRecord(metrics, "contact_count_by_run");
  const runNames = new Map(runs.map((run) => [run.id, run.name]));
  const ids = new Set([...Object.keys(framesByRun), ...Object.keys(eventsByRun), ...Object.keys(pointsByRun), ...Object.keys(contactsByRun)]);
  return Array.from(ids)
    .map((id) => {
      const frames = framesByRun[id] ?? 0;
      const events = eventsByRun[id] ?? 0;
      const points = pointsByRun[id] ?? 0;
      const contacts = contactsByRun[id] ?? 0;
      const sortPressure = Math.max(pressureByRun[id] ?? 0, eventPressureByRun[id] ?? 0, pointPressureByRun[id] ?? 0);
      return {
        id,
        name: runNames.get(id) ?? shortRunID(id),
        frames: Math.round(frames).toLocaleString(),
        events: Math.round(events).toLocaleString(),
        points: Math.round(points).toLocaleString(),
        contacts: Math.round(contacts).toLocaleString(),
        pressure: sortPressure === 0 ? "-" : `${Math.round(sortPressure * 100)}%`,
        sortPressure,
        sortTotal: frames + events + points + contacts
      };
    })
    .sort((a, b) => b.sortPressure - a.sortPressure || b.sortTotal - a.sortTotal)
    .slice(0, 5);
}

function metricNumber(metrics: MetricsResponse | null, key: string) {
  const value = metrics?.[key];
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function metricBoolean(metrics: MetricsResponse | null, key: string) {
  const value = metrics?.[key];
  return typeof value === "boolean" ? value : null;
}

function metricNumberRecord(metrics: MetricsResponse | null, key: string) {
  const value = metrics?.[key];
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>).filter((entry): entry is [string, number] => typeof entry[1] === "number" && Number.isFinite(entry[1]))
  );
}

function formatMetric(value: number | null) {
  return value === null ? "-" : Math.round(value).toString();
}

function formatLimit(value: number | null) {
  return value && value > 0 ? Math.round(value).toLocaleString() : "off";
}

function formatPressure(value: number | null) {
  return value === null ? "-" : `${Math.round(value * 100)}%`;
}

function timeDiffMS(from: string, to: string) {
  const start = new Date(from).getTime();
  const end = new Date(to).getTime();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return 0;
  return end - start;
}

function largestWindowGapMS(frames: SnapshotFrame[]) {
  let largest = 0;
  for (let i = 1; i < frames.length; i += 1) {
    const previous = new Date(frames[i - 1].sampled_at).getTime();
    const current = new Date(frames[i].sampled_at).getTime();
    if (Number.isFinite(previous) && Number.isFinite(current)) {
      largest = Math.max(largest, current - previous);
    }
  }
  return largest;
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

function formatBookmark(bookmark: ReplayBookmark) {
  return `${bookmark.label} (${formatShortTime(bookmark.at)})`;
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

function formatMilliseconds(value: number | null) {
  return value === null ? "-" : `${formatInterval(value)}ms`;
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

function splitList(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function shortRunID(id: string) {
  return id.slice(0, 8);
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
