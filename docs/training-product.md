# Training Product Workflow

ShipSystem training product features remain inside the training, demonstration, replay, and audit boundary. They do not connect to real weapon-control, fire-control, electronic-warfare, radar control, or device-command chains, and they do not produce tactical engagement advice.

## Scenario Management

Managed scenarios are validated before storage. Instructors can upload a scenario, copy an existing scenario into a new managed version, update managed database scenarios, and enable or disable managed scenarios.

Built-in and file scenarios are read-only templates. Copy them before editing or disabling. Disabling is non-destructive: disabled scenarios remain available for audit and can be re-enabled, but cannot be used to create new runs.

Scenario versions are metadata on the scenario content. When creating a managed scenario without an explicit version, the server assigns the next version for that scenario name. The database also records source, enabled state, creator, created time, and updated time. Scenario JSON can set `assessment_profile` to `standard`, `quick_review`, or `extended_review` to choose abstract record-completeness targets for that scenario. Scenario JSON can also set `assessment_rules` with action/replay/context weights for course-specific record-completeness checks; these rules are validated as training-record configuration only.

The console Scenario JSON editor includes map-assisted authoring. Load valid scenario JSON, choose map `sensor` mode to place a simulated sensor at the clicked map coordinate, or choose `zone` mode to collect vertices and finish an `exercise_boundary` zone. Choose `vertex` mode to show editable zone handles on the map; selecting a zone and vertex lets a click move that vertex, and dragging a visible handle previews and writes the new coordinate back to the JSON draft. The geometry editor can also delete the selected zone. Template buttons can add a simulated sensor, exercise boundary, or focus zone around the scenario ownship point. The editor diff summarizes added, removed, and edited sensors, zones, tracks, contacts, actions, assessment rules, and version changes before save. The assessment-rule form writes bounded action/replay targets and action/replay/context weights into the JSON draft. These tools only edit the scenario JSON draft; saving still uses the managed scenario `PUT` path and server validation.

Reusable course templates live in `course-templates/` and can also be stored as managed database templates. They package a scenario, expected training metadata, and a report review checklist while preserving the simulator-only boundary.

## Course Templates

Course templates are reusable exercise packages, not tactical guidance. File templates are loaded from `SHIP_SIM_COURSE_TEMPLATE_DIR` and are read-only. Managed templates are stored in PostgreSQL or memory storage through the course template API.

The console Course panel lists both sources, shows expected metadata, assessment-rule summaries, and checklist hints, and can create a managed scenario from a selected enabled template. Instructors/admins can create or update managed templates through the API; file templates should be changed in source control.

API routes:

- `GET /api/course-templates`
- `GET /api/course-templates/{template_id}`
- `POST /api/course-templates`
- `PUT /api/course-templates/{template_id}`
- `POST /api/course-templates/{template_id}/scenario`

## Training Records

Runs can store:

- Tags for filtering and later review.
- Trainee names or identifiers.
- Instructor notes.
- Archive state through `archived_at`.
- Event annotations tied to a specific event id or to the run generally.

Archiving a run is a metadata update, not a retention operation. It does not delete snapshots, events, tracks, annotations, or audit logs. Use the retention preview/prune workflow for capacity management.

## Replay Review

Replay review uses persisted snapshots when available. The console shows the loaded replay window, full snapshot range, replay coverage, maximum loaded-window gap, event density, and severity-colored event markers.

Reviewers can copy a replay anchor for the current frame. Anchors use URL query parameters:

```text
?run=<run_id>&at=<RFC3339 time>
```

Opening that URL loads the run and jumps to the nearest persisted replay frame. Review bookmarks are saved in browser local storage for convenience; they are not persisted audit evidence. Add annotations or instructor notes when a bookmark needs to become part of the training record.

## Assessment

Reports include an abstract training assessment. The score is based on training-record completeness signals such as action count, replay coverage, tags, trainees, instructor notes, and annotations.

Assessment profiles change only the record-completeness weights and targets:

- `standard`: balanced default for ordinary review.
- `quick_review`: lower action and replay targets for short exercises.
- `extended_review`: higher action and replay targets for longer review records.

Course templates and managed scenarios can override those presets with `assessment_rules`. The rule name is informational, action/replay targets are bounded, and action/replay/context weights must total 100. These settings affect only report completeness calculations based on abstract action logs, replay evidence, and instructor context.

The assessment is not a tactical score, engagement recommendation, fire-control quality measure, or operational readiness judgment. UI and documentation must keep this wording clear.

## Roles

