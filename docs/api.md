# API Contract and Versioning

ShipSystem exposes a training, demonstration, replay, and audit API only. The API contract must not define or imply real weapon-control, fire-control, electronic-warfare, radar control, or tactical-engagement guidance.

## Contract Source

The OpenAPI contract is `docs/openapi.json`. It covers the current HTTP API, report export, retention operations, and the WebSocket snapshot message shape.

Generate frontend TypeScript types from the contract:

```powershell
cd web
npm run generate:types
```

Or from the repository root:

```powershell
make api-types
```

The generated file is `web/src/generated/api-types.ts`. Do not edit it directly. `web/src/types.ts` is a thin compatibility layer that re-exports generated API types and keeps UI-only types such as `ConnectionState`.

When changing an API payload, update `docs/openapi.json`, regenerate frontend types, and run the normal test/build gate. `npm run typecheck` and `npm run build` also regenerate the types before checking.

## Version Policy

- OpenAPI `info.version` follows semantic versioning for the published contract.
- The current unversioned `/api/...` routes are treated as API v1. Breaking response or request changes require either a new route family such as `/api/v2/...` or a documented major contract version.
- Additive optional JSON fields are allowed in v1. Removing fields, changing required fields, changing field meanings, or changing enum values is breaking.
- `RunReport.version` is the report payload version. Current JSON reports use `version: 2`.
- Scenario payloads use `scenario.version` as scenario-content version metadata. Built-in and uploaded scenarios should keep version numbers stable for auditability.
- Snapshot and run payloads are versioned by the OpenAPI schema. Add new optional fields for compatible changes; do not change existing field meanings in-place.
- CSV, HTML, and PDF report exports are convenience templates derived from the JSON report. Additive CSV rows and additive HTML/PDF sections are compatible; removing or renaming existing report sections is breaking.

## Training Product API

Stage 9 training product routes are still training/demo/audit routes:

- `POST /api/scenarios` stores a validated managed scenario.
- `PUT /api/scenarios/{scenario_id}` updates managed database scenarios. Built-in and file scenarios are read-only templates.
- `POST /api/scenarios/{scenario_id}/copy` copies any visible scenario into a managed scenario version.
- `POST /api/scenarios/{scenario_id}/enable` and `/disable` toggle managed scenario availability without deleting records.
- `PUT /api/runs/{run_id}/metadata` updates tags, trainees, instructor notes, and archive state.
- `GET/POST /api/runs/{run_id}/annotations` reads and creates instructor event annotations.
- `GET /api/runs/{run_id}/audit` returns persisted audit log entries for the run.

Disabled scenarios remain readable for audit and can be re-enabled, but cannot be used to create new runs.

`RunReport.version: 2` adds `annotations`, `assessment`, and `audit_logs`. The assessment is an abstract training-record completeness score; it must not be used or described as real tactical advice.

## Error Shape

All JSON API errors use:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "validation failed",
    "details": ["tick_hz must be between 1 and 60"]
  }
}
```

Common error codes:

| Code | Meaning |
| --- | --- |
| `unauthorized` | Authentication is missing, invalid, or a WebSocket ticket is missing. |
| `origin_not_allowed` | CORS or WebSocket origin is not allowed. |
| `method_not_allowed` | HTTP method is unsupported for the route. |
| `invalid_json` | Request JSON is malformed, too large, has unknown fields, or is not a single object. |
| `validation_failed` | Scenario, run, or training-action validation failed; details may list field errors. |
| `scenario_not_found` | Requested scenario id does not exist. |
| `run_not_found` | Requested run is absent or not visible to the authenticated user. |
| `snapshot_not_found` | No snapshot exists near the requested time. |
| `invalid_time_range` | A time query parameter is not RFC3339. |
| `action_type_required` | Training action request omitted `type`. |
| `unsupported_report_format` | Report export format is not `json`, `csv`, `html`, or `pdf`. |
| `invalid_retention_policy` | Retention preview/prune input is invalid. |
| `empty_retention_policy` | Retention prune request would be destructive but has no actual pruning criteria. |
| `retention_preview_failed` | Retention preview failed. |
| `retention_prune_failed` | Retention prune failed after validation. |
| `readiness_failed` | Store readiness or migration status check failed. |
| `metrics_failed` | Metrics collection failed. |
| `request_failed` | Generic internal request failure. |
| `ws_ticket_failed` | Server could not issue a WebSocket ticket. |

## Compatibility Gate

Before merging an API contract change:

```powershell
go test ./...
go vet ./...
go build ./cmd/sim-server
cd web
npm run generate:types
npm test
npm run build
```

For PostgreSQL-backed contract changes, also run the documented Postgres integration workflow in `docs/database.md`.
