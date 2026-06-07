export type Vec3 = { lon: number; lat: number; alt_m: number };

export type Sensor = {
  id: string;
  name: string;
  kind: string;
  position: Vec3;
};

export type Zone = {
  id: string;
  name: string;
  kind: string;
  polygon: Vec3[];
};

export type Scenario = {
  id?: string;
  name: string;
  description?: string;
  version?: number;
  seed: number;
  tick_hz: number;
  snapshot_hz: number;
  ownship: Vec3;
  sensors: Sensor[];
  zones: Zone[];
  initial_contacts: number;
  allowed_actions?: string[];
};

export type ScenarioSummary = {
  id: string;
  name: string;
  description?: string;
  version?: number;
  source: string;
};

export type Run = {
  id: string;
  name: string;
  status: string;
  scenario: Scenario;
  owner_id?: string;
  created_at: string;
  updated_at?: string;
  started_at?: string;
  stopped_at?: string;
  restored_from_store?: boolean;
  safety_notice: string;
};

export type Track = {
  id: string;
  track_no: string;
  kind: string;
  threat_level: "low" | "medium" | "high" | string;
  position: Vec3;
  velocity: Vec3;
  confidence: number;
  updated_at: string;
  status: string;
};

export type SimEvent = {
  id: string;
  run_id: string;
  occurred_at: string;
  type: string;
  subject_id?: string;
  payload: Record<string, unknown>;
};

export type EventPage = {
  items: SimEvent[];
  next_cursor?: string;
};

export type Snapshot = {
  run_id: string;
  status: string;
  tick: number;
  time: string;
  tracks: Track[];
  contacts: unknown[];
  events: SimEvent[];
  notice: string;
  snapshot_hz: number;
};

export type SnapshotFrame = {
  run_id: string;
  status: string;
  tick: number;
  sampled_at: string;
  tracks: Track[];
  contacts: unknown[];
  notice: string;
  snapshot_hz: number;
};

export type TrackPoint = {
  track_id: string;
  sampled_at: string;
  position: Vec3;
  speed: number;
  heading: number;
  confidence: number;
};

export type SnapshotRange = {
  from: string;
  to: string;
  count: number;
};

export type SnapshotCoverage = SnapshotRange & {
  average_interval_ms: number;
};

export type ActionStat = {
  type: string;
  count: number;
};

export type ActorStat = {
  actor_id: string;
  count: number;
};

export type EventAuditSummary = {
  event_count: number;
  action_stats: ActionStat[];
  actor_stats: ActorStat[];
  first_action_at?: string;
  last_action_at?: string;
};

export type ThreatSummary = {
  initial: Record<string, number>;
  final: Record<string, number>;
  high_watermark: number;
};

export type TrackStatusSummary = {
  track_id: string;
  track_no: string;
  kind: string;
  threat_level: string;
  status: string;
  confidence: number;
  updated_at: string;
};

export type RunReport = {
  version: number;
  run: Run;
  replay_mode: "snapshot" | "legacy" | string;
  duration_seconds: number;
  track_count: number;
  action_stats: ActionStat[];
  event_audit: EventAuditSummary;
  threat_summary: ThreatSummary;
  final_tracks: TrackStatusSummary[];
  events: SimEvent[];
  snapshot_range?: SnapshotRange;
  snapshot_coverage?: SnapshotCoverage;
  safety_notice: string;
};

export type ApiError = {
  error: {
    code: string;
    message: string;
    details?: string[];
  };
};

export type ConnectionState = "idle" | "connecting" | "live" | "replay" | "error";

export type TrainingAction = "maneuver" | "decoy" | "training_response";

export const trainingActions: Array<{ type: TrainingAction; label: string }> = [
  { type: "maneuver", label: "Maneuver" },
  { type: "decoy", label: "Decoy" },
  { type: "training_response", label: "Training Response" }
];
