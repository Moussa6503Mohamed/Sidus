-- syllabus_events (T-0004): append-only audit trail of catalogue mutations. Records WHICH
-- fields changed (names only) and WHO changed them (the verified Clerk subject) — never the
-- field values, and never any source material. Rows are immutable: the trigger below rejects
-- any UPDATE or DELETE, so the trail holds even against direct SQL access. Mirrors the
-- content_source_events immutability pattern (migration 0004).
CREATE TABLE IF NOT EXISTS syllabus_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    syllabus_id UUID NOT NULL REFERENCES syllabuses (id),
    event_type TEXT NOT NULL CHECK (event_type IN ('syllabus_created', 'syllabus_updated')),
    actor_id TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_fields TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_syllabus_events_syllabus ON syllabus_events (syllabus_id);

CREATE OR REPLACE FUNCTION prevent_syllabus_event_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'syllabus_events rows are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_syllabus_events_no_update ON syllabus_events;
CREATE TRIGGER trg_syllabus_events_no_update
    BEFORE UPDATE OR DELETE ON syllabus_events
    FOR EACH ROW EXECUTE FUNCTION prevent_syllabus_event_mutation();
