// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ControlSidebar } from "./ControlSidebar";
import type { ComponentProps } from "react";
import type { CapacityTrendSample, CourseTemplate, ReplayBookmark, Run, RunReport, ScenarioSummary, SessionResponse, SimEvent, Snapshot, SnapshotFrame, Track } from "./types";

type SidebarProps = ComponentProps<typeof ControlSidebar>;

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("ControlSidebar", () => {
  it("supports token login", () => {
    const onLogin = vi.fn();
    const onTokenInput = vi.fn();

    renderSidebar({ authRequired: true, authMode: "token", tokenInput: "secret", onLogin, onTokenInput });

    fireEvent.change(screen.getByPlaceholderText("Access token"), { target: { value: "next-secret" } });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    expect(onTokenInput).toHaveBeenCalledWith("next-secret");
    expect(onLogin).toHaveBeenCalledTimes(1);
  });

  it("supports authenticated proxy retry without a token field", () => {
    const onLogin = vi.fn();

    renderSidebar({ authRequired: true, authMode: "proxy", onLogin });

    expect(screen.queryByPlaceholderText("Access token")).toBeNull();
    expect(screen.getByText("Authenticated proxy session required")).not.toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    expect(onLogin).toHaveBeenCalledTimes(1);
  });

  it("shows current access role and permissions", () => {
    renderSidebar({ session: sampleSession({ role: "viewer", permissions: { ...sampleSession().permissions, create_runs: false, control_runs: false, manage_scenarios: false } }) });

    expect(screen.getByLabelText("Access summary").textContent).toContain("viewer");
    expect(screen.getByLabelText("Access summary").textContent).toContain("map");
    expect(screen.getByLabelText("Access summary").textContent).toContain("operator-a");
    expect(screen.getByLabelText("Session permissions").textContent).toContain("View");
    expect(screen.getByLabelText("Session permissions").textContent).toContain("Scenarios");
    expect(screen.getByText("Scenarios").getAttribute("data-enabled")).toBe("false");
  });

  it("supports creating a managed scenario from a course template", () => {
    const onSelectCourseTemplate = vi.fn();
    const onApplyCourseTemplate = vi.fn();

    renderSidebar({
      courseTemplates: [
        sampleCourseTemplate(),
        sampleCourseTemplate({ id: "extended-review", name: "Extended Review", enabled: false, source: "database" })
      ],
      selectedCourseTemplateID: "quick-review",
      courseTemplateStatus: "Created scenario Quick Review Baseline from course template.",
      onSelectCourseTemplate,
      onApplyCourseTemplate
    });

    expect(screen.getByLabelText("Course template detail").textContent).toContain("Scenario Demo Scenario");
    expect(screen.getByLabelText("Course template detail").textContent).toContain("Metadata profile, trainees");
    expect(screen.getByLabelText("Course template detail").textContent).toContain("Rules quick_review_record");
    expect(screen.getByLabelText("Course template detail").textContent).toContain("Weights 30/30/40");
    expect(screen.getByLabelText("Course template detail").textContent).toContain("Review timeline");

    fireEvent.change(screen.getByDisplayValue("Quick Review"), { target: { value: "extended-review" } });
    fireEvent.click(screen.getByRole("button", { name: /create scenario/i }));

    expect(onSelectCourseTemplate).toHaveBeenCalledWith("extended-review");
    expect(onApplyCourseTemplate).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Created scenario Quick Review Baseline from course template.")).not.toBeNull();
  });

  it("covers run creation and start pause stop controls", () => {
    const onCreateRun = vi.fn();
    const onCommand = vi.fn();

    renderSidebar({ onCreateRun, onCommand });

    fireEvent.click(screen.getByRole("button", { name: /new run/i }));
    fireEvent.click(screen.getByRole("button", { name: /^start$/i }));
    fireEvent.click(screen.getByRole("button", { name: /^pause$/i }));
    fireEvent.click(screen.getByRole("button", { name: /^stop$/i }));

    expect(onCreateRun).toHaveBeenCalledTimes(1);
    expect(onCommand).toHaveBeenCalledWith("start");
    expect(onCommand).toHaveBeenCalledWith("pause");
    expect(onCommand).toHaveBeenCalledWith("stop");
  });

  it("covers replay paging, event jumps, loading retry, and playback controls", () => {
    const onReplayBoundary = vi.fn();
    const onReplayIndex = vi.fn();
    const onReplayPlayToggle = vi.fn();
    const onReplayStep = vi.fn();
    const onReplayWindow = vi.fn();
    const onReplayRetry = vi.fn();
    const onJumpToEvent = vi.fn();
    const event = sampleEvent();

    renderSidebar({
      events: [event],
      report: sampleReport({ events: [event] }),
      replayError: "Snapshot window failed.",
      onReplayBoundary,
      onReplayIndex,
      onReplayPlayToggle,
      onReplayStep,
      onReplayWindow,
      onReplayRetry,
      onJumpToEvent
    });

    fireEvent.click(screen.getByRole("button", { name: "Jump to start" }));
    fireEvent.click(screen.getByRole("button", { name: "Previous frame" }));
    fireEvent.click(screen.getByRole("button", { name: "Play replay" }));
    fireEvent.click(screen.getByRole("button", { name: "Next frame" }));
    fireEvent.click(screen.getByRole("button", { name: "Jump to end" }));
    fireEvent.click(screen.getByRole("button", { name: /previous window/i }));
    fireEvent.click(screen.getByRole("button", { name: /next window/i }));
    fireEvent.change(screen.getByRole("slider"), { target: { value: "2" } });
    fireEvent.click(screen.getAllByRole("button", { name: /jump to maneuver/i })[0]);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    expect(onReplayBoundary).toHaveBeenCalledWith("start");
    expect(onReplayBoundary).toHaveBeenCalledWith("end");
    expect(onReplayStep).toHaveBeenCalledWith(-1);
    expect(onReplayStep).toHaveBeenCalledWith(1);
    expect(onReplayPlayToggle).toHaveBeenCalledTimes(1);
    expect(onReplayWindow).toHaveBeenCalledWith("previous");
    expect(onReplayWindow).toHaveBeenCalledWith("next");
    expect(onReplayIndex).toHaveBeenCalledWith(2);
    expect(onJumpToEvent).toHaveBeenCalledWith(event);
    expect(onReplayRetry).toHaveBeenCalledTimes(1);
  });

  it("shows replay quality insights", () => {
    renderSidebar();

    expect(screen.getByLabelText("Replay quality summary").textContent).toContain("Coverage");
    expect(screen.getByLabelText("Replay quality summary").textContent).toContain("Max gap");
    expect(screen.getByLabelText("Replay quality summary").textContent).toContain("Events");
  });

  it("handles report lists returned as null", () => {
    const report = sampleReport();
    report.events = null as unknown as RunReport["events"];
    report.annotations = null as unknown as RunReport["annotations"];
    report.event_audit.action_stats = null as unknown as RunReport["event_audit"]["action_stats"];
    report.event_audit.actor_stats = null as unknown as RunReport["event_audit"]["actor_stats"];

    renderSidebar({ report });

    expect(screen.getByText("No actions yet")).not.toBeNull();
    expect(screen.getByText("Actors 0")).not.toBeNull();
  });

  it("supports replay anchors and saved bookmarks", () => {
    const onCopyReplayAnchor = vi.fn();
    const onSaveReplayBookmark = vi.fn();
    const onOpenReplayBookmark = vi.fn();
    const onDeleteReplayBookmark = vi.fn();
    const bookmark = sampleBookmark();

    renderSidebar({
      replayAnchorStatus: "Replay anchor copied.",
      replayBookmarks: [bookmark],
      onCopyReplayAnchor,
      onSaveReplayBookmark,
      onOpenReplayBookmark,
      onDeleteReplayBookmark
    });

    fireEvent.click(screen.getByRole("button", { name: "Copy replay anchor" }));
    fireEvent.click(screen.getByRole("button", { name: "Save replay bookmark" }));
    fireEvent.click(screen.getByRole("button", { name: bookmark.label }));
    fireEvent.click(screen.getByRole("button", { name: `Delete bookmark ${bookmark.label}` }));

    expect(onCopyReplayAnchor).toHaveBeenCalledTimes(1);
    expect(onSaveReplayBookmark).toHaveBeenCalledTimes(1);
    expect(onOpenReplayBookmark).toHaveBeenCalledWith(bookmark);
    expect(onDeleteReplayBookmark).toHaveBeenCalledWith(bookmark.id);
    expect(screen.getByText("Replay anchor copied.")).not.toBeNull();
  });

  it("supports managed scenario JSON editing", () => {
    const onLoadScenarioEditor = vi.fn();
    const onSaveScenarioEditor = vi.fn();
    const onScenarioEditorText = vi.fn();

    renderSidebar({
      scenarios: [sampleScenario({ source: "database" })],
      scenarioEditorText: "{\"name\":\"Demo Scenario\"}",
      scenarioEditorValid: true,
      scenarioEditorGuidance: ["Scenario JSON passes client preflight."],
      scenarioEditorDiff: "Changed: name.",
      scenarioEditorStatus: "Scenario JSON loaded.",
      onLoadScenarioEditor,
      onSaveScenarioEditor,
      onScenarioEditorText
    });

    fireEvent.change(screen.getByPlaceholderText("Load scenario JSON"), { target: { value: "{\"name\":\"Updated\"}" } });
    fireEvent.click(screen.getByRole("button", { name: /load/i }));
    fireEvent.click(screen.getByRole("button", { name: "Save scenario JSON" }));

    expect(onScenarioEditorText).toHaveBeenCalledWith("{\"name\":\"Updated\"}");
    expect(onLoadScenarioEditor).toHaveBeenCalledTimes(1);
    expect(onSaveScenarioEditor).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Database scenario editable")).not.toBeNull();
    expect(screen.getByText("Scenario JSON passes client preflight.")).not.toBeNull();
    expect(screen.getByText("Changed: name.")).not.toBeNull();
    expect(screen.getByText("Scenario JSON loaded.")).not.toBeNull();
  });

  it("supports scenario upload and enable disable controls", () => {
    const onScenarioFile = vi.fn();
    const onSetScenarioEnabled = vi.fn();
    const file = new File(["{}"], "scenario.json", { type: "application/json" });

    const { container, unmount } = renderSidebar({
      scenarios: [sampleScenario({ source: "database", enabled: true })],
      onScenarioFile,
      onSetScenarioEnabled
    });

    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });
    fireEvent.click(screen.getByRole("button", { name: "Disable" }));

    expect(onScenarioFile).toHaveBeenCalledWith(file);
    expect(onSetScenarioEnabled).toHaveBeenCalledWith(false);

    unmount();
    renderSidebar({
      scenarios: [sampleScenario({ source: "database", enabled: false })],
      onSetScenarioEnabled
    });
    fireEvent.click(screen.getByRole("button", { name: "Enable" }));

    expect(onSetScenarioEnabled).toHaveBeenCalledWith(true);
  });

  it("supports scenario map editing controls", () => {
    const onScenarioEditorMapMode = vi.fn();
    const onScenarioTemplate = vi.fn();
    const onScenarioAssessmentRules = vi.fn();
    const onScenarioEditorZoneSelect = vi.fn();
    const onScenarioEditorVertexSelect = vi.fn();
    const onScenarioEditorZoneDelete = vi.fn();
    const onScenarioZoneDraftUndo = vi.fn();
    const onScenarioZoneDraftClear = vi.fn();
    const onScenarioZoneDraftFinish = vi.fn();
    const scenarioEditorText = JSON.stringify({
      ...sampleRun().scenario,
      zones: [
        {
          id: "zone-a",
          name: "Zone A",
          kind: "exercise_boundary",
          polygon: [
            { lon: 121.5, lat: 31.2, alt_m: 0 },
            { lon: 121.6, lat: 31.2, alt_m: 0 },
            { lon: 121.6, lat: 31.3, alt_m: 0 }
          ]
        },
        {
          id: "zone-b",
          name: "Zone B",
          kind: "focus_zone",
          polygon: [
            { lon: 121.4, lat: 31.1, alt_m: 0 },
            { lon: 121.5, lat: 31.1, alt_m: 0 },
            { lon: 121.5, lat: 31.2, alt_m: 0 }
          ]
        }
      ]
    });

    renderSidebar({
      scenarios: [sampleScenario({ source: "database" })],
      scenarioEditorText,
      scenarioEditorValid: true,
      scenarioEditorMapMode: "zone",
      scenarioEditorZoneDraftCount: 3,
      selectedScenarioEditorZoneID: "zone-a",
      selectedScenarioEditorVertexIndex: 0,
      onScenarioEditorMapMode,
      onScenarioTemplate,
      onScenarioAssessmentRules,
      onScenarioEditorZoneSelect,
      onScenarioEditorVertexSelect,
      onScenarioEditorZoneDelete,
      onScenarioZoneDraftUndo,
      onScenarioZoneDraftClear,
      onScenarioZoneDraftFinish
    });

    fireEvent.click(screen.getByRole("button", { name: "Scenario map mode sensor" }));
    fireEvent.click(screen.getByRole("button", { name: "Scenario map mode zone" }));
    fireEvent.click(screen.getByRole("button", { name: "Scenario map mode vertex" }));
    fireEvent.change(screen.getByLabelText("Scenario geometry zone"), { target: { value: "zone-b" } });
    fireEvent.change(screen.getByLabelText("Scenario geometry vertex"), { target: { value: "1" } });
    fireEvent.click(screen.getByRole("button", { name: "Delete scenario zone" }));
    fireEvent.click(screen.getByRole("button", { name: "Add simulated sensor template" }));
    fireEvent.click(screen.getByRole("button", { name: "Add exercise boundary template" }));
    fireEvent.click(screen.getByRole("button", { name: "Add focus zone template" }));
    fireEvent.change(screen.getByPlaceholderText("Rule name"), { target: { value: "custom_record" } });
    fireEvent.change(screen.getByRole("spinbutton", { name: "Assessment action target" }), { target: { value: "5" } });
    fireEvent.change(screen.getByRole("spinbutton", { name: "Assessment replay target" }), { target: { value: "12" } });
    fireEvent.click(screen.getByRole("button", { name: "Apply assessment rules" }));
    fireEvent.click(screen.getByRole("button", { name: "Clear assessment rules" }));
    fireEvent.click(screen.getByRole("button", { name: "Finish scenario zone" }));
    fireEvent.click(screen.getByRole("button", { name: "Undo scenario zone point" }));
    fireEvent.click(screen.getByRole("button", { name: "Clear scenario zone draft" }));

    expect(onScenarioEditorMapMode).toHaveBeenCalledWith("sensor");
    expect(onScenarioEditorMapMode).toHaveBeenCalledWith("zone");
    expect(onScenarioEditorMapMode).toHaveBeenCalledWith("vertex");
    expect(onScenarioEditorZoneSelect).toHaveBeenCalledWith("zone-b");
    expect(onScenarioEditorVertexSelect).toHaveBeenCalledWith(1);
    expect(onScenarioEditorZoneDelete).toHaveBeenCalledTimes(1);
    expect(onScenarioTemplate).toHaveBeenCalledWith("review_sensor");
    expect(onScenarioTemplate).toHaveBeenCalledWith("exercise_boundary");
    expect(onScenarioTemplate).toHaveBeenCalledWith("focus_zone");
    expect(onScenarioAssessmentRules).toHaveBeenCalledWith({
      name: "custom_record",
      action_target: 5,
      replay_target: 12,
      action_weight: 34,
      replay_weight: 33,
      context_weight: 33
    });
    expect(onScenarioAssessmentRules).toHaveBeenCalledWith(null);
    expect(onScenarioZoneDraftFinish).toHaveBeenCalledTimes(1);
    expect(onScenarioZoneDraftUndo).toHaveBeenCalledTimes(1);
    expect(onScenarioZoneDraftClear).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Draft 3")).not.toBeNull();
  });

  it("shows capacity metrics", () => {
    renderSidebar();

    expect(screen.getByText("Capacity")).not.toBeNull();
    expect(screen.getByText("16")).not.toBeNull();
    expect(screen.getByText("240")).not.toBeNull();
    expect(screen.getByText("60")).not.toBeNull();
    expect(screen.getByText("Snapshot pressure 12%")).not.toBeNull();
    expect(screen.getByText("Event pressure 3%")).not.toBeNull();
    expect(screen.getByText("Point pressure 12%")).not.toBeNull();
    expect(screen.getByText("Snapshots/run 1,000")).not.toBeNull();
    expect(screen.getByText("Events/run 500")).not.toBeNull();
    expect(screen.getByText("Points/run 2,000")).not.toBeNull();
    expect(screen.getByText("Last 4ms")).not.toBeNull();
    expect(screen.getByLabelText("Database planning").textContent).toContain("Tables 1.5 MB");
    expect(screen.getByLabelText("Database planning").textContent).toContain("Indexes 391 KB");
    expect(screen.getByLabelText("Database planning").textContent).toContain("Index/table 25%");
    expect(screen.getByLabelText("Capacity trend").textContent).toContain("Trend 2 samples");
    expect(screen.getByLabelText("Capacity trend").textContent).toContain("Frames +20");
    expect(screen.getByLabelText("Capacity trend").textContent).toContain("DB 1.9 MB");
    expect(screen.getByLabelText("Capacity trend").textContent).toContain("Write avg 3ms -> 5ms");
    expect(screen.getByLabelText("Capacity trend").textContent).toContain("Archive steady");
    expect(screen.getByLabelText("Capacity by run").textContent).toContain("Run 1");
    expect(screen.getByLabelText("Capacity by run").textContent).toContain("E 16");
    expect(screen.getByLabelText("Capacity by run").textContent).toContain("P 240");
    expect(screen.getByLabelText("Capacity by run").textContent).toContain("C 60");
    expect(screen.getByLabelText("Capacity by run").textContent).toContain("12%");
  });

  it("covers report exports", () => {
    const onExportReport = vi.fn();

    renderSidebar({ onExportReport, reportExportStatus: "Report export failed: forbidden" });

    fireEvent.click(screen.getByRole("button", { name: "Export report JSON" }));
    fireEvent.click(screen.getByRole("button", { name: "Export report CSV" }));
    fireEvent.click(screen.getByRole("button", { name: "Export report HTML" }));
    fireEvent.click(screen.getByRole("button", { name: "Export report PDF" }));

    expect(onExportReport).toHaveBeenCalledWith("json");
    expect(onExportReport).toHaveBeenCalledWith("csv");
    expect(onExportReport).toHaveBeenCalledWith("html");
    expect(onExportReport).toHaveBeenCalledWith("pdf");
    expect(screen.getByText("Report export failed: forbidden")).not.toBeNull();
  });
});

