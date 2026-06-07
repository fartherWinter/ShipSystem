CREATE TABLE IF NOT EXISTS sim_snapshots (
    id BIGSERIAL PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES sim_runs(id) ON DELETE CASCADE,
    tick BIGINT NOT NULL,
    sampled_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,
    tracks JSONB NOT NULL DEFAULT '[]'::jsonb,
    contacts JSONB NOT NULL DEFAULT '[]'::jsonb,
    notice TEXT NOT NULL,
    snapshot_hz INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_snapshots_run_time ON sim_snapshots(run_id, sampled_at);
CREATE INDEX IF NOT EXISTS idx_snapshots_tracks ON sim_snapshots USING GIN(tracks);

INSERT INTO schema_migrations (name, version)
VALUES ('ship_sim', 2)
ON CONFLICT (name) DO UPDATE SET version = EXCLUDED.version, applied_at = now();
