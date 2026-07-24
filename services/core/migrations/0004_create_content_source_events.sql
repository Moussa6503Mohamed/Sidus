-- content_source_events: append-only audit trail of metadata changes to a content source.
-- Records WHICH fields changed (names only) and WHO changed them — never the field values,
-- and never any source material (PDFs, extracts, diagrams). Rows are immutable: the trigger
-- below rejects any UPDATE or DELETE, so the trail holds even against direct SQL access.
CREATE TABLE IF NOT EXISTS content_source_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_source_id UUID NOT NULL REFERENCES content_sources (id),
    event_type TEXT NOT NULL CHECK (event_type IN ('metadata_updated')),
    actor_id TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_fields TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_content_source_events_source ON content_source_events (content_source_id);

CREATE OR REPLACE FUNCTION prevent_content_source_event_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'content_source_events rows are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_content_source_events_no_update ON content_source_events;
CREATE TRIGGER trg_content_source_events_no_update
    BEFORE UPDATE OR DELETE ON content_source_events
    FOR EACH ROW EXECUTE FUNCTION prevent_content_source_event_mutation();
