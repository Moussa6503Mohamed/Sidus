package learner

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Session is learner-safe assessment state. Snapshot carries only Projection fields.
type Session struct {
	ID                  string        `json:"id"`
	Mode                string        `json:"mode"`
	SyllabusID          string        `json:"syllabusId"`
	CurriculumMapNodeID *string       `json:"curriculumMapNodeId"`
	Status              string        `json:"status"`
	StartedAt           time.Time     `json:"startedAt"`
	DeadlineAt          *time.Time    `json:"deadlineAt"`
	Version             int           `json:"version"`
	Items               []SessionItem `json:"items"`
}

type SessionItem struct {
	Ordinal         int             `json:"ordinal"`
	Question        Projection      `json:"question"`
	Answer          json.RawMessage `json:"answer,omitempty"`
	ResponseVersion int             `json:"responseVersion"`
}

type SessionResult struct {
	Session
	Results []AttemptResult `json:"results"`
}

type CreateSessionInput struct {
	Mode                string  `json:"mode"`
	SyllabusID          string  `json:"syllabusId"`
	CurriculumMapNodeID *string `json:"curriculumMapNodeId"`
	QuestionCount       int     `json:"questionCount"`
	DurationSeconds     int     `json:"durationSeconds"`
}

type SaveResponseInput struct {
	Ordinal         int             `json:"ordinal"`
	ExpectedVersion int             `json:"expectedVersion"`
	Answer          json.RawMessage `json:"answer"`
}

var (
	ErrSessionNotFound = errors.New("assessment session not found")
	ErrSessionClosed   = errors.New("assessment session is closed")
	ErrSessionConflict = errors.New("assessment session conflict")
	ErrSessionExpired  = errors.New("assessment session deadline elapsed")
	ErrNoQuestions     = errors.New("no eligible questions")
)

// SessionStore is deliberately separate from Store, so legacy delivery fakes cannot accidentally
// acquire write capabilities. Core mounts session routes only for implementations supporting it.
type SessionStore interface {
	CreateSession(context.Context, string, CreateSessionInput) (Session, error)
	GetSession(context.Context, string, string) (Session, error)
	SaveSessionResponse(context.Context, string, string, SaveResponseInput) (SessionItem, error)
	SubmitSession(context.Context, string, string) (SessionResult, error)
	GetSessionResult(context.Context, string, string) (SessionResult, error)
}

func sessionProjection(raw []byte) (Projection, error) {
	var p Projection
	if err := json.Unmarshal(raw, &p); err != nil || p.ID == "" || p.SyllabusID == "" || p.Prompt == "" {
		return Projection{}, ErrSessionNotFound
	}
	return p, nil
}

