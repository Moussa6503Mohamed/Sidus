-- T-0032: server-owned learner assessment sessions. Additive; never backfills attempts.
CREATE TABLE IF NOT EXISTS assessment_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  learner_subject_id TEXT NOT NULL,
  mode TEXT NOT NULL CHECK (mode IN ('practice','exam')),
  syllabus_id UUID NOT NULL REFERENCES syllabuses(id),
  curriculum_map_node_id UUID NULL REFERENCES curriculum_map_nodes(id),
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','submitted','expired')),
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deadline_at TIMESTAMPTZ NULL,
  submitted_at TIMESTAMPTZ NULL,
  version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
  result_summary JSONB NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((mode = 'exam' AND deadline_at IS NOT NULL) OR (mode = 'practice' AND deadline_at IS NULL)),
  CHECK ((status = 'open' AND submitted_at IS NULL) OR (status IN ('submitted','expired') AND submitted_at IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS assessment_sessions_one_open_exam_per_learner
 ON assessment_sessions(learner_subject_id) WHERE status = 'open' AND mode = 'exam';
CREATE INDEX IF NOT EXISTS assessment_sessions_owner_status ON assessment_sessions(learner_subject_id,status,created_at DESC);

CREATE TABLE IF NOT EXISTS assessment_session_items (
  session_id UUID NOT NULL REFERENCES assessment_sessions(id) ON DELETE RESTRICT,
  ordinal INTEGER NOT NULL CHECK (ordinal > 0),
  question_id UUID NOT NULL REFERENCES questions(id),
  attempt_id UUID NOT NULL REFERENCES learner_attempts(id),
  snapshot JSONB NOT NULL,
  response_payload JSONB NULL,
  response_version INTEGER NOT NULL DEFAULT 0 CHECK (response_version >= 0),
  PRIMARY KEY(session_id, ordinal),
  UNIQUE(session_id, question_id),
  UNIQUE(session_id, attempt_id)
);
CREATE INDEX IF NOT EXISTS assessment_session_items_session ON assessment_session_items(session_id,ordinal);

CREATE TABLE IF NOT EXISTS assessment_session_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id UUID NOT NULL REFERENCES assessment_sessions(id) ON DELETE RESTRICT,
  event_type TEXT NOT NULL CHECK (event_type IN ('session_created','response_saved','session_submitted','session_expired')),
  actor_id TEXT NOT NULL,
  changed_fields TEXT[] NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE OR REPLACE FUNCTION prevent_assessment_session_event_mutation() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'assessment session events are immutable'; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS assessment_session_events_immutable ON assessment_session_events;
CREATE TRIGGER assessment_session_events_immutable BEFORE UPDATE OR DELETE ON assessment_session_events FOR EACH ROW EXECUTE FUNCTION prevent_assessment_session_event_mutation();
CREATE OR REPLACE FUNCTION prevent_assessment_session_item_mutation() RETURNS trigger AS $$ BEGIN
  IF NEW.session_id IS DISTINCT FROM OLD.session_id OR NEW.ordinal IS DISTINCT FROM OLD.ordinal OR NEW.question_id IS DISTINCT FROM OLD.question_id OR NEW.attempt_id IS DISTINCT FROM OLD.attempt_id OR NEW.snapshot IS DISTINCT FROM OLD.snapshot THEN RAISE EXCEPTION 'assessment session item pins are immutable'; END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS assessment_session_items_pins_immutable ON assessment_session_items;
CREATE TRIGGER assessment_session_items_pins_immutable BEFORE UPDATE ON assessment_session_items FOR EACH ROW EXECUTE FUNCTION prevent_assessment_session_item_mutation();
