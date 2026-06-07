package sim

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"shipsim/internal/model"
	"shipsim/internal/store"
)

func TestManagerRunLifecycleAndAction(t *testing.T) {
	manager := NewManager(store.NewMemory(), slog.Default())
	run, err := manager.CreateRun(context.Background(), "test", DefaultScenario())
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.Status != model.RunCreated {
		t.Fatalf("expected created run, got %s", run.Status)
	}

	if _, err := manager.Start(context.Background(), run.ID); err != nil {
		t.Fatalf("start run: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	tracks, err := manager.Tracks(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("tracks: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("expected seeded tracks")
	}

	event, err := manager.SubmitAction(context.Background(), run.ID, model.Action{Type: "decoy"})
	if err != nil {
		t.Fatalf("submit action: %v", err)
	}
	if event.Type != "abstract_action" {
		t.Fatalf("expected abstract action event, got %s", event.Type)
	}

	paused, err := manager.Pause(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("pause run: %v", err)
	}
	if paused.Status != model.RunPaused {
		t.Fatalf("expected paused run, got %s", paused.Status)
	}
}

func TestStoppedRunCannotBeRestarted(t *testing.T) {
	manager := NewManager(store.NewMemory(), slog.Default())
	run, err := manager.CreateRun(context.Background(), "stopped", DefaultScenario())
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := manager.Start(context.Background(), run.ID); err != nil {
		t.Fatalf("start run: %v", err)
	}
	stopped, err := manager.Stop(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("stop run: %v", err)
	}
	if stopped.Status != model.RunStopped {
		t.Fatalf("expected stopped run, got %s", stopped.Status)
	}
	_, err = manager.Start(context.Background(), run.ID)
	if err == nil {
		t.Fatal("expected stopped run restart to fail")
	}
	var validation ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestRestoredRunningRunReturnsPaused(t *testing.T) {
	st := store.NewMemory()
	now := time.Now().UTC()
	run := model.Run{
		ID:           "restored-run",
		Name:         "restored",
		Status:       model.RunRunning,
		Scenario:     DefaultScenario(),
		CreatedAt:    now,
		UpdatedAt:    now,
		SafetyNotice: model.SafetyNotice,
	}
	if err := st.SaveRun(context.Background(), run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	manager := NewManager(st, slog.Default())
	restored, err := manager.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if restored.Status != model.RunPaused || !restored.Restored {
		t.Fatalf("expected restored paused run, got status=%s restored=%v", restored.Status, restored.Restored)
	}
}

func TestScenarioValidationRejectsUnsafeShape(t *testing.T) {
	manager := NewManager(store.NewMemory(), slog.Default())
	_, err := manager.CreateRun(context.Background(), "bad", model.Scenario{
		Name:       "bad",
		TickHz:     100,
		SnapshotHz: 10,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validation ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if len(validation.Details) == 0 {
		t.Fatal("expected validation details")
	}
}

func TestLegacyActionAliasUsesTrainingResponse(t *testing.T) {
	manager := NewManager(store.NewMemory(), slog.Default())
	run, err := manager.CreateRun(context.Background(), "alias", DefaultScenario())
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	event, err := manager.SubmitAction(context.Background(), run.ID, model.Action{Type: "intercept_attempt"})
	if err != nil {
		t.Fatalf("submit legacy action: %v", err)
	}
	if event.Payload["action"] != "training_response" {
		t.Fatalf("expected training_response alias, got %v", event.Payload["action"])
	}
}

func TestSubscribeReceivesSnapshot(t *testing.T) {
	manager := NewManager(store.NewMemory(), slog.Default())
	scenario := DefaultScenario()
	scenario.TickHz = 20
	scenario.SnapshotHz = 10
	run, err := manager.CreateRun(context.Background(), "sub", scenario)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	ch, cancel, err := manager.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer cancel()
	if _, err := manager.Start(context.Background(), run.ID); err != nil {
		t.Fatalf("start run: %v", err)
	}

	select {
	case snap := <-ch:
		if snap.RunID != run.ID {
			t.Fatalf("snapshot run mismatch: %s", snap.RunID)
		}
		if snap.Notice == "" {
			t.Fatal("expected safety notice")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for snapshot")
	}
}
