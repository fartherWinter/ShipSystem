package main

import (
	"testing"
	"time"

	"shipsim/internal/config"
	"shipsim/internal/model"
)

func TestModelRetentionPolicyUsesCurrentTimeAndCapacityLimits(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	cfg := config.Default()
	cfg.RetentionDays = 30
	cfg.MaxTrackPointsPerRun = 250000
	cfg.MaxEventsPerRun = 50000
	cfg.MaxSnapshotsPerRun = 250000

	policy := modelRetentionPolicy(cfg, now)
	if want := now.Add(-30 * 24 * time.Hour); !policy.Cutoff.Equal(want) {
		t.Fatalf("expected cutoff %s, got %s", want, policy.Cutoff)
	}
	if policy.MaxTrackPointsPerRun != 250000 || policy.MaxEventsPerRun != 50000 || policy.MaxSnapshotsPerRun != 250000 {
		t.Fatalf("unexpected capacity policy: %+v", policy)
	}
}

func TestRetentionPolicyEmptyIncludesAllCapacityLimits(t *testing.T) {
	if !retentionPolicyEmpty(model.RetentionPolicy{}) {
		t.Fatal("expected empty retention policy")
	}
	if retentionPolicyEmpty(model.RetentionPolicy{MaxEventsPerRun: 1}) {
		t.Fatal("expected max events to make policy non-empty")
	}
	if retentionPolicyEmpty(model.RetentionPolicy{MaxSnapshotsPerRun: 1}) {
		t.Fatal("expected max snapshots to make policy non-empty")
	}
}
