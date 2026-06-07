package sim

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"shipsim/internal/model"
	"shipsim/internal/store"
)

type Manager struct {
	mu                   sync.RWMutex
	store                store.Store
	logger               *slog.Logger
	engines              map[string]*Engine
	scenarios            map[string]scenarioEntry
	metrics              runtimeMetrics
	snapshotWriteTimeout time.Duration
}

type RuntimeMetrics struct {
	SnapshotWriteCount    int64   `json:"snapshot_write_count"`
	SnapshotWriteFailures int64   `json:"snapshot_write_failures"`
	SnapshotWriteLastMS   float64 `json:"snapshot_write_last_ms"`
	SnapshotWriteAvgMS    float64 `json:"snapshot_write_avg_ms"`
	SnapshotWriteMaxMS    float64 `json:"snapshot_write_max_ms"`
}

type runtimeMetrics struct {
	snapshotWriteCount    atomic.Int64
	snapshotWriteFailures atomic.Int64
	snapshotWriteTotalNS  atomic.Int64
	snapshotWriteLastNS   atomic.Int64
	snapshotWriteMaxNS    atomic.Int64
}

func (m *runtimeMetrics) recordSnapshotWrite(duration time.Duration, err error) {
	ns := duration.Nanoseconds()
	m.snapshotWriteCount.Add(1)
	if err != nil {
		m.snapshotWriteFailures.Add(1)
	}
	m.snapshotWriteTotalNS.Add(ns)
	m.snapshotWriteLastNS.Store(ns)
	for {
		current := m.snapshotWriteMaxNS.Load()
		if ns <= current || m.snapshotWriteMaxNS.CompareAndSwap(current, ns) {
			return
		}
	}
}

func (m *runtimeMetrics) snapshot() RuntimeMetrics {
	count := m.snapshotWriteCount.Load()
	total := m.snapshotWriteTotalNS.Load()
	var avg float64
	if count > 0 {
		avg = nsToMS(total / count)
	}
	return RuntimeMetrics{
		SnapshotWriteCount:    count,
		SnapshotWriteFailures: m.snapshotWriteFailures.Load(),
		SnapshotWriteLastMS:   nsToMS(m.snapshotWriteLastNS.Load()),
		SnapshotWriteAvgMS:    avg,
		SnapshotWriteMaxMS:    nsToMS(m.snapshotWriteMaxNS.Load()),
	}
}

func nsToMS(ns int64) float64 {
	return float64(ns) / float64(time.Millisecond)
}

func NewManager(st store.Store, logger *slog.Logger) *Manager {
	return NewManagerWithSnapshotWriteTimeout(st, logger, 5*time.Second)
}

func NewManagerWithSnapshotWriteTimeout(st store.Store, logger *slog.Logger, snapshotWriteTimeout time.Duration) *Manager {
	manager := &Manager{
		store:                st,
		logger:               logger,
		engines:              map[string]*Engine{},
		scenarios:            map[string]scenarioEntry{},
		snapshotWriteTimeout: snapshotWriteTimeout,
	}
	manager.RegisterScenario("default", DefaultScenario(), "builtin")
	return manager
}

var defaultAllowedActions = []string{"maneuver", "decoy", "training_response"}

type ValidationError struct {
	Details []string
}

func (e ValidationError) Error() string {
	if len(e.Details) == 0 {
		return "validation failed"
	}
	return strings.Join(e.Details, "; ")
}

func (m *Manager) CreateRun(ctx context.Context, name string, scenario model.Scenario) (model.Run, error) {
	return m.CreateRunForOwner(ctx, "", name, scenario)
}

func (m *Manager) CreateRunForOwner(ctx context.Context, ownerID, name string, scenario model.Scenario) (model.Run, error) {
	if isZeroScenario(scenario) {
		scenario = DefaultScenario()
	}
	scenario = normalizeScenario(scenario)
	if err := ValidateScenario(scenario); err != nil {
		return model.Run{}, err
	}
	if name == "" {
		name = scenario.Name
	}
	now := time.Now().UTC()
	run := model.Run{
		ID:           uuid.NewString(),
		Name:         name,
		Status:       model.RunCreated,
		Scenario:     scenario,
		OwnerID:      ownerID,
		CreatedAt:    now,
		UpdatedAt:    now,
		SafetyNotice: model.SafetyNotice,
	}
	engine := newEngineWithMetrics(run, m.store, m.logger, &m.metrics, m.snapshotWriteTimeout)
	m.mu.Lock()
	m.engines[run.ID] = engine
	m.mu.Unlock()
	if err := m.store.SaveRun(ctx, run); err != nil {
		return model.Run{}, err
	}
	if m.logger != nil {
		m.logger.Info("run created", "run_id", run.ID, "owner_id", run.OwnerID, "scenario", run.Scenario.Name)
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		RunID:      run.ID,
		ScenarioID: run.Scenario.ID,
		ActorID:    ownerID,
		Action:     "run.created",
		TargetType: "run",
		TargetID:   run.ID,
		Payload: map[string]any{
			"name":     run.Name,
			"scenario": run.Scenario.Name,
		},
	})
	return run, nil
}

func (m *Manager) ListRuns(ctx context.Context, limit int) ([]model.Run, error) {
	runs, err := m.store.ListRuns(ctx, limit)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		runs[i] = m.restoreStoredRun(runs[i])
	}
	return runs, nil
}

func (m *Manager) ListRunsForOwner(ctx context.Context, limit int, ownerID string) ([]model.Run, error) {
	runs, err := m.store.ListRunsForOwner(ctx, limit, ownerID)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		runs[i] = m.restoreStoredRun(runs[i])
	}
	return runs, nil
}

func (m *Manager) GetRun(ctx context.Context, id string) (model.Run, error) {
	if engine := m.engine(id); engine != nil {
		return engine.Run(), nil
	}
	run, err := m.store.GetRun(ctx, id)
	if err != nil {
		return model.Run{}, err
	}
	return m.restoreStoredRun(run), nil
}

