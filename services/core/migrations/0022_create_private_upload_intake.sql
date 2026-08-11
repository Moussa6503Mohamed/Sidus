-- T-0030: private upload intake metadata. Uploaded bytes live only in a configured private
-- object store; this database stores opaque references and metadata, never PDF content.
CREATE TABLE IF NOT EXISTS private_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    object_ref TEXT NOT NULL UNIQUE CHECK (btrim(object_ref) <> ''),
    original_filename TEXT NOT NULL CHECK (btrim(original_filename) <> ''),
    media_type TEXT NOT NULL CHECK (media_type = 'application/pdf'),
    byte_size BIGINT NOT NULL CHECK (byte_size > 0 AND byte_size <= 26214400),
    sha256 CHAR(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    content_source_id UUID NULL REFERENCES content_sources(id),
    status TEXT NOT NULL DEFAULT 'quarantined' CHECK (status IN ('quarantined','scan_clean','review_pending','deletion_requested')),
    retention_state TEXT NOT NULL DEFAULT 'retained' CHECK (retention_state IN ('retained','deletion_requested')),
    uploaded_by TEXT NOT NULL CHECK (btrim(uploaded_by) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_private_uploads_status ON private_uploads(status, created_at ASC);

CREATE TABLE IF NOT EXISTS private_upload_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    upload_id UUID NOT NULL REFERENCES private_uploads(id),
    event_type TEXT NOT NULL CHECK (event_type IN ('uploaded','scan_clean','review_queued','deletion_requested')),
    actor_id TEXT NOT NULL CHECK (btrim(actor_id) <> ''),
    changed_fields TEXT[] NOT NULL,
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_private_upload_events_upload ON private_upload_events(upload_id, event_time ASC);

CREATE OR REPLACE FUNCTION prevent_private_upload_event_mutation()
RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'private_upload_events are immutable'; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_private_upload_events_immutable ON private_upload_events;
CREATE TRIGGER trg_private_upload_events_immutable BEFORE UPDATE OR DELETE ON private_upload_events
FOR EACH ROW EXECUTE FUNCTION prevent_private_upload_event_mutation();

CREATE TABLE IF NOT EXISTS private_upload_review_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    upload_id UUID NOT NULL UNIQUE REFERENCES private_uploads(id),
    content_source_id UUID NOT NULL REFERENCES content_sources(id),
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','completed','published','rejected')),
    adapter_version TEXT NOT NULL DEFAULT 'sonnet-review-v1',
    review_metadata JSONB NULL,
    requested_by TEXT NOT NULL CHECK (btrim(requested_by) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_private_upload_review_jobs_status ON private_upload_review_jobs(status, created_at ASC);
