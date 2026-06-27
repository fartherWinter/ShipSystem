package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"shipsystem/backend/internal/models"
	"shipsystem/backend/internal/ws"
)

type fakeBattleStore struct {
	session           models.BattleSession
	radarTargets      map[string]models.RadarTarget
	units             map[string]models.BattleUnit
	projectiles       map[string]models.BattleProjectile
	events            map[string]models.BattleEvent
	snapshots         map[string]models.BattleSnapshot
	insertedSnapshots int
	appendCalls       int
}

func newFakeBattleStore(sessionID string) *fakeBattleStore {
	startedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	return &fakeBattleStore{
		session: models.BattleSession{
			SessionID:    sessionID,
			Name:         "Test Battle",
			ScenarioCode: "open-water-duel",
			Status:       "running",
			StartedAt:    startedAt,
			UpdatedAt:    startedAt,
		},
		radarTargets: make(map[string]models.RadarTarget),
		units:        make(map[string]models.BattleUnit),
		projectiles:  make(map[string]models.BattleProjectile),
		events:       make(map[string]models.BattleEvent),
		snapshots:    make(map[string]models.BattleSnapshot),
	}
}

func (f *fakeBattleStore) GetBattleSession(ctx context.Context, sessionID string) (models.BattleSession, error) {
	if sessionID != f.session.SessionID {
		return models.BattleSession{}, fmt.Errorf("unexpected session lookup %s", sessionID)
	}
	return f.session, nil
}

func (f *fakeBattleStore) UpdateBattleSessionScan(ctx context.Context, sessionID, status string, scanTime time.Time) error {
	if sessionID != f.session.SessionID {
		return fmt.Errorf("unexpected session update %s", sessionID)
	}
	f.session.LastScanAt = &scanTime
	if status != "" {
		f.session.Status = status
	}
	return nil
}

func (f *fakeBattleStore) UpsertRadarTargets(ctx context.Context, targets []models.RadarTarget) error {
	for _, target := range targets {
		f.radarTargets[target.TargetID] = target
	}
	return nil
}

func (f *fakeBattleStore) UpsertBattleUnits(ctx context.Context, units []models.BattleUnit) error {
	for _, unit := range units {
		f.units[unit.UnitID] = unit
	}
	return nil
}

func (f *fakeBattleStore) UpsertBattleProjectiles(ctx context.Context, projectiles []models.BattleProjectile) error {
	for _, projectile := range projectiles {
		f.projectiles[projectile.ProjectileID] = projectile
	}
	return nil
}

func (f *fakeBattleStore) CreateBattleEvents(ctx context.Context, events []models.BattleEvent) ([]models.BattleEvent, error) {
	inserted := make([]models.BattleEvent, 0, len(events))
	for _, event := range events {
		if _, exists := f.events[event.EventID]; exists {
			continue
		}
		event.ID = uint(len(f.events) + 1)
		f.events[event.EventID] = event
		inserted = append(inserted, event)
	}
	return inserted, nil
}

func (f *fakeBattleStore) AppendBattleSnapshot(ctx context.Context, snapshot *models.BattleSnapshot) (bool, error) {
	f.appendCalls++
	key := snapshot.SessionID + "|" + snapshot.SnapshotTime.UTC().Format(time.RFC3339Nano)
	if _, exists := f.snapshots[key]; exists {
		return false, nil
	}
	snapshot.ID = uint(len(f.snapshots) + 1)
	snapshot.Tick = int64(len(f.snapshots) + 1)
	f.snapshots[key] = *snapshot
	f.insertedSnapshots++
	return true, nil
}

func (f *fakeBattleStore) ListBattleUnits(ctx context.Context, sessionID string) ([]models.BattleUnit, error) {
	items := make([]models.BattleUnit, 0, len(f.units))
	for _, unit := range f.units {
		items = append(items, unit)
	}
	return items, nil
}

func (f *fakeBattleStore) ListBattleProjectiles(ctx context.Context, sessionID string) ([]models.BattleProjectile, error) {
	items := make([]models.BattleProjectile, 0, len(f.projectiles))
	for _, projectile := range f.projectiles {
		items = append(items, projectile)
	}
	return items, nil
}

func (f *fakeBattleStore) ListBattleEvents(ctx context.Context, sessionID string, limit int) ([]models.BattleEvent, error) {
	items := make([]models.BattleEvent, 0, len(f.events))
	for _, event := range f.events {
		items = append(items, event)
	}
	return items, nil
}

func (f *fakeBattleStore) ListRadarTargets(ctx context.Context, sessionID string) ([]models.RadarTarget, error) {
	items := make([]models.RadarTarget, 0, len(f.radarTargets))
	for _, target := range f.radarTargets {
		items = append(items, target)
	}
	return items, nil
}

