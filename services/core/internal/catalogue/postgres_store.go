package catalogue

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

const (
	subjectColumns  = `id, name, created_at, updated_at`
	syllabusColumns = `id, board, syllabus_code, subject_id, qualification, track, display_name, curriculum_year, status, created_at, updated_at`
)

func scanSubject(row interface{ Scan(...any) error }) (Subject, error) {
	var s Subject
	err := row.Scan(&s.ID, &s.Name, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func scanSyllabus(row interface{ Scan(...any) error }) (Syllabus, error) {
	var s Syllabus
	err := row.Scan(
		&s.ID, &s.Board, &s.SyllabusCode, &s.SubjectID, &s.Qualification, &s.Track,
		&s.DisplayName, &s.CurriculumYear, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

func (p *PostgresStore) CreateSubject(ctx context.Context, in CreateSubjectInput) (Subject, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Subject{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		`INSERT INTO subjects (name) VALUES ($1) RETURNING `+subjectColumns, in.Name,
	)
	subject, err := scanSubject(row)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return Subject{}, ErrDuplicateSubject
		}
		return Subject{}, fmt.Errorf("insert subject: %w", err)
	}

	// Subject creation and its audit event are one transaction: a failed audit insert rolls
	// back the subject row (deferred tx.Rollback above), so no subject can exist without a
	// corresponding subject_created event.
	if err := insertSubjectEvent(ctx, tx, subject.ID, EventSubjectCreated, in.ActorID, []string{"name"}); err != nil {
		return Subject{}, err
	}

	if err := tx.Commit(); err != nil {
		return Subject{}, fmt.Errorf("commit: %w", err)
	}
	return subject, nil
}

func (p *PostgresStore) ListSubjects(ctx context.Context) ([]Subject, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT `+subjectColumns+` FROM subjects ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list subjects: %w", err)
	}
	defer rows.Close()

	subjects := []Subject{}
	for rows.Next() {
		s, err := scanSubject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan subject: %w", err)
		}
		subjects = append(subjects, s)
	}
	return subjects, rows.Err()
}

