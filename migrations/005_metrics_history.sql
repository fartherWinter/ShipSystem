CREATE TABLE IF NOT EXISTS metrics_history (
    id BIGSERIAL PRIMARY KEY,
    sampled_at TIMESTAMPTZ NOT NULL,
    snapshot_frames INTEGER NOT NULL DEFAULT 0,
    event_count INTEGER NOT NULL DEFAULT 0,
    track_point_count INTEGER NOT NULL DEFAULT 0,
    contact_count INTEGER NOT NULL DEFAULT 0,
    snapshot_capacity_pressure DOUBLE PRECISION NOT NULL DEFAULT 0,
    event_capacity_pressure DOUBLE PRECISION NOT NULL DEFAULT 0,
    track_point_capacity_pressure DOUBLE PRECISION NOT NULL DEFAULT 0,
    snapshot_write_avg_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    snapshot_write_max_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    snapshot_write_failures BIGINT NOT NULL DEFAULT 0,
    db_table_bytes BIGINT NOT NULL DEFAULT 0,
    db_index_bytes BIGINT NOT NULL DEFAULT 0,
    db_total_bytes BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_metrics_history_sampled_at ON metrics_history(sampled_at DESC, id DESC);

INSERT INTO schema_migrations (name, version)
VALUES ('ship_sim', 5)
ON CONFLICT (name) DO UPDATE SET version = EXCLUDED.version, applied_at = now();