type fakeBroadcaster struct {
	events []ws.Event
}

func (f *fakeBroadcaster) Broadcast(event ws.Event) bool {
	f.events = append(f.events, event)
	return true
}

func (f *fakeBroadcaster) countByType(eventType string) int {
	count := 0
	for _, event := range f.events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func TestReceiveRadarReportDeduplicatesRepeatedEventsAndSnapshots(t *testing.T) {
	store := newFakeBattleStore("battle-repeat")
	broadcaster := &fakeBroadcaster{}
	updatedAt := time.Date(2026, 6, 10, 12, 30, 0, 0, time.UTC)
	report := RadarReportPayload{
		SessionID: "battle-repeat",
		RadarID:   "SIM-RADAR-01",
		ScanTime:  &updatedAt,
		Targets: []RadarTargetPayload{
			{
				TargetID:   "red-1",
				Side:       "red",
				Longitude:  121.5,
				Latitude:   31.24,
				Course:     270,
				SpeedKnots: 18,
				Confidence: 0.9,
				Detected:   true,
			},
		},
		State: &BattleStatePayload{
			SessionID: "battle-repeat",
			Status:    "running",
			UpdatedAt: &updatedAt,
			Units: []BattleUnitPayload{
				{
					UnitID:          "blue-1",
					ShipID:          1,
					Name:            "Blue Destroyer 01",
					Side:            "blue",
					HP:              100,
					MaxHP:           100,
					RadarRangeKm:    32,
					WeaponRangeKm:   20,
					CooldownSeconds: 5,
					Longitude:       121.48,
					Latitude:        31.22,
					Course:          90,
					SpeedKnots:      18,
					Status:          "active",
				},
			},
			Projectiles: []BattleProjectilePayload{
				{
					ProjectileID: "proj-1",
					SourceUnitID: "blue-1",
					TargetUnitID: "red-1",
					Side:         "blue",
					Longitude:    121.49,
					Latitude:     31.23,
					SpeedKmH:     1600,
					Status:       "flying",
				},
			},
			Events: []BattleEventPayload{
				{
					EventID:    "evt-1",
					Type:       "WEAPON_FIRED",
					Severity:   "INFO",
					Message:    "Blue fires.",
					OccurredAt: &updatedAt,
				},
			},
		},
	}

	if _, err := receiveRadarReportWithDeps(context.Background(), store, broadcaster, report); err != nil {
		t.Fatalf("first radar report returned error: %v", err)
	}
	if _, err := receiveRadarReportWithDeps(context.Background(), store, broadcaster, report); err != nil {
		t.Fatalf("second radar report returned error: %v", err)
	}

	if got := broadcaster.countByType("battle_event_created"); got != 1 {
		t.Fatalf("expected exactly one inserted battle event broadcast across duplicates, got %d", got)
	}
	if store.insertedSnapshots != 1 {
		t.Fatalf("expected duplicate reports to insert one snapshot, got %d", store.insertedSnapshots)
	}
	if len(store.events) != 1 {
		t.Fatalf("expected one persisted battle event, got %d", len(store.events))
	}
}

func TestReceiveRadarReportWithoutStateSkipsSnapshotAndStateBroadcast(t *testing.T) {
	store := newFakeBattleStore("battle-scan-only")
	broadcaster := &fakeBroadcaster{}
	scanTime := time.Date(2026, 6, 10, 13, 0, 0, 0, time.UTC)
	report := RadarReportPayload{
		SessionID: "battle-scan-only",
		RadarID:   "SIM-RADAR-01",
		ScanTime:  &scanTime,
		Targets: []RadarTargetPayload{
			{
				TargetID:   "red-2",
				Side:       "red",
				Longitude:  121.51,
				Latitude:   31.25,
				Course:     270,
				SpeedKnots: 16,
				Confidence: 0.88,
				Detected:   true,
			},
		},
	}

	state, err := receiveRadarReportWithDeps(context.Background(), store, broadcaster, report)
	if err != nil {
		t.Fatalf("radar report without state returned error: %v", err)
	}

	if state.SessionID != "battle-scan-only" {
		t.Fatalf("expected state for battle-scan-only, got %#v", state)
	}
	if store.insertedSnapshots != 0 {
		t.Fatalf("expected scan-only report not to insert snapshots, got %d", store.insertedSnapshots)
	}
	if got := broadcaster.countByType("battle_state_updated"); got != 0 {
		t.Fatalf("expected no battle_state_updated broadcast, got %d", got)
	}
	if got := broadcaster.countByType("radar_scan_updated"); got != 1 {
		t.Fatalf("expected one radar_scan_updated broadcast, got %d", got)
	}
}
