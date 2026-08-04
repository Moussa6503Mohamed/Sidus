-- Questions (T-0007): private, original question records for a future Exam Mode.
--
-- This table is infrastructure only. It NEVER holds copied or lightly rewritten source
-- material: no past-paper questions, mark schemes, syllabus text, extracted text, diagrams, or
-- OCR output. `prompt` carries ORIGINAL question wording written by a future private, approved
-- editorial workflow, at runtime, in the database — no question text is ever committed to this
-- repository, and this migration seeds no rows.
--
-- Every question is grounded in exactly one curriculum-map node (T-0006). The node must be
-- `verified`, must belong to the same syllabus as the question, and its approved content source
-- must still pass the T-0006 source gate. Those are point-in-time facts about other tables that
-- a bare FK cannot express, so they are enforced by Core in the application layer, inside the
-- write transaction, on EVERY question write — see services/core/internal/question.
CREATE TABLE IF NOT EXISTS questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    syllabus_id UUID NOT NULL REFERENCES syllabuses (id),
    curriculum_map_node_id UUID NOT NULL REFERENCES curriculum_map_nodes (id),
    response_type TEXT NOT NULL CHECK (response_type IN ('multiple_choice', 'short_answer', 'structured_response')),
    -- language is an opaque non-empty tag (e.g. "en"); no language registry exists yet.
    language TEXT NOT NULL,
    -- prompt is the ORIGINAL question body, authored privately at runtime. Never seeded here.
    prompt TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'verified', 'retired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_questions_syllabus ON questions (syllabus_id);
CREATE INDEX IF NOT EXISTS idx_questions_node ON questions (curriculum_map_node_id);
CREATE INDEX IF NOT EXISTS idx_questions_status ON questions (status);
