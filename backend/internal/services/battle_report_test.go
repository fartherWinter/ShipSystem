package services

import (
	"context"
	"testing"
	"time"

	"shipsystem/backend/internal/models"
)

func TestDecodeBattleSnapshot(t *testing.T) {
	snapshotTime := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	snapshot := models.BattleSnapshot{
		ID:               9,
		SessionID:        "battle-test",
		Tick:             3,
		SnapshotTime:     snapshotTime,
		UnitsJSON:        models.JSONB(`[{"sessionId":"battle-test","unitId":"blue-1","shipId":1,"name":"Blue","side":"blue","hp":80,"maxHp":100,"radarRangeKm":32,"weaponRangeKm":20,"cooldownSeconds":5,"longitude":121.49,"latitude":31.23,"course":90,"speedKnots":18,"status":"active"}]`),
		ProjectilesJSON:  models.JSONB(`[{"sessionId":"battle-test","projectileId":"p1","sourceUnitId":"blue-1","targetUnitId":"red-1","side":"blue","longitude":121.49,"latitude":31.23,"speedKmH":1600,"status":"flying"}]`),
		RadarTargetsJSON: models.JSONB(`[{"sessionId":"battle-test","radarId":"SIM","targetId":"red-1","side":"red","longitude":121.5,"latitude":31.24,"course":270,"speedKnots":20,"confidence":0.9,"detected":true}]`),
		EventsJSON:       models.JSONB(`[{"sessionId":"battle-test","eventId":"e1","type":"WEAPON_FIRED","severity":"INFO","message":"Blue fires","sourceUnitId":"blue-1","targetUnitId":"red-1","occurredAt":"2026-06-06T12:00:00Z"}]`),
	}

	view, err := decodeBattleSnapshot(snapshot)
	if err != nil {
		t.Fatalf("decodeBattleSnapshot returned error: %v", err)
	}
	if view.Tick != 3 || view.SnapshotTime != snapshotTime {
		t.Fatalf("unexpected snapshot metadata: tick=%d time=%s", view.Tick, view.SnapshotTime)
	}
	if len(view.Units) != 1 || view.Units[0].UnitID != "blue-1" {
		t.Fatalf("unexpected units: %#v", view.Units)
	}
	if len(view.Projectiles) != 1 || view.Projectiles[0].ProjectileID != "p1" {
		t.Fatalf("unexpected projectiles: %#v", view.Projectiles)
	}
	if len(view.RadarTargets) != 1 || !view.RadarTargets[0].Detected {
		t.Fatalf("unexpected radar targets: %#v", view.RadarTargets)
	}
	if len(view.Events) != 1 || view.Events[0].Type != "WEAPON_FIRED" {
		t.Fatalf("unexpected events: %#v", view.Events)
	}
}

func TestBuildDamageRankingSortsByDamage(t *testing.T) {
	ranking := buildDamageRanking([]models.BattleUnit{
		{UnitID: "blue-1", Name: "Blue", Side: "blue", HP: 80, MaxHP: 100, Status: "active"},
		{UnitID: "red-1", Name: "Red", Side: "red", HP: 20, MaxHP: 100, Status: "active"},
		{UnitID: "red-2", Name: "Red Two", Side: "red", HP: 90, MaxHP: 100, Status: "active"},
	})

	if len(ranking) != 3 {
		t.Fatalf("expected 3 ranking items, got %d", len(ranking))
	}
	if ranking[0].UnitID != "red-1" || ranking[0].DamageTaken != 80 {
		t.Fatalf("expected red-1 first with 80 damage, got %#v", ranking[0])
	}
}

func TestBattleReportDeduplicatesEventsByEventID(t *testing.T) {
	occurredAt := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	events := []models.BattleEvent{
		{EventID: "fired-1", Type: "WEAPON_FIRED", Message: "Blue fires.", OccurredAt: occurredAt},
		{EventID: "fired-1", Type: "WEAPON_FIRED", Message: "Blue fires duplicate.", OccurredAt: occurredAt.Add(time.Second)},
		{EventID: "hit-1", Type: "PROJECTILE_HIT", Message: "Hit.", OccurredAt: occurredAt.Add(2 * time.Second)},
	}

	report := buildBattleReport(models.BattleSession{SessionID: "battle-test", Status: "stopped"}, events, nil)
	if report.FiredCount != 1 || report.HitCount != 1 {
		t.Fatalf("expected duplicate eventId to be counted once, got fired=%d hit=%d", report.FiredCount, report.HitCount)
	}
}

