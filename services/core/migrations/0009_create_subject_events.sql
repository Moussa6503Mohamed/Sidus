-- subject_events (T-0004 review fix): append-only audit trail of subject-catalogue mutations.
-- Records WHICH fields changed (names only) and WHO changed them (the verified Clerk subject)
-- — never the field values, and never any source material. Rows are immutable: the trigger
-- below rejects any UPDATE or DELETE. Mirrors the syllabus_events immutability pattern
-- (migration 0006).
--
-- Seed exception: the Biology subject seeded by migration 0007 is inserted with a plain SQL
-- INSERT, not through the CreateSubject application path, so it never produces a subject_events
-- row. That bootstrap row is metadata scaffolding for the D-0004 first vertical slice, not a
-- human-actor event, and no actor id is invented for it.
CREATE TABLE IF NOT EXISTS subject_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES subjects (id),
    event_type TEXT NOT NULL CHECK (event_type IN ('subject_created')),
    actor_id TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_fields TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_subject_events_subject ON subject_events (subject_id);

CREATE OR REPLACE FUNCTION prevent_subject_event_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'subject_events rows are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_subject_events_no_update ON subject_events;
CREATE TRIGGER trg_subject_events_no_update
    BEFORE UPDATE OR DELETE ON subject_events
    FOR EACH ROW EXECUTE FUNCTION prevent_subject_event_mutation();
