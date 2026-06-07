package model

import (
	"encoding/json"
	"time"
)

type Vec3 struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
	Alt float64 `json:"alt_m"`
}

type RunStatus string

const (
	RunCreated RunStatus = "created"
	RunRunning RunStatus = "running"
	RunPaused  RunStatus = "paused"
	RunStopped RunStatus = "stopped"
)

type ThreatLevel string

const (
	ThreatLow    ThreatLevel = "low"
	ThreatMedium ThreatLevel = "medium"
	ThreatHigh   ThreatLevel = "high"
)

type Sensor struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Position Vec3   `json:"position"`
}

type Zone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Polygon []Vec3 `json:"polygon"`
}

type Scenario struct {
	ID              string    `json:"id,omitempty"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	Version         int       `json:"version,omitempty"`
	Seed            int64     `json:"seed"`
	TickHz          int       `json:"tick_hz"`
	SnapshotHz      int       `json:"snapshot_hz"`
	Ownship         Vec3      `json:"ownship"`
	Sensors         []Sensor  `json:"sensors"`
	Zones           []Zone    `json:"zones"`
	InitialContacts int       `json:"initial_contacts"`
	Tracks          []Track   `json:"tracks,omitempty"`
	Contacts        []Contact `json:"contacts,omitempty"`
	AllowedActions  []string  `json:"allowed_actions,omitempty"`
}

type Run struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Status       RunStatus `json:"status"`
	Scenario     Scenario  `json:"scenario"`
	OwnerID      string    `json:"owner_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	StoppedAt    time.Time `json:"stopped_at,omitempty"`
	Restored     bool      `json:"restored_from_store,omitempty"`
	SafetyNotice string    `json:"safety_notice"`
}

func (r Run) MarshalJSON() ([]byte, error) {
	type runJSON struct {
		ID           string     `json:"id"`
		Name         string     `json:"name"`
		Status       RunStatus  `json:"status"`
		Scenario     Scenario   `json:"scenario"`
		OwnerID      string     `json:"owner_id,omitempty"`
		CreatedAt    time.Time  `json:"created_at"`
		UpdatedAt    time.Time  `json:"updated_at"`
		StartedAt    *time.Time `json:"started_at,omitempty"`
		StoppedAt    *time.Time `json:"stopped_at,omitempty"`
		Restored     bool       `json:"restored_from_store,omitempty"`
		SafetyNotice string     `json:"safety_notice"`
	}
	return json.Marshal(runJSON{
		ID:           r.ID,
		Name:         r.Name,
		Status:       r.Status,
		Scenario:     r.Scenario,
		OwnerID:      r.OwnerID,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
		StartedAt:    optionalTime(r.StartedAt),
		StoppedAt:    optionalTime(r.StoppedAt),
		Restored:     r.Restored,
		SafetyNotice: r.SafetyNotice,
	})
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

type ScenarioSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     int    `json:"version,omitempty"`
	Source      string `json:"source"`
}

type StoreStatus struct {
	Store            string `json:"store"`
	MigrationVersion int    `json:"migration_version"`
}

type RetentionPolicy struct {
	Cutoff               time.Time
	EndedBefore          time.Time
	MaxTrackPointsPerRun int
	MaxEventsPerRun      int
	MaxSnapshotsPerRun   int
	OwnerID              string
}

type RetentionResult struct {
	RunsMatched        int64 `json:"runs_matched"`
	EventsDeleted      int64 `json:"events_deleted"`
	TrackPointsDeleted int64 `json:"track_points_deleted"`
	ContactsDeleted    int64 `json:"contacts_deleted"`
	SnapshotsDeleted   int64 `json:"snapshots_deleted"`
}

type RetentionPreview struct {
	RunsMatched        int64 `json:"runs_matched"`
	EventsMatched      int64 `json:"events_matched"`
	TrackPointsMatched int64 `json:"track_points_matched"`
	ContactsMatched    int64 `json:"contacts_matched"`
	SnapshotsMatched   int64 `json:"snapshots_matched"`
}

type Contact struct {
	ID         string    `json:"id"`
	SensorID   string    `json:"sensor_id"`
	Timestamp  time.Time `json:"timestamp"`
	Position   Vec3      `json:"position"`
	Velocity   Vec3      `json:"velocity"`
	Confidence float64   `json:"confidence"`
	Kind       string    `json:"kind"`
}

