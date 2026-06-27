package repositories

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"shipsystem/backend/internal/models"
)

const repositoryTestDSNEnv = "SHIPSYSTEM_REPOSITORY_TEST_DSN"

func TestAckAlarmConcurrentCallsAreIdempotent(t *testing.T) {
	store, db := newRepositoryIntegrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ship := createRepositoryTestShip(t, db)
	alarm := models.Alarm{
		ShipID:  ship.ID,
		Type:    "TEST",
		Level:   "WARN",
		Title:   "Concurrent ACK",
		Message: "integration concurrency test",
		Status:  "OPEN",
	}
	if err := db.Create(&alarm).Error; err != nil {
		t.Fatalf("create alarm: %v", err)
	}

	changedCount := runConcurrentAlarmACKs(t, ctx, store, alarm.ID, 16)
	if changedCount != 1 {
		t.Fatalf("expected exactly one ACK state change, got %d", changedCount)
	}

	var saved models.Alarm
	if err := db.First(&saved, alarm.ID).Error; err != nil {
		t.Fatalf("load saved alarm: %v", err)
	}
	if saved.Status != "ACKED" || saved.AckBy == nil || saved.AckAt == nil {
		t.Fatalf("expected ACKED alarm with ack metadata, got %#v", saved)
	}

	_, changed, err := store.AckAlarm(ctx, alarm.ID, 999)
	if err != nil {
		t.Fatalf("repeat ack returned error: %v", err)
	}
	if changed {
		t.Fatal("expected repeat ACK to be idempotent")
	}
}

