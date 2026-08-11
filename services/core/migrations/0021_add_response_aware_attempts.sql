-- T-0029: additive response-aware delivery. Existing MCQ attempts remain readable.
ALTER TABLE questions DROP CONSTRAINT IF EXISTS questions_response_type_check;
ALTER TABLE questions ADD CONSTRAINT questions_response_type_check CHECK (
  response_type IN ('multiple_choice','multi_select','exact_short_answer','numeric_answer','short_answer','structured_response','essay')
);

ALTER TABLE learner_attempts ADD COLUMN IF NOT EXISTS response_type TEXT NOT NULL DEFAULT 'multiple_choice';
ALTER TABLE learner_attempts ADD COLUMN IF NOT EXISTS answer_payload JSONB NULL;
ALTER TABLE learner_attempts ADD COLUMN IF NOT EXISTS result_status TEXT NULL;
ALTER TABLE learner_attempts DROP CONSTRAINT IF EXISTS learner_attempts_lifecycle_check;
ALTER TABLE learner_attempts ADD CONSTRAINT learner_attempts_lifecycle_check CHECK (
  (status = 'open' AND selected_option_id IS NULL AND answer_payload IS NULL AND is_correct IS NULL AND awarded_marks IS NULL AND submitted_at IS NULL AND result_status IS NULL)
  OR
  (status = 'submitted' AND (
    (result_status = 'marked' AND answer_payload IS NOT NULL AND is_correct IS NOT NULL AND awarded_marks IS NOT NULL AND submitted_at IS NOT NULL)
    OR (result_status = 'pending_review' AND answer_payload IS NOT NULL AND is_correct IS NULL AND awarded_marks IS NULL AND submitted_at IS NOT NULL)
    OR (result_status IS NULL AND selected_option_id IS NOT NULL AND is_correct IS NOT NULL AND awarded_marks IS NOT NULL AND submitted_at IS NOT NULL)
  ))
);
ALTER TABLE learner_attempts ADD CONSTRAINT learner_attempts_response_type_check CHECK (
  response_type IN ('multiple_choice','multi_select','exact_short_answer','numeric_answer','short_answer','structured_response','essay')
);
ALTER TABLE learner_attempts ADD CONSTRAINT learner_attempts_result_status_check CHECK (
  result_status IS NULL OR result_status IN ('marked','pending_review')
);

-- Pins include response type. Submitted rows remain wholly immutable through this trigger.
CREATE OR REPLACE FUNCTION prevent_learner_attempt_pin_mutation()
RETURNS trigger AS $$
BEGIN
    IF NEW.learner_subject_id IS DISTINCT FROM OLD.learner_subject_id
       OR NEW.question_id IS DISTINCT FROM OLD.question_id
       OR NEW.question_content_revision IS DISTINCT FROM OLD.question_content_revision
       OR NEW.canonical_rubric_version_id IS DISTINCT FROM OLD.canonical_rubric_version_id
       OR NEW.max_marks IS DISTINCT FROM OLD.max_marks
       OR NEW.response_type IS DISTINCT FROM OLD.response_type
       OR OLD.status = 'submitted' THEN
        RAISE EXCEPTION 'learner attempt pins and submitted attempts are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