func TestInferBattleWinner(t *testing.T) {
	if winner := inferBattleWinner("blue_victory", nil); winner != "blue" {
		t.Fatalf("expected blue winner, got %s", winner)
	}
	if winner := inferBattleWinner("running", nil); winner != "pending" {
		t.Fatalf("expected pending winner, got %s", winner)
	}
	winner := inferBattleWinner("stopped", []models.BattleEvent{
		{Type: "BATTLE_ENDED", Message: "Red force wins the radar barrage duel."},
	})
	if winner != "red" {
		t.Fatalf("expected red winner, got %s", winner)
	}
}

func TestFindScenarioDefaultsWhenCodeIsEmpty(t *testing.T) {
	service := NewAppService(nil, nil)

	scenario, err := service.findScenario("")
	if err != nil {
		t.Fatalf("expected empty scenario code to default, got %v", err)
	}
	if scenario.Code != "open-water-duel" {
		t.Fatalf("expected default open-water-duel scenario, got %s", scenario.Code)
	}
}

func TestFindScenarioAcceptsKnownScenario(t *testing.T) {
	service := NewAppService(nil, nil)

	scenario, err := service.findScenario(" close-quarter-barrage ")
	if err != nil {
		t.Fatalf("expected known scenario, got %v", err)
	}
	if scenario.Code != "close-quarter-barrage" {
		t.Fatalf("expected close-quarter-barrage scenario, got %s", scenario.Code)
	}
}

func TestFindScenarioRejectsUnknownScenario(t *testing.T) {
	service := NewAppService(nil, nil)

	if _, err := service.findScenario("missing-scenario"); !IsValidationError(err) {
		t.Fatalf("expected validation error for unknown scenario, got %v", err)
	}
}

func TestValidateShipNormalizesAndDefaultsStatus(t *testing.T) {
	ship := &models.Ship{
		Name:    "  Patrol 01 ",
		MMSI:    " 412000001 ",
		LengthM: 72,
		WidthM:  11,
	}

	if err := validateShip(ship); err != nil {
		t.Fatalf("expected valid ship, got %v", err)
	}
	if ship.Name != "Patrol 01" || ship.MMSI != "412000001" || ship.Status != "active" {
		t.Fatalf("expected normalized ship with default status, got %#v", ship)
	}
}

func TestValidateShipRejectsInvalidMMSI(t *testing.T) {
	ship := &models.Ship{Name: "Patrol 01", MMSI: "412ABC001", Status: "active"}

	if err := validateShip(ship); !IsValidationError(err) {
		t.Fatalf("expected MMSI validation error, got %v", err)
	}
}

func TestValidateShipRejectsInvalidDimensions(t *testing.T) {
	ship := &models.Ship{Name: "Patrol 01", MMSI: "412000001", LengthM: -1, Status: "active"}

	if err := validateShip(ship); !IsValidationError(err) {
		t.Fatalf("expected dimension validation error, got %v", err)
	}
}

func TestValidateShipRejectsInvalidStatus(t *testing.T) {
	ship := &models.Ship{Name: "Patrol 01", MMSI: "412000001", Status: "retired"}

	if err := validateShip(ship); !IsValidationError(err) {
		t.Fatalf("expected status validation error, got %v", err)
	}
}

