# Training Product Workflow

ShipSystem training product features remain inside the training, demonstration, replay, and audit boundary. They do not connect to real weapon-control, fire-control, electronic-warfare, radar control, or device-command chains, and they do not produce tactical engagement advice.

## Scenario Management

Managed scenarios are validated before storage. Operators can upload a scenario, copy an existing scenario into a new managed version, update managed database scenarios, and enable or disable managed scenarios.

Built-in and file scenarios are read-only templates. Copy them before editing or disabling. Disabling is non-destructive: disabled scenarios remain available for audit and can be re-enabled, but cannot be used to create new runs.

Scenario versions are metadata on the scenario content. When creating a managed scenario without an explicit version, the server assigns the next version for that scenario name. The database also records source, enabled state, creator, created time, and updated time.

## Training Records

Runs can store:

- Tags for filtering and later review.
- Trainee names or identifiers.
- Instructor notes.
- Archive state through `archived_at`.
- Event annotations tied to a specific event id or to the run generally.

Archiving a run is a metadata update, not a retention operation. It does not delete snapshots, events, tracks, annotations, or audit logs. Use the retention preview/prune workflow for capacity management.

## Assessment

Reports include an abstract training assessment. The score is based on training-record completeness signals such as action count, replay coverage, tags, trainees, instructor notes, and annotations.

The assessment is not a tactical score, engagement recommendation, fire-control quality measure, or operational readiness judgment. UI and documentation must keep this wording clear.

## Report Templates

Reports are available as:

- JSON: canonical `RunReport.version: 2` payload.
- CSV: tabular export derived from the JSON report.
- HTML: printable review template.
- PDF: lightweight generated report summary.

All templates include the training safety notice. The JSON report is the source of truth for integrations; CSV, HTML, and PDF are convenience exports.

## Audit Logs

The server records persisted audit entries for:

- Scenario create, update, copy, enable, and disable.
- Run creation, start, pause, stop, metadata update, and archive.
- Abstract training action submission.
- Event annotation creation.
- Report export.

Authenticated deployments record the authenticated token or proxy user where available. Demo mode can produce audit entries without an actor id.

Audit logs must not include long-lived tokens, Authorization headers, or sensitive query values. Request logs remain structured with `request_id`, `user_id`, `run_id`, `status`, and `duration_ms`; persisted audit logs focus on training-domain actions.

## Database Migration

Training product persistence requires `migrations/003_training_product.sql` and `schema_migrations.name='ship_sim'` version `3`.

The migration is additive. It adds managed scenario metadata, run metadata columns, event annotations, and audit logs. It does not delete or truncate existing training data. Take a backup or platform snapshot before applying migrations to shared, staging, or production PostgreSQL databases.

## Acceptance Loop

A complete training loop is:

1. Upload or copy a scenario.
2. Enable the scenario.
3. Create a run from the scenario.
4. Start the run.
5. Submit abstract training actions.
6. Review replay and events.
7. Add instructor annotations and notes.
8. Export JSON, CSV, HTML, or PDF reports.
9. Archive the run record.

This loop remains a simulator workflow only. It must not be extended into real device control or tactical recommendation behavior.
