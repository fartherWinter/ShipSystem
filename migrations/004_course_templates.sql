CREATE TABLE IF NOT EXISTS course_templates (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'database',
    enabled BOOLEAN NOT NULL DEFAULT true,
    training_only BOOLEAN NOT NULL DEFAULT true,
    scenario JSONB NOT NULL,
    expected_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    review_checklist JSONB NOT NULL DEFAULT '[]'::jsonb,
    safety_notice TEXT NOT NULL,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_course_templates_enabled ON course_templates(enabled);
CREATE INDEX IF NOT EXISTS idx_course_templates_source ON course_templates(source);

INSERT INTO schema_migrations (name, version)
VALUES ('ship_sim', 4)
ON CONFLICT (name) DO UPDATE SET version = EXCLUDED.version, applied_at = now();
