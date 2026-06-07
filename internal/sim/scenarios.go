package sim

import (
	"context"
	"encoding/json"
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
	id = strings.TrimSpace(id)
	m.mu.RLock()
	entry, ok := m.scenarios[id]
	m.mu.RUnlock()
	if ok {
		return cloneScenario(entry.Scenario), true
	}
	scenario, err := m.store.GetScenario(ctx, id)
	if err != nil {
		return model.Scenario{}, false
	}
	scenario = normalizeScenario(scenario)
	if err := ValidateScenario(scenario); err != nil {
		return model.Scenario{}, false
	}
	return scenario, true
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
