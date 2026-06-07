package sim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"shipsim/internal/model"
)

type scenarioEntry struct {
	Summary  model.ScenarioSummary
	Scenario model.Scenario
}

func (m *Manager) RegisterScenario(id string, scenario model.Scenario, source string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		id = strings.TrimSpace(scenario.ID)
	}
	if id == "" {
		return fmt.Errorf("scenario id is required")
	}
	scenario.ID = id
	scenario = normalizeScenario(scenario)
	if err := ValidateScenario(scenario); err != nil {
		return err
	}
	entry := scenarioEntry{
		Summary: model.ScenarioSummary{
			ID:          scenario.ID,
			Name:        scenario.Name,
			Description: scenario.Description,
			Version:     scenario.Version,
			Source:      source,
			Enabled:     true,
		},
		Scenario: cloneScenario(scenario),
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenarios[scenario.ID] = entry
	return nil
}

func (m *Manager) LoadScenarioDir(dir string) (int, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(dir)
	if errorsIsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return loaded, err
		}
		var scenario model.Scenario
		if err := json.Unmarshal(data, &scenario); err != nil {
			return loaded, fmt.Errorf("%s: %w", path, err)
		}
		id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if scenario.ID != "" {
			id = scenario.ID
		}
		if err := m.RegisterScenario(id, scenario, "file"); err != nil {
			return loaded, fmt.Errorf("%s: %w", path, err)
		}
		loaded++
	}
	return loaded, nil
}

