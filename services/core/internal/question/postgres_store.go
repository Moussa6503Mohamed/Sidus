package question

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

const questionColumns = `id, syllabus_id, curriculum_map_node_id, response_type, language, prompt, options, status, content_revision, created_at, updated_at`

const rubricColumns = `id, question_id, version, question_revision, rubric, max_marks, status, created_by, reviewed_by, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scanQuestion(row scanner) (Question, error) {
	var q Question
	var options []byte
	err := row.Scan(
		&q.ID, &q.SyllabusID, &q.CurriculumMapNodeID, &q.ResponseType, &q.Language, &q.Prompt,
		&options, &q.Status, &q.ContentRevision, &q.CreatedAt, &q.UpdatedAt,
	)
	if err == nil && options != nil {
		err = json.Unmarshal(options, &q.Options)
	}
	return q, err
}

func scanRubricVersion(row scanner) (RubricVersion, error) {
	var v RubricVersion
	var rubric []byte
	err := row.Scan(
		&v.ID, &v.QuestionID, &v.Version, &v.QuestionRevision, &rubric, &v.MaxMarks, &v.Status,
		&v.CreatedBy, &v.ReviewedBy, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return RubricVersion{}, err
	}
	v.Rubric = json.RawMessage(rubric)
	return v, nil
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// isInvalidTextRepresentation reports whether err is Postgres 22P02 (e.g. a non-UUID string
// compared against a UUID column). A malformed id is a client error, not an infrastructure
// failure, so it maps to the same stable domain error as a missing row.
func isInvalidTextRepresentation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "22P02"
}

// syllabusIsActive reports whether id resolves to a known, active catalogue syllabus.
func syllabusIsActive(ctx context.Context, q queryRower, id string) (bool, error) {
	var status string
	err := q.QueryRowContext(ctx, `SELECT status FROM syllabuses WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check syllabus: %w", err)
	}
	return status == "active", nil
}

// validateGrounding is the sole enforcement point for a question's curriculum grounding. It runs
// inside every write transaction, before any row is written, and checks that:
//
//  1. the curriculum-map node exists,
//  2. the node is verified (a draft or retired node can never ground a question),
//  3. the node belongs to syllabusID (the question's immutable syllabus), and
//  4. the node's content source still passes the T-0006 source gate: it exists, is approved, and
//     its catalogue_syllabus_id equals syllabusID.
//
// Point 4 is re-checked here rather than trusted from node creation because a source can be
// un-approved, unlinked, or re-linked to another syllabus after the node — and the question —
// were written.
func validateGrounding(ctx context.Context, q queryRower, nodeID, syllabusID string) error {
	var nodeSyllabusID, nodeStatus, sourceID string
	err := q.QueryRowContext(ctx,
		`SELECT syllabus_id, status, content_source_id FROM curriculum_map_nodes WHERE id = $1`, nodeID,
	).Scan(&nodeSyllabusID, &nodeStatus, &sourceID)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return ErrUnknownNode
	}
	if err != nil {
		return fmt.Errorf("check curriculum map node: %w", err)
	}
	if nodeSyllabusID != syllabusID {
		return ErrMismatchedNode
	}
	if nodeStatus != "verified" {
		return ErrUnverifiedNode
	}

	var sourceStatus string
	var catalogueSyllabusID *string
	err = q.QueryRowContext(ctx,
		`SELECT status, catalogue_syllabus_id FROM content_sources WHERE id = $1`, sourceID,
	).Scan(&sourceStatus, &catalogueSyllabusID)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return ErrUnknownSource
	}
	if err != nil {
		return fmt.Errorf("check content source: %w", err)
	}
	if sourceStatus != "approved" {
		return ErrUnapprovedSource
	}
	if catalogueSyllabusID == nil {
		return ErrUnlinkedSource
	}
	if *catalogueSyllabusID != syllabusID {
		return ErrMismatchedSource
	}
	return nil
}