func (m *Manager) GetRunForOwner(ctx context.Context, id, ownerID string) (model.Run, error) {
	if engine := m.engine(id); engine != nil {
		run := engine.Run()
		if ownerID != "" && run.OwnerID != ownerID {
			return model.Run{}, errors.New("run not found")
		}
		return run, nil
	}
	run, err := m.store.GetRunForOwner(ctx, id, ownerID)
	if err != nil {
		return model.Run{}, err
	}
	return m.restoreStoredRun(run), nil
}

func (m *Manager) Start(ctx context.Context, id string) (model.Run, error) {
	engine, err := m.requireEngine(ctx, id)
	if err != nil {
		return model.Run{}, err
	}
	if engine.Run().Status == model.RunStopped {
		return model.Run{}, ValidationError{Details: []string{"stopped runs cannot be restarted; create a new run"}}
	}
	run := engine.Start(ctx)
	if m.logger != nil {
		m.logger.Info("run started", "run_id", run.ID, "status", run.Status)
	}
	if err := m.store.SaveRun(ctx, run); err != nil {
		return model.Run{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{RunID: run.ID, ActorID: run.OwnerID, Action: "run.started", TargetType: "run", TargetID: run.ID})
	return run, nil
}

func (m *Manager) Pause(ctx context.Context, id string) (model.Run, error) {
	engine, err := m.requireEngine(ctx, id)
	if err != nil {
		return model.Run{}, err
	}
	run := engine.Pause()
	if m.logger != nil {
		m.logger.Info("run paused", "run_id", run.ID, "status", run.Status)
	}
	if err := m.store.SaveRun(ctx, run); err != nil {
		return model.Run{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{RunID: run.ID, ActorID: run.OwnerID, Action: "run.paused", TargetType: "run", TargetID: run.ID})
	return run, nil
}

func (m *Manager) Stop(ctx context.Context, id string) (model.Run, error) {
	engine, err := m.requireEngine(ctx, id)
	if err != nil {
		return model.Run{}, err
	}
	run := engine.Stop()
	if m.logger != nil {
		m.logger.Info("run stopped", "run_id", run.ID, "status", run.Status)
	}
	if err := m.store.SaveRun(ctx, run); err != nil {
		return model.Run{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{RunID: run.ID, ActorID: run.OwnerID, Action: "run.stopped", TargetType: "run", TargetID: run.ID})
	return run, nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	engines := m.engineSnapshot()
	var errs []error
	for _, engine := range engines {
		run := engine.Run()
		if run.Status != model.RunRunning {
			continue
		}
		stopped := engine.Stop()
		if m.logger != nil {
			m.logger.Info("run stopped during shutdown", "run_id", stopped.ID, "status", stopped.Status)
		}
		if err := m.store.SaveRun(ctx, stopped); err != nil {
			errs = append(errs, fmt.Errorf("save shutdown run %s: %w", stopped.ID, err))
			continue
		}
		if err := m.saveSnapshot(ctx, engine.Snapshot()); err != nil {
			errs = append(errs, fmt.Errorf("save shutdown snapshot %s: %w", stopped.ID, err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) SubmitAction(ctx context.Context, id string, action model.Action) (model.SimEvent, error) {
	engine, err := m.requireEngine(ctx, id)
	if err != nil {
		return model.SimEvent{}, err
	}
	action.Type = normalizeActionType(action.Type)
	if !actionAllowed(engine.Run().Scenario.AllowedActions, action.Type) {
		return model.SimEvent{}, ValidationError{Details: []string{fmt.Sprintf("action %q is not allowed by this scenario", action.Type)}}
	}
	event := engine.SubmitAction(action)
	if err := m.store.SaveEvent(ctx, event); err != nil {
		return model.SimEvent{}, err
	}
	if err := m.saveSnapshot(ctx, engine.Snapshot()); err != nil {
		return model.SimEvent{}, err
	}
	run := engine.Run()
	_ = m.recordAudit(ctx, model.AuditLog{
		RunID:      run.ID,
		ActorID:    action.ActorID,
		Action:     "run.action_submitted",
		TargetType: "event",
		TargetID:   event.ID,
		Payload: map[string]any{
			"action":        action.Type,
			"training_only": true,
		},
	})
	return event, nil
}

func (m *Manager) UpdateRunMetadata(ctx context.Context, id string, metadata model.RunMetadata, actorID string) (model.Run, error) {
	run, err := m.GetRun(ctx, id)
	if err != nil {
		return model.Run{}, err
	}
	run.Tags = normalizeStringList(metadata.Tags, 12)
	run.Trainees = normalizeStringList(metadata.Trainees, 24)
	run.InstructorNotes = strings.TrimSpace(metadata.InstructorNotes)
	run.UpdatedAt = time.Now().UTC()
	if metadata.Archived {
		if run.ArchivedAt.IsZero() {
			run.ArchivedAt = run.UpdatedAt
		}
	} else {
		run.ArchivedAt = time.Time{}
	}
	if engine := m.engine(id); engine != nil {
		engine.SetRunMetadata(run)
	}
	if err := m.store.SaveRun(ctx, run); err != nil {
		return model.Run{}, err
	}
	action := "run.metadata_updated"
	if metadata.Archived {
		action = "run.archived"
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		RunID:      run.ID,
		ActorID:    actorID,
		Action:     action,
		TargetType: "run",
		TargetID:   run.ID,
		Payload: map[string]any{
			"tags":      run.Tags,
			"trainees":  run.Trainees,
			"archived":  !run.ArchivedAt.IsZero(),
			"has_notes": run.InstructorNotes != "",
		},
	})
	return run, nil
}

func (m *Manager) AddEventAnnotation(ctx context.Context, runID string, annotation model.EventAnnotation, actorID string) (model.EventAnnotation, error) {
	if _, err := m.GetRun(ctx, runID); err != nil {
		return model.EventAnnotation{}, err
	}
	annotation.RunID = runID
	annotation.Note = strings.TrimSpace(annotation.Note)
	if annotation.Note == "" {
		return model.EventAnnotation{}, ValidationError{Details: []string{"annotation note is required"}}
	}
	if annotation.ActorID == "" {
		annotation.ActorID = actorID
	}
	saved, err := m.store.SaveEventAnnotation(ctx, annotation)
	if err != nil {
		return model.EventAnnotation{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		RunID:      runID,
		ActorID:    saved.ActorID,
		Action:     "event.annotated",
		TargetType: "event",
		TargetID:   saved.EventID,
		Payload: map[string]any{
			"annotation_id": saved.ID,
			"has_event_id":  saved.EventID != "",
		},
	})
	return saved, nil
}

func (m *Manager) EventAnnotations(ctx context.Context, runID string) ([]model.EventAnnotation, error) {
	if _, err := m.GetRun(ctx, runID); err != nil {
		return nil, err
	}
	return m.store.ListEventAnnotations(ctx, runID)
}

func (m *Manager) AuditLogs(ctx context.Context, query model.AuditLogQuery) ([]model.AuditLog, error) {
	if query.RunID != "" {
		if _, err := m.GetRun(ctx, query.RunID); err != nil {
			return nil, err
		}
	}
	return m.store.ListAuditLogs(ctx, query)
}

func (m *Manager) RecordReportExport(ctx context.Context, runID, actorID, format string) {
	_ = m.recordAudit(ctx, model.AuditLog{
		RunID:      runID,
		ActorID:    actorID,
		Action:     "report.exported",
		TargetType: "run",
		TargetID:   runID,
		Payload: map[string]any{
			"format":        format,
			"training_only": true,
		},
	})
}

func (m *Manager) Tracks(ctx context.Context, id string) ([]model.Track, error) {
	if engine := m.engine(id); engine != nil {
		return engine.Tracks(), nil
	}
	return m.store.ListTracks(ctx, id)
}

func (m *Manager) Events(ctx context.Context, id string, query model.EventQuery) (model.EventPage, error) {
	if _, err := m.GetRun(ctx, id); err != nil {
		return model.EventPage{}, err
	}
	return m.store.ListEvents(ctx, id, query)
}

func (m *Manager) TrackPoints(ctx context.Context, id string, query model.TrackPointQuery) ([]model.TrackPoint, error) {
	if _, err := m.GetRun(ctx, id); err != nil {
		return nil, err
	}
	return m.store.ListTrackPoints(ctx, id, query)
}

func (m *Manager) Snapshots(ctx context.Context, id string, query model.SnapshotQuery) ([]model.SnapshotFrame, error) {
	if _, err := m.GetRun(ctx, id); err != nil {
		return nil, err
	}
	return m.store.ListSnapshots(ctx, id, query)
}

func (m *Manager) NearestSnapshot(ctx context.Context, id string, at time.Time) (model.SnapshotFrame, error) {
	if _, err := m.GetRun(ctx, id); err != nil {
		return model.SnapshotFrame{}, err
	}
	return m.store.NearestSnapshot(ctx, id, at)
}

func (m *Manager) SnapshotRange(ctx context.Context, id string) (model.SnapshotRange, bool, error) {
	if _, err := m.GetRun(ctx, id); err != nil {
		return model.SnapshotRange{}, false, err
	}
	return m.store.SnapshotRange(ctx, id)
}

func (m *Manager) Zones(ctx context.Context, id string) ([]model.Zone, error) {
	if engine := m.engine(id); engine != nil {
		return append([]model.Zone(nil), engine.Run().Scenario.Zones...), nil
	}
	return m.store.ListZones(ctx, id)
}

func (m *Manager) Subscribe(id string) (<-chan model.Snapshot, func(), error) {
	engine, err := m.requireEngine(context.Background(), id)
	if err != nil {
		return nil, nil, err
	}
	ch, cancel := engine.Subscribe()
	return ch, cancel, nil
}

func (m *Manager) engine(id string) *Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.engines[id]
}

func (m *Manager) engineSnapshot() []*Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	engines := make([]*Engine, 0, len(m.engines))
	for _, engine := range m.engines {
		engines = append(engines, engine)
	}
	return engines
}

func (m *Manager) requireEngine(ctx context.Context, id string) (*Engine, error) {
	engine := m.engine(id)
	if engine != nil {
		return engine, nil
	}
	run, err := m.store.GetRun(ctx, id)
	if err != nil {
		return nil, errors.New("run engine not found; create a new run in this process")
	}
	run = m.restoreStoredRun(run)
	if run.Status == model.RunStopped {
		return nil, errors.New("run engine not found for stopped run")
	}
	if frame, err := m.store.LatestSnapshot(ctx, run.ID); err == nil {
		engine = newEngineFromSnapshotWithMetrics(run, m.store, m.logger, frame, &m.metrics, m.snapshotWriteTimeout)
		if m.logger != nil {
			m.logger.Info("run restored from snapshot", "run_id", run.ID, "tick", frame.Tick)
		}
	} else {
		engine = newEngineWithMetrics(run, m.store, m.logger, &m.metrics, m.snapshotWriteTimeout)
		if m.logger != nil && run.Restored {
			m.logger.Warn("run restored without snapshot fallback", "run_id", run.ID, "error", err)
		}
	}
	m.mu.Lock()
	m.engines[run.ID] = engine
	m.mu.Unlock()
	if run.Restored {
		_ = m.store.SaveRun(ctx, run)
	}
	return engine, nil
}

func (m *Manager) StoreStatus(ctx context.Context) (model.StoreStatus, error) {
	return m.store.Ready(ctx)
}

func (m *Manager) RuntimeMetrics() RuntimeMetrics {
	return m.metrics.snapshot()
}

func (m *Manager) EngineCounts() (int, int) {
	engines := m.engineSnapshot()
	running := 0
	for _, engine := range engines {
		if engine.Run().Status == model.RunRunning {
			running++
		}
	}
	return len(engines), running
}

func (m *Manager) saveSnapshot(ctx context.Context, snapshot model.Snapshot) error {
	start := time.Now()
	snapshotCtx, cancel := snapshotContext(ctx, m.snapshotWriteTimeout)
	defer cancel()
	err := m.store.SaveSnapshot(snapshotCtx, snapshot)
	m.metrics.recordSnapshotWrite(time.Since(start), err)
	return err
}

func (m *Manager) PreviewPrune(ctx context.Context, policy model.RetentionPolicy) (model.RetentionPreview, error) {
	return m.store.PreviewPrune(ctx, policy)
}

func (m *Manager) Prune(ctx context.Context, policy model.RetentionPolicy) (model.RetentionResult, error) {
	return m.store.Prune(ctx, policy)
}

func (m *Manager) Report(ctx context.Context, id string) (model.RunReport, error) {
	run, err := m.GetRun(ctx, id)
	if err != nil {
		return model.RunReport{}, err
	}
	events, err := m.store.ListEvents(ctx, id, model.EventQuery{Limit: 200})
	if err != nil {
		return model.RunReport{}, err
	}
	snapshotRange, hasSnapshots, err := m.store.SnapshotRange(ctx, id)
	if err != nil {
		return model.RunReport{}, err
	}
	var frames []model.SnapshotFrame
	if hasSnapshots {
		frames, err = m.store.ListSnapshots(ctx, id, model.SnapshotQuery{Limit: 1000})
		if err != nil {
			return model.RunReport{}, err
		}
	}
	finalTracks, err := m.reportFinalTracks(ctx, id, frames)
	if err != nil {
		return model.RunReport{}, err
	}
	annotations, err := m.store.ListEventAnnotations(ctx, id)
	if err != nil {
		return model.RunReport{}, err
	}
	auditLogs, err := m.store.ListAuditLogs(ctx, model.AuditLogQuery{RunID: id, Limit: 200})
	if err != nil {
		return model.RunReport{}, err
	}
	report := model.RunReport{
		Version:         2,
		Run:             run,
		ReplayMode:      "legacy",
		DurationSeconds: runDurationSeconds(run),
		TrackCount:      len(finalTracks),
		ActionStats:     actionStats(events.Items),
		EventAudit:      eventAuditSummary(events.Items),
		ThreatSummary:   threatSummary(frames, finalTracks),
		FinalTracks:     trackStatusSummaries(finalTracks),
		Events:          reverseEvents(events.Items),
		Annotations:     annotations,
		AuditLogs:       auditLogs,
		SafetyNotice:    model.SafetyNotice,
	}
	if hasSnapshots {
		report.ReplayMode = "snapshot"
		report.SnapshotRange = &snapshotRange
		report.SnapshotCoverage = snapshotCoverage(snapshotRange)
	}
	report.Assessment = trainingAssessment(report)
	return report, nil
}

func (m *Manager) reportFinalTracks(ctx context.Context, id string, frames []model.SnapshotFrame) ([]model.Track, error) {
	if len(frames) > 0 {
		return cloneTracks(frames[len(frames)-1].Tracks), nil
	}
	return m.Tracks(ctx, id)
}

func (m *Manager) recordAudit(ctx context.Context, log model.AuditLog) error {
	if log.OccurredAt.IsZero() {
		log.OccurredAt = time.Now().UTC()
	}
	return m.store.SaveAuditLog(ctx, log)
}

func trainingAssessment(report model.RunReport) model.TrainingAssessment {
	criteria := []model.AssessmentCriterion{
		{
			Name:  "training_actions",
			Value: boundedScore(report.EventAudit.EventCount, 6),
			Note:  "Abstract count of submitted training actions; not a tactical recommendation.",
		},
		{
			Name:  "replay_coverage",
			Value: replayCoverageScore(report),
			Note:  "Replay evidence coverage for after-action review.",
		},
		{
			Name:  "instructor_context",
			Value: instructorContextScore(report),
			Note:  "Presence of tags, trainees, instructor notes, and event annotations.",
		},
	}
	total := 0
	for _, criterion := range criteria {
		total += criterion.Value
	}
	score := total / len(criteria)
	label := "needs_review"
	switch {
	case score >= 80:
		label = "complete_training_record"
	case score >= 50:
		label = "partial_training_record"
	}
	return model.TrainingAssessment{
		Score:        score,
		Label:        label,
		Criteria:     criteria,
		SafetyNotice: model.SafetyNotice,
	}
}

func replayCoverageScore(report model.RunReport) int {
	if report.ReplayMode != "snapshot" || report.SnapshotCoverage == nil {
		return 25
	}
	if report.SnapshotCoverage.Count >= 20 {
		return 100
	}
	return boundedScore(report.SnapshotCoverage.Count, 20)
}

func instructorContextScore(report model.RunReport) int {
	points := 0
	if len(report.Run.Tags) > 0 {
		points += 25
	}
	if len(report.Run.Trainees) > 0 {
		points += 25
	}
	if strings.TrimSpace(report.Run.InstructorNotes) != "" {
		points += 25
	}
	if len(report.Annotations) > 0 {
		points += 25
	}
	return points
}

func boundedScore(value, target int) int {
	if target <= 0 {
		return 0
	}
	score := int(math.Round(float64(value) / float64(target) * 100))
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func runDurationSeconds(run model.Run) int64 {
	start := run.StartedAt
	if start.IsZero() {
		start = run.CreatedAt
	}
	end := run.StoppedAt
	if end.IsZero() {
		end = run.UpdatedAt
	}
	if end.IsZero() || end.Before(start) {
		return 0
	}
	return int64(end.Sub(start).Seconds())
}

func actionStats(events []model.SimEvent) []model.ActionStat {
	counts := map[string]int{}
	for _, event := range events {
		counts[actionType(event)]++
	}
	actions := make([]string, 0, len(counts))
	for action := range counts {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	stats := make([]model.ActionStat, 0, len(actions))
	for _, action := range actions {
		stats = append(stats, model.ActionStat{Type: action, Count: counts[action]})
	}
	return stats
}

func eventAuditSummary(events []model.SimEvent) model.EventAuditSummary {
	var firstActionAt *time.Time
	var lastActionAt *time.Time
	actorCounts := map[string]int{}
	for _, event := range events {
		occurredAt := event.OccurredAt
		if firstActionAt == nil || occurredAt.Before(*firstActionAt) {
			firstActionAt = &occurredAt
		}
		if lastActionAt == nil || occurredAt.After(*lastActionAt) {
			lastActionAt = &occurredAt
		}
		if actor := actorID(event); actor != "" {
			actorCounts[actor]++
		}
	}
	actors := make([]string, 0, len(actorCounts))
	for actor := range actorCounts {
		actors = append(actors, actor)
	}
	sort.Strings(actors)
	actorStats := make([]model.ActorStat, 0, len(actors))
	for _, actor := range actors {
		actorStats = append(actorStats, model.ActorStat{ActorID: actor, Count: actorCounts[actor]})
	}
	return model.EventAuditSummary{
		EventCount:    len(events),
		ActionStats:   actionStats(events),
		ActorStats:    actorStats,
		FirstActionAt: firstActionAt,
		LastActionAt:  lastActionAt,
	}
}

func actionType(event model.SimEvent) string {
	action, _ := event.Payload["action"].(string)
	if action != "" {
		return action
	}
	return event.Type
}

func actorID(event model.SimEvent) string {
	for _, key := range []string{"actor_id", "actor"} {
		if value, _ := event.Payload[key].(string); value != "" {
			return value
		}
	}
	return ""
}

func snapshotCoverage(snapshotRange model.SnapshotRange) *model.SnapshotCoverage {
	coverage := &model.SnapshotCoverage{
		From:  snapshotRange.From,
		To:    snapshotRange.To,
		Count: snapshotRange.Count,
	}
	if snapshotRange.Count > 1 {
		coverage.AverageIntervalMS = float64(snapshotRange.To.Sub(snapshotRange.From).Milliseconds()) / float64(snapshotRange.Count-1)
	}
	return coverage
}

func threatSummary(frames []model.SnapshotFrame, finalTracks []model.Track) model.ThreatSummary {
	initial := threatCounts(nil)
	final := threatCounts(finalTracks)
	highWatermark := final[model.ThreatHigh]
	if len(frames) > 0 {
		initial = threatCounts(frames[0].Tracks)
		for _, frame := range frames {
			counts := threatCounts(frame.Tracks)
			if counts[model.ThreatHigh] > highWatermark {
				highWatermark = counts[model.ThreatHigh]
			}
		}
	}
	return model.ThreatSummary{
		Initial:       initial,
		Final:         final,
		HighWatermark: highWatermark,
	}
}

func threatCounts(tracks []model.Track) map[model.ThreatLevel]int {
	counts := map[model.ThreatLevel]int{
		model.ThreatLow:    0,
		model.ThreatMedium: 0,
		model.ThreatHigh:   0,
	}
	for _, track := range tracks {
		counts[track.Threat]++
	}
	return counts
}

func trackStatusSummaries(tracks []model.Track) []model.TrackStatusSummary {
	sorted := cloneTracks(tracks)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TrackNo != sorted[j].TrackNo {
			return sorted[i].TrackNo < sorted[j].TrackNo
		}
		return sorted[i].ID < sorted[j].ID
	})
	out := make([]model.TrackStatusSummary, 0, len(sorted))
	for _, track := range sorted {
		out = append(out, model.TrackStatusSummary{
			TrackID:    track.ID,
			TrackNo:    track.TrackNo,
			Kind:       track.Kind,
			Threat:     track.Threat,
			Status:     track.Status,
			Confidence: track.Confidence,
			UpdatedAt:  track.UpdatedAt,
		})
	}
	return out
}

func cloneTracks(tracks []model.Track) []model.Track {
	return append([]model.Track(nil), tracks...)
}

func reverseEvents(events []model.SimEvent) []model.SimEvent {
	out := append([]model.SimEvent(nil), events...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func normalizeStringList(items []string, maxItems int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, minInt(len(items), maxItems))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if len(item) > 80 {
			item = item[:80]
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
		if len(out) == maxItems {
			break
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *Manager) restoreStoredRun(run model.Run) model.Run {
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	if m.engine(run.ID) != nil {
		return run
	}
	if run.Status == model.RunRunning {
		run.Status = model.RunPaused
		run.UpdatedAt = time.Now().UTC()
		run.Restored = true
	}
	if run.Status == model.RunPaused {
		run.Restored = true
	}
	return run
}

func normalizeScenario(s model.Scenario) model.Scenario {
	if s.ID == "" {
		s.ID = "custom"
	}
	if s.Name == "" {
		s.Name = "custom-training-scenario"
	}
	if s.Version <= 0 {
		s.Version = 1
	}
	if s.TickHz <= 0 {
		s.TickHz = 20
	}
	if s.SnapshotHz <= 0 {
		s.SnapshotHz = 10
	}
	if s.InitialContacts <= 0 {
		s.InitialContacts = 5
	}
	if s.Seed == 0 {
		s.Seed = 20260605
	}
	if s.Ownship == (model.Vec3{}) {
		s.Ownship = model.Vec3{Lon: 121.5, Lat: 31.2, Alt: 0}
	}
	if len(s.Sensors) == 0 {
		s.Sensors = []model.Sensor{
			{ID: "sim-sensor-1", Name: "Simulated Search Sensor", Kind: "simulated_sensor", Position: s.Ownship},
		}
	}
	if len(s.AllowedActions) == 0 {
		s.AllowedActions = append([]string(nil), defaultAllowedActions...)
	} else {
		for i, action := range s.AllowedActions {
			s.AllowedActions[i] = normalizeActionType(action)
		}
	}
	for i := range s.Tracks {
		if s.Tracks[i].ID == "" {
			s.Tracks[i].ID = uuid.NewString()
		}
		if s.Tracks[i].TrackNo == "" {
			s.Tracks[i].TrackNo = "T-" + s.Tracks[i].ID[:8]
		}
		if s.Tracks[i].Kind == "" {
			s.Tracks[i].Kind = "surface_contact"
		}
		if s.Tracks[i].Confidence == 0 {
			s.Tracks[i].Confidence = 0.72
		}
		if s.Tracks[i].Status == "" {
			s.Tracks[i].Status = "active"
		}
	}
	return s
}

func DefaultScenario() model.Scenario {
	ownship := model.Vec3{Lon: 121.5, Lat: 31.2, Alt: 0}
	return model.Scenario{
		ID:              "default",
		Name:            "demo-training-scenario",
		Description:     "Default safe training scenario with simulated contacts.",
		Version:         1,
		Seed:            20260605,
		TickHz:          20,
		SnapshotHz:      10,
		InitialContacts: 5,
		Ownship:         ownship,
		AllowedActions:  append([]string(nil), defaultAllowedActions...),
		Sensors: []model.Sensor{
			{ID: "sim-sensor-1", Name: "Simulated Search Sensor", Kind: "simulated_sensor", Position: ownship},
		},
		Zones: []model.Zone{
			{
				ID:   "training-area",
				Name: "Training Area",
				Kind: "exercise_boundary",
				Polygon: []model.Vec3{
					{Lon: 121.1, Lat: 30.9}, {Lon: 121.9, Lat: 30.9}, {Lon: 121.9, Lat: 31.6}, {Lon: 121.1, Lat: 31.6},
				},
			},
		},
	}
}

func isZeroScenario(s model.Scenario) bool {
	return s.Name == "" &&
		s.Seed == 0 &&
		s.TickHz == 0 &&
		s.SnapshotHz == 0 &&
		s.Ownship == (model.Vec3{}) &&
		len(s.Sensors) == 0 &&
		len(s.Zones) == 0 &&
		s.InitialContacts == 0 &&
		len(s.Tracks) == 0 &&
		len(s.Contacts) == 0 &&
		len(s.AllowedActions) == 0
}

func ValidateScenario(s model.Scenario) error {
	var details []string
	if s.Name == "" {
		details = append(details, "scenario name is required")
	}
	if s.TickHz < 1 || s.TickHz > 60 {
		details = append(details, "tick_hz must be between 1 and 60")
	}
	if s.SnapshotHz < 1 || s.SnapshotHz > 20 {
		details = append(details, "snapshot_hz must be between 1 and 20")
	}
	if s.SnapshotHz > s.TickHz {
		details = append(details, "snapshot_hz must be less than or equal to tick_hz")
	}
	if !validLonLat(s.Ownship) {
		details = append(details, "ownship lon/lat is outside valid range")
	}
	if len(s.Sensors) == 0 {
		details = append(details, "at least one simulated sensor is required")
	}
	sensorIDs := map[string]struct{}{}
	for i, sensor := range s.Sensors {
		if sensor.ID == "" {
			details = append(details, fmt.Sprintf("sensors[%d].id is required", i))
		}
		if _, ok := sensorIDs[sensor.ID]; sensor.ID != "" && ok {
			details = append(details, fmt.Sprintf("sensors[%d].id duplicates another sensor", i))
		}
		sensorIDs[sensor.ID] = struct{}{}
		if !validLonLat(sensor.Position) {
			details = append(details, fmt.Sprintf("sensors[%d].position lon/lat is outside valid range", i))
		}
	}
	if s.InitialContacts < 0 {
		details = append(details, "initial_contacts must be non-negative")
	}
	if s.InitialContacts+len(s.Tracks) > 100 {
		details = append(details, "scenario may seed at most 100 tracks")
	}
	for i, zone := range s.Zones {
		if zone.ID == "" {
			details = append(details, fmt.Sprintf("zones[%d].id is required", i))
		}
		if len(zone.Polygon) < 3 {
			details = append(details, fmt.Sprintf("zones[%d].polygon requires at least 3 points", i))
		}
		for j, point := range zone.Polygon {
			if !validLonLat(point) {
				details = append(details, fmt.Sprintf("zones[%d].polygon[%d] lon/lat is outside valid range", i, j))
			}
		}
	}
	if len(s.AllowedActions) == 0 {
		details = append(details, "allowed_actions must include at least one training action")
	}
	seenActions := map[string]struct{}{}
	for i, action := range s.AllowedActions {
		if _, ok := map[string]struct{}{"maneuver": {}, "decoy": {}, "training_response": {}}[action]; !ok {
			details = append(details, fmt.Sprintf("allowed_actions[%d] is not a supported training action", i))
		}
		if _, ok := seenActions[action]; ok {
			details = append(details, fmt.Sprintf("allowed_actions[%d] duplicates another action", i))
		}
		seenActions[action] = struct{}{}
	}
	for i, track := range s.Tracks {
		if track.ID == "" || track.TrackNo == "" {
			details = append(details, fmt.Sprintf("tracks[%d] requires id and track_no", i))
		}
		if !validLonLat(track.Position) {
			details = append(details, fmt.Sprintf("tracks[%d].position lon/lat is outside valid range", i))
		}
		if track.Confidence < 0 || track.Confidence > 1 {
			details = append(details, fmt.Sprintf("tracks[%d].confidence must be between 0 and 1", i))
		}
	}
	if len(details) > 0 {
		return ValidationError{Details: details}
	}
	return nil
}

func validLonLat(v model.Vec3) bool {
	return v.Lon >= -180 && v.Lon <= 180 && v.Lat >= -90 && v.Lat <= 90
}

func normalizeActionType(action string) string {
	switch strings.TrimSpace(action) {
	case "intercept_attempt":
		return "training_response"
	default:
		return strings.TrimSpace(action)
	}
}

func actionAllowed(allowed []string, action string) bool {
	for _, item := range allowed {
		if normalizeActionType(item) == action {
			return true
		}
	}
	return false
}

type Engine struct {
	mu                   sync.RWMutex
	run                  model.Run
	store                store.Store
	logger               *slog.Logger
	metrics              *runtimeMetrics
	snapshotWriteTimeout time.Duration
	rng                  *rand.Rand
	tick                 int64
	tracks               map[string]model.Track
	contacts             []model.Contact
	events               []model.SimEvent
	subscribers          map[chan model.Snapshot]struct{}
	cancel               context.CancelFunc
}

func NewEngine(run model.Run, st store.Store, logger *slog.Logger) *Engine {
	return newEngineWithMetrics(run, st, logger, nil, 5*time.Second)
}

func newEngineWithMetrics(run model.Run, st store.Store, logger *slog.Logger, metrics *runtimeMetrics, snapshotWriteTimeout time.Duration) *Engine {
	engine := &Engine{
		run:                  run,
		store:                st,
		logger:               logger,
		metrics:              metrics,
		snapshotWriteTimeout: snapshotWriteTimeout,
		rng:                  rand.New(rand.NewSource(run.Scenario.Seed)),
		tracks:               map[string]model.Track{},
		subscribers:          map[chan model.Snapshot]struct{}{},
	}
	engine.seedTracks(time.Now().UTC())
	return engine
}

func NewEngineFromSnapshot(run model.Run, st store.Store, logger *slog.Logger, frame model.SnapshotFrame) *Engine {
	return newEngineFromSnapshotWithMetrics(run, st, logger, frame, nil, 5*time.Second)
}

func newEngineFromSnapshotWithMetrics(run model.Run, st store.Store, logger *slog.Logger, frame model.SnapshotFrame, metrics *runtimeMetrics, snapshotWriteTimeout time.Duration) *Engine {
	engine := &Engine{
		run:                  run,
		store:                st,
		logger:               logger,
		metrics:              metrics,
		snapshotWriteTimeout: snapshotWriteTimeout,
		rng:                  rand.New(rand.NewSource(run.Scenario.Seed + frame.Tick)),
		tick:                 frame.Tick,
		tracks:               map[string]model.Track{},
		contacts:             append([]model.Contact(nil), frame.Contacts...),
		subscribers:          map[chan model.Snapshot]struct{}{},
	}
	for _, track := range frame.Tracks {
		engine.tracks[track.ID] = track
	}
	if frame.Status != "" {
		engine.run.Status = frame.Status
	}
	if engine.run.Status == model.RunRunning {
		engine.run.Status = model.RunPaused
	}
	engine.run.Restored = true
	return engine
}

func (e *Engine) Run() model.Run {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.run
}

func (e *Engine) SetRunMetadata(run model.Run) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.run.Name = run.Name
	e.run.Tags = append([]string(nil), run.Tags...)
	e.run.Trainees = append([]string(nil), run.Trainees...)
	e.run.InstructorNotes = run.InstructorNotes
	e.run.ArchivedAt = run.ArchivedAt
	e.run.UpdatedAt = run.UpdatedAt
}

func (e *Engine) Start(_ context.Context) model.Run {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.run.Status == model.RunRunning {
		return e.run
	}
	if e.run.Status == model.RunStopped {
		return e.run
	}
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.run.Status = model.RunRunning
	e.run.UpdatedAt = time.Now().UTC()
	if e.run.StartedAt.IsZero() {
		e.run.StartedAt = e.run.UpdatedAt
	}
	go e.loop(ctx)
	return e.run
}

func (e *Engine) Pause() model.Run {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	if e.run.Status != model.RunStopped {
		e.run.Status = model.RunPaused
		e.run.UpdatedAt = time.Now().UTC()
	}
	return e.run
}

func (e *Engine) Stop() model.Run {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.run.Status = model.RunStopped
	e.run.UpdatedAt = time.Now().UTC()
	e.run.StoppedAt = e.run.UpdatedAt
	return e.run
}

func (e *Engine) SubmitAction(action model.Action) model.SimEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	event := e.applyActionLocked(action, time.Now().UTC())
	e.events = append(e.events, event)
	e.broadcastLocked()
	return event
}

func (e *Engine) Snapshot() model.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.snapshotLocked()
}

func (e *Engine) Tracks() []model.Track {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracksLocked()
}

func (e *Engine) Subscribe() (<-chan model.Snapshot, func()) {
	ch := make(chan model.Snapshot, 16)
	e.mu.Lock()
	e.subscribers[ch] = struct{}{}
	snapshot := e.snapshotLocked()
	e.mu.Unlock()
	ch <- snapshot
	cancel := func() {
		e.mu.Lock()
		delete(e.subscribers, ch)
		close(ch)
		e.mu.Unlock()
	}
	return ch, cancel
}

func (e *Engine) loop(ctx context.Context) {
	tickHz := e.run.Scenario.TickHz
	snapshotEvery := max(1, tickHz/e.run.Scenario.SnapshotHz)
	ticker := time.NewTicker(time.Second / time.Duration(tickHz))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			e.mu.Lock()
			e.tick++
			e.advanceLocked(now.UTC())
			shouldBroadcast := e.tick%int64(snapshotEvery) == 0
			snapshot := e.snapshotLocked()
			e.mu.Unlock()
			if shouldBroadcast {
				e.saveSnapshot(context.Background(), snapshot)
				e.broadcast(snapshot)
			}
		}
	}
}

func (e *Engine) saveSnapshot(ctx context.Context, snapshot model.Snapshot) {
	start := time.Now()
	snapshotCtx, cancel := snapshotContext(ctx, e.snapshotWriteTimeout)
	defer cancel()
	err := e.store.SaveSnapshot(snapshotCtx, snapshot)
	if e.metrics != nil {
		e.metrics.recordSnapshotWrite(time.Since(start), err)
	}
	if err != nil && e.logger != nil {
		e.logger.Warn("snapshot persistence failed", "run_id", snapshot.RunID, "error", err)
	}
}

func snapshotContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}

func (e *Engine) seedTracks(now time.Time) {
	if len(e.run.Scenario.Tracks) > 0 {
		for _, track := range e.run.Scenario.Tracks {
			track.UpdatedAt = now
			track.Threat = e.threatFor(track)
			e.tracks[track.ID] = track
		}
		e.contacts = append(e.contacts, e.run.Scenario.Contacts...)
		return
	}
	ownship := e.run.Scenario.Ownship
	for i := 0; i < e.run.Scenario.InitialContacts; i++ {
		id := uuid.NewString()
		kind := "surface_contact"
		if i%3 == 0 {
			kind = "abstract_inbound_threat"
		}
		offset := float64(i+1) * 0.035
		track := model.Track{
			ID:         id,
			TrackNo:    "T-" + id[:8],
			Kind:       kind,
			Threat:     model.ThreatLow,
			Position:   model.Vec3{Lon: ownship.Lon + offset, Lat: ownship.Lat + offset/2, Alt: 0},
			Velocity:   model.Vec3{Lon: -0.00004 * float64(i+1), Lat: 0.00002 * float64(i+1), Alt: 0},
			Confidence: 0.72 + float64(i)*0.03,
			UpdatedAt:  now,
			Status:     "active",
		}
		e.tracks[track.ID] = track
	}
}

func (e *Engine) advanceLocked(now time.Time) {
	e.contacts = e.contacts[:0]
	for id, track := range e.tracks {
		if track.Status == "neutralized" || track.Status == "resolved_training" {
			continue
		}
		track.Position.Lon += track.Velocity.Lon
		track.Position.Lat += track.Velocity.Lat
		track.Confidence = clamp(track.Confidence+e.rng.Float64()*0.02-0.01, 0.25, 0.98)
		track.Threat = e.threatFor(track)
		track.UpdatedAt = now
		e.tracks[id] = track
		e.contacts = append(e.contacts, model.Contact{
			ID:         "C-" + track.ID[:8],
			SensorID:   e.run.Scenario.Sensors[0].ID,
			Timestamp:  now,
			Position:   track.Position,
			Velocity:   track.Velocity,
			Confidence: track.Confidence,
			Kind:       track.Kind,
		})
	}
}

func (e *Engine) threatFor(track model.Track) model.ThreatLevel {
	if track.Kind != "abstract_inbound_threat" {
		return model.ThreatLow
	}
	distance := planarDistance(e.run.Scenario.Ownship, track.Position)
	switch {
	case distance < 0.045:
		return model.ThreatHigh
	case distance < 0.09:
		return model.ThreatMedium
	default:
		return model.ThreatLow
	}
}

func (e *Engine) applyActionLocked(action model.Action, now time.Time) model.SimEvent {
	target := e.highestThreatLocked()
	payload := map[string]any{
		"action":        action.Type,
		"training_only": true,
		"note":          "Abstract training adjudication only; no real fire-control, weapon-control, or device command is produced.",
	}
	if action.ActorID != "" {
		payload["actor_id"] = action.ActorID
	}
	if target != nil {
		payload["subject_track"] = target.TrackNo
	}
	switch action.Type {
	case "maneuver":
		payload["result"] = "track confidence reduced for training display"
		if target != nil {
			target.Confidence = clamp(target.Confidence-0.08, 0.1, 1.0)
			target.UpdatedAt = now
			e.tracks[target.ID] = *target
		}
	case "decoy":
		payload["result"] = "abstract training distraction effect applied"
		if target != nil {
			target.Confidence = clamp(target.Confidence-0.16, 0.1, 1.0)
			target.UpdatedAt = now
			e.tracks[target.ID] = *target
		}
	case "training_response":
		success := e.rng.Float64() > 0.45
		payload["result"] = "abstract training response adjudicated"
		payload["success"] = success
		if success && target != nil {
			target.Status = "resolved_training"
			target.Threat = model.ThreatLow
			target.UpdatedAt = now
			e.tracks[target.ID] = *target
		}
	default:
		payload["result"] = "unknown abstract action recorded"
	}
	return model.SimEvent{
		ID:         uuid.NewString(),
		RunID:      e.run.ID,
		OccurredAt: now,
		Type:       "abstract_action",
		Payload:    payload,
	}
}

func (e *Engine) highestThreatLocked() *model.Track {
	var best *model.Track
	rank := map[model.ThreatLevel]int{model.ThreatLow: 1, model.ThreatMedium: 2, model.ThreatHigh: 3}
	for _, track := range e.tracks {
		if track.Status == "neutralized" || track.Status == "resolved_training" {
			continue
		}
		candidate := track
		if best == nil || rank[candidate.Threat] > rank[best.Threat] {
			best = &candidate
		}
	}
	return best
}

func (e *Engine) snapshotLocked() model.Snapshot {
	events := e.events
	if len(events) > 12 {
		events = events[len(events)-12:]
	}
	return model.Snapshot{
		RunID:      e.run.ID,
		Status:     e.run.Status,
		Tick:       e.tick,
		Time:       time.Now().UTC(),
		Tracks:     e.tracksLocked(),
		Contacts:   append([]model.Contact(nil), e.contacts...),
		Events:     append([]model.SimEvent(nil), events...),
		Notice:     model.SafetyNotice,
		SnapshotHz: e.run.Scenario.SnapshotHz,
	}
}

func (e *Engine) tracksLocked() []model.Track {
	tracks := make([]model.Track, 0, len(e.tracks))
	for _, track := range e.tracks {
		tracks = append(tracks, track)
	}
	return tracks
}

func (e *Engine) broadcastLocked() {
	snapshot := e.snapshotLocked()
	for ch := range e.subscribers {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

func (e *Engine) broadcast(snapshot model.Snapshot) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for ch := range e.subscribers {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

func planarDistance(a, b model.Vec3) float64 {
	x := a.Lon - b.Lon
	y := a.Lat - b.Lat
	return math.Sqrt(x*x + y*y)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