function renderSidebar(overrides: Partial<SidebarProps> = {}) {
  return render(<ControlSidebar {...defaultSidebarProps(overrides)} />);
}

function defaultSidebarProps(overrides: Partial<SidebarProps> = {}): SidebarProps {
  return {
    runs: [sampleRun()],
    courseTemplates: [sampleCourseTemplate()],
    selectedCourseTemplateID: "quick-review",
    courseTemplateStatus: "",
    courseTemplateError: "",
    scenarios: [sampleScenario()],
    selectedScenarioID: "demo",
    scenarioEditorText: "",
    scenarioEditorStatus: "",
    scenarioEditorError: "",
    scenarioEditorLoading: false,
    scenarioEditorValid: false,
    scenarioEditorGuidance: ["Load or paste scenario JSON before saving."],
    scenarioEditorDiff: "No scenario JSON loaded.",
    scenarioEditorMapMode: "off",
    scenarioEditorZoneDraftCount: 0,
    selectedScenarioEditorZoneID: "",
    selectedScenarioEditorVertexIndex: 0,
    run: sampleRun(),
    snapshot: sampleSnapshot(),
    snapshotFrames: sampleFrames(),
    replayFrame: sampleFrames()[1],
    replayIndex: 1,
    replayPlaying: false,
    replaySpeed: 1,
    replayAnchorStatus: "",
    reportExportStatus: "",
    replayBookmarks: [],
    report: sampleReport(),
    metrics: {
      snapshot_frames: 120,
      snapshot_frames_by_run: { "run-1": 120 },
      snapshot_capacity_pressure: 0.12,
      snapshot_capacity_pressure_by_run: { "run-1": 0.12 },
      event_count: 16,
      event_count_by_run: { "run-1": 16 },
      event_capacity_pressure: 0.03,
      event_capacity_pressure_by_run: { "run-1": 0.03 },
      track_point_count: 240,
      track_point_count_by_run: { "run-1": 240 },
      track_point_capacity_pressure: 0.12,
      track_point_capacity_pressure_by_run: { "run-1": 0.12 },
      contact_count: 60,
      contact_count_by_run: { "run-1": 60 },
      snapshot_write_failures: 0,
      snapshot_write_last_ms: 4,
      snapshot_write_avg_ms: 5,
      snapshot_write_max_ms: 8,
      db_ready: true,
      db_table_bytes: 1_600_000,
      db_index_bytes: 400_000,
      db_total_bytes: 2_000_000,
      max_snapshots_per_run: 1000,
      max_events_per_run: 500,
      max_track_points_per_run: 2000
    },
    capacityTrend: sampleCapacityTrend(),
    session: sampleSession(),
    tracks: [sampleTrack()],
    visibleTrackCount: 1,
    threatFilter: "all",
    authRequired: false,
    authMode: "off",
    tokenInput: "",
    connectionState: "replay",
    events: [sampleEvent()],
    replayLoading: false,
    replayError: "",
    error: "",
    busy: false,
    onCreateRun: vi.fn(),
    onSelectCourseTemplate: vi.fn(),
    onApplyCourseTemplate: vi.fn(),
    onCommand: vi.fn(),
    onAction: vi.fn(),
    onSelectRun: vi.fn(),
    onSelectScenario: vi.fn(),
    onScenarioFile: vi.fn(),
    onCopyScenario: vi.fn(),
    onSetScenarioEnabled: vi.fn(),
    onScenarioEditorText: vi.fn(),
    onLoadScenarioEditor: vi.fn(),
    onSaveScenarioEditor: vi.fn(),
    onScenarioEditorMapMode: vi.fn(),
    onScenarioTemplate: vi.fn(),
    onScenarioAssessmentRules: vi.fn(),
    onScenarioEditorZoneSelect: vi.fn(),
    onScenarioEditorVertexSelect: vi.fn(),
    onScenarioEditorZoneDelete: vi.fn(),
    onScenarioZoneDraftUndo: vi.fn(),
    onScenarioZoneDraftClear: vi.fn(),
    onScenarioZoneDraftFinish: vi.fn(),
    onSaveRunMetadata: vi.fn(),
    onAddAnnotation: vi.fn(),
    onThreatFilter: vi.fn(),
    onReplayIndex: vi.fn(),
    onReplayStep: vi.fn(),
    onReplayBoundary: vi.fn(),
    onReplayWindow: vi.fn(),
    onReplayRetry: vi.fn(),
    onReplayPlayToggle: vi.fn(),
    onReplaySpeed: vi.fn(),
    onCopyReplayAnchor: vi.fn(),
    onSaveReplayBookmark: vi.fn(),
    onOpenReplayBookmark: vi.fn(),
    onDeleteReplayBookmark: vi.fn(),
    onJumpToEvent: vi.fn(),
    onLiveView: vi.fn(),
    onExportReport: vi.fn(),
    onTokenInput: vi.fn(),
    onLogin: vi.fn(),
    onLogout: vi.fn(),
    ...overrides
  };
}

