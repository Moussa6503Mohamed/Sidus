package learner

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

const projectionColumns = `q.id, q.syllabus_id, q.curriculum_map_node_id, q.response_type, q.language, q.prompt, q.options, q.content_revision`

// Historical questions have NULL origin_type and keep pre-T-0018 behavior. New original rows need
// no external provenance. Licensed adaptations must retain one immutable provenance row whose
// source is still approved, rights-complete, and linked to question syllabus.
const licensedEligibilityPredicate = `
	  AND (
		q.origin_type IS NULL
		OR q.origin_type = 'original'
		OR (
			q.origin_type = 'licensed_adaptation'
			AND qp.question_id IS NOT NULL
			AND btrim(qp.source_locator) <> ''
			AND ps.status = 'approved'
			AND ps.catalogue_syllabus_id = q.syllabus_id
			AND NULLIF(btrim(ps.owner), '') IS NOT NULL
			AND NULLIF(btrim(ps.title), '') IS NOT NULL
			AND NULLIF(btrim(ps.source_url), '') IS NOT NULL
			AND NULLIF(btrim(ps.source_hash), '') IS NOT NULL
			AND NULLIF(btrim(ps.licence_reference), '') IS NOT NULL
			AND NULLIF(btrim(ps.permitted_use), '') IS NOT NULL
			AND NULLIF(btrim(ps.allowed_audience), '') IS NOT NULL
		)
	  )
`

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
	LEFT JOIN question_provenance qp ON qp.question_id = q.id AND qp.origin_type = q.origin_type
	LEFT JOIN content_sources ps ON ps.id = qp.content_source_id
	WHERE q.status = 'verified'
	  AND rv.status = 'verified'
	  AND rv.question_revision = q.content_revision
	  AND n.status = 'verified'
	  AND s.status = 'approved'
	  AND s.catalogue_syllabus_id = q.syllabus_id
