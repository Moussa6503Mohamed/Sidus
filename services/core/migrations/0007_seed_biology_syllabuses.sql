-- Seed the curriculum catalogue with ONLY the D-0004 first-vertical-slice biology
-- syllabuses. Metadata only: board, code, subject, qualification/level, track, display name.
-- No objectives, assessment rules, years/editions, rights claims, or any other syllabus.
-- Idempotent: safe to re-run (ON CONFLICT DO NOTHING against the unique keys).
--
-- Both rows are seeded 'active': they are the confirmed first slice and must resolve for the
-- existing 0610/5090 content-source metadata. Any future syllabus is added as data by an
-- admin via the catalogue API — never seeded here from inventory/filenames.

INSERT INTO subjects (name)
VALUES ('Biology')
ON CONFLICT (name) DO NOTHING;

INSERT INTO syllabuses (board, syllabus_code, subject_id, qualification, track, display_name, status)
SELECT
    'Cambridge International',
    '0610',
    s.id,
    'Cambridge IGCSE',
    'Extended',
    'Cambridge IGCSE Biology 0610 (Extended)',
    'active'
FROM subjects s
WHERE s.name = 'Biology'
ON CONFLICT (board, syllabus_code, COALESCE(track, '')) DO NOTHING;

INSERT INTO syllabuses (board, syllabus_code, subject_id, qualification, track, display_name, status)
SELECT
    'Cambridge International',
    '5090',
    s.id,
    'Cambridge O Level',
    NULL,
    'Cambridge O Level Biology 5090',
    'active'
FROM subjects s
WHERE s.name = 'Biology'
ON CONFLICT (board, syllabus_code, COALESCE(track, '')) DO NOTHING;
