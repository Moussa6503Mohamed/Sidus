-- T-0011 realigns the metadata-only Biology catalogue scope.
--
-- Active: Cambridge IGCSE Biology 0610 Extended and one Cambridge International
-- AS & A Level Biology 9700 row. Historical: Cambridge O Level Biology 5090.
-- This migration intentionally touches catalogue metadata only. It does not create or alter
-- content sources, links, events, curriculum-map nodes, questions, or rubrics. The 5090 row
-- remains in place with its stable id and timestamps; only its lifecycle status changes.

INSERT INTO syllabuses (
    board,
    syllabus_code,
    subject_id,
    qualification,
    track,
    display_name,
    curriculum_year,
    status
)
SELECT
    'Cambridge International',
    '9700',
    s.id,
    'International AS & A Level',
    NULL,
    'Cambridge International AS & A Level Biology',
    NULL,
    'active'
FROM subjects s
WHERE s.name = 'Biology'
ON CONFLICT (board, syllabus_code, COALESCE(track, '')) DO UPDATE
SET
    subject_id = EXCLUDED.subject_id,
    qualification = EXCLUDED.qualification,
    track = EXCLUDED.track,
    display_name = EXCLUDED.display_name,
    curriculum_year = EXCLUDED.curriculum_year,
    status = EXCLUDED.status;

UPDATE syllabuses
SET status = 'retired'
WHERE board = 'Cambridge International'
  AND syllabus_code = '5090'
  AND track IS NULL
  AND status <> 'retired';
