// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ControlSidebar } from "./ControlSidebar";
import type { ComponentProps } from "react";
import type { Run, RunReport, ScenarioSummary, SimEvent, Snapshot, SnapshotFrame, Track } from "./types";

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

  it("covers report exports", () => {
    const onExportReport = vi.fn();

    renderSidebar({ onExportReport });

    fireEvent.click(screen.getByRole("button", { name: "Export report JSON" }));
    fireEvent.click(screen.getByRole("button", { name: "Export report CSV" }));
    fireEvent.click(screen.getByRole("button", { name: "Export report HTML" }));
    fireEvent.click(screen.getByRole("button", { name: "Export report PDF" }));

    expect(onExportReport).toHaveBeenCalledWith("json");
    expect(onExportReport).toHaveBeenCalledWith("csv");
    expect(onExportReport).toHaveBeenCalledWith("html");
    expect(onExportReport).toHaveBeenCalledWith("pdf");
  });
});

function renderSidebar(overrides: Partial<SidebarProps> = {}) {
  const props: SidebarProps = {
    runs: [sampleRun()],
    scenarios: [sampleScenario()],
    selectedScenarioID: "demo",
    run: sampleRun(),
    snapshot: sampleSnapshot(),
    snapshotFrames: sampleFrames(),
    replayFrame: sampleFrames()[1],
    replayIndex: 1,
    replayPlaying: false,
    replaySpeed: 1,
    report: sampleReport(),
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
    onCommand: vi.fn(),
    onAction: vi.fn(),
    onSelectRun: vi.fn(),
    onSelectScenario: vi.fn(),
    onScenarioFile: vi.fn(),
    onCopyScenario: vi.fn(),
    onSetScenarioEnabled: vi.fn(),
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
    onJumpToEvent: vi.fn(),
    onLiveView: vi.fn(),
    onExportReport: vi.fn(),
    onTokenInput: vi.fn(),
    onLogin: vi.fn(),
    onLogout: vi.fn(),
    ...overrides
  };
  return render(<ControlSidebar {...props} />);
}

function sampleScenario(): ScenarioSummary {
  return { id: "demo", name: "Demo Scenario", version: 1, source: "builtin", enabled: true };
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
