-- T-0034: durable, owner-scoped written-response AI marking state.  Attempts remain immutable.
CREATE TABLE IF NOT EXISTS written_marking_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  attempt_id UUID NOT NULL UNIQUE REFERENCES learner_attempts(id) ON DELETE RESTRICT,
  learner_subject_id TEXT NOT NULL,
  question_id UUID NOT NULL REFERENCES questions(id) ON DELETE RESTRICT,
  question_content_revision INTEGER NOT NULL CHECK (question_content_revision > 0),
  canonical_rubric_version_id UUID NOT NULL REFERENCES question_rubric_versions(id) ON DELETE RESTRICT,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','withheld')),
  retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0 AND retry_count <= 3),
  withheld_reason TEXT NULL CHECK (withheld_reason IS NULL OR char_length(withheld_reason) <= 128),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  accepted_at TIMESTAMPTZ NULL,
  CHECK ((status = 'accepted' AND accepted_at IS NOT NULL AND withheld_reason IS NULL) OR
         (status = 'withheld' AND accepted_at IS NULL AND withheld_reason IS NOT NULL) OR
         (status = 'pending' AND accepted_at IS NULL AND withheld_reason IS NULL))
);
CREATE INDEX IF NOT EXISTS written_marking_requests_owner ON written_marking_requests(learner_subject_id, created_at DESC);

CREATE TABLE IF NOT EXISTS written_marking_results (
  request_id UUID PRIMARY KEY REFERENCES written_marking_requests(id) ON DELETE RESTRICT,
  criterion_marks JSONB NOT NULL,
  awarded_marks INTEGER NOT NULL CHECK (awarded_marks >= 0),
  max_marks INTEGER NOT NULL CHECK (max_marks > 0 AND awarded_marks <= max_marks),
  model TEXT NOT NULL CHECK (char_length(model) BETWEEN 1 AND 128),
  model_version TEXT NOT NULL CHECK (char_length(model_version) BETWEEN 1 AND 128),
  cost_usd_micros BIGINT NOT NULL CHECK (cost_usd_micros >= 0),
  confidence DOUBLE PRECISION NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS written_marking_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id UUID NOT NULL REFERENCES written_marking_requests(id) ON DELETE RESTRICT,
  event_type TEXT NOT NULL CHECK (event_type IN ('marking_requested','marking_retry','marking_accepted','marking_withheld')),
  actor_id TEXT NOT NULL,
  changed_fields TEXT[] NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE OR REPLACE FUNCTION prevent_written_marking_event_mutation() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'written marking events are immutable'; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS written_marking_events_immutable ON written_marking_events;
CREATE TRIGGER written_marking_events_immutable BEFORE UPDATE OR DELETE ON written_marking_events FOR EACH ROW EXECUTE FUNCTION prevent_written_marking_event_mutation();
CREATE OR REPLACE FUNCTION prevent_written_marking_request_terminal_mutation() RETURNS trigger AS $$ BEGIN
  IF OLD.status IN ('accepted','withheld') AND NEW IS DISTINCT FROM OLD THEN RAISE EXCEPTION 'final written marking requests are immutable'; END IF;
  IF NEW.attempt_id IS DISTINCT FROM OLD.attempt_id OR NEW.learner_subject_id IS DISTINCT FROM OLD.learner_subject_id OR NEW.question_id IS DISTINCT FROM OLD.question_id OR NEW.question_content_revision IS DISTINCT FROM OLD.question_content_revision OR NEW.canonical_rubric_version_id IS DISTINCT FROM OLD.canonical_rubric_version_id THEN RAISE EXCEPTION 'written marking pins are immutable'; END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS written_marking_requests_terminal_immutable ON written_marking_requests;
CREATE TRIGGER written_marking_requests_terminal_immutable BEFORE UPDATE ON written_marking_requests FOR EACH ROW EXECUTE FUNCTION prevent_written_marking_request_terminal_mutation();
CREATE OR REPLACE FUNCTION prevent_written_marking_result_mutation() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'written marking results are immutable'; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS written_marking_results_immutable ON written_marking_results;
CREATE TRIGGER written_marking_results_immutable BEFORE UPDATE OR DELETE ON written_marking_results FOR EACH ROW EXECUTE FUNCTION prevent_written_marking_result_mutation();
