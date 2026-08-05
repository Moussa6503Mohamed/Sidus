-- T-0013: original multiple-choice options are question content. Existing rows remain valid with
-- NULL options; Core enforces response-type-specific shape on all new writes. No content is seeded.
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS options JSONB NULL;
