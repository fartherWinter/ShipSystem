package store

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"shipsim/internal/model"
)

type Memory struct {
	mu      sync.RWMutex
	runs    map[string]model.Run
	events  map[string][]model.SimEvent
	tracks  map[string][]model.Track
	points  map[string][]model.TrackPoint
	snaps   map[string][]model.SnapshotFrame
	zones   map[string][]model.Zone
	contact map[string][]model.Contact
	scenes  map[string]model.ScenarioRecord
	notes   map[string][]model.EventAnnotation
	audit   []model.AuditLog
}

func NewMemory() *Memory {
	return &Memory{
		runs:    map[string]model.Run{},
		events:  map[string][]model.SimEvent{},
		tracks:  map[string][]model.Track{},
		points:  map[string][]model.TrackPoint{},
		snaps:   map[string][]model.SnapshotFrame{},
		zones:   map[string][]model.Zone{},
		contact: map[string][]model.Contact{},
		scenes:  map[string]model.ScenarioRecord{},
		notes:   map[string][]model.EventAnnotation{},
	}
}

func (m *Memory) Name() string {
	return "memory"
}

func (m *Memory) Ready(_ context.Context) (model.StoreStatus, error) {
	return model.StoreStatus{Store: m.Name(), MigrationVersion: 0}, nil
}

func (m *Memory) SaveRun(_ context.Context, run model.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	m.runs[run.ID] = run
	m.zones[run.ID] = append([]model.Zone(nil), run.Scenario.Zones...)
	return nil
}

func (m *Memory) GetRun(_ context.Context, id string) (model.Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	run, ok := m.runs[id]
	if !ok {
		return model.Run{}, errors.New("run not found")
	}
	return run, nil
}

func (m *Memory) GetRunForOwner(_ context.Context, id, ownerID string) (model.Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	run, ok := m.runs[id]
	if !ok || !ownerMatches(run, ownerID) {
		return model.Run{}, errors.New("run not found")
	}
	return run, nil
}