func (m *Manager) ListScenarios(ctx context.Context) ([]model.ScenarioSummary, error) {
	m.mu.RLock()
	items := make([]model.ScenarioSummary, 0, len(m.scenarios))
	for _, entry := range m.scenarios {
		items = append(items, entry.Summary)
	}
	m.mu.RUnlock()
	stored, err := m.store.ListScenarioSummaries(ctx)
	if err != nil {
		return nil, err
	}
	items = append(items, stored...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (m *Manager) Scenario(ctx context.Context, id string) (model.Scenario, bool) {
	record, ok := m.ScenarioRecord(ctx, id)
	if !ok {
		return model.Scenario{}, false
	}
	return cloneScenario(record.Scenario), true
}

func (m *Manager) ScenarioForRun(ctx context.Context, id string) (model.Scenario, bool, error) {
	record, ok := m.ScenarioRecord(ctx, id)
	if !ok {
		return model.Scenario{}, false, nil
	}
	if !record.Enabled {
		return model.Scenario{}, true, ValidationError{Details: []string{"scenario is disabled; enable or copy it before creating a run"}}
	}
	return cloneScenario(record.Scenario), true, nil
}

func (m *Manager) ScenarioRecord(ctx context.Context, id string) (model.ScenarioRecord, bool) {
	id = strings.TrimSpace(id)
	m.mu.RLock()
	entry, ok := m.scenarios[id]
	m.mu.RUnlock()
	if ok {
		return model.ScenarioRecord{
			ID:       entry.Summary.ID,
			Scenario: cloneScenario(entry.Scenario),
			Source:   entry.Summary.Source,
			Enabled:  true,
		}, true
	}
	record, err := m.store.GetScenarioRecord(ctx, id)
	if err != nil {
		return model.ScenarioRecord{}, false
	}
	record.Scenario = normalizeScenario(record.Scenario)
	if err := ValidateScenario(record.Scenario); err != nil {
		return model.ScenarioRecord{}, false
	}
	return record, true
}

func (m *Manager) CreateScenario(ctx context.Context, actorID string, scenario model.Scenario) (model.ScenarioSummary, error) {
	if isZeroScenario(scenario) {
		return model.ScenarioSummary{}, ValidationError{Details: []string{"scenario body is required"}}
	}
	if scenario.Version <= 0 {
		scenario.Version = m.nextScenarioVersion(ctx, scenario.Name)
	}
	scenario.ID = ""
	scenario = normalizeScenario(scenario)
	if err := ValidateScenario(scenario); err != nil {
		return model.ScenarioSummary{}, err
	}
	summary, err := m.store.SaveScenario(ctx, model.ScenarioRecord{
		Scenario:  scenario,
		Source:    "database",
		Enabled:   true,
		CreatedBy: actorID,
	})
	if err != nil {
		return model.ScenarioSummary{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		ScenarioID: summary.ID,
		ActorID:    actorID,
		Action:     "scenario.created",
		TargetType: "scenario",
		TargetID:   summary.ID,
		Payload: map[string]any{
			"name":    summary.Name,
			"version": summary.Version,
		},
	})
	return summary, nil
}

func (m *Manager) UpdateScenario(ctx context.Context, id, actorID string, scenario model.Scenario) (model.ScenarioSummary, error) {
	record, ok := m.ScenarioRecord(ctx, id)
	if !ok {
		return model.ScenarioSummary{}, errors.New("scenario not found")
	}
	if record.Source != "database" {
		return model.ScenarioSummary{}, ValidationError{Details: []string{"built-in and file scenarios are read-only; copy the scenario before editing"}}
	}
	if scenario.Version <= 0 {
		scenario.Version = record.Scenario.Version
	}
	scenario.ID = id
	scenario = normalizeScenario(scenario)
	if err := ValidateScenario(scenario); err != nil {
		return model.ScenarioSummary{}, err
	}
	record.Scenario = scenario
	record.CreatedBy = actorID
	summary, err := m.store.SaveScenario(ctx, record)
	if err != nil {
		return model.ScenarioSummary{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		ScenarioID: summary.ID,
		ActorID:    actorID,
		Action:     "scenario.updated",
		TargetType: "scenario",
		TargetID:   summary.ID,
		Payload: map[string]any{
			"name":    summary.Name,
			"version": summary.Version,
		},
	})
	return summary, nil
}

func (m *Manager) SetScenarioEnabled(ctx context.Context, id string, enabled bool, actorID string) (model.ScenarioSummary, error) {
	record, ok := m.ScenarioRecord(ctx, id)
	if !ok {
		return model.ScenarioSummary{}, errors.New("scenario not found")
	}
	if record.Source != "database" {
		return model.ScenarioSummary{}, ValidationError{Details: []string{"built-in and file scenarios are always available templates; copy them before disabling"}}
	}
	summary, err := m.store.SetScenarioEnabled(ctx, id, enabled, actorID)
	if err != nil {
		return model.ScenarioSummary{}, err
	}
	action := "scenario.disabled"
	if enabled {
		action = "scenario.enabled"
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		ScenarioID: summary.ID,
		ActorID:    actorID,
		Action:     action,
		TargetType: "scenario",
		TargetID:   summary.ID,
		Payload: map[string]any{
			"name":    summary.Name,
			"enabled": summary.Enabled,
		},
	})
	return summary, nil
}

func (m *Manager) CopyScenario(ctx context.Context, id, name, actorID string) (model.ScenarioSummary, error) {
	record, ok := m.ScenarioRecord(ctx, id)
	if !ok {
		return model.ScenarioSummary{}, errors.New("scenario not found")
	}
	scenario := cloneScenario(record.Scenario)
	if strings.TrimSpace(name) != "" {
		scenario.Name = strings.TrimSpace(name)
	} else {
		scenario.Name = scenario.Name + " copy"
	}
	scenario.ID = ""
	scenario.Version = m.nextScenarioVersion(ctx, scenario.Name)
	summary, err := m.CreateScenario(ctx, actorID, scenario)
	if err != nil {
		return model.ScenarioSummary{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		ScenarioID: summary.ID,
		ActorID:    actorID,
		Action:     "scenario.copied",
		TargetType: "scenario",
		TargetID:   summary.ID,
		Payload: map[string]any{
			"source_scenario_id": id,
			"name":               summary.Name,
			"version":            summary.Version,
		},
	})
	return summary, nil
}

func (m *Manager) nextScenarioVersion(ctx context.Context, name string) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return 1
	}
	summaries, err := m.ListScenarios(ctx)
	if err != nil {
		return 1
	}
	version := 0
	for _, summary := range summaries {
		if summary.Name == name && summary.Version > version {
			version = summary.Version
		}
	}
	return version + 1
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

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
