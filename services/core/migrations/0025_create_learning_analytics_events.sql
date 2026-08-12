-- T-0035: learner-owned outcome snapshots. Never stores answers, rubrics, source metadata or model trace.
CREATE TABLE IF NOT EXISTS learning_analytics_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  learner_subject_id TEXT NOT NULL,
  attempt_id UUID NOT NULL REFERENCES learner_attempts(id) ON DELETE RESTRICT,
  event_type TEXT NOT NULL CHECK (event_type IN ('deterministic_scored','pending_marking','automated_marking_accepted','automated_marking_withheld')),
  syllabus_id UUID NOT NULL REFERENCES syllabuses(id) ON DELETE RESTRICT,
  module_id UUID NOT NULL REFERENCES curriculum_map_nodes(id) ON DELETE RESTRICT,
  module_code TEXT NOT NULL CHECK (char_length(module_code) BETWEEN 1 AND 128),
  module_label TEXT NOT NULL CHECK (char_length(module_label) BETWEEN 1 AND 512),
  response_type TEXT NOT NULL,
  awarded_marks INTEGER NULL CHECK (awarded_marks >= 0),
  max_marks INTEGER NULL CHECK (max_marks > 0 AND (awarded_marks IS NULL OR awarded_marks <= max_marks)),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (attempt_id, event_type),
  CHECK ((event_type IN ('deterministic_scored','automated_marking_accepted') AND awarded_marks IS NOT NULL AND max_marks IS NOT NULL) OR
         (event_type IN ('pending_marking','automated_marking_withheld') AND awarded_marks IS NULL AND max_marks IS NULL))
);
CREATE INDEX IF NOT EXISTS learning_analytics_events_owner_time ON learning_analytics_events(learner_subject_id, created_at DESC);
CREATE INDEX IF NOT EXISTS learning_analytics_events_owner_module ON learning_analytics_events(learner_subject_id, syllabus_id, module_id);
CREATE OR REPLACE FUNCTION prevent_learning_analytics_event_mutation() RETURNS trigger AS $$
BEGIN RAISE EXCEPTION 'learning analytics events are immutable'; END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS learning_analytics_events_immutable ON learning_analytics_events;
CREATE TRIGGER learning_analytics_events_immutable BEFORE UPDATE OR DELETE ON learning_analytics_events
FOR EACH ROW EXECUTE FUNCTION prevent_learning_analytics_event_mutation();
