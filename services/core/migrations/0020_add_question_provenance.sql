-- T-0018: metadata-only question origin and licensed-adaptation provenance.
--
-- Existing questions remain NULL and receive no provenance row: no backfill, origin inference,
-- seed content, licence fact, or data mutation occurs here. New-row enforcement lives in Core so
-- historical rows remain usable without being silently labelled.
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS origin_type TEXT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'questions_origin_type_check'
    ) THEN
        ALTER TABLE questions ADD CONSTRAINT questions_origin_type_check
            CHECK (origin_type IS NULL OR origin_type IN ('original', 'licensed_adaptation'));
    END IF;
END $$;

-- Composite key lets provenance prove its origin_type matches its parent question. PostgreSQL
-- requires referenced columns to have a matching unique constraint even though id alone is the PK.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'questions_id_origin_type_key'
    ) THEN
        ALTER TABLE questions ADD CONSTRAINT questions_id_origin_type_key UNIQUE (id, origin_type);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS question_provenance (
    question_id UUID PRIMARY KEY,
    content_source_id UUID NOT NULL REFERENCES content_sources (id),
    source_locator TEXT NOT NULL CHECK (btrim(source_locator) <> ''),
    origin_type TEXT NOT NULL CHECK (origin_type = 'licensed_adaptation'),
    -- Verified Clerk subject from authenticated create request. Never caller supplied.
    verified_actor_id TEXT NOT NULL CHECK (btrim(verified_actor_id) <> ''),
    verified_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT question_provenance_question_origin_fkey
        FOREIGN KEY (question_id, origin_type) REFERENCES questions (id, origin_type)
);

CREATE INDEX IF NOT EXISTS idx_question_provenance_content_source
    ON question_provenance (content_source_id);

-- Provenance is immutable after its atomic insert with question creation.
CREATE OR REPLACE FUNCTION prevent_question_provenance_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'question_provenance rows are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_question_provenance_no_update ON question_provenance;
CREATE TRIGGER trg_question_provenance_no_update
    BEFORE UPDATE OR DELETE ON question_provenance
    FOR EACH ROW EXECUTE FUNCTION prevent_question_provenance_mutation();

-- Origin is immutable too, including NULL on historical rows. No later write may classify history.
CREATE OR REPLACE FUNCTION prevent_question_origin_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.origin_type IS DISTINCT FROM NEW.origin_type THEN
        RAISE EXCEPTION 'question origin_type is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_questions_origin_no_update ON questions;
CREATE TRIGGER trg_questions_origin_no_update
    BEFORE UPDATE OF origin_type ON questions
    FOR EACH ROW EXECUTE FUNCTION prevent_question_origin_mutation();

-- Separate append-only, names-only provenance audit. No source locator, licence reference,
-- rights value, question text, or source content is stored in changed_fields.
CREATE TABLE IF NOT EXISTS question_provenance_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions (id),
    event_type TEXT NOT NULL CHECK (event_type = 'provenance_recorded'),
    actor_id TEXT NOT NULL CHECK (btrim(actor_id) <> ''),
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_fields TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_question_provenance_events_question
    ON question_provenance_events (question_id);

CREATE OR REPLACE FUNCTION prevent_question_provenance_event_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'question_provenance_events rows are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_question_provenance_events_no_update ON question_provenance_events;
CREATE TRIGGER trg_question_provenance_events_no_update
    BEFORE UPDATE OR DELETE ON question_provenance_events
    FOR EACH ROW EXECUTE FUNCTION prevent_question_provenance_event_mutation();
