-- question_events (T-0007): append-only audit trail of question and rubric-version mutations.
-- Records WHICH fields changed (names only) and WHO changed them (the verified Clerk subject) —
-- never field values, never a question prompt, never rubric structure, and never any source
-- material. Rows are immutable: the trigger below rejects any UPDATE or DELETE. Mirrors
-- content_source_events / syllabus_events / curriculum_map_events (migrations 0004, 0006, 0011).
CREATE TABLE IF NOT EXISTS question_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions (id),
    event_type TEXT NOT NULL CHECK (event_type IN (
        'question_created', 'question_updated', 'question_verified', 'question_retired',
        'rubric_version_created', 'rubric_version_verified'
    )),
    actor_id TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_fields TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_question_events_question ON question_events (question_id);

CREATE OR REPLACE FUNCTION prevent_question_event_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'question_events rows are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_question_events_no_update ON question_events;
CREATE TRIGGER trg_question_events_no_update
    BEFORE UPDATE OR DELETE ON question_events
    FOR EACH ROW EXECUTE FUNCTION prevent_question_event_mutation();
