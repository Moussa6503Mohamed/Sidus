-- content_source_reviews: append-only audit trail of approve/reject decisions.
-- Rows are immutable: the trigger below rejects any UPDATE or DELETE.
CREATE TABLE IF NOT EXISTS content_source_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_source_id UUID NOT NULL REFERENCES content_sources (id),
    decision TEXT NOT NULL CHECK (decision IN ('approved', 'rejected')),
    reviewer_id TEXT NOT NULL,
    decision_date TIMESTAMPTZ NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_content_source_reviews_source ON content_source_reviews (content_source_id);

CREATE OR REPLACE FUNCTION prevent_content_source_review_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'content_source_reviews rows are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_content_source_reviews_no_update ON content_source_reviews;
CREATE TRIGGER trg_content_source_reviews_no_update
    BEFORE UPDATE OR DELETE ON content_source_reviews
    FOR EACH ROW EXECUTE FUNCTION prevent_content_source_review_mutation();