function sampleCapacityTrend(): CapacityTrendSample[] {
  return [
    {
      sampled_at: "2026-06-08T00:01:00Z",
      snapshot_frames: 100,
      event_count: 10,
      track_point_count: 200,
      contact_count: 50,
      snapshot_capacity_pressure: 0.1,
      event_capacity_pressure: 0.02,
      track_point_capacity_pressure: 0.1,
      snapshot_write_avg_ms: 3,
      snapshot_write_max_ms: 6,
      snapshot_write_failures: 0,
      db_table_bytes: 1_200_000,
      db_index_bytes: 300_000,
      db_total_bytes: 1_500_000
    },
    {
      sampled_at: "2026-06-08T00:03:00Z",
      snapshot_frames: 120,
      event_count: 16,
      track_point_count: 240,
      contact_count: 60,
      snapshot_capacity_pressure: 0.12,
      event_capacity_pressure: 0.03,
      track_point_capacity_pressure: 0.12,
      snapshot_write_avg_ms: 5,
      snapshot_write_max_ms: 8,
      snapshot_write_failures: 0,
      db_table_bytes: 1_600_000,
      db_index_bytes: 400_000,
      db_total_bytes: 2_000_000
    }
  ];
}

function sampleCourseTemplate(overrides: Partial<CourseTemplate> = {}): CourseTemplate {
  return {
    id: "quick-review",
    name: "Quick Review",
    training_only: true,
    scenario: {
      ...sampleRun().scenario,
      assessment_profile: "quick_review",
      assessment_rules: {
        name: "quick_review_record",
        action_target: 3,
        replay_target: 8,
        action_weight: 30,
        replay_weight: 30,
        context_weight: 40
      }
    },
    expected_metadata: {
      profile: "quick_review",
      trainees: ["student"]
    },
    review_checklist: [
      { id: "timeline", label: "Review timeline", evidence: "Report event list" },
      { id: "metadata", label: "Confirm metadata", evidence: "Run metadata panel" }
    ],
    safety_notice: "Training simulation only.",
    source: "file",
    enabled: true,
    ...overrides
  };
}

