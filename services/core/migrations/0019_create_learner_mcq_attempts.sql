-- T-0016: minimal owned, single-question Practice Mode MCQ attempts. Additive, no seed/backfill.
CREATE TABLE IF NOT EXISTS learner_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    learner_subject_id TEXT NOT NULL CHECK (length(btrim(learner_subject_id)) > 0),
    question_id UUID NOT NULL REFERENCES questions(id),
    question_content_revision INTEGER NOT NULL CHECK (question_content_revision > 0),
    canonical_rubric_version_id UUID NOT NULL REFERENCES question_rubric_versions(id),
    selected_option_id TEXT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'submitted')),
    is_correct BOOLEAN NULL,
    awarded_marks INTEGER NULL CHECK (awarded_marks IS NULL OR awarded_marks >= 0),
    max_marks INTEGER NOT NULL CHECK (max_marks > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at TIMESTAMPTZ NULL,
    CONSTRAINT learner_attempts_lifecycle_check CHECK (
        (status = 'open' AND selected_option_id IS NULL AND is_correct IS NULL
            AND awarded_marks IS NULL AND submitted_at IS NULL)
        OR
        (status = 'submitted' AND selected_option_id IS NOT NULL AND length(btrim(selected_option_id)) > 0
            AND is_correct IS NOT NULL AND awarded_marks IS NOT NULL AND submitted_at IS NOT NULL)
    ),
    CONSTRAINT learner_attempts_award_check CHECK (awarded_marks IS NULL OR awarded_marks <= max_marks)
);

CREATE INDEX IF NOT EXISTS idx_learner_attempts_owner_created
    ON learner_attempts (learner_subject_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_learner_attempts_question
    ON learner_attempts (question_id);

CREATE TABLE IF NOT EXISTS learner_attempt_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id UUID NOT NULL REFERENCES learner_attempts(id),
    event_type TEXT NOT NULL CHECK (event_type IN ('attempt_created', 'attempt_submitted')),
    actor_id TEXT NOT NULL CHECK (length(btrim(actor_id)) > 0),
    changed_fields TEXT[] NOT NULL DEFAULT '{}',
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_learner_attempt_events_attempt
    ON learner_attempt_events (attempt_id, event_time ASC);

CREATE OR REPLACE FUNCTION prevent_learner_attempt_event_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'learner attempt events are append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS learner_attempt_events_immutable ON learner_attempt_events;
CREATE TRIGGER learner_attempt_events_immutable
    BEFORE UPDATE OR DELETE ON learner_attempt_events
    FOR EACH ROW EXECUTE FUNCTION prevent_learner_attempt_event_mutation();

CREATE OR REPLACE FUNCTION prevent_learner_attempt_pin_mutation()
RETURNS trigger AS $$
BEGIN
    IF NEW.learner_subject_id IS DISTINCT FROM OLD.learner_subject_id
       OR NEW.question_id IS DISTINCT FROM OLD.question_id
       OR NEW.question_content_revision IS DISTINCT FROM OLD.question_content_revision
       OR NEW.canonical_rubric_version_id IS DISTINCT FROM OLD.canonical_rubric_version_id
       OR NEW.max_marks IS DISTINCT FROM OLD.max_marks
       OR OLD.status = 'submitted' THEN
        RAISE EXCEPTION 'learner attempt pins and submitted attempts are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS learner_attempts_immutable_pins ON learner_attempts;
CREATE TRIGGER learner_attempts_immutable_pins
    BEFORE UPDATE ON learner_attempts
    FOR EACH ROW EXECUTE FUNCTION prevent_learner_attempt_pin_mutation();