func (p *PostgresStore) CreateSyllabus(ctx context.Context, in CreateSyllabusInput) (Syllabus, error) {
	status := in.Status
	if status == "" {
		status = StatusDraft
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Syllabus{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO syllabuses (board, syllabus_code, subject_id, qualification, track, display_name, curriculum_year, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+syllabusColumns,
		in.Board, in.SyllabusCode, in.SubjectID, in.Qualification, in.Track, in.DisplayName, in.CurriculumYear, status,
	)
	syllabus, err := scanSyllabus(row)
	if err != nil {
		if mapped := mapSyllabusWriteErr(err); mapped != nil {
			return Syllabus{}, mapped
		}
		return Syllabus{}, fmt.Errorf("insert syllabus: %w", err)
	}

	// changed_fields on create lists the metadata fields that were populated (names only).
	changed := []string{"board", "syllabusCode", "subjectId", "qualification", "displayName", "status"}
	if in.Track != nil {
		changed = append(changed, "track")
	}
	if in.CurriculumYear != nil {
		changed = append(changed, "curriculumYear")
	}
	if err := insertEvent(ctx, tx, syllabus.ID, EventSyllabusCreated, in.ActorID, changed); err != nil {
		return Syllabus{}, err
	}

	if err := tx.Commit(); err != nil {
		return Syllabus{}, fmt.Errorf("commit: %w", err)
	}
	return syllabus, nil
}

// updateColumn pairs a JSON field name with its DB column, the supplied value, and the
// current stored value so UpdateSyllabus can record only fields that actually change.
type updateColumn struct {
	field   string
	column  string
	value   *string
	current *string
}

func (p *PostgresStore) UpdateSyllabus(ctx context.Context, id string, in UpdateSyllabusInput) (Syllabus, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Syllabus{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT `+syllabusColumns+` FROM syllabuses WHERE id = $1 FOR UPDATE`, id)
	current, err := scanSyllabus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Syllabus{}, ErrNotFound
	}
	if err != nil {
		return Syllabus{}, fmt.Errorf("lock syllabus: %w", err)
	}

	var statusValue *string
	if in.Status != nil {
		s := string(*in.Status)
		statusValue = &s
	}
	currentStatus := string(current.Status)
	cols := []updateColumn{
		{"board", "board", in.Board, &current.Board},
		{"syllabusCode", "syllabus_code", in.SyllabusCode, &current.SyllabusCode},
		{"subjectId", "subject_id", in.SubjectID, &current.SubjectID},
		{"qualification", "qualification", in.Qualification, &current.Qualification},
		{"track", "track", in.Track, current.Track},
		{"displayName", "display_name", in.DisplayName, &current.DisplayName},
		{"curriculumYear", "curriculum_year", in.CurriculumYear, current.CurriculumYear},
		{"status", "status", statusValue, &currentStatus},
	}

	var setClauses []string
	var args []any
	var changed []string
	for _, c := range cols {
		if c.value == nil {
			continue
		}
		if c.current != nil && *c.current == *c.value {
			continue
		}
		args = append(args, *c.value)
		setClauses = append(setClauses, c.column+" = $"+strconv.Itoa(len(args)))
		changed = append(changed, c.field)
	}
	if len(changed) == 0 {
		return Syllabus{}, ErrNoChanges
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, id)
	query := `UPDATE syllabuses SET ` + strings.Join(setClauses, ", ") +
		` WHERE id = $` + strconv.Itoa(len(args)) + ` RETURNING ` + syllabusColumns

	row = tx.QueryRowContext(ctx, query, args...)
	updated, err := scanSyllabus(row)
	if err != nil {
		if mapped := mapSyllabusWriteErr(err); mapped != nil {
			return Syllabus{}, mapped
		}
		return Syllabus{}, fmt.Errorf("update syllabus: %w", err)
	}

	if err := insertEvent(ctx, tx, id, EventSyllabusUpdated, in.ActorID, changed); err != nil {
		return Syllabus{}, err
	}

	if err := tx.Commit(); err != nil {
		return Syllabus{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

func (p *PostgresStore) GetSyllabus(ctx context.Context, id string, activeOnly bool) (Syllabus, error) {
	query := `SELECT ` + syllabusColumns + ` FROM syllabuses WHERE id = $1`
	if activeOnly {
		query += ` AND status = 'active'`
	}
	row := p.db.QueryRowContext(ctx, query, id)
	syllabus, err := scanSyllabus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Syllabus{}, ErrNotFound
	}
	if err != nil {
		return Syllabus{}, fmt.Errorf("get syllabus: %w", err)
	}
	return syllabus, nil
}

func (p *PostgresStore) ListSyllabuses(ctx context.Context, activeOnly bool) ([]Syllabus, error) {
	query := `SELECT ` + syllabusColumns + ` FROM syllabuses`
	if activeOnly {
		query += ` WHERE status = 'active'`
	}
	query += ` ORDER BY board ASC, syllabus_code ASC, COALESCE(track, '') ASC`

	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list syllabuses: %w", err)
	}
	defer rows.Close()

	syllabuses := []Syllabus{}
	for rows.Next() {
		s, err := scanSyllabus(rows)
		if err != nil {
			return nil, fmt.Errorf("scan syllabus: %w", err)
		}
		syllabuses = append(syllabuses, s)
	}
	return syllabuses, rows.Err()
}

func (p *PostgresStore) ResolveActiveSyllabusByCode(ctx context.Context, code string) (string, bool, error) {
	// At most two rows are enough to distinguish unique from ambiguous.
	rows, err := p.db.QueryContext(ctx,
		`SELECT id FROM syllabuses WHERE syllabus_code = $1 AND status = 'active' LIMIT 2`, code,
	)
	if err != nil {
		return "", false, fmt.Errorf("resolve syllabus: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", false, fmt.Errorf("scan syllabus id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", false, fmt.Errorf("resolve syllabus: %w", err)
	}
	if len(ids) != 1 {
		// zero (unknown/inactive) or multiple (ambiguous across boards) — not resolvable.
		return "", false, nil
	}
	return ids[0], true, nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, syllabusID string, eventType EventType, actor string, changed []string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO syllabus_events (syllabus_id, event_type, actor_id, changed_fields)
		VALUES ($1, $2, $3, $4)`,
		syllabusID, eventType, actor, pq.Array(changed),
	); err != nil {
		return fmt.Errorf("insert syllabus event: %w", err)
	}
	return nil
}

func insertSubjectEvent(ctx context.Context, tx *sql.Tx, subjectID string, eventType SubjectEventType, actor string, changed []string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO subject_events (subject_id, event_type, actor_id, changed_fields)
		VALUES ($1, $2, $3, $4)`,
		subjectID, eventType, actor, pq.Array(changed),
	); err != nil {
		return fmt.Errorf("insert subject event: %w", err)
	}
	return nil
}

// mapSyllabusWriteErr translates a Postgres constraint violation on a syllabus write into a
// domain error, or returns nil if err is not a recognized constraint violation.
func mapSyllabusWriteErr(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return nil
	}
	switch pqErr.Code {
	case "23505": // unique_violation → (board, code, track) collision
		return ErrDuplicateSyllabus
	case "23503": // foreign_key_violation → subject_id does not exist
		return ErrSubjectNotFound
	default:
		return nil
	}
}