function sampleSession(overrides: Partial<SessionResponse> = {}): SessionResponse {
  return {
    auth_mode: "proxy",
    authenticated: true,
    user_id: "operator-a",
    role: "operator",
    role_source: "map",
    permissions: {
      view_runs: true,
      export_reports: true,
      request_websocket_ticket: true,
      create_runs: true,
      control_runs: true,
      submit_training_actions: true,
      annotate_events: true,
      edit_run_metadata: true,
      manage_scenarios: false,
      manage_retention: false
    },
    safety_notice: "Training simulation only.",
    ...overrides
  };
}

function sampleScenario(overrides: Partial<ScenarioSummary> = {}): ScenarioSummary {
  return { id: "demo", name: "Demo Scenario", version: 1, source: "builtin", enabled: true, ...overrides };
}

function sampleRun(): Run {
  return {
    id: "run-1",
    name: "Run 1",
    status: "paused",
    scenario: {
      id: "demo",
      name: "Demo Scenario",
      seed: 7,
      tick_hz: 10,
      snapshot_hz: 2,
      ownship: { lon: 121.5, lat: 31.2, alt_m: 0 },
      sensors: [],
      zones: [],
      initial_contacts: 1
    },
    created_at: "2026-06-08T00:00:00Z",
    updated_at: "2026-06-08T00:02:00Z",
    tags: ["demo"],
    trainees: ["student"],
    instructor_notes: "Reviewed.",
    safety_notice: "Training simulation only."
  };
}

