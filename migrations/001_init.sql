CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS schema_migrations (
    name TEXT PRIMARY KEY,
    version INTEGER NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sim_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    owner_id TEXT,
    scenario_name TEXT,
    scenario JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    stopped_at TIMESTAMPTZ,
    safety_notice TEXT NOT NULL
);

ALTER TABLE sim_runs ADD COLUMN IF NOT EXISTS owner_id TEXT;
ALTER TABLE sim_runs ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS scenarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, version)
);

CREATE TABLE IF NOT EXISTS sensors (
    id TEXT NOT NULL,
    run_id UUID NOT NULL REFERENCES sim_runs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    position GEOMETRY(PointZ, 4326),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, id)
);

CREATE TABLE IF NOT EXISTS contacts_raw (
    id BIGSERIAL PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES sim_runs(id) ON DELETE CASCADE,
    sensor_id TEXT,
    contact_id TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    position GEOMETRY(PointZ, 4326) NOT NULL,
    velocity_x DOUBLE PRECISION,
    velocity_y DOUBLE PRECISION,
    velocity_z DOUBLE PRECISION,
    confidence DOUBLE PRECISION,
    kind TEXT,
    FOREIGN KEY (run_id, sensor_id) REFERENCES sensors(run_id, id)
);

CREATE TABLE IF NOT EXISTS tracks (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES sim_runs(id) ON DELETE CASCADE,
    track_no TEXT NOT NULL,
    kind TEXT,
    threat_level TEXT,
    latest_position GEOMETRY(PointZ, 4326),
    confidence DOUBLE PRECISION,
    status TEXT,
    first_seen_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS track_points (
    id BIGSERIAL PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES sim_runs(id) ON DELETE CASCADE,
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    sampled_at TIMESTAMPTZ NOT NULL,
    position GEOMETRY(PointZ, 4326) NOT NULL,
    speed DOUBLE PRECISION,
    heading DOUBLE PRECISION,
    confidence DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS sim_events (
    id BIGSERIAL PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES sim_runs(id) ON DELETE CASCADE,
    occurred_at TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL,
    subject_id TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS zones (
    id TEXT NOT NULL,
    run_id UUID NOT NULL REFERENCES sim_runs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    geom GEOMETRY(Polygon, 4326) NOT NULL,
    PRIMARY KEY (run_id, id)
);

CREATE INDEX IF NOT EXISTS idx_contacts_run_time ON contacts_raw(run_id, observed_at);
CREATE INDEX IF NOT EXISTS idx_contacts_geom ON contacts_raw USING GIST(position);
CREATE INDEX IF NOT EXISTS idx_tracks_run_latest ON tracks(run_id, last_seen_at);
CREATE INDEX IF NOT EXISTS idx_tracks_latest_geom ON tracks USING GIST(latest_position);
CREATE INDEX IF NOT EXISTS idx_track_points_run_time ON track_points(run_id, sampled_at);
CREATE INDEX IF NOT EXISTS idx_track_points_geom ON track_points USING GIST(position);
CREATE INDEX IF NOT EXISTS idx_events_run_time ON sim_events(run_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_events_payload ON sim_events USING GIN(payload);
CREATE INDEX IF NOT EXISTS idx_zones_geom ON zones USING GIST(geom);

INSERT INTO schema_migrations (name, version)
VALUES ('ship_sim', 1)
ON CONFLICT (name) DO UPDATE SET version = EXCLUDED.version, applied_at = now();
