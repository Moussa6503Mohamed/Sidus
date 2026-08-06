-- T-0014: explicit canonical rubric selection. Existing questions remain NULL; no rubric is
-- selected or backfilled automatically. Statements are additive and safe to rerun.
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS canonical_rubric_version_id UUID NULL
        REFERENCES question_rubric_versions(id);

CREATE INDEX IF NOT EXISTS idx_questions_canonical_rubric_version_id
    ON questions (canonical_rubric_version_id)
    WHERE canonical_rubric_version_id IS NOT NULL;

-- Extend existing audit allowlist for names-only canonical selection event. Replacing check is
-- safe on rerun and changes no event row.
ALTER TABLE question_events
    DROP CONSTRAINT IF EXISTS question_events_event_type_check;
ALTER TABLE question_events
    ADD CONSTRAINT question_events_event_type_check CHECK (event_type IN (
        'question_created', 'question_updated', 'question_verified', 'question_retired',
        'rubric_version_created', 'rubric_version_verified', 'canonical_rubric_selected'
    ));
