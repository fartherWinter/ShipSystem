import { useEffect, useMemo, useRef, useState } from "react";
import { X } from "lucide-react";
import { ControlSidebar, type ReportExportFormat, type ReplaySpeed } from "./ControlSidebar";
import { SimulationMap } from "./SimulationMap";
import {
  addAnnotation,
  ApiRequestError,
  commandRun,
  copyScenario,
  createScenario,
  createScenarioFromCourseTemplate,
  createWebSocketTicket,
  createRun,
  clearApiToken,
  downloadRunReport,
  getSession,
  getScenario,
  getMetrics,
  getRun,
  getNearestSnapshot,
  getRunReport,
  listEvents,
  listCourseTemplates,
  getMetricsHistory,
  listRuns,
  listScenarios,
  listSnapshots,
  listTrackPoints,
  listTracks,
  listZones,
  reportFilename,
  setScenarioEnabled,
  setApiToken,
  submitTrainingAction,
  toWsUrl,
  updateScenario,
  updateRunMetadata
} from "./api";
import { apiBase, authMode } from "./config";
import type {
  AssessmentRules,
  CapacityTrendSample,
  ConnectionState,
  MetricsResponse,
  ReplayBookmark,
  Run,
  RunMetadata,
  RunReport,
  CourseTemplate,
  Scenario,
  ScenarioEditorMapMode,
  ScenarioEditorTemplate,
  ScenarioSummary,
  SessionResponse,
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
const replayBookmarkStorageKey = "ship-sim-replay-bookmarks";
type ThreatFilter = "all" | "high" | "medium" | "low";

export function App() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [scenarios, setScenarios] = useState<ScenarioSummary[]>([]);
  const [courseTemplates, setCourseTemplates] = useState<CourseTemplate[]>([]);
  const [selectedCourseTemplateID, setSelectedCourseTemplateID] = useState("");
  const [courseTemplateStatus, setCourseTemplateStatus] = useState("");
  const [courseTemplateError, setCourseTemplateError] = useState("");
  const [selectedScenarioID, setSelectedScenarioID] = useState("");
  const [scenarioEditorText, setScenarioEditorText] = useState("");
  const [scenarioEditorBaselineText, setScenarioEditorBaselineText] = useState("");
  const [scenarioEditorStatus, setScenarioEditorStatus] = useState("");
  const [scenarioEditorError, setScenarioEditorError] = useState("");
  const [scenarioEditorLoading, setScenarioEditorLoading] = useState(false);
  const [scenarioEditorMapMode, setScenarioEditorMapMode] = useState<ScenarioEditorMapMode>("off");
  const [scenarioEditorZoneDraft, setScenarioEditorZoneDraft] = useState<Vec3[]>([]);
  const [selectedScenarioEditorZoneID, setSelectedScenarioEditorZoneID] = useState("");
  const [selectedScenarioEditorVertexIndex, setSelectedScenarioEditorVertexIndex] = useState(0);
  const [run, setRun] = useState<Run | null>(null);
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [snapshotFrames, setSnapshotFrames] = useState<SnapshotFrame[]>([]);
  const [replayFrame, setReplayFrame] = useState<SnapshotFrame | null>(null);
  const [replayIndex, setReplayIndex] = useState(0);
  const [replayPlaying, setReplayPlaying] = useState(false);
  const [replaySpeed, setReplaySpeed] = useState<ReplaySpeed>(1);
  const [report, setReport] = useState<RunReport | null>(null);
  const [metrics, setMetrics] = useState<MetricsResponse | null>(null);
  const [capacityTrend, setCapacityTrend] = useState<CapacityTrendSample[]>([]);
  const [session, setSession] = useState<SessionResponse | null>(null);
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
  const [replayLoading, setReplayLoading] = useState(false);
  const [replayError, setReplayError] = useState("");
  const [replayAnchorStatus, setReplayAnchorStatus] = useState("");
  const [reportExportStatus, setReportExportStatus] = useState("");
  const [replayBookmarks, setReplayBookmarks] = useState<ReplayBookmark[]>(() => loadReplayBookmarks());
  const [streamAttempt, setStreamAttempt] = useState(0);
  const replayActiveRef = useRef(false);

  const displaySnapshot = replayFrame ? snapshotFromFrame(replayFrame) : snapshot;
  const allTracks = replayFrame?.tracks ?? snapshot?.tracks ?? tracks;
  const visibleTracks = allTracks.filter((track) => threatFilter === "all" || track.threat_level === threatFilter);
  const scenarioEditorAnalysis = useMemo(
    () => analyzeScenarioEditor(scenarioEditorText, scenarioEditorBaselineText),
    [scenarioEditorBaselineText, scenarioEditorText]
  );
  const scenarioEditorScenario = scenarioEditorAnalysis.scenario;
  const center = scenarioEditorScenario?.ownship ?? run?.scenario?.ownship ?? defaultCenter;

  useEffect(() => {
    bootstrap().catch(handleError);
  }, []);

  useEffect(() => {
    replayActiveRef.current = replayFrame !== null;
  }, [replayFrame]);

  useEffect(() => {
    saveReplayBookmarks(replayBookmarks);
  }, [replayBookmarks]);

  useEffect(() => {
    const sample = capacityTrendSample(metrics);
    if (!sample) return;
    setCapacityTrend((items) => appendCapacityTrendSample(items, sample));
  }, [metrics]);

  useEffect(() => {
    if (authRequired) return;
    const interval = window.setInterval(() => {
      void getMetrics()
        .then(setMetrics)
        .catch(() => {
          // Metrics refresh is advisory; user actions still surface request errors.
        });
    }, 30_000);
    return () => window.clearInterval(interval);
  }, [authRequired]);

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
            const nextEvents = nextSnapshot.events ?? [];
            setSnapshot(nextSnapshot);
            setTracks(nextSnapshot.tracks ?? []);
            if (nextEvents.length > 0) {
              setEvents((items) => mergeEvents(nextEvents, items));
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
    const replayAnchor = parseReplayAnchor();
    const [nextSession, nextCourseTemplates, nextScenarios, nextRuns, nextMetricsHistory, nextMetrics] = await Promise.all([
      getSession(),
      listCourseTemplates(),
      listScenarios(),
      listRuns(),
      getMetricsHistory(),
      getMetrics()
    ]);
    setSession(nextSession);
    setCourseTemplates(nextCourseTemplates);
    setScenarios(nextScenarios);
    setRuns(nextRuns);
    if (!selectedCourseTemplateID && nextCourseTemplates[0]) {
      setSelectedCourseTemplateID(nextCourseTemplates[0].id);
    }
    setCapacityTrend(nextMetricsHistory.slice(-48));
    setMetrics(nextMetrics);
    if (!selectedScenarioID && nextScenarios[0]) {
      setSelectedScenarioID(nextScenarios[0].id);
    }
    setAuthRequired(false);
    if (replayAnchor) {
      const anchoredRun = nextRuns.find((item) => item.id === replayAnchor.runID) ?? (await getRun(replayAnchor.runID));
      await loadRunData(anchoredRun, replayAnchor.at);
      setReplayAnchorStatus(`Opened replay anchor at ${new Date(replayAnchor.at).toLocaleString()}.`);
    }
  }

  function handleSelectScenario(scenarioID: string) {
    setSelectedScenarioID(scenarioID);
    setScenarioEditorText("");
    setScenarioEditorBaselineText("");
    setScenarioEditorStatus("");
    setScenarioEditorError("");
    setScenarioEditorMapMode("off");
    setScenarioEditorZoneDraft([]);
    setSelectedScenarioEditorZoneID("");
    setSelectedScenarioEditorVertexIndex(0);
  }

  async function handleApplyCourseTemplate() {
    if (!selectedCourseTemplateID) return;
    setBusy(true);
    setError("");
    setCourseTemplateStatus("");
    setCourseTemplateError("");
    try {
      const summary = await createScenarioFromCourseTemplate(selectedCourseTemplateID);
      const nextScenarios = await listScenarios();
      setScenarios(nextScenarios);
      setSelectedScenarioID(summary.id);
      setScenarioEditorText("");
      setScenarioEditorBaselineText("");
      setCourseTemplateStatus(`Created scenario ${summary.name} from course template.`);
    } catch (err) {
      setCourseTemplateError(errorMessage(err));
      handleError(err);
    } finally {
      setBusy(false);
    }
  }

  async function refreshRuns() {
    const [nextRuns, nextMetrics] = await Promise.all([listRuns(), getMetrics()]);
    setRuns(nextRuns);
    setMetrics(nextMetrics);
  }

  async function loadRunData(nextRun: Run, replayAnchorAt = "") {
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
    setReplayError("");
    setReportExportStatus("");
    const [nextZones, nextTracks, eventPage, nextReport] = await Promise.all([
      listZones(nextRun.id),
      listTracks(nextRun.id),
      listEvents(nextRun.id, 50),
      getRunReport(nextRun.id)
    ]);
    const anchor = replayAnchorAt || nextReport.snapshot_range?.to;
    const loadedFrames = await loadSnapshotWindowWithStatus(nextRun.id, nextReport, anchor);
    const nearestAnchor =
      replayAnchorAt && nextReport.replay_mode !== "legacy" ? await getNearestSnapshot(nextRun.id, replayAnchorAt) : null;
    const nextFrames = nearestAnchor ? mergeSnapshotFrames(loadedFrames, [nearestAnchor]) : loadedFrames;
    setZones(nextZones);
    setTracks(nextTracks);
    setSnapshotFrames(nextFrames);
    setReport(nextReport);
    setEvents(mergeEvents(eventPage.items, nextReport.events ?? []));
    const anchoredIndex = nearestAnchor
      ? nextFrames.findIndex((frame) => frame.sampled_at === nearestAnchor.sampled_at && frame.tick === nearestAnchor.tick)
      : -1;
    const latestIndex = Math.max(0, replayAnchorAt && anchoredIndex >= 0 ? anchoredIndex : nextFrames.length - 1);
    const shouldShowReplay = nextFrames.length > 0 && (nextReport.run.status !== "running" || Boolean(replayAnchorAt));
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
      const savedScenario = await createScenario(scenario);
      const nextScenarios = await listScenarios();
      setScenarios(nextScenarios);
      setSelectedScenarioID(savedScenario.id);
      const nextRun = await createRun({ scenario_id: savedScenario.id });
      await loadRunData(nextRun);
      await refreshRuns();
      prependLocalEvent(`Created run ${shortID(nextRun.id)} from uploaded scenario ${savedScenario.name}`, nextRun.id);
    });
  }

  async function handleCopyScenario(name: string) {
    if (!selectedScenarioID) return;
    await withBusy(async () => {
      const copied = await copyScenario(selectedScenarioID, name);
      const nextScenarios = await listScenarios();
      setScenarios(nextScenarios);
      setSelectedScenarioID(copied.id);
    });
  }

  async function handleSetScenarioEnabled(enabled: boolean) {
    if (!selectedScenarioID) return;
    await withBusy(async () => {
      const summary = await setScenarioEnabled(selectedScenarioID, enabled);
      const nextScenarios = await listScenarios();
      setScenarios(nextScenarios);
      setSelectedScenarioID(summary.id);
    });
  }

  async function handleLoadScenarioEditor() {
    if (!selectedScenarioID) return;
    setScenarioEditorLoading(true);
    setScenarioEditorStatus("");
    setScenarioEditorError("");
    try {
      const scenario = await getScenario(selectedScenarioID);
      const text = JSON.stringify(scenario, null, 2);
      setScenarioEditorText(text);
      setScenarioEditorBaselineText(text);
      setScenarioEditorStatus("Scenario JSON loaded.");
      setScenarioEditorMapMode("off");
      setScenarioEditorZoneDraft([]);
      setSelectedScenarioEditorZoneID(scenario.zones?.[0]?.id ?? "");
      setSelectedScenarioEditorVertexIndex(0);
    } catch (err) {
      setScenarioEditorError(errorMessage(err));
    } finally {
      setScenarioEditorLoading(false);
    }
  }

  async function handleSaveScenarioEditor() {
    if (!selectedScenarioID) return;
    setScenarioEditorLoading(true);
    setScenarioEditorStatus("");
    setScenarioEditorError("");
    try {
      if (!scenarioEditorAnalysis.valid) {
        throw new Error("Fix scenario JSON validation issues before saving.");
      }
      const scenario = JSON.parse(scenarioEditorText) as Scenario;
      const summary = await updateScenario(selectedScenarioID, scenario);
      const nextScenarios = await listScenarios();
      setScenarios(nextScenarios);
      setSelectedScenarioID(summary.id);
      setScenarioEditorBaselineText(scenarioEditorText);
      setScenarioEditorStatus(`Saved ${summary.name} v${summary.version ?? 1}.`);
    } catch (err) {
      setScenarioEditorError(errorMessage(err));
    } finally {
      setScenarioEditorLoading(false);
    }
  }

  function handleScenarioEditorMapMode(mode: ScenarioEditorMapMode) {
    setScenarioEditorMapMode(mode);
    setScenarioEditorStatus(
      mode === "sensor"
        ? "Click the map to place a simulated sensor in the scenario JSON."
        : mode === "zone"
          ? "Click the map to add zone vertices, then finish the zone."
          : mode === "vertex"
            ? "Select a zone vertex, then click the map to move it."
          : "Scenario map editing paused."
    );
  }

  function handleScenarioMapClick(point: Vec3) {
    if (scenarioEditorMapMode === "sensor") {
      applyScenarioEditorDraft(
        (scenario) => {
          const sensors = [...(scenario.sensors ?? [])];
          const index = sensors.length + 1;
          sensors.push({
            id: uniqueScenarioID(
              sensors.map((sensor) => sensor.id),
              `sim-sensor-${index}`
            ),
            name: `Simulated Sensor ${index}`,
            kind: "simulated_sensor",
            position: roundVec(point)
          });
          return { ...scenario, sensors };
        },
        "Simulated sensor added from map."
      );
      return;
    }
    if (scenarioEditorMapMode === "zone") {
      const nextDraft = [...scenarioEditorZoneDraft, roundVec(point)];
      setScenarioEditorZoneDraft(nextDraft);
      setScenarioEditorStatus(`Zone vertex ${nextDraft.length} added from map.`);
      return;
    }
    if (scenarioEditorMapMode === "vertex") {
      handleMoveScenarioVertex(point);
    }
  }

  function handleScenarioZoneDraftUndo() {
    setScenarioEditorZoneDraft((points) => {
      const next = points.slice(0, -1);
      setScenarioEditorStatus(next.length === 0 ? "Zone draft cleared." : `Zone draft has ${next.length} vertices.`);
      return next;
    });
  }

  function handleScenarioZoneDraftClear() {
    setScenarioEditorZoneDraft([]);
    setScenarioEditorStatus("Zone draft cleared.");
  }

  function handleScenarioZoneDraftFinish() {
    if (scenarioEditorZoneDraft.length < 3) {
      setScenarioEditorError("Add at least three map vertices before finishing a zone.");
      return;
    }
    applyScenarioEditorDraft(
      (scenario) => {
        const zones = [...(scenario.zones ?? [])];
        const index = zones.length + 1;
        zones.push({
          id: uniqueScenarioID(
            zones.map((zone) => zone.id),
            `training-zone-${index}`
          ),
          name: `Training Zone ${index}`,
          kind: "exercise_boundary",
          polygon: scenarioEditorZoneDraft
        });
        return { ...scenario, zones };
      },
      "Zone added from map draft."
    );
    setScenarioEditorZoneDraft([]);
    setScenarioEditorMapMode("off");
  }

  function handleMoveScenarioVertex(point: Vec3) {
    moveScenarioVertex(
      selectedScenarioEditorZoneID,
      selectedScenarioEditorVertexIndex,
      point,
      "Zone vertex moved from map click."
    );
  }

  function handleDragScenarioVertex(zoneID: string, vertexIndex: number, point: Vec3) {
    setSelectedScenarioEditorZoneID(zoneID);
    setSelectedScenarioEditorVertexIndex(vertexIndex);
    moveScenarioVertex(zoneID, vertexIndex, point, "Zone vertex moved from drag handle.");
  }

  function moveScenarioVertex(zoneID: string, vertexIndex: number, point: Vec3, status: string) {
    if (!zoneID) {
      setScenarioEditorError("Select a zone before moving a vertex.");
      return;
    }
    if (vertexIndex < 0) {
      setScenarioEditorError("Select a zone vertex before moving it.");
      return;
    }
    applyScenarioEditorDraft(
      (scenario) => {
        const zones = [...(scenario.zones ?? [])];
        const zoneIndex = zones.findIndex((zone) => zone.id === zoneID);
        if (zoneIndex < 0) return scenario;
        const zone = zones[zoneIndex];
        const polygon = [...(zone.polygon ?? [])];
        if (vertexIndex >= polygon.length) return scenario;
        polygon[vertexIndex] = roundVec(point);
        zones[zoneIndex] = { ...zone, polygon };
        return { ...scenario, zones };
      },
      status
    );
  }

  function handleDeleteScenarioZone() {
    if (!selectedScenarioEditorZoneID) return;
    applyScenarioEditorDraft(
      (scenario) => {
        const zones = (scenario.zones ?? []).filter((zone) => zone.id !== selectedScenarioEditorZoneID);
        return { ...scenario, zones };
      },
      "Scenario zone removed from JSON."
    );
    setSelectedScenarioEditorZoneID("");
    setSelectedScenarioEditorVertexIndex(0);
  }

  function handleScenarioTemplate(template: ScenarioEditorTemplate) {
    applyScenarioEditorDraft(
      (scenario) => applyScenarioTemplate(scenario, template),
      scenarioTemplateStatus(template)
    );
  }

  function handleScenarioAssessmentRules(rules: AssessmentRules | null) {
    applyScenarioEditorDraft(
      (scenario) => {
        const next = { ...scenario };
        if (rules) {
          next.assessment_rules = rules;
        } else {
          delete next.assessment_rules;
        }
        return next;
      },
      rules ? "Assessment rules applied to scenario JSON." : "Assessment rules cleared from scenario JSON."
    );
  }

  function applyScenarioEditorDraft(mutator: (scenario: Scenario) => Scenario, status: string) {
    const parsed = parseScenarioEditorScenario(scenarioEditorText);
    if (!parsed) {
      setScenarioEditorError("Load valid scenario JSON before using map editing.");
      return;
    }
    const nextScenario = mutator(parsed);
    setScenarioEditorText(JSON.stringify(nextScenario, null, 2));
    setScenarioEditorStatus(status);
    setScenarioEditorError("");
  }

  async function handleSaveRunMetadata(metadata: RunMetadata) {
    if (!run) return;
    await withBusy(async () => {
      const nextRun = await updateRunMetadata(run.id, metadata);
      setRun(nextRun);
      await refreshRuns();
      await refreshReplayData(nextRun.id);
    });
  }

  async function handleAddAnnotation(eventID: string, note: string) {
    if (!run) return;
    await withBusy(async () => {
      await addAnnotation(run.id, { event_id: eventID, note });
      await refreshReplayData(run.id);
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
    const nextFrames = await loadSnapshotWindowWithStatus(runID, nextReport, anchor);
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
      const frames = await loadSnapshotWindowWithStatus(run.id, report, anchor);
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
      const windowFrames = await loadSnapshotWindowWithStatus(run.id, report, nearest.sampled_at);
      const frames = mergeSnapshotFrames(windowFrames, [nearest]);
      const nextIndex = Math.max(0, frames.findIndex((frame) => frame.sampled_at === nearest.sampled_at && frame.tick === nearest.tick));
      setSnapshotFrames(frames);
      setReplayIndex(nextIndex);
      setReplayFrame(frames[nextIndex] ?? nearest);
      setReplayPlaying(false);
      setConnectionState("replay");
    });
  }

  async function handleReplayWindow(direction: "previous" | "next") {
    if (!run || !report?.snapshot_range || snapshotFrames.length === 0) return;
    await withBusy(async () => {
      const edge = direction === "previous" ? snapshotFrames[0]?.sampled_at : snapshotFrames[snapshotFrames.length - 1]?.sampled_at;
      const edgeMS = edge ? new Date(edge).getTime() : NaN;
      const anchorMS = direction === "previous" ? edgeMS - snapshotWindowMS : edgeMS + snapshotWindowMS;
      const anchor = Number.isFinite(anchorMS) ? new Date(anchorMS).toISOString() : edge;
      const frames = await loadSnapshotWindowWithStatus(run.id, report, anchor);
      const nextIndex = direction === "previous" ? Math.max(0, frames.length - 1) : 0;
      setSnapshotFrames(frames);
      setReplayIndex(nextIndex);
      setReplayFrame(frames[nextIndex] ?? null);
      setReplayPlaying(false);
      setConnectionState(frames.length > 0 ? "replay" : "idle");
    });
  }

  async function handleReplayRetry() {
    if (!run) return;
    await withBusy(async () => {
      await refreshReplayData(run.id);
    });
  }

  async function handleCopyReplayAnchor() {
    const anchor = currentReplayAnchor(run, replayFrame, snapshot);
    if (!anchor) {
      setReplayAnchorStatus("Select a replay frame before copying an anchor.");
      return;
    }
    const url = buildReplayAnchorURL(anchor.runID, anchor.at);
    try {
      if (!globalThis.navigator?.clipboard?.writeText) {
        throw new Error("Clipboard API unavailable.");
      }
      await globalThis.navigator.clipboard.writeText(url);
      setReplayAnchorStatus("Replay anchor copied.");
    } catch {
      setReplayAnchorStatus(`Replay anchor ready: ${url}`);
    }
  }

  function handleSaveReplayBookmark() {
    const anchor = currentReplayAnchor(run, replayFrame, snapshot);
    if (!anchor) {
      setReplayAnchorStatus("Select a replay frame before saving a bookmark.");
      return;
    }
    const bookmark: ReplayBookmark = {
      id: `${anchor.runID}:${anchor.at}`,
      run_id: anchor.runID,
      at: anchor.at,
      label: `${run?.name ?? shortID(anchor.runID)} @ ${new Date(anchor.at).toLocaleTimeString()}`,
      created_at: new Date().toISOString()
    };
    setReplayBookmarks((items) => [bookmark, ...items.filter((item) => item.id !== bookmark.id)].slice(0, 20));
    setReplayAnchorStatus("Replay bookmark saved.");
  }

  async function handleOpenReplayBookmark(bookmark: ReplayBookmark) {
    await withBusy(async () => {
      const targetRun = run?.id === bookmark.run_id ? run : runs.find((item) => item.id === bookmark.run_id) ?? (await getRun(bookmark.run_id));
      await loadRunData(targetRun, bookmark.at);
      replaceReplayAnchorURL(bookmark.run_id, bookmark.at);
      setReplayAnchorStatus(`Opened ${bookmark.label}.`);
    });
  }

  function handleDeleteReplayBookmark(id: string) {
    setReplayBookmarks((items) => items.filter((item) => item.id !== id));
    setReplayAnchorStatus("Replay bookmark removed.");
  }

  function handleLiveView() {
    setReplayFrame(null);
    setReplayPlaying(false);
    setConnectionState(snapshot ? "live" : "connecting");
  }

  async function handleExportReport(format: ReportExportFormat) {
    if (!run) return;
    setBusy(true);
    setError("");
    setReportExportStatus(`Exporting ${format.toUpperCase()} report...`);
    try {
      const blob = await downloadRunReport(run.id, format);
      saveBlob(blob, reportFilename(run.id, format));
      setReportExportStatus(`Exported ${format.toUpperCase()} report.`);
    } catch (err) {
      setReportExportStatus(`Report export failed: ${errorMessage(err)}`);
      handleError(err);
    } finally {
      setBusy(false);
    }
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

  async function loadSnapshotWindowWithStatus(runID: string, nextReport: RunReport, anchor?: string) {
    setReplayLoading(true);
    setReplayError("");
    try {
      return await loadSnapshotWindow(runID, nextReport, anchor);
    } catch (err) {
      setReplayError(errorMessage(err));
      throw err;
    } finally {
      setReplayLoading(false);
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
    setCourseTemplates([]);
    setSelectedCourseTemplateID("");
    setCourseTemplateStatus("");
    setCourseTemplateError("");
    setSnapshot(null);
    setSnapshotFrames([]);
    setReplayFrame(null);
    setReplayIndex(0);
    setReplayPlaying(false);
    setReplayError("");
    setTracks([]);
    setReport(null);
    setMetrics(null);
    setCapacityTrend([]);
    setSession(null);
    setReportExportStatus("");
    setEvents([]);
    setTokenInput("");
    setConnectionState("idle");
  }

  function handleError(err: unknown) {
    if (err instanceof ApiRequestError && err.status === 401) {
      setAuthRequired(true);
      setError(authMode === "proxy" ? "Open this deployment through the authenticated proxy." : "Sign in with an access token to use this deployment.");
      return;
    }
    if (err instanceof ApiRequestError && (err.status === 403 || err.status === 404)) {
      setError("No access to this resource, or it no longer exists.");
      return;
    }
    setError(errorMessage(err));
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
        courseTemplates={courseTemplates}
        selectedCourseTemplateID={selectedCourseTemplateID}
        courseTemplateStatus={courseTemplateStatus}
        courseTemplateError={courseTemplateError}
        scenarios={scenarios}
        selectedScenarioID={selectedScenarioID}
        scenarioEditorText={scenarioEditorText}
        scenarioEditorStatus={scenarioEditorStatus}
        scenarioEditorError={scenarioEditorError}
        scenarioEditorLoading={scenarioEditorLoading}
        scenarioEditorValid={scenarioEditorAnalysis.valid}
        scenarioEditorGuidance={scenarioEditorAnalysis.guidance}
        scenarioEditorDiff={scenarioEditorAnalysis.diff}
        scenarioEditorMapMode={scenarioEditorMapMode}
        scenarioEditorZoneDraftCount={scenarioEditorZoneDraft.length}
        selectedScenarioEditorZoneID={selectedScenarioEditorZoneID}
        selectedScenarioEditorVertexIndex={selectedScenarioEditorVertexIndex}
        run={run}
        snapshot={displaySnapshot}
        snapshotFrames={snapshotFrames}
        replayFrame={replayFrame}
        replayIndex={replayIndex}
        replayPlaying={replayPlaying}
        replaySpeed={replaySpeed}
        replayAnchorStatus={replayAnchorStatus}
        reportExportStatus={reportExportStatus}
        replayBookmarks={replayBookmarks}
        report={report}
        metrics={metrics}
        capacityTrend={capacityTrend}
        session={session}
        tracks={allTracks}
        visibleTrackCount={visibleTracks.length}
        threatFilter={threatFilter}
        authRequired={authRequired}
        authMode={authMode}
        tokenInput={tokenInput}
        connectionState={connectionState}
        events={events}
        replayLoading={replayLoading}
        replayError={replayError}
        error={error}
        busy={busy}
        onCreateRun={handleCreateRun}
        onSelectCourseTemplate={setSelectedCourseTemplateID}
        onApplyCourseTemplate={handleApplyCourseTemplate}
        onCommand={handleCommand}
        onAction={handleAction}
        onSelectRun={handleSelectRun}
        onSelectScenario={handleSelectScenario}
        onScenarioFile={handleScenarioFile}
        onCopyScenario={handleCopyScenario}
        onSetScenarioEnabled={handleSetScenarioEnabled}
        onScenarioEditorText={setScenarioEditorText}
        onLoadScenarioEditor={handleLoadScenarioEditor}
        onSaveScenarioEditor={handleSaveScenarioEditor}
        onScenarioEditorMapMode={handleScenarioEditorMapMode}
        onScenarioTemplate={handleScenarioTemplate}
        onScenarioAssessmentRules={handleScenarioAssessmentRules}
        onScenarioEditorZoneSelect={(zoneID) => {
          setSelectedScenarioEditorZoneID(zoneID);
          setSelectedScenarioEditorVertexIndex(0);
        }}
        onScenarioEditorVertexSelect={setSelectedScenarioEditorVertexIndex}
        onScenarioEditorZoneDelete={handleDeleteScenarioZone}
        onScenarioZoneDraftUndo={handleScenarioZoneDraftUndo}
        onScenarioZoneDraftClear={handleScenarioZoneDraftClear}
        onScenarioZoneDraftFinish={handleScenarioZoneDraftFinish}
        onSaveRunMetadata={handleSaveRunMetadata}
        onAddAnnotation={handleAddAnnotation}
        onThreatFilter={setThreatFilter}
        onReplayIndex={handleReplayIndex}
        onReplayStep={handleReplayStep}
        onReplayBoundary={handleReplayBoundary}
        onReplayWindow={handleReplayWindow}
        onReplayRetry={handleReplayRetry}
        onReplayPlayToggle={handleReplayPlayToggle}
        onReplaySpeed={setReplaySpeed}
        onCopyReplayAnchor={handleCopyReplayAnchor}
        onSaveReplayBookmark={handleSaveReplayBookmark}
        onOpenReplayBookmark={handleOpenReplayBookmark}
        onDeleteReplayBookmark={handleDeleteReplayBookmark}
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
          scenarioSensors={scenarioEditorScenario?.sensors ?? []}
          scenarioZones={scenarioEditorScenario?.zones ?? []}
          scenarioZoneDraft={scenarioEditorZoneDraft}
          scenarioEditorMapMode={scenarioEditorMapMode}
          selectedScenarioZoneID={selectedScenarioEditorZoneID}
          selectedScenarioVertexIndex={selectedScenarioEditorVertexIndex}
          selectedTrackID={selectedTrackID}
          onSelectTrack={handleSelectTrack}
          onScenarioMapClick={handleScenarioMapClick}
          onScenarioVertexSelect={(zoneID, vertexIndex) => {
            setSelectedScenarioEditorZoneID(zoneID);
            setSelectedScenarioEditorVertexIndex(vertexIndex);
          }}
          onScenarioVertexMove={handleDragScenarioVertex}
        />
        <TrackList tracks={visibleTracks} selectedTrackID={selectedTrackID} onSelectTrack={handleSelectTrack} />
        <TrackDetail track={selectedLiveTrack} points={trackPoints} onClose={() => handleSelectTrack(null)} />
      </section>
    </main>
  );
}

function parseReplayAnchor() {
  try {
    const url = new URL(globalThis.location.href);
    const runID = url.searchParams.get("run") || url.searchParams.get("replay_run") || "";
    const at = url.searchParams.get("at") || url.searchParams.get("replay_at") || "";
    const atMS = at ? new Date(at).getTime() : NaN;
    if (!runID || !Number.isFinite(atMS)) return null;
    return { runID, at };
  } catch {
    return null;
  }
}

function currentReplayAnchor(run: Run | null, replayFrame: SnapshotFrame | null, snapshot: Snapshot | null) {
  const at = replayFrame?.sampled_at ?? snapshot?.time ?? "";
  if (!run || !at) return null;
  return { runID: run.id, at };
}

function buildReplayAnchorURL(runID: string, at: string) {
  const url = new URL(globalThis.location.href);
  url.searchParams.set("run", runID);
  url.searchParams.set("at", at);
  url.searchParams.delete("replay_run");
  url.searchParams.delete("replay_at");
  return url.toString();
}

function replaceReplayAnchorURL(runID: string, at: string) {
  try {
    globalThis.history.replaceState(null, "", buildReplayAnchorURL(runID, at));
  } catch {
    // History updates are only a convenience for browser review sessions.
  }
}

function loadReplayBookmarks() {
  try {
    const raw = globalThis.localStorage?.getItem(replayBookmarkStorageKey);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as ReplayBookmark[];
    return Array.isArray(parsed) ? parsed.filter(validReplayBookmark).slice(0, 20) : [];
  } catch {
    return [];
  }
}

function saveReplayBookmarks(bookmarks: ReplayBookmark[]) {
  try {
    globalThis.localStorage?.setItem(replayBookmarkStorageKey, JSON.stringify(bookmarks.slice(0, 20)));
  } catch {
    // A blocked localStorage should not break replay review.
  }
}

function validReplayBookmark(value: ReplayBookmark) {
  return (
    typeof value?.id === "string" &&
    typeof value.run_id === "string" &&
    typeof value.at === "string" &&
    Number.isFinite(new Date(value.at).getTime()) &&
    typeof value.label === "string" &&
    typeof value.created_at === "string"
  );
}

function parseScenarioEditorScenario(text: string) {
  try {
    return JSON.parse(text) as Scenario;
  } catch {
    return null;
  }
}

function uniqueScenarioID(existing: string[], base: string) {
  const taken = new Set(existing.filter(Boolean));
  if (!taken.has(base)) return base;
  let index = 2;
  while (taken.has(`${base}-${index}`)) {
    index += 1;
  }
  return `${base}-${index}`;
}

function applyScenarioTemplate(scenario: Scenario, template: ScenarioEditorTemplate): Scenario {
  const center = roundVec(scenario.ownship ?? defaultCenter);
  if (template === "review_sensor") {
    const sensors = [...(scenario.sensors ?? [])];
    sensors.push({
      id: uniqueScenarioID(
        sensors.map((sensor) => sensor.id),
        "sim-sensor-template"
      ),
      name: "Template Simulated Sensor",
      kind: "simulated_sensor",
      position: center
    });
    return { ...scenario, sensors };
  }

  const zones = [...(scenario.zones ?? [])];
  if (template === "focus_zone") {
    zones.push({
      id: uniqueScenarioID(
        zones.map((zone) => zone.id),
        "template-focus-zone"
      ),
      name: "Template Focus Zone",
      kind: "annotation_area",
      polygon: rectangleAround(center, 0.16, 0.1)
    });
    return { ...scenario, zones };
  }

  zones.push({
    id: uniqueScenarioID(
      zones.map((zone) => zone.id),
      "template-exercise-boundary"
    ),
    name: "Template Exercise Boundary",
    kind: "exercise_boundary",
    polygon: rectangleAround(center, 0.32, 0.22)
  });
  return { ...scenario, zones };
}

function scenarioTemplateStatus(template: ScenarioEditorTemplate) {
  switch (template) {
    case "review_sensor":
      return "Simulated sensor template added.";
    case "focus_zone":
      return "Focus zone template added.";
    case "exercise_boundary":
      return "Exercise boundary template added.";
  }
}

function rectangleAround(center: Vec3, lonDelta: number, latDelta: number): Vec3[] {
  return [
    { lon: roundCoordinate(center.lon - lonDelta), lat: roundCoordinate(center.lat - latDelta), alt_m: 0 },
    { lon: roundCoordinate(center.lon + lonDelta), lat: roundCoordinate(center.lat - latDelta), alt_m: 0 },
    { lon: roundCoordinate(center.lon + lonDelta), lat: roundCoordinate(center.lat + latDelta), alt_m: 0 },
    { lon: roundCoordinate(center.lon - lonDelta), lat: roundCoordinate(center.lat + latDelta), alt_m: 0 }
  ];
}

function roundVec(point: Vec3): Vec3 {
  return {
    lon: roundCoordinate(point.lon),
    lat: roundCoordinate(point.lat),
    alt_m: Math.round((point.alt_m ?? 0) * 10) / 10
  };
}

function roundCoordinate(value: number) {
  return Math.round(value * 1_000_000) / 1_000_000;
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

function analyzeScenarioEditor(text: string, baselineText: string) {
  const trimmed = text.trim();
  if (!trimmed) {
    return { valid: false, guidance: ["Load or paste scenario JSON before saving."], diff: "No scenario JSON loaded.", scenario: null };
  }
  let scenario: Scenario;
  try {
    scenario = JSON.parse(trimmed) as Scenario;
  } catch (err) {
    return { valid: false, guidance: [`Invalid JSON: ${errorMessage(err)}`], diff: "Cannot compare until JSON is valid.", scenario: null };
  }
  const guidance = validateScenarioDraft(scenario);
  const diff = scenarioDiffSummary(trimmed, baselineText);
  return {
    valid: guidance.length === 0,
    guidance: guidance.length === 0 ? ["Scenario JSON passes client preflight."] : guidance,
    diff,
    scenario
  };
}

function validateScenarioDraft(scenario: Scenario) {
  const guidance: string[] = [];
  if (!scenario.name?.trim()) guidance.push("name is required.");
  if (!Number.isFinite(scenario.seed)) guidance.push("seed is required.");
  if (!Number.isFinite(scenario.tick_hz) || scenario.tick_hz < 1 || scenario.tick_hz > 60) {
    guidance.push("tick_hz must be between 1 and 60.");
  }
  if (!Number.isFinite(scenario.snapshot_hz) || scenario.snapshot_hz < 1 || scenario.snapshot_hz > 20) {
    guidance.push("snapshot_hz must be between 1 and 20.");
  }
  if (Number.isFinite(scenario.tick_hz) && Number.isFinite(scenario.snapshot_hz) && scenario.snapshot_hz > scenario.tick_hz) {
    guidance.push("snapshot_hz must be less than or equal to tick_hz.");
  }
  if (!scenario.ownship || !validLonLat(scenario.ownship)) guidance.push("ownship lon/lat must be valid.");
  if (!Array.isArray(scenario.sensors) || scenario.sensors.length === 0) guidance.push("at least one simulated sensor is required.");
  if (Array.isArray(scenario.sensors)) {
    const sensorIDs = new Set<string>();
    scenario.sensors.forEach((sensor, index) => {
      if (!sensor.id?.trim()) guidance.push(`sensors[${index}].id is required.`);
      if (sensor.id && sensorIDs.has(sensor.id)) guidance.push(`sensor id '${sensor.id}' must be unique.`);
      if (sensor.id) sensorIDs.add(sensor.id);
      if (!sensor.name?.trim()) guidance.push(`sensors[${index}].name is required.`);
      if (!sensor.kind?.trim()) guidance.push(`sensors[${index}].kind is required.`);
      if (!sensor.position || !validLonLat(sensor.position)) guidance.push(`sensors[${index}].position lon/lat must be valid.`);
    });
  }
  if (!Array.isArray(scenario.zones)) guidance.push("zones must be an array.");
  if (Array.isArray(scenario.zones)) {
    const zoneIDs = new Set<string>();
    scenario.zones.forEach((zone, index) => {
      if (!zone.id?.trim()) guidance.push(`zones[${index}].id is required.`);
      if (zone.id && zoneIDs.has(zone.id)) guidance.push(`zone id '${zone.id}' must be unique.`);
      if (zone.id) zoneIDs.add(zone.id);
      if (!zone.name?.trim()) guidance.push(`zones[${index}].name is required.`);
      if (!zone.kind?.trim()) guidance.push(`zones[${index}].kind is required.`);
      if (!Array.isArray(zone.polygon) || zone.polygon.length < 3) {
        guidance.push(`zones[${index}].polygon must contain at least three points.`);
      } else if (zone.polygon.some((point) => !validLonLat(point))) {
        guidance.push(`zones[${index}].polygon lon/lat values must be valid.`);
      }
    });
  }
  if (!Number.isFinite(scenario.initial_contacts) || scenario.initial_contacts < 0) guidance.push("initial_contacts must be zero or greater.");
  if ((scenario.initial_contacts ?? 0) + (scenario.tracks?.length ?? 0) > 100) guidance.push("scenario may seed at most 100 tracks.");
  if (Array.isArray(scenario.allowed_actions)) {
    const allowedActions = new Set(["maneuver", "decoy", "training_response"]);
    scenario.allowed_actions.forEach((action) => {
      if (!allowedActions.has(String(action))) guidance.push(`allowed action '${action}' is not supported.`);
    });
  }
  if (Array.isArray(scenario.tracks)) {
    scenario.tracks.forEach((track, index) => {
      if (track.position && !validLonLat(track.position)) guidance.push(`tracks[${index}].position lon/lat must be valid.`);
    });
  }
  if (
    scenario.assessment_profile &&
    !["standard", "quick_review", "extended_review"].includes(String(scenario.assessment_profile))
  ) {
    guidance.push("assessment_profile must be standard, quick_review, or extended_review.");
  }
  if (scenario.assessment_rules) {
    guidance.push(...validateAssessmentRulesDraft(scenario.assessment_rules));
  }
  return guidance;
}

function validateAssessmentRulesDraft(rules: AssessmentRules) {
  const guidance: string[] = [];
  if (rules.name?.trim() && unsafeAssessmentRuleText(rules.name)) {
    guidance.push("assessment_rules.name must preserve the training-record boundary.");
  }
  if (!validNumberInRange(rules.action_target, 1, 1000)) {
    guidance.push("assessment_rules.action_target must be between 1 and 1000.");
  }
  if (!validNumberInRange(rules.replay_target, 1, 1_000_000)) {
    guidance.push("assessment_rules.replay_target must be between 1 and 1000000.");
  }
  const weights = [
    ["action_weight", rules.action_weight],
    ["replay_weight", rules.replay_weight],
    ["context_weight", rules.context_weight]
  ] as const;
  weights.forEach(([name, value]) => {
    if (!validNumberInRange(value, 0, 100)) {
      guidance.push(`assessment_rules.${name} must be between 0 and 100.`);
    }
  });
  const weightValues = weights.map(([, value]) => value);
  if (weightValues.every((value) => typeof value === "number" && Number.isFinite(value)) && weightValues.reduce((sum, value) => sum + value, 0) !== 100) {
    guidance.push("assessment_rules weights must sum to 100.");
  }
  return guidance;
}

function validNumberInRange(value: unknown, min: number, max: number) {
  return typeof value === "number" && Number.isFinite(value) && value >= min && value <= max;
}

function unsafeAssessmentRuleText(value: string) {
  const lower = value.toLowerCase();
  return ["tactical", "engagement", "fire-control", "fire control", "weapon", "readiness", "kill"].some((term) => lower.includes(term));
}

function validLonLat(value?: Vec3) {
  if (!value) return false;
  return value.lon >= -180 && value.lon <= 180 && value.lat >= -90 && value.lat <= 90;
}

function scenarioDiffSummary(text: string, baselineText: string) {
  if (!baselineText.trim()) return "No baseline loaded; save will update the selected managed scenario.";
  try {
    const next = JSON.parse(text) as Scenario & Record<string, unknown>;
    const baseline = JSON.parse(baselineText) as Scenario & Record<string, unknown>;
    const keys = Array.from(new Set([...Object.keys(next), ...Object.keys(baseline)])).sort();
    const changed = keys.filter((key) => JSON.stringify(next[key]) !== JSON.stringify(baseline[key]));
    if (changed.length === 0) return "No top-level changes.";
    const details = scenarioDiffDetails(next, baseline);
    const changedLabel = `Changed: ${changed.slice(0, 8).join(", ")}${changed.length > 8 ? "..." : ""}.`;
    return details.length > 0 ? `${changedLabel} ${details.slice(0, 5).join("; ")}.` : changedLabel;
  } catch {
    return "Cannot compare until JSON is valid.";
  }
}

function scenarioDiffDetails(next: Scenario, baseline: Scenario) {
  const details: string[] = [];
  details.push(...scenarioCollectionDiff("sensors", next.sensors, baseline.sensors));
  details.push(...scenarioCollectionDiff("zones", next.zones, baseline.zones));
  details.push(...scenarioCollectionDiff("tracks", next.tracks, baseline.tracks));
  details.push(...scenarioCollectionDiff("contacts", next.contacts, baseline.contacts));
  details.push(...scenarioActionDiff(next.allowed_actions, baseline.allowed_actions));
  if (JSON.stringify(next.assessment_rules ?? null) !== JSON.stringify(baseline.assessment_rules ?? null)) {
    details.push("assessment rules changed");
  }
  if (next.version !== baseline.version) {
    details.push(`version ${baseline.version ?? 1}->${next.version ?? 1}`);
  }
  return details;
}

type ScenarioDiffItem = { id?: string; name?: string; polygon?: Vec3[] } | string;

function scenarioCollectionDiff(label: string, nextItems: ScenarioDiffItem[] = [], baselineItems: ScenarioDiffItem[] = []) {
  const nextMap = scenarioItemMap(nextItems);
  const baselineMap = scenarioItemMap(baselineItems);
  const added = Array.from(nextMap.keys()).filter((id) => !baselineMap.has(id));
  const removed = Array.from(baselineMap.keys()).filter((id) => !nextMap.has(id));
  const edited = Array.from(nextMap.keys()).filter((id) => {
    const baselineItem = baselineMap.get(id);
    return baselineItem !== undefined && JSON.stringify(nextMap.get(id)) !== JSON.stringify(baselineItem);
  });
  const details: string[] = [];
  if (added.length > 0) {
    details.push(`${label} added ${formatDiffLabels(added.map((id) => scenarioItemLabel(nextMap.get(id), id)))}`);
  }
  if (removed.length > 0) {
    details.push(`${label} removed ${formatDiffLabels(removed.map((id) => scenarioItemLabel(baselineMap.get(id), id)))}`);
  }
  if (edited.length > 0) {
    details.push(`${label} edited ${formatDiffLabels(edited.map((id) => scenarioItemChangeLabel(id, nextMap.get(id), baselineMap.get(id))))}`);
  }
  if (details.length === 0 && nextItems.length !== baselineItems.length) {
    details.push(`${label} ${baselineItems.length}->${nextItems.length}`);
  }
  return details;
}

function scenarioActionDiff(nextActions: TrainingAction[] = [], baselineActions: TrainingAction[] = []) {
  return scenarioCollectionDiff("actions", nextActions, baselineActions);
}

function scenarioItemMap(items: ScenarioDiffItem[]) {
  const entries = items.map((item, index) => [scenarioItemID(item, index), item] as const);
  return new Map(entries);
}

function scenarioItemID(item: ScenarioDiffItem, index: number) {
  if (typeof item === "string") return item;
  if (typeof item.id === "string" && item.id) return item.id;
  if (typeof item.name === "string" && item.name) return item.name;
  return `#${index + 1}`;
}

function scenarioItemLabel(item: ScenarioDiffItem | undefined, fallback: string) {
  if (!item || typeof item === "string") return item ?? fallback;
  return item.name && item.id && item.name !== item.id ? `${item.name} (${item.id})` : item.id ?? item.name ?? fallback;
}

function scenarioItemChangeLabel(id: string, nextItem: ScenarioDiffItem | undefined, baselineItem: ScenarioDiffItem | undefined) {
  if (isZoneDiffItem(nextItem) && isZoneDiffItem(baselineItem)) {
    if (nextItem.polygon.length !== baselineItem.polygon.length) {
      return `${id} vertices ${baselineItem.polygon.length}->${nextItem.polygon.length}`;
    }
    const vertexIndex = firstChangedVertexIndex(nextItem.polygon, baselineItem.polygon);
    if (vertexIndex >= 0) return `${id} vertex ${vertexIndex + 1}`;
  }
  return scenarioItemLabel(nextItem, id);
}

function isZoneDiffItem(item: ScenarioDiffItem | undefined): item is Zone {
  return Boolean(item && typeof item !== "string" && Array.isArray(item.polygon));
}

function firstChangedVertexIndex(nextPolygon: Vec3[], baselinePolygon: Vec3[]) {
  for (let index = 0; index < nextPolygon.length; index += 1) {
    if (JSON.stringify(nextPolygon[index]) !== JSON.stringify(baselinePolygon[index])) {
      return index;
    }
  }
  return -1;
}

function formatDiffLabels(labels: string[]) {
  return `${labels.slice(0, 3).join(", ")}${labels.length > 3 ? ` +${labels.length - 3}` : ""}`;
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

function capacityTrendSample(metrics: MetricsResponse | null): CapacityTrendSample | null {
  if (!metrics) return null;
  const sampledAtValue = metrics.sampled_at;
  const sampledAt = typeof sampledAtValue === "string" && Number.isFinite(new Date(sampledAtValue).getTime())
    ? sampledAtValue
    : new Date().toISOString();
  return {
    sampled_at: sampledAt,
    snapshot_frames: appMetricNumber(metrics, "snapshot_frames"),
    event_count: appMetricNumber(metrics, "event_count"),
    track_point_count: appMetricNumber(metrics, "track_point_count"),
    contact_count: appMetricNumber(metrics, "contact_count"),
    snapshot_capacity_pressure: appMetricNumber(metrics, "snapshot_capacity_pressure"),
    event_capacity_pressure: appMetricNumber(metrics, "event_capacity_pressure"),
    track_point_capacity_pressure: appMetricNumber(metrics, "track_point_capacity_pressure"),
    snapshot_write_avg_ms: appMetricNumber(metrics, "snapshot_write_avg_ms"),
    snapshot_write_max_ms: appMetricNumber(metrics, "snapshot_write_max_ms"),
    snapshot_write_failures: appMetricNumber(metrics, "snapshot_write_failures"),
    db_table_bytes: appMetricNumber(metrics, "db_table_bytes"),
    db_index_bytes: appMetricNumber(metrics, "db_index_bytes"),
    db_total_bytes: appMetricNumber(metrics, "db_total_bytes")
  };
}

function appendCapacityTrendSample(samples: CapacityTrendSample[], sample: CapacityTrendSample) {
  const next = samples.filter((item) => item.sampled_at !== sample.sampled_at);
  next.push(sample);
  return next
    .sort((a, b) => new Date(a.sampled_at).getTime() - new Date(b.sampled_at).getTime())
    .slice(-48);
}

function appMetricNumber(metrics: MetricsResponse, key: string) {
  const value = metrics[key];
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function shortID(id: string) {
  return id.slice(0, 8);
}

function errorMessage(err: unknown) {
  if (err instanceof ApiRequestError) {
    const details = err.details.length > 0 ? `: ${err.details.join("; ")}` : "";
    return `${err.message}${details}`;
  }
  return err instanceof Error ? err.message : "Request failed.";
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