Authenticated proxy deployments can pass `SHIP_SIM_AUTH_ROLE_HEADER` with one of:

- `viewer`: read-only API access plus WebSocket ticket creation for viewing live replay.
- `operator`: run creation/control, abstract training actions, run metadata, and annotations; no scenario management or retention pruning.
- `instructor`: full training product workflow, including scenario management, course template management, and retention operations.
- `admin`: same application permissions as instructor, reserved for deployment policy.

If the role header is absent in proxy mode, the app defaults to `instructor` for backward compatibility. The trusted reverse proxy must remove any client-supplied user or role header before setting authenticated values.

Proxy deployments that cannot provide a role header can set `SHIP_SIM_AUTH_ROLE_MAP` with comma-separated `user=role` entries. `SHIP_SIM_AUTH_DEFAULT_ROLE` applies to authenticated proxy users that have no role header and no map entry. `/api/session` reports `role_source` as `header`, `map`, `default`, `compat_default`, `token`, or `local` so operators can verify how a role was assigned.

The console reads `GET /api/session` after login and shows the current role plus
descriptive permission chips in the Access panel. These chips help instructors
and operators understand their deployment role; they do not replace backend
authorization.

## Report Templates

Reports are available as:

- JSON: canonical `RunReport.version: 2` payload.
- CSV: tabular export derived from the JSON report.
- HTML: printable review template.
- PDF: lightweight generated report summary.

All templates include the training safety notice. The JSON report is the source of truth for integrations; CSV, HTML, and PDF are convenience exports.

## Archive Export

Use `scripts/export-run-archive.ps1` to create a local read-only archive bundle before retention pruning or long-term storage handoff:

```powershell
.\scripts\export-run-archive.ps1 -RunID <run-id>
.\scripts\export-run-archive.ps1 -RunID <run-id> -Compress
.\scripts\export-run-archive.ps1 -RunID <run-id> -Compress -UploadUrl $env:SHIP_SIM_ARCHIVE_UPLOAD_URL
```

The bundle includes JSON/CSV/HTML/PDF reports, run metadata, events, annotations, audit logs, tracks, zones, snapshot pages, track-point pages, and a manifest. Pass `-Compress` to produce a zip handoff bundle, or `-SkipSnapshots` / `-SkipTrackPoints` for very large exercises when a report-only archive is intentional. `-UploadUrl` performs a pre-signed `PUT` upload of the compressed zip to object storage; use `-UploadHeader "Name: Value"` for required provider headers.

Compressed archives can restore replay evidence into PostgreSQL/PostGIS:

```powershell
go run ./cmd/archive-restore -archive outputs/run-<run-id>-archive.zip -database-url $env:DATABASE_URL
```

The restore command imports the run, persisted events, snapshot frames, derived track points, annotations, and audit-log entries so the normal report and replay APIs can serve the archived exercise again. Restore into a clean or retention-pruned run id by default; `-allow-append` is only for intentional duplicate-data recovery. Database-generated event, annotation, and audit ids are newly assigned during restore, so keep the original archive manifest as the immutable handoff record.

## Audit Logs

The server records persisted audit entries for:

- Scenario create, update, copy, enable, and disable.
- Course template create and update.
- Run creation, start, pause, stop, metadata update, and archive.
- Abstract training action submission.
- Event annotation creation.
- Report export.

Authenticated deployments record the authenticated token or proxy user where available. Demo mode can produce audit entries without an actor id.

Audit logs must not include long-lived tokens, Authorization headers, or sensitive query values. Request logs remain structured with `request_id`, `user_id`, `run_id`, `status`, and `duration_ms`; persisted audit logs focus on training-domain actions.

## Database Migration

Training product persistence requires migrations through `migrations/005_metrics_history.sql` and `schema_migrations.name='ship_sim'` version `5`.

The migrations are additive. They add managed scenario metadata, run metadata columns, event annotations, audit logs, managed course template storage, and metrics trend samples. They do not delete or truncate existing training data. Take a backup or platform snapshot before applying migrations to shared, staging, or production PostgreSQL databases.

## Acceptance Loop

A complete training loop is:

1. Select a course template or upload/copy a scenario.
2. Create or enable the managed scenario.
3. Create a run from the scenario.
4. Start the run.
5. Submit abstract training actions.
6. Review replay and events.
7. Add instructor annotations and notes.
8. Export JSON, CSV, HTML, or PDF reports.
9. Export a run archive when the exercise needs long-term evidence.
10. Archive the run record.

This loop remains a simulator workflow only. It must not be extended into real device control or tactical recommendation behavior.
