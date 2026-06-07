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
- `ship_sim_snapshot_writes_total`
- `ship_sim_snapshot_write_failures_total`
- `ship_sim_snapshot_write_duration_seconds`
- `ship_sim_db_ready`
- `ship_sim_db_migration_version`

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
- `run_id`
- `method`
- `path`
- `status`
- `duration_ms`

The server logs the URL path only, not the raw query string. It does not log `Authorization`, `X-Ship-Sim-Token`, WebSocket tickets, or `access_token` query values. Keep this rule for new middleware and handlers: log stable identifiers and statuses, never bearer material or sensitive query parameters.

Operators can correlate API errors and replay/report issues by `request_id` and `run_id`. Clients may send `X-Request-ID`; otherwise the server generates one and echoes it in the response header.
