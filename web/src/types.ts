import type { TrainingAction } from "./generated/api-types";

export type {
  Action,
  ActionStat,
  ActorStat,
  AssessmentCriterion,
  AuditLog,
  ApiError,
  ApiErrorBody,
  Contact,
  CopyScenarioRequest,
  CreateRunRequest,
  EventAnnotation,
  EventAnnotationInput,
  EventAuditSummary,
  EventPage,
  HealthResponse,
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

export const trainingActions: Array<{ type: TrainingAction; label: string }> = [
  { type: "maneuver", label: "Maneuver" },
  { type: "decoy", label: "Decoy" },
  { type: "training_response", label: "Training Response" }
];
