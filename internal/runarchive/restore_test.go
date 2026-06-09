package runarchive

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"shipsim/internal/model"
	"shipsim/internal/store"
)

func TestRestoreZipArchive(t *testing.T) {
	ctx := context.Background()
	archivePath := writeTestArchive(t, true)
	st := store.NewMemory()

	summary, err := Restore(ctx, st, archivePath, RestoreOptions{})
	if err != nil {
		t.Fatalf("restore archive: %v", err)
	}
	if summary.RunID != testRunID || summary.Events != 1 || summary.Snapshots != 1 || summary.Annotations != 1 || summary.AuditLogs != 1 || !summary.TrainingOnly {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	run, err := st.GetRun(ctx, testRunID)
	if err != nil {
		t.Fatalf("get restored run: %v", err)
	}
	if !run.Restored || run.SafetyNotice != "Training simulation only." {
		t.Fatalf("expected restored training run, got %+v", run)
	}
	frames, err := st.ListSnapshots(ctx, testRunID, model.SnapshotQuery{Limit: 10})
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(frames) != 1 || frames[0].Tick != 20 {
		t.Fatalf("expected one restored frame, got %+v", frames)
	}
	events, err := st.ListEvents(ctx, testRunID, model.EventQuery{Limit: 10})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events.Items) != 1 || events.Items[0].Type != "training_action" {
		t.Fatalf("expected restored event, got %+v", events.Items)
	}
	annotations, err := st.ListEventAnnotations(ctx, testRunID)
	if err != nil {
		t.Fatalf("list annotations: %v", err)
	}
	if len(annotations) != 1 || !strings.Contains(annotations[0].Note, "Reviewed") {
		t.Fatalf("expected restored annotation, got %+v", annotations)
	}
	logs, err := st.ListAuditLogs(ctx, model.AuditLogQuery{RunID: testRunID, Limit: 10})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Action != "run.action_submitted" {
		t.Fatalf("expected restored audit log, got %+v", logs)
	}
	counts, err := st.RunDataCounts(ctx, testRunID)
	if err != nil {
		t.Fatalf("run data counts: %v", err)
	}
	if counts.Snapshots != 1 || counts.Events != 1 || counts.TrackPoints != 1 {
		t.Fatalf("expected replay data counts, got %+v", counts)
	}
}

func TestRestoreRejectsExistingReplayData(t *testing.T) {
	ctx := context.Background()
	archivePath := writeTestArchive(t, false)
	st := store.NewMemory()
	if _, err := Restore(ctx, st, archivePath, RestoreOptions{}); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	if _, err := Restore(ctx, st, archivePath, RestoreOptions{}); err == nil {
		t.Fatalf("expected duplicate restore to fail")
	}
}

const testRunID = "11111111-1111-1111-1111-111111111111"

func writeTestArchive(t *testing.T, compressed bool) string {
	t.Helper()
	root := t.TempDir()
	archiveDir := filepath.Join(root, "archive")
	snapshotDir := filepath.Join(archiveDir, "data", "snapshots")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}
	writeJSON(t, filepath.Join(archiveDir, "manifest.json"), map[string]any{
		"run_id":        testRunID,
		"training_only": true,
		"safety_notice": "Training simulation only.",
	})
	writeJSON(t, filepath.Join(archiveDir, "data", "run.json"), testRun())
	writeJSON(t, filepath.Join(archiveDir, "data", "events.json"), []model.SimEvent{testEvent()})
	writeJSON(t, filepath.Join(archiveDir, "data", "annotations.json"), []model.EventAnnotation{testAnnotation()})
	writeJSON(t, filepath.Join(archiveDir, "data", "audit.json"), []model.AuditLog{testAuditLog()})
	writeJSON(t, filepath.Join(snapshotDir, "snapshots-page-0001.json"), []model.SnapshotFrame{testFrame()})
	if !compressed {
		return archiveDir
	}
	zipPath := filepath.Join(root, "archive.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(zipFile)
	err = filepath.WalkDir(archiveDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(archiveDir, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		part, err := writer.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = part.Write(body)
		return err
	})
	if err != nil {
		t.Fatalf("write zip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := zipFile.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return zipPath
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func testRun() model.Run {
	now := testTime()
	return model.Run{
		ID:        testRunID,
		Name:      "Restored Run",
		Status:    model.RunStopped,
		CreatedAt: now.Add(-2 * time.Minute),
		UpdatedAt: now,
		StartedAt: now.Add(-90 * time.Second),
		StoppedAt: now,
		Scenario: model.Scenario{
			ID:              "scenario-1",
			Name:            "Archive Scenario",
			Seed:            7,
			TickHz:          10,
			SnapshotHz:      2,
			Ownship:         model.Vec3{Lon: 121.5, Lat: 31.2},
			InitialContacts: 1,
			Zones: []model.Zone{
				{
					ID:   "zone-1",
					Name: "Training Area",
					Kind: "exercise_boundary",
					Polygon: []model.Vec3{
						{Lon: 121.4, Lat: 31.1},
						{Lon: 121.6, Lat: 31.1},
						{Lon: 121.6, Lat: 31.3},
					},
				},
			},
		},
		SafetyNotice: "Training simulation only.",
	}
}

func testEvent() model.SimEvent {
	return model.SimEvent{
		ID:         "event-1",
		RunID:      testRunID,
		OccurredAt: testTime().Add(-30 * time.Second),
		Type:       "training_action",
		Payload:    map[string]any{"action": "maneuver", "result": "recorded"},
	}
}

func testAnnotation() model.EventAnnotation {
	return model.EventAnnotation{
		ID:        "annotation-1",
		RunID:     testRunID,
		EventID:   "event-1",
		Note:      "Reviewed for training replay.",
		ActorID:   "instructor",
		CreatedAt: testTime().Add(-20 * time.Second),
	}
}

func testAuditLog() model.AuditLog {
	return model.AuditLog{
		ID:         "audit-1",
		RunID:      testRunID,
		ActorID:    "instructor",
		Action:     "run.action_submitted",
		TargetType: "event",
		TargetID:   "event-1",
		OccurredAt: testTime().Add(-25 * time.Second),
		Payload:    map[string]any{"training_only": true},
	}
}

func testFrame() model.SnapshotFrame {
	return model.SnapshotFrame{
		RunID:     testRunID,
		Status:    model.RunStopped,
		Tick:      20,
		SampledAt: testTime(),
		Tracks: []model.Track{
			{
				ID:         "track-1",
				TrackNo:    "T-001",
				Kind:       "training-contact",
				Threat:     model.ThreatMedium,
				Position:   model.Vec3{Lon: 121.55, Lat: 31.25},
				Velocity:   model.Vec3{Lon: 1, Lat: 0},
				Confidence: 0.8,
				UpdatedAt:  testTime(),
				Status:     "tracked",
			},
		},
		Notice:     "Training simulation only.",
		SnapshotHz: 2,
	}
}

func testTime() time.Time {
	return time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
}