// validateNodeFilter resolves an optional list filter against the curriculum map. It is weaker
// than validateGrounding on purpose: a reader may legitimately filter by a node that has since
// been retired or whose source has regressed, and would then get an empty list. What it must not
// do is silently accept a node that does not exist or belongs to another syllabus.
func validateNodeFilter(ctx context.Context, q queryRower, nodeID, syllabusID string) error {
	var nodeSyllabusID string
	err := q.QueryRowContext(ctx,
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

func (p *PostgresStore) CreateQuestion(ctx context.Context, in CreateInput) (Question, error) {
	if !IsValidResponseType(in.ResponseType) || validateOptionsForResponseType(in.ResponseType, in.Options) != nil {
		return Question{}, ErrInvalidOptions
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Question{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	active, err := syllabusIsActive(ctx, tx, in.SyllabusID)
	if err != nil {
		return Question{}, err
	}
	if !active {
		return Question{}, ErrUnknownSyllabus
	}

	if err := validateGrounding(ctx, tx, in.CurriculumMapNodeID, in.SyllabusID); err != nil {
		return Question{}, err
	}

	var optionsJSON any
	if in.Options != nil {
		encoded, err := json.Marshal(in.Options)
		if err != nil {
			return Question{}, ErrInvalidOptions
		}
		optionsJSON = encoded
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO questions (syllabus_id, curriculum_map_node_id, response_type, language, prompt, options)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+questionColumns,
		in.SyllabusID, in.CurriculumMapNodeID, string(in.ResponseType), in.Language, in.Prompt, optionsJSON,
	)
	q, err := scanQuestion(row)
	if err != nil {
		return Question{}, fmt.Errorf("insert question: %w", err)
	}

	// Field NAMES only — a prompt value is never written to the audit trail.
	changed := []string{"syllabusId", "curriculumMapNodeId", "responseType", "language", "prompt"}
	if in.Options != nil {
		changed = append(changed, "options")
	}
	if err := insertEvent(ctx, tx, q.ID, EventQuestionCreated, in.ActorID, changed); err != nil {
		return Question{}, err
	}

	if err := tx.Commit(); err != nil {
		return Question{}, fmt.Errorf("commit: %w", err)
	}
	return q, nil
}

func (p *PostgresStore) UpdateQuestion(ctx context.Context, id string, in UpdateInput) (Question, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Question{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := lockQuestion(ctx, tx, id)
	if err != nil {
		return Question{}, err
	}
	if current.Status != StatusDraft {
		return Question{}, ErrInvalidTransition
	}

	// The grounding gate re-runs on every update, against the effective node (the supplied one if
	// present, otherwise the stored one), before any diff, write, or event.
	effectiveNodeID := current.CurriculumMapNodeID
	if in.CurriculumMapNodeID != nil {
		effectiveNodeID = *in.CurriculumMapNodeID
	}
	if err := validateGrounding(ctx, tx, effectiveNodeID, current.SyllabusID); err != nil {
		return Question{}, err
	}

	var setClauses []string
	var args []any
	var changed []string
	effectiveResponseType := current.ResponseType
	if in.ResponseType != nil {
		effectiveResponseType = *in.ResponseType
	}
	effectiveOptions := current.Options
	if in.Options != nil {
		effectiveOptions = *in.Options
	}
	if effectiveResponseType != ResponseMultipleChoice {
		if in.Options != nil {
			return Question{}, ErrInvalidOptions
		}
		effectiveOptions = nil
	}
	if err := validateOptionsForResponseType(effectiveResponseType, effectiveOptions); err != nil {
		return Question{}, err
	}

	addString := func(field, column string, value *string, cur string) {
		if value == nil || *value == cur {
			return
		}
		args = append(args, *value)
		setClauses = append(setClauses, column+" = $"+strconv.Itoa(len(args)))
		changed = append(changed, field)
	}

	addString("curriculumMapNodeId", "curriculum_map_node_id", in.CurriculumMapNodeID, current.CurriculumMapNodeID)
	addString("language", "language", in.Language, current.Language)
	addString("prompt", "prompt", in.Prompt, current.Prompt)
	if in.ResponseType != nil && *in.ResponseType != current.ResponseType {
		args = append(args, string(*in.ResponseType))
		setClauses = append(setClauses, "response_type = $"+strconv.Itoa(len(args)))
		changed = append(changed, "responseType")
	}
	if !reflect.DeepEqual(effectiveOptions, current.Options) {
		if effectiveOptions == nil {
			setClauses = append(setClauses, "options = NULL")
		} else {
			encoded, err := json.Marshal(effectiveOptions)
			if err != nil {
				return Question{}, ErrInvalidOptions
			}
			args = append(args, encoded)
			setClauses = append(setClauses, "options = $"+strconv.Itoa(len(args)))
		}
		changed = append(changed, "options")
	}

	if len(changed) == 0 {
		return Question{}, ErrNoChanges
	}

	// Exactly one increment per successful content update, inside the same transaction and under
	// the row lock taken above. ErrNoChanges returns before this point, and any rejected write
	// rolls the transaction back, so a revision is never skipped or double-counted. Every rubric
	// version stamped with the old revision becomes stale for question verification here —
	// without being deleted, downgraded, or edited.
	setClauses = append(setClauses, "content_revision = content_revision + 1")
	changed = append(changed, "contentRevision")

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, id)
	query := `UPDATE questions SET ` + strings.Join(setClauses, ", ") +
		` WHERE id = $` + strconv.Itoa(len(args)) + ` RETURNING ` + questionColumns

	updated, err := scanQuestion(tx.QueryRowContext(ctx, query, args...))
	if err != nil {
		return Question{}, fmt.Errorf("update question: %w", err)
	}

	if err := insertEvent(ctx, tx, id, EventQuestionUpdated, in.ActorID, changed); err != nil {
		return Question{}, err
	}

	if err := tx.Commit(); err != nil {
		return Question{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

func (p *PostgresStore) VerifyQuestion(ctx context.Context, id string, actorID string) (Question, error) {
	return p.transitionStatus(ctx, id, actorID, EventQuestionVerified, StatusVerified, []Status{StatusDraft}, true)
}

func (p *PostgresStore) RetireQuestion(ctx context.Context, id string, actorID string) (Question, error) {
	return p.transitionStatus(ctx, id, actorID, EventQuestionRetired, StatusRetired, []Status{StatusDraft, StatusVerified}, false)
}

func (p *PostgresStore) transitionStatus(
	ctx context.Context,
	id, actorID string,
	eventType EventType,
	next Status,
	allowedFrom []Status,
	requireVerifiedRubric bool,
) (Question, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Question{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := lockQuestion(ctx, tx, id)
	if err != nil {
		return Question{}, err
	}

	allowed := false
	for _, s := range allowedFrom {
		if current.Status == s {
			allowed = true
			break
		}
	}
	if !allowed {
		return Question{}, ErrInvalidTransition
	}

	// Verify/retire are question writes too: the grounding gate re-runs against the stored node
	// before the status change and its event.
	if err := validateGrounding(ctx, tx, current.CurriculumMapNodeID, current.SyllabusID); err != nil {
		return Question{}, err
	}

	if requireVerifiedRubric {
		if err := requireCurrentVerifiedRubric(ctx, tx, id, current.ContentRevision); err != nil {
			return Question{}, err
		}
	}

	updated, err := scanQuestion(tx.QueryRowContext(ctx,
		`UPDATE questions SET status = $1, updated_at = now() WHERE id = $2 RETURNING `+questionColumns,
		string(next), id,
	))
	if err != nil {
		return Question{}, fmt.Errorf("update question status: %w", err)
	}

	if err := insertEvent(ctx, tx, id, eventType, actorID, []string{"status"}); err != nil {
		return Question{}, err
	}

	if err := tx.Commit(); err != nil {
		return Question{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

// requireCurrentVerifiedRubric enforces that the question has at least one verified rubric
// version reviewed against its CURRENT content revision.
//
// A verified rubric for an older revision is not enough: the reviewer approved marking criteria
// for wording, a response type, a language, or a curriculum-map node that the question no longer
// has. Such versions stay readable and immutable — they are only stale for verification — so the
// fix is to append a new version at the current revision and verify that.
//
// It runs inside the verifying transaction, under the caller's row lock on the question, so the
// revision it compares against cannot move underneath it.
func requireCurrentVerifiedRubric(ctx context.Context, tx *sql.Tx, questionID string, revision int) error {
	var verifiedTotal, verifiedCurrent int
	if err := tx.QueryRowContext(ctx, `
		SELECT count(*), count(*) FILTER (WHERE question_revision = $2)
		FROM question_rubric_versions
		WHERE question_id = $1 AND status = 'verified'`,
		questionID, revision,
	).Scan(&verifiedTotal, &verifiedCurrent); err != nil {
		return fmt.Errorf("count verified rubric versions: %w", err)
	}
	if verifiedCurrent > 0 {
		return nil
	}
	if verifiedTotal == 0 {
		return ErrMissingVerifiedRubric
	}
	return ErrStaleVerifiedRubric
}

func (p *PostgresStore) GetQuestion(ctx context.Context, id string) (Question, error) {
	query := `SELECT ` + questionColumns + ` FROM questions WHERE id = $1`
	q, err := scanQuestion(p.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return Question{}, ErrNotFound
	}
	if err != nil {
		return Question{}, fmt.Errorf("get question: %w", err)
	}
	return q, nil
}

func (p *PostgresStore) ListQuestions(ctx context.Context, syllabusID string, nodeID *string, status *Status) ([]Question, error) {
	// An unknown or inactive syllabus is a bad request, not an empty result: an empty 200 would
	// make a typo indistinguishable from a real syllabus that has no questions yet.
	active, err := syllabusIsActive(ctx, p.db, syllabusID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrUnknownSyllabus
	}

	// The optional node filter is validated for the same reason as the syllabus: an unknown,
	// malformed, or foreign node id must be a stable 400, not a 200 with an empty list that a
	// caller cannot tell from "this node has no questions yet".
	if nodeID != nil {
		if err := validateNodeFilter(ctx, p.db, *nodeID, syllabusID); err != nil {
			return nil, err
		}
	}

	args := []any{syllabusID}
	query := `SELECT ` + questionColumns + ` FROM questions WHERE syllabus_id = $1`
	if nodeID != nil {
		args = append(args, *nodeID)
		query += ` AND curriculum_map_node_id = $` + strconv.Itoa(len(args))
	}
	if status != nil {
		args = append(args, *status)
		query += ` AND status = $` + strconv.Itoa(len(args))
	}
	query += ` ORDER BY created_at ASC`

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		if isInvalidTextRepresentation(err) {
			// A malformed node filter is a client error, and no node can match it.
			return nil, ErrUnknownNode
		}
		return nil, fmt.Errorf("list questions: %w", err)
	}
	defer rows.Close()

	questions := []Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan question: %w", err)
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

func (p *PostgresStore) CreateRubricVersion(ctx context.Context, questionID string, in CreateRubricVersionInput) (RubricVersion, error) {
	// Validate the rubric before opening a transaction: a bad payload must never reach the DB.
	if err := ValidateRubric(in.Rubric, in.MaxMarks); err != nil {
		return RubricVersion{}, err
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return RubricVersion{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// The row lock both gates the lifecycle check and serializes version-number allocation.
	current, err := lockQuestion(ctx, tx, questionID)
	if err != nil {
		return RubricVersion{}, err
	}
	if current.Status != StatusDraft {
		return RubricVersion{}, ErrInvalidTransition
	}
	if err := validateGrounding(ctx, tx, current.CurriculumMapNodeID, current.SyllabusID); err != nil {
		return RubricVersion{}, err
	}
	if err := ValidateRubricForQuestion(in.Rubric, in.MaxMarks, current.ResponseType, current.Options); err != nil {
		return RubricVersion{}, err
	}

	var nextVersion int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(max(version), 0) + 1 FROM question_rubric_versions WHERE question_id = $1`, questionID,
	).Scan(&nextVersion); err != nil {
		return RubricVersion{}, fmt.Errorf("allocate rubric version: %w", err)
	}

	// The question's current content revision is stamped onto the version under the same row lock
	// that allocated its number, so a concurrent edit cannot slip between the two: the reviewer
	// will verify a rubric against exactly the content read here.
	version, err := scanRubricVersion(tx.QueryRowContext(ctx, `
		INSERT INTO question_rubric_versions (question_id, version, question_revision, rubric, max_marks, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+rubricColumns,
		questionID, nextVersion, current.ContentRevision, []byte(in.Rubric), in.MaxMarks, in.ActorID,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return RubricVersion{}, ErrDuplicateRubricVersion
		}
		return RubricVersion{}, fmt.Errorf("insert rubric version: %w", err)
	}

	// Names only: neither the rubric structure, the marks values, nor the revision number is
	// recorded — only the names of the fields the write set.
	if err := insertEvent(ctx, tx, questionID, EventRubricVersionCreated, in.ActorID, []string{"rubricVersion", "questionRevision"}); err != nil {
		return RubricVersion{}, err
	}

	if err := tx.Commit(); err != nil {
		return RubricVersion{}, fmt.Errorf("commit: %w", err)
	}
	return version, nil
}

func (p *PostgresStore) ListRubricVersions(ctx context.Context, questionID string) ([]RubricVersion, error) {
	if _, err := p.getQuestionRow(ctx, questionID); err != nil {
		return nil, err
	}

	rows, err := p.db.QueryContext(ctx,
		`SELECT `+rubricColumns+` FROM question_rubric_versions WHERE question_id = $1 ORDER BY version ASC`,
		questionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list rubric versions: %w", err)
	}
	defer rows.Close()

	versions := []RubricVersion{}
	for rows.Next() {
		v, err := scanRubricVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan rubric version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func (p *PostgresStore) VerifyRubricVersion(ctx context.Context, questionID string, version int, actorID string) (RubricVersion, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return RubricVersion{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := lockQuestion(ctx, tx, questionID)
	if err != nil {
		return RubricVersion{}, err
	}
	// A retired question's rubrics are frozen; draft and verified questions may gain a verified
	// rubric version.
	if current.Status != StatusDraft && current.Status != StatusVerified {
		return RubricVersion{}, ErrInvalidTransition
	}
	if err := validateGrounding(ctx, tx, current.CurriculumMapNodeID, current.SyllabusID); err != nil {
		return RubricVersion{}, err
	}

	stored, err := scanRubricVersion(tx.QueryRowContext(ctx,
		`SELECT `+rubricColumns+` FROM question_rubric_versions WHERE question_id = $1 AND version = $2 FOR UPDATE`,
		questionID, version,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return RubricVersion{}, ErrRubricVersionNotFound
	}
	if err != nil {
		return RubricVersion{}, fmt.Errorf("lock rubric version: %w", err)
	}
	if stored.Status != RubricDraft {
		return RubricVersion{}, ErrInvalidTransition
	}
	// Re-validate the stored rubric against its stored maximum marks: verification is the point
	// at which a rubric becomes usable, so it must still be internally consistent.
	if err := ValidateRubric(stored.Rubric, stored.MaxMarks); err != nil {
		return RubricVersion{}, err
	}

	updated, err := scanRubricVersion(tx.QueryRowContext(ctx, `
		UPDATE question_rubric_versions SET status = 'verified', reviewed_by = $1, updated_at = now()
		WHERE question_id = $2 AND version = $3
		RETURNING `+rubricColumns,
		actorID, questionID, version,
	))
	if err != nil {
		return RubricVersion{}, fmt.Errorf("verify rubric version: %w", err)
	}

	if err := insertEvent(ctx, tx, questionID, EventRubricVersionVerify, actorID, []string{"rubricVersionStatus"}); err != nil {
		return RubricVersion{}, err
	}

	if err := tx.Commit(); err != nil {
		return RubricVersion{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

// lockQuestion loads and row-locks a question inside a write transaction.
func lockQuestion(ctx context.Context, tx *sql.Tx, id string) (Question, error) {
	q, err := scanQuestion(tx.QueryRowContext(ctx,
		`SELECT `+questionColumns+` FROM questions WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return Question{}, ErrNotFound
	}
	if err != nil {
		return Question{}, fmt.Errorf("lock question: %w", err)
	}
	return q, nil
}

// getQuestionRow loads a question without a lock, mapping a missing or malformed id to
// ErrNotFound.
func (p *PostgresStore) getQuestionRow(ctx context.Context, id string) (Question, error) {
	q, err := scanQuestion(p.db.QueryRowContext(ctx,
		`SELECT `+questionColumns+` FROM questions WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return Question{}, ErrNotFound
	}
	if err != nil {
		return Question{}, fmt.Errorf("get question: %w", err)
	}
	return q, nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, questionID string, eventType EventType, actor string, changed []string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO question_events (question_id, event_type, actor_id, changed_fields)
		VALUES ($1, $2, $3, $4)`,
		questionID, string(eventType), actor, pq.Array(changed),
	); err != nil {
		return fmt.Errorf("insert question event: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
