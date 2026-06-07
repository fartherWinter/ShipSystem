package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"shipsim/internal/model"
)

func TestMemoryStoreRunEventAndTrackPoints(t *testing.T) {
	testStoreContract(t, NewMemory())
}

func TestPostgresStoreRunEventAndTrackPoints(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	st, err := NewPostgres(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	testStoreContract(t, st)
}

func TestMigrationStatusRequiresCurrentVersion(t *testing.T) {
	tests := []struct {
		name    string
		current int
		wantErr bool
	}{
		{name: "empty database", current: 0, wantErr: true},
		{name: "v1 database", current: 1, wantErr: true},
		{name: "v2 database", current: CurrentMigrationVersion, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := MigrationStatus{Current: tt.current, Required: CurrentMigrationVersion}
			err := status.Error()
			if tt.wantErr && err == nil {
				t.Fatalf("expected migration error for version %d", tt.current)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected current migration to pass: %v", err)
			}
			if status.Ready() == tt.wantErr {
				t.Fatalf("unexpected ready state for version %d", tt.current)
			}
		})
	}
}

func TestStatementTimeoutFromContext(t *testing.T) {
	if _, ok := statementTimeoutFromContext(context.Background()); ok {
		t.Fatal("expected no statement timeout without context deadline")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, ok := statementTimeoutFromContext(ctx)
	if !ok {
		t.Fatal("expected statement timeout with context deadline")
	}
	if value == "" || !strings.HasSuffix(value, "ms") {
		t.Fatalf("expected millisecond statement timeout, got %q", value)
	}
}

func testStoreContract(t *testing.T, st Store) {
	t.Helper()
	defer st.Close()
	ctx := context.Background()
	status, err := st.Ready(ctx)
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	if status.Store == "" {
		t.Fatal("expected store name")
	}
	if status.Store == "postgres" && status.MigrationVersion < CurrentMigrationVersion {
		t.Fatalf("expected postgres migration version >= %d, got %d", CurrentMigrationVersion, status.MigrationVersion)
	}
	runID := uuid.NewString()
	trackID := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Millisecond)
	run := model.Run{
		ID:     runID,
		Name:   "store-contract",
		Status: model.RunCreated,
		Scenario: model.Scenario{
			Name:            "store-contract",
			Seed:            1,
			TickHz:          20,
			SnapshotHz:      10,
			InitialContacts: 1,
			Ownship:         model.Vec3{Lon: 121.5, Lat: 31.2},
			AllowedActions:  []string{"maneuver"},
			Sensors: []model.Sensor{
				{ID: "sim-sensor-1", Name: "Simulated Sensor", Kind: "simulated_sensor", Position: model.Vec3{Lon: 121.5, Lat: 31.2}},
			},
			Zones: []model.Zone{
				{
					ID:   "area",
					Name: "Area",
					Kind: "exercise_boundary",
					Polygon: []model.Vec3{
						{Lon: 121.1, Lat: 30.9},
						{Lon: 121.9, Lat: 30.9},
						{Lon: 121.9, Lat: 31.6},
					},
				},
			},
		},
		CreatedAt:    now,
		SafetyNotice: model.SafetyNotice,
	}
	if err := st.SaveRun(ctx, run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runs, err := st.ListRuns(ctx, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("expected listed run")
	}

	event := model.SimEvent{
		ID:         uuid.NewString(),
		RunID:      runID,
		OccurredAt: now,
		Type:       "abstract_action",
		Payload:    map[string]any{"result": "recorded"},
	}
	if err := st.SaveEvent(ctx, event); err != nil {
		t.Fatalf("save event: %v", err)
	}
	page, err := st.ListEvents(ctx, runID, model.EventQuery{Limit: 1})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one event, got %d", len(page.Items))
	}

	track := model.Track{
		ID:         trackID,
		TrackNo:    "T-1",
		Kind:       "surface_contact",
		Threat:     model.ThreatLow,
		Position:   model.Vec3{Lon: 121.55, Lat: 31.22},
		Velocity:   model.Vec3{Lon: 0.01, Lat: 0.02},
		Confidence: 0.9,
		UpdatedAt:  now,
		Status:     "active",
	}
	contact := model.Contact{
		ID:         "C-1",
		SensorID:   "sim-sensor-1",
		Timestamp:  now,
		Position:   track.Position,
		Velocity:   track.Velocity,
		Confidence: track.Confidence,
		Kind:       track.Kind,
	}
	snapshot := model.Snapshot{
		RunID:      runID,
		Status:     model.RunRunning,
		Tick:       12,
		Time:       now,
		Tracks:     []model.Track{track},
		Contacts:   []model.Contact{contact},
		Notice:     model.SafetyNotice,
		SnapshotHz: 10,
	}
	if err := st.SaveSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	frames, err := st.ListSnapshots(ctx, runID, model.SnapshotQuery{Limit: 5})
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(frames) != 1 || frames[0].Tick != 12 || len(frames[0].Tracks) != 1 {
		t.Fatalf("unexpected snapshot frames: %+v", frames)
	}
	nearest, err := st.NearestSnapshot(ctx, runID, now.Add(250*time.Millisecond))
	if err != nil {
		t.Fatalf("nearest snapshot: %v", err)
	}
	if nearest.Tick != 12 {
		t.Fatalf("expected nearest tick 12, got %d", nearest.Tick)
	}
	latest, err := st.LatestSnapshot(ctx, runID)
	if err != nil {
		t.Fatalf("latest snapshot: %v", err)
	}
	if latest.Tick != 12 {
		t.Fatalf("expected latest tick 12, got %d", latest.Tick)
	}
	snapshotRange, ok, err := st.SnapshotRange(ctx, runID)
	if err != nil {
		t.Fatalf("snapshot range: %v", err)
	}
	if !ok || snapshotRange.Count != 1 {
		t.Fatalf("expected one snapshot range, got ok=%v range=%+v", ok, snapshotRange)
	}
	points, err := st.ListTrackPoints(ctx, runID, model.TrackPointQuery{TrackID: trackID, Limit: 5})
	if err != nil {
		t.Fatalf("list track points: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected one track point, got %d", len(points))
	}

	for i := 0; i < 2; i++ {
		occurredAt := now.Add(time.Duration(i+1) * time.Millisecond)
		event := model.SimEvent{
			ID:         uuid.NewString(),
			RunID:      runID,
			OccurredAt: occurredAt,
			Type:       "capacity_event",
			Payload:    map[string]any{"index": i},
		}
		if err := st.SaveEvent(ctx, event); err != nil {
			t.Fatalf("save capacity event: %v", err)
		}
		track.UpdatedAt = occurredAt
		track.Position.Lon += 0.001
		contact.Timestamp = occurredAt
		contact.Position = track.Position
		snapshot.Tick = int64(13 + i)
		snapshot.Time = occurredAt
		snapshot.Tracks = []model.Track{track}
		snapshot.Contacts = []model.Contact{contact}
		if err := st.SaveSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("save capacity snapshot: %v", err)
		}
	}
	capacityPreview, err := st.PreviewPrune(ctx, model.RetentionPolicy{
		MaxTrackPointsPerRun: 1,
		MaxEventsPerRun:      1,
		MaxSnapshotsPerRun:   1,
	})
	if err != nil {
		t.Fatalf("preview capacity prune: %v", err)
	}
	if capacityPreview.EventsMatched < 2 || capacityPreview.TrackPointsMatched < 2 || capacityPreview.SnapshotsMatched < 2 {
		t.Fatalf("expected capacity preview to match excess history, got %+v", capacityPreview)
	}
	capacityPruned, err := st.Prune(ctx, model.RetentionPolicy{
		MaxTrackPointsPerRun: 1,
		MaxEventsPerRun:      1,
		MaxSnapshotsPerRun:   1,
	})
	if err != nil {
		t.Fatalf("capacity prune: %v", err)
	}
	if capacityPruned.EventsDeleted < 2 || capacityPruned.TrackPointsDeleted < 2 || capacityPruned.SnapshotsDeleted < 2 {
		t.Fatalf("expected capacity prune to delete excess history, got %+v", capacityPruned)
	}

	preview, err := st.PreviewPrune(ctx, model.RetentionPolicy{
		Cutoff:               now.Add(time.Second),
		MaxTrackPointsPerRun: 1000,
		MaxEventsPerRun:      1000,
		MaxSnapshotsPerRun:   1000,
	})
	if err != nil {
		t.Fatalf("preview prune: %v", err)
	}
	if preview.EventsMatched == 0 || preview.TrackPointsMatched == 0 || preview.ContactsMatched == 0 || preview.SnapshotsMatched == 0 {
		t.Fatalf("expected prune preview to match saved history, got %+v", preview)
	}

	pruned, err := st.Prune(ctx, model.RetentionPolicy{
		Cutoff:               now.Add(time.Second),
		MaxTrackPointsPerRun: 1000,
		MaxEventsPerRun:      1000,
		MaxSnapshotsPerRun:   1000,
	})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned.EventsDeleted == 0 || pruned.TrackPointsDeleted == 0 || pruned.ContactsDeleted == 0 || pruned.SnapshotsDeleted == 0 {
		t.Fatalf("expected prune to delete saved history, got %+v", pruned)
	}
}
