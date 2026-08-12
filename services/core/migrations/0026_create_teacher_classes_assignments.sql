-- T-0036: consented, owner-scoped teacher classes and immutable assignment snapshots.
CREATE TABLE IF NOT EXISTS teacher_classes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), owner_subject_id TEXT NOT NULL,
  name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS teacher_classes_owner ON teacher_classes(owner_subject_id, created_at DESC);

CREATE TABLE IF NOT EXISTS teacher_class_invites (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), class_id UUID NOT NULL REFERENCES teacher_classes(id) ON DELETE RESTRICT,
  token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'), expires_at TIMESTAMPTZ NOT NULL,
  max_uses INTEGER NOT NULL CHECK (max_uses BETWEEN 1 AND 100), accepted_uses INTEGER NOT NULL DEFAULT 0 CHECK (accepted_uses >= 0 AND accepted_uses <= max_uses),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS teacher_class_invites_class ON teacher_class_invites(class_id, expires_at);

CREATE TABLE IF NOT EXISTS teacher_class_memberships (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), class_id UUID NOT NULL REFERENCES teacher_classes(id) ON DELETE RESTRICT,
  learner_subject_id TEXT NOT NULL, status TEXT NOT NULL CHECK (status IN ('active','revoked')),
  consented_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(class_id, learner_subject_id), CHECK ((status='active' AND revoked_at IS NULL) OR (status='revoked' AND revoked_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS teacher_class_memberships_learner ON teacher_class_memberships(learner_subject_id, status);

CREATE TABLE IF NOT EXISTS teacher_class_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), class_id UUID NOT NULL REFERENCES teacher_classes(id) ON DELETE RESTRICT,
  membership_id UUID NULL REFERENCES teacher_class_memberships(id) ON DELETE RESTRICT, event_type TEXT NOT NULL CHECK (event_type IN ('class_created','invite_created','membership_accepted','membership_revoked')),
  actor_id TEXT NOT NULL, changed_fields TEXT[] NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS teacher_assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), class_id UUID NOT NULL REFERENCES teacher_classes(id) ON DELETE RESTRICT,
  owner_subject_id TEXT NOT NULL, title TEXT NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 160),
  syllabus_id UUID NOT NULL REFERENCES syllabuses(id) ON DELETE RESTRICT, module_id UUID NOT NULL REFERENCES curriculum_map_nodes(id) ON DELETE RESTRICT,
  marking_mode TEXT NOT NULL DEFAULT 'automated' CHECK (marking_mode IN ('automated','manual_teacher')),
  settings_snapshot JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS teacher_assignments_class ON teacher_assignments(class_id, created_at DESC);
CREATE TABLE IF NOT EXISTS teacher_assignment_items (
  assignment_id UUID NOT NULL REFERENCES teacher_assignments(id) ON DELETE RESTRICT, ordinal INTEGER NOT NULL CHECK (ordinal > 0),
  question_id UUID NOT NULL REFERENCES questions(id) ON DELETE RESTRICT, snapshot JSONB NOT NULL,
  PRIMARY KEY(assignment_id, ordinal), UNIQUE(assignment_id, question_id)
);
CREATE TABLE IF NOT EXISTS teacher_assignment_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), assignment_id UUID NOT NULL REFERENCES teacher_assignments(id) ON DELETE RESTRICT,
  learner_subject_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'started' CHECK (status IN ('started','completed')),
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(), completed_at TIMESTAMPTZ NULL,
  UNIQUE(assignment_id, learner_subject_id)
);
CREATE TABLE IF NOT EXISTS teacher_assignment_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), assignment_id UUID NOT NULL REFERENCES teacher_assignments(id) ON DELETE RESTRICT,
  actor_id TEXT NOT NULL, event_type TEXT NOT NULL CHECK (event_type IN ('assignment_created','assignment_started')),
  changed_fields TEXT[] NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(assignment_id,event_type,actor_id)
);

CREATE OR REPLACE FUNCTION prevent_teacher_event_mutation() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'teacher audit events are immutable'; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS teacher_class_events_immutable ON teacher_class_events;
CREATE TRIGGER teacher_class_events_immutable BEFORE UPDATE OR DELETE ON teacher_class_events FOR EACH ROW EXECUTE FUNCTION prevent_teacher_event_mutation();
DROP TRIGGER IF EXISTS teacher_assignment_events_immutable ON teacher_assignment_events;
CREATE TRIGGER teacher_assignment_events_immutable BEFORE UPDATE OR DELETE ON teacher_assignment_events FOR EACH ROW EXECUTE FUNCTION prevent_teacher_event_mutation();
CREATE OR REPLACE FUNCTION prevent_teacher_assignment_snapshot_mutation() RETURNS trigger AS $$ BEGIN
 IF NEW.class_id IS DISTINCT FROM OLD.class_id OR NEW.owner_subject_id IS DISTINCT FROM OLD.owner_subject_id OR NEW.title IS DISTINCT FROM OLD.title OR NEW.syllabus_id IS DISTINCT FROM OLD.syllabus_id OR NEW.module_id IS DISTINCT FROM OLD.module_id OR NEW.marking_mode IS DISTINCT FROM OLD.marking_mode OR NEW.settings_snapshot IS DISTINCT FROM OLD.settings_snapshot THEN RAISE EXCEPTION 'assignment settings are immutable'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS teacher_assignments_immutable ON teacher_assignments;
CREATE TRIGGER teacher_assignments_immutable BEFORE UPDATE ON teacher_assignments FOR EACH ROW EXECUTE FUNCTION prevent_teacher_assignment_snapshot_mutation();
CREATE OR REPLACE FUNCTION prevent_teacher_assignment_item_mutation() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'assignment items are immutable'; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS teacher_assignment_items_immutable ON teacher_assignment_items;
CREATE TRIGGER teacher_assignment_items_immutable BEFORE UPDATE OR DELETE ON teacher_assignment_items FOR EACH ROW EXECUTE FUNCTION prevent_teacher_assignment_item_mutation();