function sampleTrack(): Track {
  return {
    id: "track-1",
    track_no: "T-001",
    kind: "training-contact",
    threat_level: "medium",
    position: { lon: 121.6, lat: 31.3, alt_m: 0 },
    velocity: { lon: 0, lat: 0, alt_m: 0 },
    confidence: 0.8,
    updated_at: "2026-06-08T00:02:00Z",
    status: "tracked"
  };
}

function sampleSnapshot(): Snapshot {
  return {
    run_id: "run-1",
    status: "paused",
    tick: 20,
    time: "2026-06-08T00:02:00Z",
    tracks: [sampleTrack()],
    contacts: [],
    events: [],
    notice: "Training simulation only.",
    snapshot_hz: 2
  };
}

function sampleFrames(): SnapshotFrame[] {
  return [
    {
      run_id: "run-1",
      status: "paused",
      tick: 10,
      sampled_at: "2026-06-08T00:01:00Z",
      tracks: [sampleTrack()],
      contacts: [],
      notice: "Training simulation only.",
      snapshot_hz: 2
    },
    {
      run_id: "run-1",
      status: "paused",
      tick: 20,
      sampled_at: "2026-06-08T00:02:00Z",
      tracks: [sampleTrack()],
      contacts: [],
      notice: "Training simulation only.",
      snapshot_hz: 2
    },
    {
      run_id: "run-1",
      status: "paused",
      tick: 30,
      sampled_at: "2026-06-08T00:03:00Z",
      tracks: [sampleTrack()],
      contacts: [],
      notice: "Training simulation only.",
      snapshot_hz: 2
    }
  ];
}

