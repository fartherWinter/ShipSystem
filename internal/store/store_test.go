package store

import (
	"context"
	"os"
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

	preview, err := st.PreviewPrune(ctx, model.RetentionPolicy{
		Cutoff:               now.Add(time.Second),
		MaxTrackPointsPerRun: 1000,
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
	})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned.EventsDeleted == 0 || pruned.TrackPointsDeleted == 0 || pruned.ContactsDeleted == 0 || pruned.SnapshotsDeleted == 0 {
		t.Fatalf("expected prune to delete saved history, got %+v", pruned)
	}
}
