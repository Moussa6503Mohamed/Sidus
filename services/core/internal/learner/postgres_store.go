package learner

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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

const projectionColumns = `q.id, q.syllabus_id, q.curriculum_map_node_id, q.response_type, q.language, q.prompt, q.options, q.content_revision`

// eligibleQuestionsQuery is the single authority for learner eligibility. It is a plain SELECT —
// not cached, not a materialized view — so every read re-evaluates the gate against current
// table state:
//
//   - the question itself is verified;
//   - it has a canonical rubric version (the join drops the row entirely when
//     canonical_rubric_version_id is NULL, since NULL never equals an id);
//   - that canonical rubric version actually belongs to this question (rv.question_id = q.id).
//     canonical_rubric_version_id is a plain FK to question_rubric_versions.id with no
//     database-level constraint tying it to the owning question, so this ownership check is the
//     only thing standing between a learner and a rubric row that happens to share an id
//     coincidence path with a different question — it must be explicit, never implied by the id
//     join alone;
//   - that canonical version is itself verified and stamped at the question's CURRENT content
//     revision (never a stale or "latest" version — D-0016's explicit-selection rule flows
//     straight through to what a learner may read);
//   - the grounding curriculum-map node is verified; and
//   - the node's content source is approved and still linked to the question's own syllabus
//     (catalogue_syllabus_id is nullable; NULL = X is NULL/false in SQL, so an unlinked source is
//     excluded without a separate IS NOT NULL check).
const eligibleQuestionsQuery = `
	SELECT ` + projectionColumns + `
	FROM questions q
	JOIN curriculum_map_nodes n ON n.id = q.curriculum_map_node_id
	JOIN content_sources s ON s.id = n.content_source_id
	JOIN question_rubric_versions rv ON rv.id = q.canonical_rubric_version_id AND rv.question_id = q.id
	WHERE q.status = 'verified'
	  AND rv.status = 'verified'
	  AND rv.question_revision = q.content_revision
	  AND n.status = 'verified'
	  AND s.status = 'approved'
	  AND s.catalogue_syllabus_id = q.syllabus_id
`

type scanner interface{ Scan(...any) error }

func scanProjection(row scanner) (Projection, error) {
	var p Projection
	var options []byte
	err := row.Scan(
		&p.ID, &p.SyllabusID, &p.CurriculumMapNodeID, &p.ResponseType, &p.Language, &p.Prompt,
		&options, &p.ContentRevision,
	)
	if err == nil && options != nil {
		err = json.Unmarshal(options, &p.Options)
	}
	return p, err
}

// isInvalidTextRepresentation reports whether err is Postgres 22P02 (e.g. a non-UUID string
// compared against a UUID column). A malformed id is a client error, not an infrastructure
// failure, so it maps to the same stable domain error as a missing row (D-0010 precedent).
func isInvalidTextRepresentation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "22P02"
}

func syllabusIsActive(ctx context.Context, db *sql.DB, id string) (bool, error) {
	var status string
	err := db.QueryRowContext(ctx, `SELECT status FROM syllabuses WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check syllabus: %w", err)
	}
	return status == "active", nil
}

// validateNodeFilter checks that an optional node filter exists and belongs to syllabusID. It is
// deliberately weaker than the eligibility gate: a filter naming a real node of the right
// syllabus that currently has no eligible questions returns an empty list, not an error — only an
// unknown or foreign node id is rejected, so a typo is never indistinguishable from "nothing here
// yet".
func validateNodeFilter(ctx context.Context, db *sql.DB, nodeID, syllabusID string) error {
	var nodeSyllabusID string
	err := db.QueryRowContext(ctx,
		`SELECT syllabus_id FROM curriculum_map_nodes WHERE id = $1`, nodeID,
	).Scan(&nodeSyllabusID)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return ErrUnknownNode
	}
	if err != nil {
		return fmt.Errorf("check curriculum map node filter: %w", err)
	}
	if nodeSyllabusID != syllabusID {
		return ErrMismatchedNode
	}
	return nil
}

func (p *PostgresStore) ListQuestions(ctx context.Context, syllabusID string, nodeID *string) ([]Projection, error) {
	active, err := syllabusIsActive(ctx, p.db, syllabusID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrUnknownSyllabus
	}

	if nodeID != nil {
		if err := validateNodeFilter(ctx, p.db, *nodeID, syllabusID); err != nil {
			return nil, err
		}
	}

	args := []any{syllabusID}
	query := eligibleQuestionsQuery + ` AND q.syllabus_id = $1`
	if nodeID != nil {
		args = append(args, *nodeID)
		query += ` AND q.curriculum_map_node_id = $2`
	}
	query += ` ORDER BY q.created_at ASC`

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		if isInvalidTextRepresentation(err) {
			return nil, ErrUnknownNode
		}
		return nil, fmt.Errorf("list learner questions: %w", err)
	}
	defer rows.Close()

	items := []Projection{}
	for rows.Next() {
		proj, err := scanProjection(rows)
		if err != nil {
			return nil, fmt.Errorf("scan learner question: %w", err)
		}
		items = append(items, proj)
	}
	return items, rows.Err()
}

const syllabusProjectionColumns = `id, board, syllabus_code, qualification, track, display_name`

func scanSyllabus(row scanner) (Syllabus, error) {
	var s Syllabus
	err := row.Scan(&s.ID, &s.Board, &s.SyllabusCode, &s.Qualification, &s.Track, &s.DisplayName)
	return s, err
}

// ListActiveSyllabuses returns the learner-safe discovery projection for every catalogue
// syllabus currently `active`. It reads the same `syllabuses` table as the editorial catalogue
// package (services/core/internal/catalogue) directly by SQL rather than importing that package,
// for the same reason the question-eligibility gate above never imports `question`: a future
// widening of catalogue.Syllabus can never leak into this projection by accident, because this
// type is hand-written and only reads the six columns it needs.
func (p *PostgresStore) ListActiveSyllabuses(ctx context.Context) ([]Syllabus, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT `+syllabusProjectionColumns+`
		FROM syllabuses
		WHERE status = 'active'
		ORDER BY board ASC, syllabus_code ASC, COALESCE(track, '') ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active syllabuses: %w", err)
	}
	defer rows.Close()

	items := []Syllabus{}
	for rows.Next() {
		s, err := scanSyllabus(rows)
		if err != nil {
			return nil, fmt.Errorf("scan syllabus: %w", err)
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (p *PostgresStore) GetQuestion(ctx context.Context, id string) (Projection, error) {
	query := eligibleQuestionsQuery + ` AND q.id = $1`
	proj, err := scanProjection(p.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return Projection{}, ErrNotFound
	}
	if err != nil {
		return Projection{}, fmt.Errorf("get learner question: %w", err)
	}
	return proj, nil
}
