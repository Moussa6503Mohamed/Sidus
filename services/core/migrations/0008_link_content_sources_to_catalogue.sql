-- Link content_sources to the curriculum catalogue (T-0004), non-destructively.
--
-- 1. Add a NULLABLE FK from content_sources to syllabuses. Existing rows (incl. the seeded
--    0610/5090 provenance rows from migration 0003) get NULL and are NOT auto-mapped; a human
--    links them in a later task. No data loss, no rewrite of audit tables.
ALTER TABLE content_sources
    ADD COLUMN IF NOT EXISTS catalogue_syllabus_id UUID REFERENCES syllabuses (id);

CREATE INDEX IF NOT EXISTS idx_content_sources_catalogue_syllabus
    ON content_sources (catalogue_syllabus_id);

-- 2. Drop the hard-coded two-code CHECK on syllabus_code. The registry (syllabuses table),
--    not a fixed enum, is now the authority for valid syllabus codes. The legacy free-text
--    syllabus_code column is retained for backward compatibility with existing 0610/5090
--    source metadata; new writes also populate catalogue_syllabus_id via registry resolution.
ALTER TABLE content_sources
    DROP CONSTRAINT IF EXISTS content_sources_syllabus_code_check;
