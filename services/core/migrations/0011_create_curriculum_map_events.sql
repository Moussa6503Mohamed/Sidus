-- curriculum_map_events (T-0006): append-only audit trail of curriculum-map node mutations.
-- Records WHICH fields changed (names only) and WHO changed them (the verified Clerk subject)
-- — never field values, and never any source material. Rows are immutable: the trigger below
-- rejects any UPDATE or DELETE. Mirrors the syllabus_events / content_source_events
-- immutability pattern (migrations 0004, 0006).
CREATE TABLE IF NOT EXISTS curriculum_map_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL REFERENCES curriculum_map_nodes (id),
    event_type TEXT NOT NULL CHECK (event_type IN (
        'node_created', 'node_updated', 'node_verified', 'node_retired'
    )),
    actor_id TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_fields TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_curriculum_map_events_node ON curriculum_map_events (node_id);

CREATE OR REPLACE FUNCTION prevent_curriculum_map_event_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'curriculum_map_events rows are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_curriculum_map_events_no_update ON curriculum_map_events;
CREATE TRIGGER trg_curriculum_map_events_no_update
    BEFORE UPDATE OR DELETE ON curriculum_map_events
    FOR EACH ROW EXECUTE FUNCTION prevent_curriculum_map_event_mutation();
