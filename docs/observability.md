# Observability

ShipSystem observability covers training simulation operations only. Metrics and logs must not be connected to real weapon-control, fire-control, electronic-warfare, radar control, or device-command chains.

## Health and Readiness

- `GET /healthz`: process liveness. It returns the training-only safety notice and does not require authentication.
- `GET /readyz`: store readiness. In authenticated deployments it requires authentication, because it exposes store type and migration version.

`/readyz` returns `503` when PostgreSQL is unavailable or migrations are not current. Use it for readiness probes, not liveness probes.

## Metrics

JSON metrics remain available at:

```text
GET /metrics
```

Prometheus metrics are available at:

```text
GET /metrics/prometheus
```

Both metrics endpoints follow the app authentication mode. With `SHIP_SIM_AUTH_MODE=token`, scrape with an `Authorization: Bearer ...` header. With proxy auth, scrape through the trusted authenticated proxy. With auth off, local demo metrics are unauthenticated.

The JSON metrics payload also includes `sampled_at`, `snapshot_frames_by_run`, `event_count_by_run`, `track_point_count_by_run`, `contact_count_by_run`, per-run capacity pressure for snapshots/events/track points, configured per-run limits, snapshot write last/average/max duration, and `db_table_bytes`, `db_index_bytes`, and `db_total_bytes` where the backing store can report them. `GET /metrics/history?limit=48` returns store-backed rolling trend samples captured from recent `/metrics` calls. Memory mode keeps the recent window in process; PostgreSQL mode persists it in `metrics_history` so the console can restore capacity trend context after app restart. The console Capacity panel uses those fields to show per-run storage pressure, restored growth trends after browser refresh, replay write latency trends, database table/index size movement, and archive-watch guidance.

Key Prometheus metrics:

- `ship_sim_http_requests_total`
- `ship_sim_http_request_errors_total`
- `ship_sim_http_request_duration_seconds_sum`
- `ship_sim_http_request_duration_seconds_count`
- `ship_sim_http_request_duration_seconds_max`
- `ship_sim_websocket_connections`
- `ship_sim_runs_active`
- `ship_sim_engines_total`
- `ship_sim_engines_running`
- `ship_sim_snapshot_frames_total`
- `ship_sim_snapshot_capacity_pressure`
- `ship_sim_snapshot_capacity_limit`
- `ship_sim_events_total`
- `ship_sim_event_capacity_pressure`
- `ship_sim_event_capacity_limit`
- `ship_sim_track_points_total`
- `ship_sim_track_point_capacity_pressure`
- `ship_sim_track_point_capacity_limit`
- `ship_sim_contacts_total`
- `ship_sim_snapshot_writes_total`
- `ship_sim_snapshot_write_failures_total`
- `ship_sim_snapshot_write_duration_seconds`
- `ship_sim_db_ready`
- `ship_sim_db_migration_version`
- `ship_sim_db_table_bytes`
- `ship_sim_db_index_bytes`
- `ship_sim_db_total_bytes`

Prometheus scrape example:

```yaml
scrape_configs:
  - job_name: shipsim
    metrics_path: /metrics/prometheus
    static_configs:
      - targets: ["shipsim.example.internal:8080"]
```

For token auth, configure the scraper's secret store rather than hard-coding tokens in the Prometheus config repository.

## Logs

HTTP request logs include:

- `request_id`
- `user_id`
- `role`
- `run_id`
- `method`
- `path`
- `status`
- `duration_ms`

The server logs the URL path only, not the raw query string. It does not log `Authorization`, `X-Ship-Sim-Token`, WebSocket tickets, or `access_token` query values. Keep this rule for new middleware and handlers: log stable identifiers and statuses, never bearer material or sensitive query parameters.

Operators can correlate API errors and replay/report issues by `request_id` and `run_id`. Clients may send `X-Request-ID`; otherwise the server generates one and echoes it in the response header.
