package contentsource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

// PostgresStore is a Store backed by PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore wraps an open *sql.DB as a Store.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

const sourceColumns = `id, title, owner, source_url, source_hash, licence_reference, permitted_use, allowed_audience, syllabus_code, status, created_at, updated_at`

func scanSource(row interface{ Scan(...any) error }) (Source, error) {
	var s Source
	err := row.Scan(
		&s.ID, &s.Title, &s.Owner, &s.SourceURL, &s.SourceHash, &s.LicenceReference,
		&s.PermittedUse, &s.AllowedAudience, &s.SyllabusCode, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

func (p *PostgresStore) Create(ctx context.Context, in CreateInput) (Source, error) {
	row := p.db.QueryRowContext(ctx, `
		INSERT INTO content_sources (title, owner, source_url, source_hash, licence_reference, permitted_use, allowed_audience, syllabus_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+sourceColumns,
		in.Title, in.Owner, in.SourceURL, in.SourceHash, in.LicenceReference, in.PermittedUse, in.AllowedAudience, in.SyllabusCode,
	)

	source, err := scanSource(row)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return Source{}, ErrDuplicateSourceURL
		}
		return Source{}, fmt.Errorf("insert content source: %w", err)
	}
	return source, nil
}

func (p *PostgresStore) Get(ctx context.Context, id string) (Source, error) {
	row := p.db.QueryRowContext(ctx, `SELECT `+sourceColumns+` FROM content_sources WHERE id = $1`, id)
	source, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	if err != nil {
		return Source{}, fmt.Errorf("get content source: %w", err)
	}
	return source, nil
}

func (p *PostgresStore) List(ctx context.Context, status *Status) ([]Source, error) {
	query := `SELECT ` + sourceColumns + ` FROM content_sources`
	args := []any{}
	if status != nil {
		query += ` WHERE status = $1`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at ASC`

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list content sources: %w", err)
	}
	defer rows.Close()

	sources := []Source{}
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan content source: %w", err)
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (p *PostgresStore) Approve(ctx context.Context, id string, in ApproveInput) (Source, []string, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Source{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT `+sourceColumns+` FROM content_sources WHERE id = $1 FOR UPDATE`, id)
	source, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, nil, ErrNotFound
	}
	if err != nil {
		return Source{}, nil, fmt.Errorf("lock content source: %w", err)
	}

	if source.Status != StatusPending {
		return Source{}, nil, ErrInvalidTransition
	}

	if missing := MissingApprovalFields(source); len(missing) > 0 {
		return source, missing, nil
	}

	row = tx.QueryRowContext(ctx, `
		UPDATE content_sources SET status = $1, updated_at = now() WHERE id = $2
		RETURNING `+sourceColumns,
		StatusApproved, id,
	)
	source, err = scanSource(row)
	if err != nil {
		return Source{}, nil, fmt.Errorf("update content source: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_source_reviews (content_source_id, decision, reviewer_id, decision_date)
		VALUES ($1, $2, $3, $4)`,
		id, StatusApproved, in.ReviewerID, in.DecisionDate,
	); err != nil {
		return Source{}, nil, fmt.Errorf("insert review: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Source{}, nil, fmt.Errorf("commit: %w", err)
	}

	return source, nil, nil
}

// updateColumn pairs a JSON field name with its database column, the supplied value, and
// the currently stored value (nil means currently unset) so Update can detect real changes.
type updateColumn struct {
	field   string
	column  string
	value   *string
	current *string
}

func (in UpdateInput) columns(current Source) []updateColumn {
	// Order mirrors UpdatableFields so changed-field names and SQL are deterministic.
	return []updateColumn{
		{"title", "title", in.Title, &current.Title},
		{"owner", "owner", in.Owner, current.Owner},
		{"sourceUrl", "source_url", in.SourceURL, &current.SourceURL},
		{"sourceHash", "source_hash", in.SourceHash, current.SourceHash},
		{"licenceReference", "licence_reference", in.LicenceReference, current.LicenceReference},
		{"permittedUse", "permitted_use", in.PermittedUse, current.PermittedUse},
		{"allowedAudience", "allowed_audience", in.AllowedAudience, current.AllowedAudience},
		{"syllabusCode", "syllabus_code", in.SyllabusCode, current.SyllabusCode},
	}
}

func (p *PostgresStore) Update(ctx context.Context, id string, in UpdateInput) (Source, []string, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Source{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT `+sourceColumns+` FROM content_sources WHERE id = $1 FOR UPDATE`, id)
	source, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, nil, ErrNotFound
	}
	if err != nil {
		return Source{}, nil, fmt.Errorf("lock content source: %w", err)
	}

	if source.Status != StatusPending {
		return Source{}, nil, ErrInvalidTransition
	}

	var setClauses []string
	var args []any
	var changed []string
	suppliedCount := 0
	for _, c := range in.columns(source) {
		if c.value == nil {
			continue
		}
		suppliedCount++
		if c.current != nil && *c.current == *c.value {
			continue // supplied value matches what is already stored: not a real change
		}
		args = append(args, *c.value)
		setClauses = append(setClauses, c.column+" = $"+strconv.Itoa(len(args)))
		changed = append(changed, c.field)
	}
	if suppliedCount == 0 {
		return Source{}, nil, ErrNoUpdatableFields
	}
	if len(changed) == 0 {
		return Source{}, nil, ErrNoChanges
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, id)
	query := `UPDATE content_sources SET ` + strings.Join(setClauses, ", ") +
		` WHERE id = $` + strconv.Itoa(len(args)) + ` RETURNING ` + sourceColumns

	row = tx.QueryRowContext(ctx, query, args...)
	source, err = scanSource(row)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return Source{}, nil, ErrDuplicateSourceURL
		}
		return Source{}, nil, fmt.Errorf("update content source: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_source_events (content_source_id, event_type, actor_id, changed_fields)
		VALUES ($1, $2, $3, $4)`,
		id, EventMetadataUpdated, in.ActorID, pq.Array(changed),
	); err != nil {
		return Source{}, nil, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Source{}, nil, fmt.Errorf("commit: %w", err)
	}

	return source, changed, nil
}

func (p *PostgresStore) Reject(ctx context.Context, id string, in RejectInput) (Source, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Source{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT `+sourceColumns+` FROM content_sources WHERE id = $1 FOR UPDATE`, id)
	source, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	if err != nil {
		return Source{}, fmt.Errorf("lock content source: %w", err)
	}

	if source.Status != StatusPending {
		return Source{}, ErrInvalidTransition
	}

	row = tx.QueryRowContext(ctx, `
		UPDATE content_sources SET status = $1, updated_at = now() WHERE id = $2
		RETURNING `+sourceColumns,
		StatusRejected, id,
	)
	source, err = scanSource(row)
	if err != nil {
		return Source{}, fmt.Errorf("update content source: %w", err)
	}

	reason := in.Reason
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_source_reviews (content_source_id, decision, reviewer_id, decision_date, reason)
		VALUES ($1, $2, $3, $4, $5)`,
		id, StatusRejected, in.ReviewerID, in.DecisionDate, reason,
	); err != nil {
		return Source{}, fmt.Errorf("insert review: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Source{}, fmt.Errorf("commit: %w", err)
	}

	return source, nil
}
