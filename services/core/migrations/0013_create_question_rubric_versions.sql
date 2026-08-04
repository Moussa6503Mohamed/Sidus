-- Question rubric versions (T-0007): immutable, monotonically numbered marking rubrics for an
-- original question.
--
-- Like `questions`, this table is infrastructure only: `rubric` holds ORIGINAL rubric structure
-- authored by a future private, approved editorial workflow at runtime. It NEVER holds an
-- official mark scheme, copied marking points, or any other derivative source material, and this
-- migration seeds no rows.
--
-- Version numbers are allocated by Core inside the writing transaction (row lock on the parent
-- question), so they are positive, gapless-by-construction, and unique per question. Once
-- written, the content of a version never changes: the trigger below rejects any UPDATE that
-- touches question_id, version, rubric, or max_marks. Only verification metadata
-- (status/reviewed_by/updated_at) may change, and DELETE is rejected outright.
CREATE TABLE IF NOT EXISTS question_rubric_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions (id),
    version INTEGER NOT NULL CHECK (version > 0),
    -- rubric is a validation-safe JSON structure: {"criteria":[{"id":..,"marks":..,"descriptor"?:..}]}.
    -- Core validates the shape and that criterion marks sum to max_marks before any write.
    rubric JSONB NOT NULL,
    max_marks INTEGER NOT NULL CHECK (max_marks > 0),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'verified')),
    -- created_by / reviewed_by are verified Clerk subjects, never caller-supplied identities.
    created_by TEXT NOT NULL,
    reviewed_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A version number is allocated once per question and can never be reused.
CREATE UNIQUE INDEX IF NOT EXISTS uq_question_rubric_versions_question_version
    ON question_rubric_versions (question_id, version);

CREATE INDEX IF NOT EXISTS idx_question_rubric_versions_question ON question_rubric_versions (question_id);
CREATE INDEX IF NOT EXISTS idx_question_rubric_versions_status ON question_rubric_versions (status);

CREATE OR REPLACE FUNCTION prevent_question_rubric_version_rewrite()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'question_rubric_versions rows cannot be deleted';
    END IF;
    IF NEW.question_id IS DISTINCT FROM OLD.question_id
        OR NEW.version IS DISTINCT FROM OLD.version
        OR NEW.rubric IS DISTINCT FROM OLD.rubric
        OR NEW.max_marks IS DISTINCT FROM OLD.max_marks
        OR NEW.created_by IS DISTINCT FROM OLD.created_by
        OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'question_rubric_versions content is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_question_rubric_versions_immutable ON question_rubric_versions;
CREATE TRIGGER trg_question_rubric_versions_immutable
    BEFORE UPDATE OR DELETE ON question_rubric_versions
    FOR EACH ROW EXECUTE FUNCTION prevent_question_rubric_version_rewrite();