function sampleEvent(): SimEvent {
  return {
    id: "event-1",
    run_id: "run-1",
    occurred_at: "2026-06-08T00:02:00Z",
    type: "training_action",
    payload: { action: "maneuver", result: "recorded", severity: "info" }
  };
}

function sampleBookmark(): ReplayBookmark {
  return {
    id: "run-1:2026-06-08T00:02:00Z",
    run_id: "run-1",
    at: "2026-06-08T00:02:00Z",
    label: "Run 1 @ 00:02:00",
    created_at: "2026-06-08T00:04:00Z"
  };
}

function sampleReport(overrides: Partial<RunReport> = {}): RunReport {
  return {
    version: 2,
    run: sampleRun(),
    replay_mode: "snapshot",
    duration_seconds: 120,
    track_count: 1,
    action_stats: [],
    event_audit: {
      event_count: 1,
      action_stats: [{ type: "maneuver", count: 1 }],
      actor_stats: []
    },
    threat_summary: {
      initial: { medium: 1 },
      final: { medium: 1 },
      high_watermark: 0
    },
    final_tracks: [],
    events: [sampleEvent()],
    annotations: [
      {
        id: "annotation-1",
        run_id: "run-1",
        event_id: "event-1",
        note: "Reviewed.",
        actor_id: "instructor",
        created_at: "2026-06-08T00:03:00Z"
      }
    ],
    assessment: {
      score: 80,
      label: "complete_training_record",
      criteria: [{ name: "training_actions", value: 80, note: "Abstract training evaluation only." }],
      safety_notice: "Training simulation only."
    },
    audit_logs: [
      {
        id: "audit-1",
        run_id: "run-1",
        actor_id: "instructor",
        action: "run.action_submitted",
        target_type: "event",
        target_id: "event-1",
        occurred_at: "2026-06-08T00:02:30Z",
        payload: { training_only: true }
      }
    ],
    snapshot_range: {
      from: "2026-06-08T00:00:00Z",
      to: "2026-06-08T00:05:00Z",
      count: 12
    },
    snapshot_coverage: {
      from: "2026-06-08T00:00:00Z",
      to: "2026-06-08T00:05:00Z",
      count: 12,
      average_interval_ms: 500
    },
    safety_notice: "Training simulation only.",
    ...overrides
  };
}
