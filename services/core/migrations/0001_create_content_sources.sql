-- content_sources: rights/provenance record for any external material considered for ingestion.
-- Only title and source_url are required at creation; rights fields are filled in before
-- a source can move out of 'pending' (see 0002 and the approval validation in application code).
CREATE TABLE IF NOT EXISTS content_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    owner TEXT,
    source_url TEXT NOT NULL UNIQUE,
    source_hash TEXT,
    licence_reference TEXT,
    permitted_use TEXT,
    allowed_audience TEXT,
    syllabus_code TEXT CHECK (syllabus_code IN ('0610', '5090')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_content_sources_status ON content_sources (status);
