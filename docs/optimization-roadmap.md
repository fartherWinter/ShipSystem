# Optimization Roadmap

ShipSystem is a training, demonstration, replay, and audit simulator only. Product and engineering improvements must keep the boundary clear: no real weapon-control, fire-control, electronic-warfare, live radar control, or tactical engagement guidance.

## Implemented Polish Baseline

- README now describes the current React/MapLibre training console instead of calling it a frontend skeleton.
- The console exposes a managed scenario JSON editor entry point backed by the existing scenario `GET` and `PUT` API.
- The scenario editor includes client-side JSON preflight and a top-level change summary before save.
- Replay review now surfaces coverage, maximum loaded-window gap, and event density so instructors can quickly judge replay evidence quality.
- API error details are shown in the UI when the backend returns validation details.
- Scenario `assessment_profile` supports `standard`, `quick_review`, and `extended_review` record-completeness targets.
- Proxy authentication can enforce `viewer`, `operator`, `instructor`, and `admin` roles.
- Proxy deployments can assign roles from a trusted role header, user-role map, or least-privilege default role.
- `docs/proxy-identity-runbook.md` documents external identity-provider header hardening, role assignment priority, and validation checks.
- The API and console expose the current session, trusted proxy role, and descriptive permission capabilities through `GET /api/session` and the Access panel.
- Runtime metrics and the console capacity panel expose snapshot capacity pressure and configured retention limits.
- Product terminology is centralized in `docs/glossary.md`.
- Replay review supports shareable URL anchors, browser-local review bookmarks, and severity-colored event markers.
- Course templates in `course-templates/` package validated scenarios, expected metadata, and report review checklists.
- Course templates and managed scenarios can carry configurable `assessment_rules` for record-completeness targets and weights.
- The Scenario JSON editor includes an assessment-rule form that writes structured record-completeness rules into the JSON draft.
- Managed course template APIs and the console Course panel can list file/database templates and create managed scenarios from enabled templates.
- `scripts/export-run-archive.ps1` exports completed run evidence bundles before retention pruning or cold-storage handoff.
- Run archive export can create compressed zip handoff bundles with optional cleanup of the expanded archive directory and pre-signed object-store `PUT` upload.
- `cmd/archive-restore` restores compressed archive replay evidence into PostgreSQL/PostGIS for cold-storage recovery, refusing duplicate replay-data append unless explicitly requested.
- Scenario editor map mode can place simulated sensors, draw exercise zones, select/delete zones, and move existing zone vertices through either click-to-move controls or direct MapLibre drag handles.
- Scenario editor template buttons can add reusable simulated sensor, exercise-boundary, and focus-zone geometry, and the editor diff drills into added, removed, and edited scenario objects plus assessment-rule and version changes.
- Capacity panel shows snapshot write latency plus sampled snapshot, event, track-point, and raw-contact counts with per-run pressure for capped resources.
- Capacity panel now keeps a rolling trend window for count growth, write latency, DB table/index/total bytes, and archive-watch guidance.
- `/metrics/history` keeps a store-backed rolling capacity trend window; PostgreSQL mode persists samples in `metrics_history` so the console can restore trend context after browser refresh or app restart.
- Capacity panel includes deployment planning fields for current DB table bytes, index bytes, total bytes, and index/table ratio.
- Focused frontend tests cover token/proxy auth prompts, scenario upload and enable/disable controls, and report export success/failure status rendering.

## Near-Term Backlog

- No open near-term items remain from the initial polish slice; continue adding workflow-specific regression tests as new console capabilities land.

## Medium-Term Backlog

- No open medium-term items remain from the initial optimization slice; keep expanding visual authoring only when new scenario object types are introduced.

## Later Backlog

- No open later items remain from the initial optimization slice; future storage work should be driven by measured archive volume, provider requirements, and recovery-time targets.
