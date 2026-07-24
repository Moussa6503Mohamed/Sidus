-- Curriculum catalogue (T-0004): metadata-only registry of subjects and syllabuses.
-- Holds curriculum identity only (board, code, subject, qualification/level, track, display
-- name, lifecycle status) — never source material, questions, objectives, rights claims, or
-- inferred years. This is the single authority for which syllabuses exist; it lets future
-- approved Cambridge syllabuses be onboarded as data, without code changes.

CREATE TABLE IF NOT EXISTS subjects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS syllabuses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board TEXT NOT NULL,
    syllabus_code TEXT NOT NULL,
    subject_id UUID NOT NULL REFERENCES subjects (id),
    qualification TEXT NOT NULL,
    -- track is the tier/route within a syllabus (e.g. 'Extended'); null when not applicable.
    track TEXT,
    display_name TEXT NOT NULL,
    -- curriculum_year / edition is stored only when explicitly known; otherwise null. Never
    -- inferred from filenames or exam-year metadata.
    curriculum_year TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'retired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Safe uniqueness: a syllabus code is NOT assumed globally unique. Identity is the
-- (board, code, track) triple. COALESCE(track,'') collapses NULL and '' so a null-track row
-- cannot silently duplicate an existing (board, code) pair, while different boards may reuse
-- a code.
CREATE UNIQUE INDEX IF NOT EXISTS uq_syllabuses_board_code_track
    ON syllabuses (board, syllabus_code, COALESCE(track, ''));

CREATE INDEX IF NOT EXISTS idx_syllabuses_subject ON syllabuses (subject_id);
CREATE INDEX IF NOT EXISTS idx_syllabuses_status ON syllabuses (status);
CREATE INDEX IF NOT EXISTS idx_syllabuses_code ON syllabuses (syllabus_code);
