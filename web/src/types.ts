import type { TrainingAction } from "./generated/api-types";

export type {
  Action,
  ActionStat,
  ActorStat,
  AssessmentCriterion,
  AssessmentRules,
  AuditLog,
  ApiError,
  ApiErrorBody,
  Contact,
  CourseChecklistItem,
  CourseTemplate,
  CopyScenarioRequest,
  CreateRunRequest,
  EventAnnotation,
  EventAnnotationInput,
  EventAuditSummary,
  EventPage,
  HealthResponse,
  MetricsHistorySample,
  MetricsResponse,
  ReadyResponse,
  RetentionPolicyInput,
  RetentionPreview,
  RetentionResult,
  Run,
  RunMetadata,
  RunReport,
  RunStatus,
  Scenario,
  ScenarioSummary,
  Sensor,
  SessionResponse,
  SimEvent,
  Snapshot,
  SnapshotCoverage,
  SnapshotFrame,
  SnapshotRange,
  StoreStatus,
  ThreatLevel,
  ThreatSummary,
  Track,
  TrackPoint,
  TrackStatusSummary,
  TrainingAssessment,
  TrainingAction,
  Vec3,
  WebSocketSnapshotMessage,
  WebSocketTicket,
  Zone
} from "./generated/api-types";

export type ConnectionState = "idle" | "connecting" | "live" | "replay" | "error";

export type ScenarioEditorMapMode = "off" | "sensor" | "zone" | "vertex";

export type ScenarioEditorTemplate = "review_sensor" | "exercise_boundary" | "focus_zone";

export type ReplayBookmark = {
  id: string;
  run_id: string;
  at: string;
  label: string;
  created_at: string;
};

export type CapacityTrendSample = {
  sampled_at: string;
  snapshot_frames: number;
  event_count: number;
  track_point_count: number;
  contact_count: number;
  snapshot_capacity_pressure: number;
  event_capacity_pressure: number;
  track_point_capacity_pressure: number;
  snapshot_write_avg_ms: number;
  snapshot_write_max_ms: number;
  snapshot_write_failures: number;
  db_table_bytes: number;
  db_index_bytes: number;
  db_total_bytes: number;
};

export const trainingActions: Array<{ type: TrainingAction; label: string }> = [
  { type: "maneuver", label: "Maneuver" },
  { type: "decoy", label: "Decoy" },
  { type: "training_response", label: "Training Response" }
];