type Track struct {
	ID         string      `json:"id"`
	TrackNo    string      `json:"track_no"`
	Kind       string      `json:"kind"`
	Threat     ThreatLevel `json:"threat_level"`
	Position   Vec3        `json:"position"`
	Velocity   Vec3        `json:"velocity"`
	Confidence float64     `json:"confidence"`
	UpdatedAt  time.Time   `json:"updated_at"`
	Status     string      `json:"status"`
}

type SimEvent struct {
	ID         string         `json:"id"`
	RunID      string         `json:"run_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Type       string         `json:"type"`
	SubjectID  string         `json:"subject_id,omitempty"`
	Payload    map[string]any `json:"payload"`
}

type Action struct {
	Type    string         `json:"type"`
	ActorID string         `json:"actor_id,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Snapshot struct {
	RunID      string     `json:"run_id"`
	Status     RunStatus  `json:"status"`
	Tick       int64      `json:"tick"`
	Time       time.Time  `json:"time"`
	Tracks     []Track    `json:"tracks"`
	Contacts   []Contact  `json:"contacts"`
	Events     []SimEvent `json:"events"`
	Notice     string     `json:"notice"`
	SnapshotHz int        `json:"snapshot_hz"`
}

type SnapshotFrame struct {
	RunID      string    `json:"run_id"`
	Status     RunStatus `json:"status"`
	Tick       int64     `json:"tick"`
	SampledAt  time.Time `json:"sampled_at"`
	Tracks     []Track   `json:"tracks"`
	Contacts   []Contact `json:"contacts"`
	Notice     string    `json:"notice"`
	SnapshotHz int       `json:"snapshot_hz"`
}

type SnapshotQuery struct {
	From  time.Time
	To    time.Time
	Limit int
}

type EventQuery struct {
	Limit  int
	Cursor string
}

type EventPage struct {
	Items      []SimEvent `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

type TrackPointQuery struct {
	TrackID string
	From    time.Time
	To      time.Time
	Limit   int
}

type TrackPoint struct {
	TrackID    string    `json:"track_id"`
	SampledAt  time.Time `json:"sampled_at"`
	Position   Vec3      `json:"position"`
	Speed      float64   `json:"speed"`
	Heading    float64   `json:"heading"`
	Confidence float64   `json:"confidence"`
}

type SnapshotRange struct {
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	Count int       `json:"count"`
}

type SnapshotCoverage struct {
	From              time.Time `json:"from"`
	To                time.Time `json:"to"`
	Count             int       `json:"count"`
	AverageIntervalMS float64   `json:"average_interval_ms"`
}

type ActionStat struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type ActorStat struct {
	ActorID string `json:"actor_id"`
	Count   int    `json:"count"`
}

type EventAuditSummary struct {
	EventCount    int          `json:"event_count"`
	ActionStats   []ActionStat `json:"action_stats"`
	ActorStats    []ActorStat  `json:"actor_stats"`
	FirstActionAt *time.Time   `json:"first_action_at,omitempty"`
	LastActionAt  *time.Time   `json:"last_action_at,omitempty"`
}

type ThreatSummary struct {
	Initial       map[ThreatLevel]int `json:"initial"`
	Final         map[ThreatLevel]int `json:"final"`
	HighWatermark int                 `json:"high_watermark"`
}

type TrackStatusSummary struct {
	TrackID    string      `json:"track_id"`
	TrackNo    string      `json:"track_no"`
	Kind       string      `json:"kind"`
	Threat     ThreatLevel `json:"threat_level"`
	Status     string      `json:"status"`
	Confidence float64     `json:"confidence"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type RunReport struct {
	Version          int                  `json:"version"`
	Run              Run                  `json:"run"`
	ReplayMode       string               `json:"replay_mode"`
	DurationSeconds  int64                `json:"duration_seconds"`
	TrackCount       int                  `json:"track_count"`
	ActionStats      []ActionStat         `json:"action_stats"`
	EventAudit       EventAuditSummary    `json:"event_audit"`
	ThreatSummary    ThreatSummary        `json:"threat_summary"`
	FinalTracks      []TrackStatusSummary `json:"final_tracks"`
	Events           []SimEvent           `json:"events"`
	SnapshotRange    *SnapshotRange       `json:"snapshot_range,omitempty"`
	SnapshotCoverage *SnapshotCoverage    `json:"snapshot_coverage,omitempty"`
	SafetyNotice     string               `json:"safety_notice"`
}

const SafetyNotice = "Training/simulation only. This system does not provide real fire-control, weapon-control, electronic-warfare, or tactical engagement guidance."
