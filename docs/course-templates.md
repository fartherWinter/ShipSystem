# Course Templates

Course templates package reusable training exercises without expanding ShipSystem beyond the simulator boundary. A template contains a validated scenario, expected run metadata, and a report review checklist. It must not include tactical recommendations, real device integration steps, or operational readiness scoring.

## Included Templates

File templates live in `course-templates/` and are loaded from `SHIP_SIM_COURSE_TEMPLATE_DIR` at server startup:

- `quick-review-baseline.json`: short review flow using `quick_review` assessment targets, limited abstract actions, and a compact simulated area.
- `extended-review-audit.json`: longer review flow using `extended_review` targets, multiple simulated sensors, annotation expectations, and archive-export practice.

Each template has:

- `training_only: true`.
- `scenario`: JSON that can be uploaded through the scenario API or pasted into the Scenario JSON editor.
- `scenario.assessment_rules`: configurable training-record completeness targets and weights for the template. The action, replay, and context weights must sum to 100.
- `expected_metadata`: tags, trainee/note expectations, and archive guidance.
- `review_checklist`: evidence prompts for reports, replay coverage, annotations, and archive manifests.
- `safety_notice`: explicit simulator-only boundary.

Managed templates can also be created and updated through `/api/course-templates`. File templates are read-only in the API; create a managed template when a deployment needs local customization without changing source-controlled files.

## Usage

1. Select a template in the console Course panel or call `GET /api/course-templates`.
2. Create a managed scenario from the selected enabled template.
3. Create a run from that managed scenario.
4. Apply the template's expected metadata through the Training Record panel.
5. Review replay using event jumps, replay anchors, and local bookmarks.
6. Export reports and, for completed exercises, run `scripts/export-run-archive.ps1`.
7. Archive the run metadata after report/archive export is complete.

Example API flow:

```powershell
Invoke-RestMethod -Uri http://localhost:8080/api/course-templates
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/course-templates/quick-review-baseline/scenario
```

The Go test suite validates that every template scenario passes `ValidateScenario`, includes assessment rules, includes expected metadata, has a checklist, and preserves the training-only notice.