func TestUpdateDispatchStatusConcurrentCallsCreateOneLog(t *testing.T) {
	store, db := newRepositoryIntegrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	event := models.DispatchEvent{
		Title:       "Concurrent dispatch",
		Description: "integration concurrency test",
		Status:      "NEW",
		Priority:    "normal",
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatalf("create dispatch event: %v", err)
	}

	changedCount := runConcurrentDispatchStatusUpdates(t, ctx, store, event.ID, 16)
	if changedCount != 1 {
		t.Fatalf("expected exactly one dispatch status state change, got %d", changedCount)
	}

	var saved models.DispatchEvent
	if err := db.First(&saved, event.ID).Error; err != nil {
		t.Fatalf("load saved dispatch event: %v", err)
	}
	if saved.Status != "PROCESSING" {
		t.Fatalf("expected PROCESSING status, got %s", saved.Status)
	}

	var logCount int64
	if err := db.Model(&models.DispatchEventLog{}).Where("event_id = ?", event.ID).Count(&logCount).Error; err != nil {
		t.Fatalf("count dispatch logs: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("expected one dispatch transition log, got %d", logCount)
	}

	operatorID := uint(42)
	_, changed, err := store.UpdateDispatchStatus(ctx, event.ID, "PROCESSING", "repeat", &operatorID)
	if err != nil {
		t.Fatalf("repeat dispatch status update returned error: %v", err)
	}
	if changed {
		t.Fatal("expected repeat dispatch status update to be idempotent")
	}
	if err := db.Model(&models.DispatchEventLog{}).Where("event_id = ?", event.ID).Count(&logCount).Error; err != nil {
		t.Fatalf("count dispatch logs after repeat: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("expected repeat update not to create another log, got %d", logCount)
	}
}

func TestCreateLocationRejectsSoftDeletedShip(t *testing.T) {
	store, db := newRepositoryIntegrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ship := createRepositoryTestShip(t, db)
	if err := store.DeleteShip(ctx, ship.ID); err != nil {
		t.Fatalf("delete ship: %v", err)
	}

	err := store.CreateLocation(ctx, &models.ShipLocation{
		ShipID:     ship.ID,
		Longitude:  121.49,
		Latitude:   31.23,
		SpeedKnots: 20,
		Course:     90,
		ReportedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected soft-deleted ship location report to fail")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record-not-found for soft-deleted ship, got %v", err)
	}

	var count int64
	if err := db.Model(&models.ShipLocation{}).Where("ship_id = ?", ship.ID).Count(&count).Error; err != nil {
		t.Fatalf("count ship locations: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no location rows for soft-deleted ship, got %d", count)
	}
}

func TestCreateBattleEventsReturnsOnlyInsertedEvents(t *testing.T) {
	store, db := newRepositoryIntegrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := createRepositoryTestBattleSession(t, db)
	event := models.BattleEvent{
		SessionID:  session.SessionID,
		EventID:    "event-once",
		Type:       "WEAPON_FIRED",
		Severity:   "INFO",
		Message:    "Blue fires.",
		OccurredAt: time.Now().UTC(),
	}

	inserted, err := store.CreateBattleEvents(ctx, []models.BattleEvent{event})
	if err != nil {
		t.Fatalf("create battle event: %v", err)
	}
	if len(inserted) != 1 || inserted[0].EventID != event.EventID {
		t.Fatalf("expected one inserted event, got %#v", inserted)
	}

	inserted, err = store.CreateBattleEvents(ctx, []models.BattleEvent{event})
	if err != nil {
		t.Fatalf("repeat battle event: %v", err)
	}
	if len(inserted) != 0 {
		t.Fatalf("expected duplicate event to be ignored, got %#v", inserted)
	}
}

func TestAppendBattleSnapshotConcurrentCallsAreIdempotentBySnapshotTime(t *testing.T) {
	store, db := newRepositoryIntegrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := createRepositoryTestBattleSession(t, db)
	snapshotTime := time.Now().UTC().Truncate(time.Millisecond)
	insertedCount := runConcurrentBattleSnapshotAppends(t, ctx, store, session.SessionID, snapshotTime, 16)
	if insertedCount != 1 {
		t.Fatalf("expected exactly one inserted snapshot, got %d", insertedCount)
	}

	var count int64
	if err := db.Model(&models.BattleSnapshot{}).Where("session_id = ?", session.SessionID).Count(&count).Error; err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one snapshot row, got %d", count)
	}

	inserted, err := store.AppendBattleSnapshot(ctx, repositoryTestSnapshot(session.SessionID, snapshotTime))
	if err != nil {
		t.Fatalf("repeat snapshot append: %v", err)
	}
	if inserted {
		t.Fatal("expected repeat snapshot append to be idempotent")
	}
}

func TestStopBattleSessionConcurrentCallsAreIdempotent(t *testing.T) {
	store, db := newRepositoryIntegrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := createRepositoryTestBattleSession(t, db)
	changedCount := runConcurrentBattleSessionStops(t, ctx, store, session.SessionID, 16)
	if changedCount != 1 {
		t.Fatalf("expected exactly one battle session stop state change, got %d", changedCount)
	}

	var saved models.BattleSession
	if err := db.Where("session_id = ?", session.SessionID).First(&saved).Error; err != nil {
		t.Fatalf("load stopped battle session: %v", err)
	}
	if saved.Status != "stopped" || saved.StoppedAt == nil {
		t.Fatalf("expected stopped battle session with stoppedAt, got %#v", saved)
	}
	stoppedAt := *saved.StoppedAt

	_, changed, err := store.StopBattleSession(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("repeat battle session stop returned error: %v", err)
	}
	if changed {
		t.Fatal("expected repeat battle session stop to be idempotent")
	}

	var repeated models.BattleSession
	if err := db.Where("session_id = ?", session.SessionID).First(&repeated).Error; err != nil {
		t.Fatalf("load repeated stopped battle session: %v", err)
	}
	if repeated.StoppedAt == nil || !repeated.StoppedAt.Equal(stoppedAt) {
		t.Fatalf("expected repeat stop not to rewrite stoppedAt: first=%v repeat=%v", stoppedAt, repeated.StoppedAt)
	}
}

func newRepositoryIntegrationStore(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv(repositoryTestDSNEnv)
	if dsn == "" {
		t.Skipf("set %s to run repository DB integration tests", repositoryTestDSNEnv)
	}

	tablePrefix := fmt.Sprintf("repo_it_%d_", time.Now().UnixNano())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix},
	})
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(20)
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(
			&models.DispatchEventLog{},
			&models.DispatchEvent{},
			&models.Alarm{},
			&models.ShipLocation{},
			&models.Ship{},
			&models.BattleSnapshot{},
			&models.BattleEvent{},
			&models.BattleSession{},
		)
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(
		&models.Ship{},
		&models.ShipLocation{},
		&models.Alarm{},
		&models.DispatchEvent{},
		&models.DispatchEventLog{},
		&models.BattleSession{},
		&models.BattleEvent{},
		&models.BattleSnapshot{},
	); err != nil {
		t.Fatalf("migrate integration tables: %v", err)
	}

	return NewStore(db), db
}

