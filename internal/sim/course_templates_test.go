package sim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shipsim/internal/model"
)

type courseTemplate struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	TrainingOnly     bool           `json:"training_only"`
	Scenario         model.Scenario `json:"scenario"`
	ExpectedMetadata map[string]any `json:"expected_metadata"`
	ReviewChecklist  []struct {
		ID       string `json:"id"`
		Label    string `json:"label"`
		Evidence string `json:"evidence"`
	} `json:"review_checklist"`
	SafetyNotice string `json:"safety_notice"`
}

func TestCourseTemplatesValidate(t *testing.T) {
	dir := filepath.Join("..", "..", "course-templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read course templates: %v", err)
	}
	templates := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		templates++
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var template courseTemplate
		if err := json.Unmarshal(data, &template); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if template.ID == "" || template.Name == "" {
			t.Fatalf("%s: id and name are required", path)
		}
		if !template.TrainingOnly {
			t.Fatalf("%s: training_only must be true", path)
		}
		if !strings.Contains(strings.ToLower(template.SafetyNotice), "training") {
			t.Fatalf("%s: safety notice must preserve training boundary", path)
		}
		if len(template.ExpectedMetadata) == 0 {
			t.Fatalf("%s: expected_metadata is required", path)
		}
		if len(template.ReviewChecklist) == 0 {
			t.Fatalf("%s: review_checklist is required", path)
		}
		if template.Scenario.AssessmentRules == nil {
			t.Fatalf("%s: scenario.assessment_rules is required for configurable record-completeness checks", path)
		}
		scenario := normalizeScenario(template.Scenario)
		if err := ValidateScenario(scenario); err != nil {
			t.Fatalf("%s: scenario validation failed: %v", path, err)
		}
	}
	if templates < 2 {
		t.Fatalf("expected at least two course templates, got %d", templates)
	}
}