func (p *PostgresStore) CreateSession(ctx context.Context, subject string, in CreateSessionInput) (Session, error) {
	if in.Mode != "practice" && in.Mode != "exam" {
		return Session{}, ErrInvalidAnswer
	}
	if in.QuestionCount < 1 || in.QuestionCount > 500 || strings.TrimSpace(in.SyllabusID) == "" {
		return Session{}, ErrInvalidAnswer
	}
	if in.Mode == "exam" && (in.DurationSeconds < 60 || in.DurationSeconds > 4*60*60) {
		return Session{}, ErrInvalidAnswer
	}
	questions, err := p.ListQuestions(ctx, in.SyllabusID, in.CurriculumMapNodeID)
	if err != nil {
		return Session{}, err
	}
	if len(questions) == 0 {
		return Session{}, ErrNoQuestions
	}
	if in.QuestionCount > len(questions) {
		in.QuestionCount = len(questions)
	}
	questions = questions[:in.QuestionCount]

	// Pin through existing attempt creation before exposing a session. Every pin re-runs eligibility.
	attempts := make([]Attempt, 0, len(questions))
	for _, q := range questions {
		a, err := p.CreateAttempt(ctx, subject, q.ID)
		if err != nil {
			return Session{}, err
		}
		attempts = append(attempts, a)
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, fmt.Errorf("begin session: %w", err)
	}
	defer tx.Rollback()
	var deadline any
	if in.Mode == "exam" {
		deadline = time.Now().UTC().Add(time.Duration(in.DurationSeconds) * time.Second)
	}
	var session Session
	err = tx.QueryRowContext(ctx, `INSERT INTO assessment_sessions (learner_subject_id,mode,syllabus_id,curriculum_map_node_id,deadline_at) VALUES ($1,$2,$3,$4,$5) RETURNING id,mode,syllabus_id,curriculum_map_node_id,status,started_at,deadline_at,version`, subject, in.Mode, in.SyllabusID, in.CurriculumMapNodeID, deadline).Scan(&session.ID, &session.Mode, &session.SyllabusID, &session.CurriculumMapNodeID, &session.Status, &session.StartedAt, &session.DeadlineAt, &session.Version)
	if err != nil {
		if isUniqueViolation(err) {
			return Session{}, ErrSessionConflict
		}
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	for i, q := range questions {
		snapshot, _ := json.Marshal(q)
		if _, err := tx.ExecContext(ctx, `INSERT INTO assessment_session_items(session_id,ordinal,question_id,attempt_id,snapshot) VALUES($1,$2,$3,$4,$5)`, session.ID, i+1, q.ID, attempts[i].AttemptID, snapshot); err != nil {
			return Session{}, fmt.Errorf("insert session item: %w", err)
		}
		session.Items = append(session.Items, SessionItem{Ordinal: i + 1, Question: q})
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO assessment_session_events(session_id,event_type,actor_id,changed_fields) VALUES($1,'session_created',$2,ARRAY['status','deadlineAt'])`, session.ID, subject); err != nil {
		return Session{}, fmt.Errorf("session event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Session{}, fmt.Errorf("commit session: %w", err)
	}
	return session, nil
}

func isUniqueViolation(err error) bool { return strings.Contains(err.Error(), "duplicate key") }

func (p *PostgresStore) loadSession(ctx context.Context, subject, id string, lock bool) (Session, error) {
	q := `SELECT id,mode,syllabus_id,curriculum_map_node_id,status,started_at,deadline_at,version FROM assessment_sessions WHERE id=$1 AND learner_subject_id=$2`
	if lock {
		q += ` FOR UPDATE`
	}
	var s Session
	err := p.db.QueryRowContext(ctx, q, id, subject).Scan(&s.ID, &s.Mode, &s.SyllabusID, &s.CurriculumMapNodeID, &s.Status, &s.StartedAt, &s.DeadlineAt, &s.Version)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("load session: %w", err)
	}
	return s, nil
}

func (p *PostgresStore) GetSession(ctx context.Context, subject, id string) (Session, error) {
	s, err := p.loadSession(ctx, subject, id, false)
	if err != nil {
		return Session{}, err
	}
	rows, err := p.db.QueryContext(ctx, `SELECT ordinal,snapshot,response_payload,response_version FROM assessment_session_items WHERE session_id=$1 ORDER BY ordinal`, id)
	if err != nil {
		return Session{}, fmt.Errorf("list session items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item SessionItem
		var raw, answer []byte
		if err := rows.Scan(&item.Ordinal, &raw, &answer, &item.ResponseVersion); err != nil {
			return Session{}, fmt.Errorf("scan session item: %w", err)
		}
		item.Answer = json.RawMessage(answer)
		item.Question, err = sessionProjection(raw)
		if err != nil {
			return Session{}, err
		}
		s.Items = append(s.Items, item)
	}
	return s, rows.Err()
}

func validSessionAnswer(q Projection, answer json.RawMessage) bool {
	// Reuse the exact parser and response validator used by individual attempts.
	if len(answer) == 0 || len(answer) > 4096 {
		return false
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(answer, &obj) != nil || len(obj) != 1 {
		return false
	}
	var raw string
	for key, v := range obj {
		switch q.ResponseType {
		case ResponseMultipleChoice:
			if key != "selectedOptionId" {
				return false
			}
			var s string
			if json.Unmarshal(v, &s) != nil {
				return false
			}
			raw = s
		case ResponseMultiSelect:
			if key != "selectedOptionIds" {
				return false
			}
			raw = string(answer)
		case ResponseNumericAnswer:
			if key != "value" {
				return false
			}
			raw = string(answer)
		default:
			if key != "text" {
				return false
			}
			raw = string(answer)
		}
	}
	// validate only syntactic answer here. Canonical pinned rubric validates on final marking.
	if q.ResponseType == ResponseMultipleChoice {
		return strings.TrimSpace(raw) != ""
	}
	return true
}

// sessionSubmitValue converts a stored session response payload (always the single-key JSON
// object validSessionAnswer accepted, e.g. {"selectedOptionId":"opt-b"}) into the exact form
// submitAttemptTx/markAnswer expects. Every response type except multiple_choice already
// consumes that JSON object as-is; multiple_choice uniquely expects the bare option ID string
// (matching the direct-submit handler's decodeSubmit contract), so it must be unwrapped here or
// every session MCQ answer would fail to match any option.
func sessionSubmitValue(t ResponseType, answer []byte) (string, error) {
	if t != ResponseMultipleChoice {
		return string(answer), nil
	}
	var payload struct {
		SelectedOptionID string `json:"selectedOptionId"`
	}
	if json.Unmarshal(answer, &payload) != nil || strings.TrimSpace(payload.SelectedOptionID) == "" {
		return "", ErrInvalidAnswer
	}
	return strings.TrimSpace(payload.SelectedOptionID), nil
}

func (p *PostgresStore) SaveSessionResponse(ctx context.Context, subject, id string, in SaveResponseInput) (SessionItem, error) {
	if in.Ordinal < 1 || in.ExpectedVersion < 0 {
		return SessionItem{}, ErrInvalidAnswer
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return SessionItem{}, fmt.Errorf("begin save response: %w", err)
	}
	defer tx.Rollback()
	var status string
	var deadline *time.Time
	err = tx.QueryRowContext(ctx, `SELECT status,deadline_at FROM assessment_sessions WHERE id=$1 AND learner_subject_id=$2 FOR UPDATE`, id, subject).Scan(&status, &deadline)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return SessionItem{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionItem{}, fmt.Errorf("lock session: %w", err)
	}
	if status != "open" {
		return SessionItem{}, ErrSessionClosed
	}
	if deadline != nil && !time.Now().UTC().Before(*deadline) {
		return SessionItem{}, ErrSessionExpired
	}
	var item SessionItem
	var raw, existingAnswer []byte
	err = tx.QueryRowContext(ctx, `SELECT ordinal,snapshot,response_payload,response_version FROM assessment_session_items WHERE session_id=$1 AND ordinal=$2 FOR UPDATE`, id, in.Ordinal).Scan(&item.Ordinal, &raw, &existingAnswer, &item.ResponseVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionItem{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionItem{}, fmt.Errorf("lock item: %w", err)
	}
	item.Question, err = sessionProjection(raw)
	if err != nil {
		return SessionItem{}, err
	}
	if item.ResponseVersion != in.ExpectedVersion {
		return SessionItem{}, ErrSessionConflict
	}
	if !validSessionAnswer(item.Question, in.Answer) {
		return SessionItem{}, ErrInvalidAnswer
	}
	var savedAnswer []byte
	err = tx.QueryRowContext(ctx, `UPDATE assessment_session_items SET response_payload=$1,response_version=response_version+1 WHERE session_id=$2 AND ordinal=$3 RETURNING response_payload,response_version`, in.Answer, id, in.Ordinal).Scan(&savedAnswer, &item.ResponseVersion)
	if err != nil {
		return SessionItem{}, fmt.Errorf("save item: %w", err)
	}
	item.Answer = json.RawMessage(savedAnswer)
	if _, err = tx.ExecContext(ctx, `UPDATE assessment_sessions SET version=version+1,updated_at=now() WHERE id=$1`, id); err != nil {
		return SessionItem{}, fmt.Errorf("touch session: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO assessment_session_events(session_id,event_type,actor_id,changed_fields) VALUES($1,'response_saved',$2,ARRAY['answer','responseVersion'])`, id, subject); err != nil {
		return SessionItem{}, fmt.Errorf("save event: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return SessionItem{}, fmt.Errorf("commit save: %w", err)
	}
	return item, nil
}

func (p *PostgresStore) SubmitSession(ctx context.Context, subject, id string) (SessionResult, error) {
	// Lock/close first: duplicate requests get immutable final result, never duplicate marking.
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return SessionResult{}, fmt.Errorf("begin submit session: %w", err)
	}
	defer tx.Rollback()
	var s SessionResult
	var deadline *time.Time
	err = tx.QueryRowContext(ctx, `SELECT id,mode,syllabus_id,curriculum_map_node_id,status,started_at,deadline_at,version FROM assessment_sessions WHERE id=$1 AND learner_subject_id=$2 FOR UPDATE`, id, subject).Scan(&s.ID, &s.Mode, &s.SyllabusID, &s.CurriculumMapNodeID, &s.Status, &s.StartedAt, &deadline, &s.Version)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return SessionResult{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionResult{}, fmt.Errorf("lock submit session: %w", err)
	}
	if s.Status == "submitted" {
		tx.Rollback()
		return p.GetSessionResult(ctx, subject, id)
	}
	if s.Status != "open" {
		return SessionResult{}, ErrSessionClosed
	}
	if deadline != nil && time.Now().UTC().After(*deadline) {
		if _, err = tx.ExecContext(ctx, `UPDATE assessment_sessions SET status='expired',submitted_at=now(),updated_at=now() WHERE id=$1`, id); err != nil {
			return SessionResult{}, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO assessment_session_events(session_id,event_type,actor_id,changed_fields) VALUES($1,'session_expired',$2,ARRAY['status','submittedAt'])`, id, subject); err != nil {
			return SessionResult{}, err
		}
		if err = tx.Commit(); err != nil {
			return SessionResult{}, err
		}
		return SessionResult{}, ErrSessionExpired
	}
	rows, err := tx.QueryContext(ctx, `SELECT i.ordinal,i.attempt_id,i.response_payload,a.response_type FROM assessment_session_items i JOIN learner_attempts a ON a.id=i.attempt_id WHERE i.session_id=$1 ORDER BY i.ordinal`, id)
	if err != nil {
		return SessionResult{}, err
	}
	type pending struct {
		ordinal      int
		attempt      string
		answer       []byte
		responseType ResponseType
	}
	var pendingItems []pending
	for rows.Next() {
		var x pending
		if err := rows.Scan(&x.ordinal, &x.attempt, &x.answer, &x.responseType); err != nil {
			rows.Close()
			return SessionResult{}, err
		}
		pendingItems = append(pendingItems, x)
	}
	rows.Close()
	// Mark every pinned attempt before changing session status. Any failed item rolls back every
	// attempt/result/event mutation, so retry remains safe and finalization is atomic.
	for _, x := range pendingItems {
		if len(x.answer) == 0 {
			continue
		}
		submitValue, err := sessionSubmitValue(x.responseType, x.answer)
		if err != nil {
			return SessionResult{}, err
		}
		result, err := submitAttemptTx(ctx, tx, subject, x.attempt, submitValue)
		if err != nil {
			return SessionResult{}, err
		}
		s.Results = append(s.Results, result)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE assessment_sessions SET status='submitted',submitted_at=now(),updated_at=now(),version=version+1 WHERE id=$1`, id); err != nil {
		return SessionResult{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO assessment_session_events(session_id,event_type,actor_id,changed_fields) VALUES($1,'session_submitted',$2,ARRAY['status','submittedAt'])`, id, subject); err != nil {
		return SessionResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return SessionResult{}, err
	}
	return p.GetSessionResult(ctx, subject, id)
}

func (p *PostgresStore) GetSessionResult(ctx context.Context, subject, id string) (SessionResult, error) {
	s, err := p.GetSession(ctx, subject, id)
	if err != nil {
		return SessionResult{}, err
	}
	if s.Status == "open" {
		return SessionResult{}, ErrSessionClosed
	}
	r := SessionResult{Session: s}
	// Join the pinned canonical rubric so multiple_choice results can carry the same
	// learner-safe correct-option/feedback data submitAttemptTx returns synchronously —
	// otherwise every reconnect/refresh (and SubmitSession's own return value, which is
	// re-derived through this method) would silently drop it.
	rows, err := p.db.QueryContext(ctx, `SELECT a.id,a.question_id,a.selected_option_id,COALESCE(a.is_correct,false),COALESCE(a.awarded_marks,0),a.max_marks,a.response_type,a.answer_payload,COALESCE(a.result_status,'marked'),rv.rubric FROM assessment_session_items i JOIN learner_attempts a ON a.id=i.attempt_id LEFT JOIN question_rubric_versions rv ON rv.id=a.canonical_rubric_version_id AND rv.question_id=a.question_id WHERE i.session_id=$1 AND a.status='submitted' ORDER BY i.ordinal`, id)
	if err != nil {
		return SessionResult{}, fmt.Errorf("list session results: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var x AttemptResult
		var selected sql.NullString
		var rubricRaw []byte
		if err := rows.Scan(&x.AttemptID, &x.QuestionID, &selected, &x.IsCorrect, &x.AwardedMarks, &x.MaxMarks, &x.ResponseType, &x.Answer, &x.ResultStatus, &rubricRaw); err != nil {
			return SessionResult{}, err
		}
		x.SelectedOptionID = selected.String
		if x.ResponseType == ResponseMultipleChoice {
			marking, _ := parseMarkingRubric(rubricRaw)
			x.CorrectOptionID = marking.CorrectOptionID
			x.Feedback = marking.Feedback
		}
		r.Results = append(r.Results, x)
	}
	return r, rows.Err()
}