func createRepositoryTestShip(t *testing.T, db *gorm.DB) models.Ship {
	t.Helper()
	ship := models.Ship{
		Name:     "Repository Integration Ship",
		MMSI:     fmt.Sprintf("%09d", time.Now().UnixNano()%1_000_000_000),
		ShipType: "test",
		Flag:     "CN",
		Status:   "active",
	}
	if err := db.Create(&ship).Error; err != nil {
		t.Fatalf("create ship: %v", err)
	}
	return ship
}

func createRepositoryTestBattleSession(t *testing.T, db *gorm.DB) models.BattleSession {
	t.Helper()
	session := models.BattleSession{
		SessionID:    fmt.Sprintf("battle-it-%d", time.Now().UnixNano()),
		Name:         "Repository Integration Battle",
		ScenarioCode: "open-water-duel",
		Status:       "running",
		StartedAt:    time.Now().UTC(),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create battle session: %v", err)
	}
	return session
}

func runConcurrentAlarmACKs(t *testing.T, ctx context.Context, store *Store, alarmID uint, workers int) int {
	t.Helper()
	start := make(chan struct{})
	results := make(chan repositoryConcurrentResult, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, changed, err := store.AckAlarm(ctx, alarmID, uint(index+1))
			results <- repositoryConcurrentResult{changed: changed, err: err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	return countChangedResults(t, results)
}

func runConcurrentDispatchStatusUpdates(t *testing.T, ctx context.Context, store *Store, eventID uint, workers int) int {
	t.Helper()
	start := make(chan struct{})
	results := make(chan repositoryConcurrentResult, workers)
	var wg sync.WaitGroup
	operatorID := uint(42)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, changed, err := store.UpdateDispatchStatus(ctx, eventID, "PROCESSING", "start processing", &operatorID)
			results <- repositoryConcurrentResult{changed: changed, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	return countChangedResults(t, results)
}

func runConcurrentBattleSnapshotAppends(t *testing.T, ctx context.Context, store *Store, sessionID string, snapshotTime time.Time, workers int) int {
	t.Helper()
	start := make(chan struct{})
	results := make(chan repositoryConcurrentResult, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			inserted, err := store.AppendBattleSnapshot(ctx, repositoryTestSnapshot(sessionID, snapshotTime))
			results <- repositoryConcurrentResult{changed: inserted, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	return countChangedResults(t, results)
}

func runConcurrentBattleSessionStops(t *testing.T, ctx context.Context, store *Store, sessionID string, workers int) int {
	t.Helper()
	start := make(chan struct{})
	results := make(chan repositoryConcurrentResult, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, changed, err := store.StopBattleSession(ctx, sessionID)
			results <- repositoryConcurrentResult{changed: changed, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	return countChangedResults(t, results)
}

func repositoryTestSnapshot(sessionID string, snapshotTime time.Time) *models.BattleSnapshot {
	return &models.BattleSnapshot{
		SessionID:        sessionID,
		SnapshotTime:     snapshotTime,
		UnitsJSON:        models.JSONB(`[]`),
		ProjectilesJSON:  models.JSONB(`[]`),
		RadarTargetsJSON: models.JSONB(`[]`),
		EventsJSON:       models.JSONB(`[]`),
	}
}

type repositoryConcurrentResult struct {
	changed bool
	err     error
}

func countChangedResults(t *testing.T, results <-chan repositoryConcurrentResult) int {
	t.Helper()
	changedCount := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent repository call returned error: %v", result.err)
		}
		if result.changed {
			changedCount++
		}
	}
	return changedCount
}
