-- Question content revision (T-0007 review fix 1): tie a rubric version to the exact question
-- content it was reviewed against.
--
-- Before this migration a draft question could gain a VERIFIED rubric version and then have its
-- prompt, response type, language, or curriculum-map node changed; question verification accepted
-- any verified rubric, including one reviewed against wording that no longer exists. A rubric is
-- only meaningful for the question text it was written for, so verification must require a
-- verified rubric for the question's CURRENT content.
--
-- Purely additive: two columns and a widened immutability trigger. No table is dropped, no column
-- is removed or retyped, and no row is seeded — questions and rubric versions still exist only in
-- a runtime database, written by a future private, approved editorial workflow.

-- questions.content_revision starts at 1 and is incremented by exactly one by Core inside the
-- transaction of every SUCCESSFUL draft-content update (curriculum_map_node_id, response_type,
-- language, prompt). A rejected write and a no-op update ("no changes") never increment it.
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS content_revision INTEGER NOT NULL DEFAULT 1;

ALTER TABLE questions
    DROP CONSTRAINT IF EXISTS questions_content_revision_positive;
ALTER TABLE questions
    ADD CONSTRAINT questions_content_revision_positive CHECK (content_revision > 0);

-- question_rubric_versions.question_revision records the questions.content_revision that was
-- current when the version was created — i.e. the exact question content the reviewer marked
-- against. A version is CURRENT only while it equals the question's content_revision; an edit
-- makes older versions stale for question verification without deleting or downgrading them.
ALTER TABLE question_rubric_versions
    ADD COLUMN IF NOT EXISTS question_revision INTEGER NOT NULL DEFAULT 1;

ALTER TABLE question_rubric_versions
    DROP CONSTRAINT IF EXISTS question_rubric_versions_question_revision_positive;
ALTER TABLE question_rubric_versions
    ADD CONSTRAINT question_rubric_versions_question_revision_positive CHECK (question_revision > 0);

-- Verifying a question reads this column, so it must be as immutable as the rubric itself:
-- otherwise a direct UPDATE could re-point a stale version at the current revision and bypass the
-- whole check. Only status / reviewed_by / updated_at remain mutable.
CREATE OR REPLACE FUNCTION prevent_question_rubric_version_rewrite()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'question_rubric_versions rows cannot be deleted';
    END IF;
    IF NEW.question_id IS DISTINCT FROM OLD.question_id
        OR NEW.version IS DISTINCT FROM OLD.version
        OR NEW.question_revision IS DISTINCT FROM OLD.question_revision
        OR NEW.rubric IS DISTINCT FROM OLD.rubric
        OR NEW.max_marks IS DISTINCT FROM OLD.max_marks
        OR NEW.created_by IS DISTINCT FROM OLD.created_by
        OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'question_rubric_versions content is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Finding the current verified rubric for a question is the hot path of question verification.
CREATE INDEX IF NOT EXISTS idx_question_rubric_versions_question_revision
    ON question_rubric_versions (question_id, question_revision, status);