func TestDispatchStatusTransitionRules(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "new to processing", from: "NEW", to: "PROCESSING", want: true},
		{name: "processing to completed", from: "PROCESSING", to: "COMPLETED", want: true},
		{name: "same status idempotent", from: "DISPATCHED", to: "DISPATCHED", want: true},
		{name: "completed cannot reopen", from: "COMPLETED", to: "PROCESSING", want: false},
		{name: "unknown status rejected", from: "UNKNOWN", to: "PROCESSING", want: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := canTransitionDispatch(tt.from, tt.to); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestNormalizeDispatchStatusUpdate(t *testing.T) {
	status, remark, changed := normalizeDispatchStatusUpdate("NEW", " PROCESSING ", "  assign rescue team  ")
	if status != "PROCESSING" || remark != "assign rescue team" || !changed {
		t.Fatalf("expected trimmed changed update, got status=%q remark=%q changed=%v", status, remark, changed)
	}

	status, remark, changed = normalizeDispatchStatusUpdate("PROCESSING", " PROCESSING ", " repeated ack ")
	if status != "PROCESSING" || remark != "repeated ack" || changed {
		t.Fatalf("expected same-status update to be idempotent, got status=%q remark=%q changed=%v", status, remark, changed)
	}
}

func TestListAlarmsRejectsInvalidStatusFilter(t *testing.T) {
	service := NewAppService(nil, nil)

	_, _, err := service.ListAlarms(context.Background(), "BROKEN", 1, 20)
	if !IsValidationError(err) {
		t.Fatalf("expected alarm status validation error, got %v", err)
	}
}

func TestListDispatchEventsRejectsInvalidStatusFilter(t *testing.T) {
	service := NewAppService(nil, nil)

	_, _, err := service.ListDispatchEvents(context.Background(), "BROKEN", 1, 20)
	if !IsValidationError(err) {
		t.Fatalf("expected dispatch status validation error, got %v", err)
	}
}

func TestCreateDispatchEventRejectsInvalidPriority(t *testing.T) {
	service := NewAppService(nil, nil)
	event := &models.DispatchEvent{Title: "Rescue task", Priority: "critical"}

	err := service.CreateDispatchEvent(context.Background(), event)
	if !IsValidationError(err) {
		t.Fatalf("expected dispatch priority validation error, got %v", err)
	}
}

func TestValidateRadarReportAcceptsSimulationPayload(t *testing.T) {
	err := validateRadarReport(RadarReportPayload{
		SessionID: "battle-1",
		RadarID:   "SIM-RADAR-01",
		Targets: []RadarTargetPayload{
			{
				TargetID:   "red-1",
				Side:       "red",
				Longitude:  121.49,
				Latitude:   31.23,
				Course:     270,
				SpeedKnots: 21,
				Confidence: 0.82,
				Detected:   true,
			},
		},
		State: &BattleStatePayload{
			SessionID: "battle-1",
			Status:    "running",
			Units: []BattleUnitPayload{
				{
					UnitID:          "blue-1",
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
					ProjectileID: "p-1",
					SourceUnitID: "blue-1",
					TargetUnitID: "red-1",
					Side:         "blue",
					Longitude:    121.48,
					Latitude:     31.22,
					SpeedKmH:     1600,
					Status:       "flying",
				},
			},
			Events: []BattleEventPayload{
				{
					Type:     "WEAPON_FIRED",
					Severity: "INFO",
					Message:  "Blue fires.",
				},
			},
		},
	})

	if err != nil {
		t.Fatalf("expected valid radar report, got %v", err)
	}
}

func TestValidateRadarReportRejectsInvalidNavigationValues(t *testing.T) {
	report := RadarReportPayload{
		SessionID: "battle-1",
		RadarID:   "SIM-RADAR-01",
		Targets: []RadarTargetPayload{
			{TargetID: "red-1", Side: "red", Longitude: 181, Latitude: 31.23, Course: 90, SpeedKnots: 10, Confidence: 0.7},
		},
	}

	if err := validateRadarReport(report); !IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}

	report.Targets[0].Longitude = 121.49
	report.Targets[0].Confidence = 1.2
	if err := validateRadarReport(report); !IsValidationError(err) {
		t.Fatalf("expected confidence validation error, got %v", err)
	}
}

func TestValidateRadarReportRejectsOversizedPayload(t *testing.T) {
	report := RadarReportPayload{
		SessionID: "battle-1",
		RadarID:   "SIM-RADAR-01",
		Targets:   make([]RadarTargetPayload, maxRadarTargetsPerReport+1),
	}

	if err := validateRadarReport(report); !IsValidationError(err) {
		t.Fatalf("expected oversized radar report validation error, got %v", err)
	}
}

func TestValidateRadarReportRejectsMismatchedStateSession(t *testing.T) {
	report := RadarReportPayload{
		SessionID: "battle-1",
		RadarID:   "SIM-RADAR-01",
		State:     &BattleStatePayload{SessionID: "battle-2", Status: "running"},
	}

	if err := validateRadarReport(report); !IsValidationError(err) {
		t.Fatalf("expected mismatched session validation error, got %v", err)
	}
}

func TestValidateRadarReportRejectsPartialEventCoordinates(t *testing.T) {
	longitude := 121.49
	report := RadarReportPayload{
		SessionID: "battle-1",
		RadarID:   "SIM-RADAR-01",
		State: &BattleStatePayload{
			Status: "running",
			Events: []BattleEventPayload{
				{Type: "PROJECTILE_HIT", Severity: "WARN", Message: "hit", Longitude: &longitude},
			},
		},
	}

	if err := validateRadarReport(report); !IsValidationError(err) {
		t.Fatalf("expected partial coordinate validation error, got %v", err)
	}
}

func TestIsValidationError(t *testing.T) {
	if !IsValidationError(&ValidationError{message: "invalid"}) {
		t.Fatal("expected pointer validation error to be recognized")
	}
	if !IsValidationError(ValidationError{message: "invalid"}) {
		t.Fatal("expected value validation error to be recognized")
	}
	if IsValidationError(nil) {
		t.Fatal("nil should not be a validation error")
	}
}

func TestIsAuthenticationError(t *testing.T) {
	if !IsAuthenticationError(&AuthenticationError{message: "denied"}) {
		t.Fatal("expected pointer authentication error to be recognized")
	}
	if !IsAuthenticationError(AuthenticationError{message: "denied"}) {
		t.Fatal("expected value authentication error to be recognized")
	}
	if IsAuthenticationError(nil) {
		t.Fatal("nil should not be an authentication error")
	}
}
