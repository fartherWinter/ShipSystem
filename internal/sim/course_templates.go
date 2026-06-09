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

func (m *Manager) RegisterCourseTemplate(id string, template model.CourseTemplate, source string) error {
	template, err := normalizeCourseTemplate(id, template, source)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.courseTemplates[template.ID] = cloneCourseTemplate(template)
	return nil
}

func (m *Manager) LoadCourseTemplateDir(dir string) (int, error) {
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
		var template model.CourseTemplate
		if err := json.Unmarshal(data, &template); err != nil {
			return loaded, fmt.Errorf("%s: %w", path, err)
		}
		id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if template.ID != "" {
			id = template.ID
		}
		if err := m.RegisterCourseTemplate(id, template, "file"); err != nil {
			return loaded, fmt.Errorf("%s: %w", path, err)
		}
		loaded++
	}
	return loaded, nil
}

func (m *Manager) ListCourseTemplates(ctx context.Context) ([]model.CourseTemplate, error) {
	m.mu.RLock()
	items := make([]model.CourseTemplate, 0, len(m.courseTemplates))
	for _, template := range m.courseTemplates {
		items = append(items, cloneCourseTemplate(template))
	}
	m.mu.RUnlock()
	stored, err := m.store.ListCourseTemplates(ctx)
	if err != nil {
		return nil, err
	}
	for _, template := range stored {
		normalized, err := normalizeCourseTemplate(template.ID, template, template.Source)
		if err != nil {
			return nil, err
		}
		items = append(items, normalized)
	}
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

func (m *Manager) CourseTemplate(ctx context.Context, id string) (model.CourseTemplate, bool) {
	id = strings.TrimSpace(id)
	m.mu.RLock()
	template, ok := m.courseTemplates[id]
	m.mu.RUnlock()
	if ok {
		return cloneCourseTemplate(template), true
	}
	template, err := m.store.GetCourseTemplate(ctx, id)
	if err != nil {
		return model.CourseTemplate{}, false
	}
	normalized, err := normalizeCourseTemplate(template.ID, template, template.Source)
	if err != nil {
		return model.CourseTemplate{}, false
	}
	return normalized, true
}

func (m *Manager) CreateCourseTemplate(ctx context.Context, actorID string, template model.CourseTemplate) (model.CourseTemplate, error) {
	template.ID = strings.TrimSpace(template.ID)
	normalized, err := normalizeCourseTemplate(template.ID, template, "database")
	if err != nil {
		return model.CourseTemplate{}, err
	}
	normalized.CreatedBy = actorID
	saved, err := m.store.SaveCourseTemplate(ctx, normalized)
	if err != nil {
		return model.CourseTemplate{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		ActorID:    actorID,
		Action:     "course_template.created",
		TargetType: "course_template",
		TargetID:   saved.ID,
		Payload: map[string]any{
			"name": saved.Name,
		},
	})
	return saved, nil
}

func (m *Manager) UpdateCourseTemplate(ctx context.Context, id, actorID string, template model.CourseTemplate) (model.CourseTemplate, error) {
	existing, ok := m.CourseTemplate(ctx, id)
	if !ok {
		return model.CourseTemplate{}, errors.New("course template not found")
	}
	if existing.Source != "database" {
		return model.CourseTemplate{}, ValidationError{Details: []string{"file course templates are read-only; create a managed template before editing"}}
	}
	template.ID = id
	normalized, err := normalizeCourseTemplate(id, template, "database")
	if err != nil {
		return model.CourseTemplate{}, err
	}
	normalized.CreatedBy = actorID
	normalized.CreatedAt = existing.CreatedAt
	saved, err := m.store.SaveCourseTemplate(ctx, normalized)
	if err != nil {
		return model.CourseTemplate{}, err
	}
	_ = m.recordAudit(ctx, model.AuditLog{
		ActorID:    actorID,
		Action:     "course_template.updated",
		TargetType: "course_template",
		TargetID:   saved.ID,
		Payload: map[string]any{
			"name": saved.Name,
		},
	})
	return saved, nil
}

func (m *Manager) CreateScenarioFromCourseTemplate(ctx context.Context, templateID, actorID string) (model.ScenarioSummary, error) {
	template, ok := m.CourseTemplate(ctx, templateID)
	if !ok {
		return model.ScenarioSummary{}, errors.New("course template not found")
	}
	if !template.Enabled {
		return model.ScenarioSummary{}, ValidationError{Details: []string{"course template is disabled"}}
	}
	scenario := cloneScenario(template.Scenario)
	scenario.ID = ""
	return m.CreateScenario(ctx, actorID, scenario)
}

func normalizeCourseTemplate(id string, template model.CourseTemplate, source string) (model.CourseTemplate, error) {
	template.ID = strings.TrimSpace(firstNonEmpty(template.ID, id))
	if template.ID == "" && source != "database" {
		return model.CourseTemplate{}, ValidationError{Details: []string{"course template id is required"}}
	}
	template.Name = strings.TrimSpace(template.Name)
	if template.Name == "" {
		return model.CourseTemplate{}, ValidationError{Details: []string{"course template name is required"}}
	}
	if source == "" {
		source = template.Source
	}
	if source == "" {
		source = "database"
	}
	template.Source = source
	if !template.Enabled && template.UpdatedAt.IsZero() {
		template.Enabled = true
	}
	if !template.TrainingOnly {
		return model.CourseTemplate{}, ValidationError{Details: []string{"course template training_only must be true"}}
	}
	if len(template.ExpectedMetadata) == 0 {
		return model.CourseTemplate{}, ValidationError{Details: []string{"course template expected_metadata is required"}}
	}
	if len(template.ReviewChecklist) == 0 {
		return model.CourseTemplate{}, ValidationError{Details: []string{"course template review_checklist is required"}}
	}
	for index, item := range template.ReviewChecklist {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Label) == "" || strings.TrimSpace(item.Evidence) == "" {
			return model.CourseTemplate{}, ValidationError{Details: []string{fmt.Sprintf("review_checklist[%d] must include id, label, and evidence", index)}}
		}
	}
	if !strings.Contains(strings.ToLower(template.SafetyNotice), "training") && !strings.Contains(strings.ToLower(template.SafetyNotice), "simulation") {
		return model.CourseTemplate{}, ValidationError{Details: []string{"course template safety_notice must preserve the training/simulation boundary"}}
	}
	template.Scenario = normalizeScenario(template.Scenario)
	if err := ValidateScenario(template.Scenario); err != nil {
		return model.CourseTemplate{}, err
	}
	return cloneCourseTemplate(template), nil
}

func cloneCourseTemplate(template model.CourseTemplate) model.CourseTemplate {
	out := template
	out.Scenario = cloneScenario(template.Scenario)
	out.ExpectedMetadata = cloneAnyMap(template.ExpectedMetadata)
	out.ReviewChecklist = append([]model.CourseChecklistItem(nil), template.ReviewChecklist...)
	return out
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
