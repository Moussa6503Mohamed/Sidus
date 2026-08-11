package uploadintake

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
)

type PostgresStore struct{ db *sql.DB }

func NewPostgresStore(db *sql.DB) *PostgresStore { return &PostgresStore{db} }

const uploadColumns = `id,object_ref,original_filename,media_type,byte_size,sha256,content_source_id,status,retention_state,uploaded_by,created_at,updated_at`

func scanUpload(s interface{ Scan(...any) error }) (Upload, error) {
	var v Upload
	e := s.Scan(&v.ID, &v.ObjectRef, &v.OriginalFilename, &v.MediaType, &v.ByteSize, &v.SHA256, &v.ContentSourceID, &v.Status, &v.RetentionState, &v.UploadedBy, &v.CreatedAt, &v.UpdatedAt)
	return v, e
}
func invalidUUID(e error) bool { var p *pq.Error; return errors.As(e, &p) && p.Code == "22P02" }
func (p *PostgresStore) Create(c context.Context, in CreateInput) (Upload, error) {
	tx, e := p.db.BeginTx(c, nil)
	if e != nil {
		return Upload{}, fmt.Errorf("begin upload: %w", e)
	}
	defer tx.Rollback()
	v, e := scanUpload(tx.QueryRowContext(c, `INSERT INTO private_uploads(object_ref,original_filename,media_type,byte_size,sha256,uploaded_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+uploadColumns, in.ObjectRef, in.OriginalFilename, in.MediaType, in.ByteSize, in.SHA256, in.ActorID))
	if e != nil {
		return Upload{}, fmt.Errorf("insert upload: %w", e)
	}
	if _, e = tx.ExecContext(c, `INSERT INTO private_upload_events(upload_id,event_type,actor_id,changed_fields) VALUES($1,'uploaded',$2,ARRAY['objectRef','sha256','status'])`, v.ID, in.ActorID); e != nil {
		return Upload{}, fmt.Errorf("audit upload: %w", e)
	}
	if e = tx.Commit(); e != nil {
		return Upload{}, fmt.Errorf("commit upload: %w", e)
	}
	return v, nil
}
func (p *PostgresStore) List(c context.Context) ([]Upload, error) {
	r, e := p.db.QueryContext(c, `SELECT `+uploadColumns+` FROM private_uploads ORDER BY created_at DESC`)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	out := []Upload{}
	for r.Next() {
		v, e := scanUpload(r)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, r.Err()
}
func (p *PostgresStore) transition(c context.Context, id, actor string, from, to Status, event string) (Upload, error) {
	tx, e := p.db.BeginTx(c, nil)
	if e != nil {
		return Upload{}, e
	}
	defer tx.Rollback()
	v, e := scanUpload(tx.QueryRowContext(c, `SELECT `+uploadColumns+` FROM private_uploads WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(e, sql.ErrNoRows) || invalidUUID(e) {
		return Upload{}, ErrNotFound
	}
	if e != nil {
		return Upload{}, e
	}
	if v.Status != from {
		return Upload{}, ErrInvalidTransition
	}
	v, e = scanUpload(tx.QueryRowContext(c, `UPDATE private_uploads SET status=$1, retention_state=CASE WHEN $1='deletion_requested' THEN 'deletion_requested' ELSE retention_state END,updated_at=now() WHERE id=$2 RETURNING `+uploadColumns, to, id))
	if e != nil {
		return Upload{}, e
	}
	if _, e = tx.ExecContext(c, `INSERT INTO private_upload_events(upload_id,event_type,actor_id,changed_fields) VALUES($1,$2,$3,ARRAY['status'])`, id, event, actor); e != nil {
		return Upload{}, e
	}
	if e = tx.Commit(); e != nil {
		return Upload{}, e
	}
	return v, nil
}
func (p *PostgresStore) MarkScanClean(c context.Context, id, actor string) (Upload, error) {
	return p.transition(c, id, actor, StatusQuarantined, StatusScanClean, "scan_clean")
}
func (p *PostgresStore) RequestDeletion(c context.Context, id, actor string) (Upload, error) {
	tx, e := p.db.BeginTx(c, nil)
	if e != nil {
		return Upload{}, e
	}
	defer tx.Rollback()
	v, e := scanUpload(tx.QueryRowContext(c, `SELECT `+uploadColumns+` FROM private_uploads WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(e, sql.ErrNoRows) || invalidUUID(e) {
		return Upload{}, ErrNotFound
	}
	if e != nil {
		return Upload{}, e
	}
	if v.Status == StatusDeletionRequested {
		return Upload{}, ErrInvalidTransition
	}
	v, e = scanUpload(tx.QueryRowContext(c, `UPDATE private_uploads SET status='deletion_requested',retention_state='deletion_requested',updated_at=now() WHERE id=$1 RETURNING `+uploadColumns, id))
	if e != nil {
		return Upload{}, e
	}
	if _, e = tx.ExecContext(c, `INSERT INTO private_upload_events(upload_id,event_type,actor_id,changed_fields)VALUES($1,'deletion_requested',$2,ARRAY['status','retentionState'])`, id, actor); e != nil {
		return Upload{}, e
	}
	if e = tx.Commit(); e != nil {
		return Upload{}, e
	}
	return v, nil
}
func (p *PostgresStore) QueueReview(c context.Context, id, sourceID, actor string) (ReviewJob, error) {
	tx, e := p.db.BeginTx(c, nil)
	if e != nil {
		return ReviewJob{}, e
	}
	defer tx.Rollback()
	var status Status
	e = tx.QueryRowContext(c, `SELECT status FROM private_uploads WHERE id=$1 FOR UPDATE`, id).Scan(&status)
	if errors.Is(e, sql.ErrNoRows) || invalidUUID(e) {
		return ReviewJob{}, ErrNotFound
	}
	if e != nil {
		return ReviewJob{}, e
	}
	if status != StatusScanClean {
		return ReviewJob{}, ErrInvalidTransition
	}
	var ok bool
	e = tx.QueryRowContext(c, `SELECT EXISTS(SELECT 1 FROM content_sources WHERE id=$1 AND status='approved' AND catalogue_syllabus_id IS NOT NULL)`, sourceID).Scan(&ok)
	if invalidUUID(e) {
		return ReviewJob{}, ErrSourceNotApproved
	}
	if e != nil {
		return ReviewJob{}, e
	}
	if !ok {
		return ReviewJob{}, ErrSourceNotApproved
	}
	var j ReviewJob
	e = tx.QueryRowContext(c, `INSERT INTO private_upload_review_jobs(upload_id,content_source_id,requested_by)VALUES($1,$2,$3) RETURNING id,upload_id,content_source_id,status,adapter_version,created_at`, id, sourceID, actor).Scan(&j.ID, &j.UploadID, &j.ContentSourceID, &j.Status, &j.AdapterVersion, &j.CreatedAt)
	if e != nil {
		return ReviewJob{}, e
	}
	_, e = tx.ExecContext(c, `UPDATE private_uploads SET status='review_pending',content_source_id=$1,updated_at=now() WHERE id=$2`, sourceID, id)
	if e != nil {
		return ReviewJob{}, e
	}
	_, e = tx.ExecContext(c, `INSERT INTO private_upload_events(upload_id,event_type,actor_id,changed_fields)VALUES($1,'review_queued',$2,ARRAY['status','contentSourceId'])`, id, actor)
	if e != nil {
		return ReviewJob{}, e
	}
	if e = tx.Commit(); e != nil {
		return ReviewJob{}, e
	}
	return j, nil
}
