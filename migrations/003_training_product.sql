ALTER TABLE scenarios ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE scenarios ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'database';
ALTER TABLE scenarios ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE scenarios ADD COLUMN IF NOT EXISTS created_by TEXT;
ALTER TABLE scenarios ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE scenarios
SET description = COALESCE(NULLIF(body->>'description', ''), description),
    updated_at = COALESCE(updated_at, created_at)
WHERE description = '';

ALTER TABLE sim_runs ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE sim_runs ADD COLUMN IF NOT EXISTS trainees JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE sim_runs ADD COLUMN IF NOT EXISTS instructor_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE sim_runs ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS event_annotations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES sim_runs(id) ON DELETE CASCADE,
    event_id TEXT,
    note TEXT NOT NULL,
    actor_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID REFERENCES sim_runs(id) ON DELETE CASCADE,
    scenario_id TEXT,
    actor_id TEXT,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_scenarios_enabled ON scenarios(enabled);
CREATE INDEX IF NOT EXISTS idx_event_annotations_run_time ON event_annotations(run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_run_time ON audit_logs(run_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_scenario_time ON audit_logs(scenario_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);

INSERT INTO schema_migrations (name, version)
VALUES ('ship_sim', 3)
ON CONFLICT (name) DO UPDATE SET version = EXCLUDED.version, applied_at = now();