func (m *Memory) ListRuns(_ context.Context, limit int) ([]model.Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	runs := make([]model.Run, 0, len(m.runs))
	for _, run := range m.runs {
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

func (m *Memory) ListRunsForOwner(_ context.Context, limit int, ownerID string) ([]model.Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	runs := make([]model.Run, 0, len(m.runs))
	for _, run := range m.runs {
		if ownerMatches(run, ownerID) {
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

func (m *Memory) SaveEvent(_ context.Context, event model.SimEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[event.RunID] = append(m.events[event.RunID], event)
	return nil
}

func (m *Memory) ListEvents(_ context.Context, runID string, query model.EventQuery) (model.EventPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	events := m.events[runID]
	offset := 0
	if query.Cursor != "" {
		parsed, err := strconv.Atoi(query.Cursor)
		if err != nil || parsed < 0 {
			return model.EventPage{}, errors.New("invalid event cursor")
		}
		offset = parsed
	}
	limit := normalizeLimit(query.Limit, 50, 200)
	if offset > len(events) {
		offset = len(events)
	}
	items := make([]model.SimEvent, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		items = append(items, events[i])
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := model.EventPage{Items: append([]model.SimEvent(nil), items[offset:end]...)}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func (m *Memory) SaveSnapshot(_ context.Context, snapshot model.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	frame := snapshotFrame(snapshot)
	m.snaps[snapshot.RunID] = append(m.snaps[snapshot.RunID], frame)
	m.tracks[snapshot.RunID] = cloneTracks(snapshot.Tracks)
	m.contact[snapshot.RunID] = cloneContacts(snapshot.Contacts)
	for _, track := range snapshot.Tracks {
		m.points[snapshot.RunID] = append(m.points[snapshot.RunID], model.TrackPoint{
			TrackID:    track.ID,
			SampledAt:  track.UpdatedAt,
			Position:   track.Position,
			Speed:      absFloat(track.Velocity.Lon) + absFloat(track.Velocity.Lat) + absFloat(track.Velocity.Alt),
			Heading:    0,
			Confidence: track.Confidence,
		})
	}
	return nil
}

func (m *Memory) ListSnapshots(_ context.Context, runID string, query model.SnapshotQuery) ([]model.SnapshotFrame, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limit := normalizeLimit(query.Limit, 200, 1000)
	frames := make([]model.SnapshotFrame, 0, min(limit, len(m.snaps[runID])))
	for _, frame := range m.snaps[runID] {
		if !query.From.IsZero() && frame.SampledAt.Before(query.From) {
			continue
		}
		if !query.To.IsZero() && frame.SampledAt.After(query.To) {
			continue
		}
		frames = append(frames, cloneSnapshotFrame(frame))
		if len(frames) == limit {
			break
		}
	}
	return frames, nil
}

func (m *Memory) NearestSnapshot(_ context.Context, runID string, at time.Time) (model.SnapshotFrame, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	frames := m.snaps[runID]
	if len(frames) == 0 {
		return model.SnapshotFrame{}, errors.New("snapshot not found")
	}
	if at.IsZero() {
		return cloneSnapshotFrame(frames[len(frames)-1]), nil
	}
	best := frames[0]
	bestDistance := absDuration(best.SampledAt.Sub(at))
	for _, frame := range frames[1:] {
		distance := absDuration(frame.SampledAt.Sub(at))
		if distance < bestDistance {
			best = frame
			bestDistance = distance
		}
	}
	return cloneSnapshotFrame(best), nil
}

func (m *Memory) LatestSnapshot(_ context.Context, runID string) (model.SnapshotFrame, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	frames := m.snaps[runID]
	if len(frames) == 0 {
		return model.SnapshotFrame{}, errors.New("snapshot not found")
	}
	return cloneSnapshotFrame(frames[len(frames)-1]), nil
}

func (m *Memory) SnapshotRange(_ context.Context, runID string) (model.SnapshotRange, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	frames := m.snaps[runID]
	if len(frames) == 0 {
		return model.SnapshotRange{}, false, nil
	}
	return model.SnapshotRange{
		From:  frames[0].SampledAt,
		To:    frames[len(frames)-1].SampledAt,
		Count: len(frames),
	}, true, nil
}

func (m *Memory) ListTracks(_ context.Context, runID string) ([]model.Track, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneTracks(m.tracks[runID]), nil
}

func (m *Memory) ListTrackPoints(_ context.Context, runID string, query model.TrackPointQuery) ([]model.TrackPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limit := normalizeLimit(query.Limit, 200, 1000)
	points := make([]model.TrackPoint, 0, min(limit, len(m.points[runID])))
	for _, point := range m.points[runID] {
		if query.TrackID != "" && point.TrackID != query.TrackID {
			continue
		}
		if !query.From.IsZero() && point.SampledAt.Before(query.From) {
			continue
		}
		if !query.To.IsZero() && point.SampledAt.After(query.To) {
			continue
		}
		points = append(points, point)
		if len(points) == limit {
			break
		}
	}
	return points, nil
}

func (m *Memory) ListZones(_ context.Context, runID string) ([]model.Zone, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]model.Zone(nil), m.zones[runID]...), nil
}

func (m *Memory) ListScenarioSummaries(_ context.Context) ([]model.ScenarioSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	summaries := make([]model.ScenarioSummary, 0, len(m.scenes))
	for _, record := range m.scenes {
		summaries = append(summaries, scenarioSummary(record))
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Name != summaries[j].Name {
			return summaries[i].Name < summaries[j].Name
		}
		return summaries[i].Version > summaries[j].Version
	})
	return summaries, nil
}

func (m *Memory) GetScenario(ctx context.Context, id string) (model.Scenario, error) {
	record, err := m.GetScenarioRecord(ctx, id)
	if err != nil {
		return model.Scenario{}, err
	}
	return cloneScenario(record.Scenario), nil
}

func (m *Memory) GetScenarioRecord(_ context.Context, id string) (model.ScenarioRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.scenes[id]
	if !ok {
		return model.ScenarioRecord{}, errors.New("scenario not found")
	}
	record.Scenario = cloneScenario(record.Scenario)
	return record, nil
}

func (m *Memory) SaveScenario(_ context.Context, record model.ScenarioRecord) (model.ScenarioSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	record.Scenario.ID = record.ID
	if record.Source == "" {
		record.Source = "database"
	}
	if !record.Enabled && record.UpdatedAt.IsZero() {
		record.Enabled = true
	}
	if record.CreatedAt.IsZero() {
		if existing, ok := m.scenes[record.ID]; ok {
			record.CreatedAt = existing.CreatedAt
		} else {
			record.CreatedAt = now
		}
	}
	record.UpdatedAt = now
	record.Scenario = cloneScenario(record.Scenario)
	m.scenes[record.ID] = record
	return scenarioSummary(record), nil
}

func (m *Memory) SetScenarioEnabled(_ context.Context, id string, enabled bool, actorID string) (model.ScenarioSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.scenes[id]
	if !ok {
		return model.ScenarioSummary{}, errors.New("scenario not found")
	}
	record.Enabled = enabled
	record.UpdatedAt = time.Now().UTC()
	if actorID != "" && record.CreatedBy == "" {
		record.CreatedBy = actorID
	}
	m.scenes[id] = record
	return scenarioSummary(record), nil
}

func (m *Memory) SaveEventAnnotation(_ context.Context, annotation model.EventAnnotation) (model.EventAnnotation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runs[annotation.RunID]; !ok {
		return model.EventAnnotation{}, errors.New("run not found")
	}
	if annotation.ID == "" {
		annotation.ID = uuid.NewString()
	}
	if annotation.CreatedAt.IsZero() {
		annotation.CreatedAt = time.Now().UTC()
	}
	m.notes[annotation.RunID] = append(m.notes[annotation.RunID], annotation)
	return annotation, nil
}

func (m *Memory) ListEventAnnotations(_ context.Context, runID string) ([]model.EventAnnotation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	annotations := append([]model.EventAnnotation(nil), m.notes[runID]...)
	sort.Slice(annotations, func(i, j int) bool {
		return annotations[i].CreatedAt.Before(annotations[j].CreatedAt)
	})
	return annotations, nil
}

func (m *Memory) SaveAuditLog(_ context.Context, log model.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if log.ID == "" {
		log.ID = uuid.NewString()
	}
	if log.OccurredAt.IsZero() {
		log.OccurredAt = time.Now().UTC()
	}
	log.Payload = cloneMap(log.Payload)
	m.audit = append(m.audit, log)
	return nil
}

func (m *Memory) ListAuditLogs(_ context.Context, query model.AuditLogQuery) ([]model.AuditLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limit := normalizeLimit(query.Limit, 50, 200)
	logs := make([]model.AuditLog, 0, min(limit, len(m.audit)))
	for i := len(m.audit) - 1; i >= 0; i-- {
		log := m.audit[i]
		if query.RunID != "" && log.RunID != query.RunID {
			continue
		}
		if query.ScenarioID != "" && log.ScenarioID != query.ScenarioID {
			continue
		}
		log.Payload = cloneMap(log.Payload)
		logs = append(logs, log)
		if len(logs) == limit {
			break
		}
	}
	return logs, nil
}

func (m *Memory) PreviewPrune(_ context.Context, policy model.RetentionPolicy) (model.RetentionPreview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var preview model.RetentionPreview
	matchedRuns := map[string]struct{}{}
	if !policy.EndedBefore.IsZero() {
		for _, run := range m.runs {
			if !ownerMatches(run, policy.OwnerID) || run.StoppedAt.IsZero() || !run.StoppedAt.Before(policy.EndedBefore) {
				continue
			}
			matchedRuns[run.ID] = struct{}{}
			preview.RunsMatched++
			preview.EventsMatched += int64(len(m.events[run.ID]))
			preview.ContactsMatched += int64(len(m.contact[run.ID]))
			preview.TrackPointsMatched += int64(len(m.points[run.ID]))
			preview.SnapshotsMatched += int64(len(m.snaps[run.ID]))
		}
	}
	if !policy.Cutoff.IsZero() {
		for runID, events := range m.events {
			if _, ok := matchedRuns[runID]; ok || !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			for _, event := range events {
				if event.OccurredAt.Before(policy.Cutoff) {
					preview.EventsMatched++
				}
			}
		}
		for runID, contacts := range m.contact {
			if _, ok := matchedRuns[runID]; ok || !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			for _, contact := range contacts {
				if contact.Timestamp.Before(policy.Cutoff) {
					preview.ContactsMatched++
				}
			}
		}
		for runID, points := range m.points {
			if _, ok := matchedRuns[runID]; ok || !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			for _, point := range points {
				if point.SampledAt.Before(policy.Cutoff) {
					preview.TrackPointsMatched++
				}
			}
		}
		for runID, frames := range m.snaps {
			if _, ok := matchedRuns[runID]; ok || !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			for _, frame := range frames {
				if frame.SampledAt.Before(policy.Cutoff) {
					preview.SnapshotsMatched++
				}
			}
		}
	}
	if policy.MaxTrackPointsPerRun > 0 {
		for runID, points := range m.points {
			if _, ok := matchedRuns[runID]; ok || !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			if len(points) > policy.MaxTrackPointsPerRun {
				preview.TrackPointsMatched += int64(len(points) - policy.MaxTrackPointsPerRun)
			}
		}
	}
	if policy.MaxEventsPerRun > 0 {
		for runID, events := range m.events {
			if _, ok := matchedRuns[runID]; ok || !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			if len(events) > policy.MaxEventsPerRun {
				preview.EventsMatched += int64(len(events) - policy.MaxEventsPerRun)
			}
		}
	}
	if policy.MaxSnapshotsPerRun > 0 {
		for runID, frames := range m.snaps {
			if _, ok := matchedRuns[runID]; ok || !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			if len(frames) > policy.MaxSnapshotsPerRun {
				preview.SnapshotsMatched += int64(len(frames) - policy.MaxSnapshotsPerRun)
			}
		}
	}
	return preview, nil
}

func (m *Memory) Prune(_ context.Context, policy model.RetentionPolicy) (model.RetentionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result model.RetentionResult
	if !policy.EndedBefore.IsZero() {
		for _, run := range m.runs {
			if !ownerMatches(run, policy.OwnerID) || run.StoppedAt.IsZero() || !run.StoppedAt.Before(policy.EndedBefore) {
				continue
			}
			result.RunsMatched++
			result.EventsDeleted += int64(len(m.events[run.ID]))
			result.ContactsDeleted += int64(len(m.contact[run.ID]))
			result.TrackPointsDeleted += int64(len(m.points[run.ID]))
			result.SnapshotsDeleted += int64(len(m.snaps[run.ID]))
			delete(m.events, run.ID)
			delete(m.contact, run.ID)
			delete(m.points, run.ID)
			delete(m.snaps, run.ID)
		}
	}
	if !policy.Cutoff.IsZero() {
		for runID, events := range m.events {
			if !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			kept := events[:0]
			for _, event := range events {
				if event.OccurredAt.Before(policy.Cutoff) {
					result.EventsDeleted++
					continue
				}
				kept = append(kept, event)
			}
			m.events[runID] = kept
		}
		for runID, contacts := range m.contact {
			if !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			kept := contacts[:0]
			for _, contact := range contacts {
				if contact.Timestamp.Before(policy.Cutoff) {
					result.ContactsDeleted++
					continue
				}
				kept = append(kept, contact)
			}
			m.contact[runID] = kept
		}
		for runID, points := range m.points {
			if !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			kept := points[:0]
			for _, point := range points {
				if point.SampledAt.Before(policy.Cutoff) {
					result.TrackPointsDeleted++
					continue
				}
				kept = append(kept, point)
			}
			m.points[runID] = kept
		}
		for runID, frames := range m.snaps {
			if !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			kept := frames[:0]
			for _, frame := range frames {
				if frame.SampledAt.Before(policy.Cutoff) {
					result.SnapshotsDeleted++
					continue
				}
				kept = append(kept, frame)
			}
			m.snaps[runID] = kept
		}
	}
	if policy.MaxTrackPointsPerRun > 0 {
		for runID, points := range m.points {
			if !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			if len(points) <= policy.MaxTrackPointsPerRun {
				continue
			}
			sort.SliceStable(points, func(i, j int) bool {
				return points[i].SampledAt.Before(points[j].SampledAt)
			})
			deleteCount := len(points) - policy.MaxTrackPointsPerRun
			result.TrackPointsDeleted += int64(deleteCount)
			m.points[runID] = append([]model.TrackPoint(nil), points[deleteCount:]...)
		}
	}
	if policy.MaxEventsPerRun > 0 {
		for runID, events := range m.events {
			if !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			if len(events) <= policy.MaxEventsPerRun {
				continue
			}
			sort.SliceStable(events, func(i, j int) bool {
				return events[i].OccurredAt.Before(events[j].OccurredAt)
			})
			deleteCount := len(events) - policy.MaxEventsPerRun
			result.EventsDeleted += int64(deleteCount)
			m.events[runID] = append([]model.SimEvent(nil), events[deleteCount:]...)
		}
	}
	if policy.MaxSnapshotsPerRun > 0 {
		for runID, frames := range m.snaps {
			if !m.runOwnerMatches(runID, policy.OwnerID) {
				continue
			}
			if len(frames) <= policy.MaxSnapshotsPerRun {
				continue
			}
			sort.SliceStable(frames, func(i, j int) bool {
				return frames[i].SampledAt.Before(frames[j].SampledAt)
			})
			deleteCount := len(frames) - policy.MaxSnapshotsPerRun
			result.SnapshotsDeleted += int64(deleteCount)
			m.snaps[runID] = append([]model.SnapshotFrame(nil), frames[deleteCount:]...)
		}
	}
	return result, nil
}

func (m *Memory) Close() {}

func (m *Memory) runOwnerMatches(runID, ownerID string) bool {
	run, ok := m.runs[runID]
	return ok && ownerMatches(run, ownerID)
}

func ownerMatches(run model.Run, ownerID string) bool {
	return ownerID == "" || run.OwnerID == ownerID
}

func normalizeLimit(value, fallback, maxValue int) int {
	if value <= 0 {
		return fallback
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func snapshotFrame(snapshot model.Snapshot) model.SnapshotFrame {
	sampledAt := snapshot.Time
	if sampledAt.IsZero() {
		sampledAt = time.Now().UTC()
	}
	return model.SnapshotFrame{
		RunID:      snapshot.RunID,
		Status:     snapshot.Status,
		Tick:       snapshot.Tick,
		SampledAt:  sampledAt,
		Tracks:     cloneTracks(snapshot.Tracks),
		Contacts:   cloneContacts(snapshot.Contacts),
		Notice:     snapshot.Notice,
		SnapshotHz: snapshot.SnapshotHz,
	}
}

func cloneSnapshotFrame(frame model.SnapshotFrame) model.SnapshotFrame {
	frame.Tracks = cloneTracks(frame.Tracks)
	frame.Contacts = cloneContacts(frame.Contacts)
	return frame
}

func cloneTracks(tracks []model.Track) []model.Track {
	return append([]model.Track(nil), tracks...)
}

func cloneContacts(contacts []model.Contact) []model.Contact {
	return append([]model.Contact(nil), contacts...)
}

func cloneScenario(s model.Scenario) model.Scenario {
	out := s
	out.Sensors = append([]model.Sensor(nil), s.Sensors...)
	out.Zones = append([]model.Zone(nil), s.Zones...)
	for i := range out.Zones {
		out.Zones[i].Polygon = append([]model.Vec3(nil), s.Zones[i].Polygon...)
	}
	out.Tracks = append([]model.Track(nil), s.Tracks...)
	out.Contacts = append([]model.Contact(nil), s.Contacts...)
	out.AllowedActions = append([]string(nil), s.AllowedActions...)
	return out
}

func scenarioSummary(record model.ScenarioRecord) model.ScenarioSummary {
	return model.ScenarioSummary{
		ID:          record.ID,
		Name:        record.Scenario.Name,
		Description: record.Scenario.Description,
		Version:     record.Scenario.Version,
		Source:      record.Source,
		Enabled:     record.Enabled,
		CreatedBy:   record.CreatedBy,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}