` + licensedEligibilityPredicate

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

// ListModules mirrors ListQuestions' live eligibility gate, but projects only four learner-safe
// module fields. A module remains discoverable only while it has at least one eligible question;
// source provenance, editorial lifecycle and parent hierarchy never cross this boundary.
func (p *PostgresStore) ListModules(ctx context.Context, syllabusID string) ([]Module, error) {
	active, err := syllabusIsActive(ctx, p.db, syllabusID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrUnknownSyllabus
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT DISTINCT n.id, n.syllabus_id, n.node_code, n.label
		FROM questions q
		JOIN curriculum_map_nodes n ON n.id = q.curriculum_map_node_id
		JOIN content_sources s ON s.id = n.content_source_id
		JOIN question_rubric_versions rv ON rv.id = q.canonical_rubric_version_id AND rv.question_id = q.id
		LEFT JOIN question_provenance qp ON qp.question_id = q.id AND qp.origin_type = q.origin_type
		LEFT JOIN content_sources ps ON ps.id = qp.content_source_id
		WHERE q.syllabus_id = $1
		  AND q.status = 'verified'
		  AND rv.status = 'verified'
		  AND rv.question_revision = q.content_revision
		  AND n.status = 'verified'
		  AND s.status = 'approved'
		  AND s.catalogue_syllabus_id = q.syllabus_id
		`+licensedEligibilityPredicate+`
		ORDER BY n.node_code ASC, n.label ASC, n.id ASC
	`, syllabusID)
	if err != nil {
		if isInvalidTextRepresentation(err) {
			return nil, ErrUnknownSyllabus
		}
		return nil, fmt.Errorf("list learner modules: %w", err)
	}
	defer rows.Close()
	items := []Module{}
	for rows.Next() {
		var item Module
		if err := rows.Scan(&item.ID, &item.SyllabusID, &item.Code, &item.Label); err != nil {
			return nil, fmt.Errorf("scan learner module: %w", err)
		}
		items = append(items, item)
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

func (p *PostgresStore) CreateAttempt(ctx context.Context, learnerSubjectID, questionID string) (Attempt, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Attempt{}, fmt.Errorf("begin create learner attempt: %w", err)
	}
	defer tx.Rollback()

	var revision, maxMarks int
	var responseType ResponseType
	var canonicalID string
	var rubricRaw, optionsRaw []byte
	err = tx.QueryRowContext(ctx, `
		SELECT q.content_revision, q.canonical_rubric_version_id, rv.max_marks, rv.rubric, q.options, q.response_type
		FROM questions q
		JOIN curriculum_map_nodes n ON n.id = q.curriculum_map_node_id
		JOIN content_sources s ON s.id = n.content_source_id
		JOIN question_rubric_versions rv ON rv.id = q.canonical_rubric_version_id AND rv.question_id = q.id
		LEFT JOIN question_provenance qp ON qp.question_id = q.id AND qp.origin_type = q.origin_type
		LEFT JOIN content_sources ps ON ps.id = qp.content_source_id
		WHERE q.id = $1 AND q.status = 'verified'
		  AND rv.status = 'verified' AND rv.question_revision = q.content_revision
		  AND n.status = 'verified' AND s.status = 'approved'
		  AND s.catalogue_syllabus_id = q.syllabus_id
		`+licensedEligibilityPredicate+`
		FOR SHARE OF q, rv, n, s
	`, questionID).Scan(&revision, &canonicalID, &maxMarks, &rubricRaw, &optionsRaw, &responseType)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return Attempt{}, ErrNotFound
	}
	if err != nil {
		return Attempt{}, fmt.Errorf("pin eligible learner question: %w", err)
	}
	if err := validatePinnedAnswerSchema(responseType, rubricRaw, optionsRaw); err != nil {
		return Attempt{}, ErrNotFound
	}

	var attempt Attempt
	err = tx.QueryRowContext(ctx, `
		INSERT INTO learner_attempts
			(learner_subject_id, question_id, question_content_revision, canonical_rubric_version_id, max_marks, response_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, question_id, status, max_marks, response_type
	`, learnerSubjectID, questionID, revision, canonicalID, maxMarks, responseType).Scan(
		&attempt.AttemptID, &attempt.QuestionID, &attempt.Status, &attempt.MaxMarks, &attempt.ResponseType,
	)
	if err != nil {
		return Attempt{}, fmt.Errorf("insert learner attempt: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO learner_attempt_events (attempt_id, event_type, actor_id, changed_fields)
		VALUES ($1, 'attempt_created', $2, ARRAY['status'])
	`, attempt.AttemptID, learnerSubjectID); err != nil {
		return Attempt{}, fmt.Errorf("insert learner attempt event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Attempt{}, fmt.Errorf("commit learner attempt: %w", err)
	}
	return attempt, nil
}

func markingCoversOptions(marking markingRubric, options []Option) bool {
	if len(options) < 2 || len(marking.Feedback.IncorrectExplanations) != len(options)-1 {
		return false
	}
	current := make(map[string]struct{}, len(options))
	for _, option := range options {
		if option.ID == "" {
			return false
		}
		if _, duplicate := current[option.ID]; duplicate {
			return false
		}
		current[option.ID] = struct{}{}
	}
	if _, ok := current[marking.CorrectOptionID]; !ok {
		return false
	}
	explained := make(map[string]struct{}, len(marking.Feedback.IncorrectExplanations))
	for _, item := range marking.Feedback.IncorrectExplanations {
		if item.OptionID == marking.CorrectOptionID {
			return false
		}
		if _, ok := current[item.OptionID]; !ok {
			return false
		}
		if _, duplicate := explained[item.OptionID]; duplicate {
			return false
		}
		explained[item.OptionID] = struct{}{}
	}
	for optionID := range current {
		if optionID == marking.CorrectOptionID {
			continue
		}
		if _, ok := explained[optionID]; !ok {
			return false
		}
	}
	return true
}

func optionBelongsToRubric(marking markingRubric, selected string) bool {
	if selected == marking.CorrectOptionID {
		return true
	}
	for _, item := range marking.Feedback.IncorrectExplanations {
		if item.OptionID == selected {
			return true
		}
	}
	return false
}

func (p *PostgresStore) SubmitAttempt(ctx context.Context, learnerSubjectID, attemptID, selectedOptionID string) (AttemptResult, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("begin submit learner attempt: %w", err)
	}
	defer tx.Rollback()

	var result AttemptResult
	var status AttemptStatus
	var rubricRaw, optionsRaw []byte
	var responseType ResponseType
	err = tx.QueryRowContext(ctx, `
		SELECT a.id, a.question_id, a.status, a.max_marks, rv.rubric, q.options, a.response_type
		FROM learner_attempts a
		JOIN questions q ON q.id = a.question_id
		JOIN question_rubric_versions rv
		  ON rv.id = a.canonical_rubric_version_id
		 AND rv.question_id = a.question_id
		 AND rv.question_revision = a.question_content_revision
		LEFT JOIN question_provenance qp ON qp.question_id = q.id AND qp.origin_type = q.origin_type
		LEFT JOIN content_sources ps ON ps.id = qp.content_source_id
		WHERE a.id = $1 AND a.learner_subject_id = $2
		`+licensedEligibilityPredicate+`
		FOR UPDATE OF a
	`, attemptID, learnerSubjectID).Scan(
		&result.AttemptID, &result.QuestionID, &status, &result.MaxMarks, &rubricRaw, &optionsRaw, &responseType,
	)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return AttemptResult{}, ErrAttemptNotFound
	}
	if err != nil {
		return AttemptResult{}, fmt.Errorf("lock learner attempt: %w", err)
	}
	if status != AttemptOpen {
		return AttemptResult{}, ErrAttemptSubmitted
	}
	if err := validatePinnedAnswerSchema(responseType, rubricRaw, optionsRaw); err != nil {
		return AttemptResult{}, ErrAttemptNotFound
	}
	answer, marked, correct, err := markAnswer(responseType, rubricRaw, selectedOptionID)
	if err != nil {
		return AttemptResult{}, err
	}
	result.ResponseType, result.Answer = responseType, answer
	if marked {
		result.ResultStatus = "marked"
		result.IsCorrect = correct
		if correct {
			result.AwardedMarks = result.MaxMarks
		}
	} else {
		result.ResultStatus = "pending_review"
	}
	if responseType == ResponseMultipleChoice {
		marking, _ := parseMarkingRubric(rubricRaw)
		result.SelectedOptionID = strings.TrimSpace(selectedOptionID)
		result.CorrectOptionID = marking.CorrectOptionID
		result.Feedback = marking.Feedback
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE learner_attempts
		SET selected_option_id = $1, answer_payload = $2, result_status = $3, status = 'submitted', is_correct = $4,
		    awarded_marks = $5, submitted_at = now()
		WHERE id = $6 AND status = 'open'
	`, nullableSelected(responseType, selectedOptionID), answer, result.ResultStatus, nullableBool(marked, result.IsCorrect), nullableMarks(marked, result.AwardedMarks), attemptID); err != nil {
		return AttemptResult{}, fmt.Errorf("submit learner attempt: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO learner_attempt_events (attempt_id, event_type, actor_id, changed_fields)
		VALUES ($1, 'attempt_submitted', $2,
		        ARRAY['answer','resultStatus','status','isCorrect','awardedMarks','submittedAt'])
	`, attemptID, learnerSubjectID); err != nil {
		return AttemptResult{}, fmt.Errorf("insert learner submission event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return AttemptResult{}, fmt.Errorf("commit learner submission: %w", err)
	}
	return result, nil
}

func nullableSelected(t ResponseType, raw string) any {
	if t == ResponseMultipleChoice {
		return strings.TrimSpace(raw)
	}
	return nil
}
func nullableBool(marked, correct bool) any {
	if !marked {
		return nil
	}
	return correct
}
func nullableMarks(marked bool, marks int) any {
	if !marked {
		return nil
	}
	return marks
}

func markAnswer(t ResponseType, rubricRaw []byte, raw string) (json.RawMessage, bool, bool, error) {
	if t == ResponseMultipleChoice {
		marking, err := parseMarkingRubric(rubricRaw)
		if err != nil {
			return nil, false, false, ErrAttemptNotFound
		}
		selected := strings.TrimSpace(raw)
		if !optionBelongsToRubric(marking, selected) {
			return nil, false, false, ErrInvalidOption
		}
		answer, _ := json.Marshal(map[string]string{"selectedOptionId": selected})
		return answer, true, selected == marking.CorrectOptionID, nil
	}
	fields, ok := exactObject(rubricRaw, learnerRubricFields)
	if !ok {
		return nil, false, false, ErrAttemptNotFound
	}
	if t == ResponseShortAnswer || t == ResponseStructuredResponse || t == ResponseEssay {
		var payload struct {
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(raw), &payload) != nil || strings.TrimSpace(payload.Text) == "" {
			return nil, false, false, ErrInvalidAnswer
		}
		answer, _ := json.Marshal(map[string]string{"text": strings.TrimSpace(payload.Text)})
		return answer, false, false, nil
	}
	answerKey, ok := exactObject(fields["answerKey"], map[string]struct{}{"correctOptionIds": {}, "acceptedAnswers": {}, "numericValue": {}, "tolerance": {}})
	if !ok {
		return nil, false, false, ErrAttemptNotFound
	}
	switch t {
	case ResponseMultiSelect:
		var payload struct {
			IDs []string `json:"selectedOptionIds"`
		}
		if json.Unmarshal([]byte(raw), &payload) != nil || len(payload.IDs) == 0 {
			return nil, false, false, ErrInvalidAnswer
		}
		var correct []string
		if json.Unmarshal(answerKey["correctOptionIds"], &correct) != nil {
			return nil, false, false, ErrAttemptNotFound
		}
		seen := map[string]struct{}{}
		for _, id := range payload.IDs {
			id = strings.TrimSpace(id)
			if id == "" {
				return nil, false, false, ErrInvalidAnswer
			}
			if _, dup := seen[id]; dup {
				return nil, false, false, ErrInvalidAnswer
			}
			seen[id] = struct{}{}
		}
		correctSet := map[string]struct{}{}
		for _, id := range correct {
			correctSet[id] = struct{}{}
		}
		equal := len(seen) == len(correctSet)
		for id := range seen {
			if _, ok := correctSet[id]; !ok {
				equal = false
			}
		}
		out, _ := json.Marshal(map[string][]string{"selectedOptionIds": payload.IDs})
		return out, true, equal, nil
	case ResponseExactShortAnswer:
		var payload struct {
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(raw), &payload) != nil || normalizeAnswer(payload.Text) == "" {
			return nil, false, false, ErrInvalidAnswer
		}
		var accepted []string
		if json.Unmarshal(answerKey["acceptedAnswers"], &accepted) != nil {
			return nil, false, false, ErrAttemptNotFound
		}
		correct := false
		for _, item := range accepted {
			if normalizeAnswer(payload.Text) == normalizeAnswer(item) {
				correct = true
			}
		}
		out, _ := json.Marshal(map[string]string{"text": strings.TrimSpace(payload.Text)})
		return out, true, correct, nil
	case ResponseNumericAnswer:
		var payload struct {
			Value float64 `json:"value"`
		}
		if json.Unmarshal([]byte(raw), &payload) != nil || math.IsNaN(payload.Value) || math.IsInf(payload.Value, 0) {
			return nil, false, false, ErrInvalidAnswer
		}
		var value, tolerance float64
		if json.Unmarshal(answerKey["numericValue"], &value) != nil || json.Unmarshal(answerKey["tolerance"], &tolerance) != nil {
			return nil, false, false, ErrAttemptNotFound
		}
		out, _ := json.Marshal(map[string]float64{"value": payload.Value})
		return out, true, math.Abs(payload.Value-value) <= tolerance, nil
	}
	return nil, false, false, ErrInvalidAnswer
}
