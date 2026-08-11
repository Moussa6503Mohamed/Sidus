package learner

import (
	"encoding/json"
	"strings"
	"testing"
)

// These integration tests run against the same disposable, migrated Postgres instance as
// postgres_store_test.go (see that file's fixture-pattern comment). They exercise
// PostgresStore's assessment-session methods directly (sessions.go), which handlers_test.go's
// in-memory fakes cannot cover: real transactions, real DB triggers, and real optimistic
// concurrency. Skipped unless TEST_DATABASE_URL is set.

// newSessionFixture seeds one eligible multiple_choice question (correct answer opt-b, 2 marks)
// on a syllabus/node dedicated to this fixture, ready for CreateSession to pin.
//
// CreateSession's ListQuestions(f.syllabusID) picks its item(s) oldest-created-first across the
// WHOLE syllabus, and pins each through CreateAttempt's stricter schema check (which, unlike
// ListQuestions' eligibility gate, also requires a complete answerKey/feedback shape).
// newFixture's f.syllabusID is the single real "0610" syllabus every fixture in this package
// shares and never cleans up, so by the time this file's tests run after the rest of the
// package, that syllabus already carries earlier tests' minimal-rubric questions (e.g. the
// GetQuestion fixtures, which only need a bare rubric). CreateSession would then grab one of
// those, fail CreateAttempt's schema check, and return ErrNotFound instead of running this
// file's own scenario. A syllabus seeded fresh for this fixture alone has no such leakage.
func newSessionFixture(t *testing.T) (fixture, string) {
	t.Helper()
	f := newFixture(t)
	f.syllabusID = seedActiveTestSyllabus(t, f.db)
	f.sourceID = seedApprovedLinkedSource(t, f.db, f.syllabusID)
	f.nodeID = seedNode(t, f.db, f.syllabusID, f.sourceID, "verified")
	questionID, _ := seedPracticeQuestion(t, f)
	return f, questionID
}

func createPracticeSession(t *testing.T, f fixture, subject string) Session {
	t.Helper()
	s, err := f.store.CreateSession(f.ctx, subject, CreateSessionInput{
		Mode:          "practice",
		SyllabusID:    f.syllabusID,
		QuestionCount: 1,
	})
	if err != nil {
		t.Fatalf("CreateSession(practice): %v", err)
	}
	if len(s.Items) != 1 {
		t.Fatalf("session items = %d, want 1", len(s.Items))
	}
	return s
}

func createExamSession(t *testing.T, f fixture, subject string, durationSeconds int) Session {
	t.Helper()
	s, err := f.store.CreateSession(f.ctx, subject, CreateSessionInput{
		Mode:            "exam",
		SyllabusID:      f.syllabusID,
		QuestionCount:   1,
		DurationSeconds: durationSeconds,
	})
	if err != nil {
		t.Fatalf("CreateSession(exam): %v", err)
	}
	return s
}

func correctMCQAnswer() json.RawMessage {
	return json.RawMessage(`{"selectedOptionId":"opt-b"}`)
}

// --- Owner isolation ---

func TestPostgresStore_Integration_Session_OwnerIsolation(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")

	if _, err := f.store.GetSession(f.ctx, "owner-b", s.ID); err != ErrSessionNotFound {
		t.Fatalf("foreign GetSession err = %v, want ErrSessionNotFound", err)
	}
	if _, err := f.store.SaveSessionResponse(f.ctx, "owner-b", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: correctMCQAnswer(),
	}); err != ErrSessionNotFound {
		t.Fatalf("foreign SaveSessionResponse err = %v, want ErrSessionNotFound", err)
	}
	if _, err := f.store.SubmitSession(f.ctx, "owner-b", s.ID); err != ErrSessionNotFound {
		t.Fatalf("foreign SubmitSession err = %v, want ErrSessionNotFound", err)
	}
	if _, err := f.store.GetSessionResult(f.ctx, "owner-b", s.ID); err != ErrSessionNotFound {
		t.Fatalf("foreign GetSessionResult err = %v, want ErrSessionNotFound", err)
	}

	// The legitimate owner is unaffected by the foreign attempts above.
	got, err := f.store.GetSession(f.ctx, "owner-a", s.ID)
	if err != nil || got.ID != s.ID {
		t.Fatalf("owner GetSession = %+v, %v", got, err)
	}
}

func TestPostgresStore_Integration_Session_UnknownOrMalformedID_NotFound(t *testing.T) {
	f, _ := newSessionFixture(t)
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		if _, err := f.store.GetSession(f.ctx, "owner-a", id); err != ErrSessionNotFound {
			t.Fatalf("GetSession(%s) err = %v, want ErrSessionNotFound", id, err)
		}
	}
}

// --- Server deadline enforcement ---

func TestPostgresStore_Integration_Session_DeadlineEnforcement(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createExamSession(t, f, "owner-a", 60)

	if _, err := f.db.Exec(`UPDATE assessment_sessions SET deadline_at = now() - interval '1 second' WHERE id = $1`, s.ID); err != nil {
		t.Fatalf("force expiry: %v", err)
	}

	if _, err := f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: correctMCQAnswer(),
	}); err != ErrSessionExpired {
		t.Fatalf("autosave after deadline err = %v, want ErrSessionExpired", err)
	}

	if _, err := f.store.SubmitSession(f.ctx, "owner-a", s.ID); err != ErrSessionExpired {
		t.Fatalf("submit after deadline err = %v, want ErrSessionExpired", err)
	}

	result, err := f.store.GetSessionResult(f.ctx, "owner-a", s.ID)
	if err != nil {
		t.Fatalf("GetSessionResult after expiry: %v", err)
	}
	if result.Status != "expired" {
		t.Fatalf("status = %q, want expired", result.Status)
	}
	if len(result.Results) != 0 {
		t.Fatalf("expired session awarded marks with no saved answer: %+v", result.Results)
	}

	// A second submit on an already-expired (not open, not submitted) session is a hard close,
	// not a silent retry-success.
	if _, err := f.store.SubmitSession(f.ctx, "owner-a", s.ID); err != ErrSessionClosed {
		t.Fatalf("resubmit expired err = %v, want ErrSessionClosed", err)
	}
}

func TestPostgresStore_Integration_Session_PracticeModeHasNoDeadline(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")
	if s.DeadlineAt != nil {
		t.Fatalf("practice session deadline = %v, want nil", s.DeadlineAt)
	}
	if _, err := f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: correctMCQAnswer(),
	}); err != nil {
		t.Fatalf("practice autosave should never expire: %v", err)
	}
}

// --- Duplicate final submit idempotency ---

func TestPostgresStore_Integration_Session_DuplicateFinalSubmitIdempotent(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")
	if _, err := f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: correctMCQAnswer(),
	}); err != nil {
		t.Fatalf("autosave: %v", err)
	}

	first, err := f.store.SubmitSession(f.ctx, "owner-a", s.ID)
	if err != nil {
		t.Fatalf("first SubmitSession: %v", err)
	}
	if len(first.Results) != 1 || !first.Results[0].IsCorrect {
		t.Fatalf("first submit result = %+v", first.Results)
	}

	second, err := f.store.SubmitSession(f.ctx, "owner-a", s.ID)
	if err != nil {
		t.Fatalf("duplicate SubmitSession: %v", err)
	}
	if second.Status != "submitted" || len(second.Results) != 1 {
		t.Fatalf("duplicate submit result = %+v", second)
	}
	if second.Results[0].AttemptID != first.Results[0].AttemptID || second.Results[0].AwardedMarks != first.Results[0].AwardedMarks {
		t.Fatalf("duplicate submit re-marked: first=%+v second=%+v", first.Results[0], second.Results[0])
	}

	// The underlying attempt was marked exactly once: one attempt_created + one attempt_submitted
	// event, never two, proving the duplicate request never re-entered submitAttemptTx.
	var eventCount int
	if err := f.db.QueryRow(`SELECT count(*) FROM learner_attempt_events WHERE attempt_id = $1`, first.Results[0].AttemptID).Scan(&eventCount); err != nil {
		t.Fatalf("count attempt events: %v", err)
	}
	if eventCount != 2 {
		t.Fatalf("attempt events = %d, want 2 (created+submitted, no re-mark)", eventCount)
	}

	var submittedCount int
	if err := f.db.QueryRow(`SELECT count(*) FROM assessment_session_events WHERE session_id = $1 AND event_type = 'session_submitted'`, s.ID).Scan(&submittedCount); err != nil {
		t.Fatalf("count session_submitted events: %v", err)
	}
	if submittedCount != 1 {
		t.Fatalf("session_submitted events = %d, want 1 (duplicate must not re-fire)", submittedCount)
	}
}

func TestPostgresStore_Integration_Session_SubmitSkipsUnansweredItems(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")

	result, err := f.store.SubmitSession(f.ctx, "owner-a", s.ID)
	if err != nil {
		t.Fatalf("SubmitSession with no answers: %v", err)
	}
	if result.Status != "submitted" {
		t.Fatalf("status = %q, want submitted", result.Status)
	}
	if len(result.Results) != 0 {
		t.Fatalf("unanswered item produced a result: %+v", result.Results)
	}
}

// --- Immutable item snapshot / pins ---

func TestPostgresStore_Integration_Session_ItemSnapshotImmutableAtDBLevel(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")

	_, err := f.db.Exec(`UPDATE assessment_session_items SET snapshot = '{}' WHERE session_id = $1 AND ordinal = 1`, s.ID)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("snapshot mutation err = %v, want immutable trigger rejection", err)
	}

	_, err = f.db.Exec(`UPDATE assessment_session_items SET ordinal = 2 WHERE session_id = $1 AND ordinal = 1`, s.ID)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("ordinal mutation err = %v, want immutable trigger rejection", err)
	}
}

func TestPostgresStore_Integration_Session_SnapshotPinnedAgainstLaterQuestionEdits(t *testing.T) {
	f, questionID := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")
	originalPrompt := s.Items[0].Question.Prompt

	if _, err := f.db.Exec(`UPDATE questions SET prompt = 'opaque-edited-after-pin', content_revision = 99 WHERE id = $1`, questionID); err != nil {
		t.Fatalf("edit underlying question: %v", err)
	}

	got, err := f.store.GetSession(f.ctx, "owner-a", s.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Items[0].Question.Prompt != originalPrompt {
		t.Fatalf("pinned snapshot prompt = %q, want unchanged %q", got.Items[0].Question.Prompt, originalPrompt)
	}
	if got.Items[0].Question.Prompt == "opaque-edited-after-pin" {
		t.Fatal("snapshot leaked a post-pin edit to the underlying question")
	}
}

// --- Autosave revision conflict / idempotency ---

func TestPostgresStore_Integration_Session_AutosaveRevisionConflictAndProgression(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")
	if s.Items[0].ResponseVersion != 0 {
		t.Fatalf("initial response version = %d, want 0", s.Items[0].ResponseVersion)
	}

	item, err := f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: json.RawMessage(`{"selectedOptionId":"opt-a"}`),
	})
	if err != nil {
		t.Fatalf("first autosave: %v", err)
	}
	if item.ResponseVersion != 1 {
		t.Fatalf("response version after first save = %d, want 1", item.ResponseVersion)
	}

	// A stale/duplicate autosave carrying the version the client already superseded is rejected,
	// not silently applied or silently ignored — the caller must re-fetch and retry.
	if _, err := f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: correctMCQAnswer(),
	}); err != ErrSessionConflict {
		t.Fatalf("stale autosave err = %v, want ErrSessionConflict", err)
	}

	// Retrying with the now-current version succeeds and advances again.
	item, err = f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 1, Answer: correctMCQAnswer(),
	})
	if err != nil {
		t.Fatalf("second autosave: %v", err)
	}
	if item.ResponseVersion != 2 {
		t.Fatalf("response version after second save = %d, want 2", item.ResponseVersion)
	}

	got, err := f.store.GetSession(f.ctx, "owner-a", s.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Version <= s.Version {
		t.Fatalf("session version = %d, want > initial %d", got.Version, s.Version)
	}
}

func TestPostgresStore_Integration_Session_SaveResponseRejectsClosedSession(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")
	if _, err := f.store.SubmitSession(f.ctx, "owner-a", s.ID); err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}
	if _, err := f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: correctMCQAnswer(),
	}); err != ErrSessionClosed {
		t.Fatalf("autosave after submit err = %v, want ErrSessionClosed", err)
	}
}

// --- Multiple-choice marking regression (session answers are stored as {"selectedOptionId":...},
// but submitAttemptTx's multiple_choice path expects a bare option-ID string, matching the
// direct-submit handler's contract. Before sessions.go unwrapped it via sessionSubmitValue, every
// multiple_choice session submission compared the whole JSON object against option IDs and never
// matched, so no MCQ session/Exam submission could ever be marked correct.) ---

func TestPostgresStore_Integration_Session_SubmitMarksMultipleChoiceCorrectly(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")
	if _, err := f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: correctMCQAnswer(),
	}); err != nil {
		t.Fatalf("autosave: %v", err)
	}

	result, err := f.store.SubmitSession(f.ctx, "owner-a", s.ID)
	if err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results = %+v, want 1", result.Results)
	}
	r := result.Results[0]
	if !r.IsCorrect || r.AwardedMarks != 2 || r.SelectedOptionID != "opt-b" || r.CorrectOptionID != "opt-b" {
		t.Fatalf("result = %+v, want correct opt-b marked for 2 marks", r)
	}
}

func TestPostgresStore_Integration_Session_SubmitMarksMultipleChoiceIncorrect(t *testing.T) {
	f, _ := newSessionFixture(t)
	s := createPracticeSession(t, f, "owner-a")
	if _, err := f.store.SaveSessionResponse(f.ctx, "owner-a", s.ID, SaveResponseInput{
		Ordinal: 1, ExpectedVersion: 0, Answer: json.RawMessage(`{"selectedOptionId":"opt-a"}`),
	}); err != nil {
		t.Fatalf("autosave: %v", err)
	}

	result, err := f.store.SubmitSession(f.ctx, "owner-a", s.ID)
	if err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}
	r := result.Results[0]
	if r.IsCorrect || r.AwardedMarks != 0 || r.SelectedOptionID != "opt-a" || r.CorrectOptionID != "opt-b" {
		t.Fatalf("result = %+v, want incorrect opt-a marked for 0 marks", r)
	}
}

// --- One open exam per learner ---

func TestPostgresStore_Integration_Session_OneOpenExamPerLearner(t *testing.T) {
	f, _ := newSessionFixture(t)
	first := createExamSession(t, f, "owner-a", 3600)

	if _, err := f.store.CreateSession(f.ctx, "owner-a", CreateSessionInput{
		Mode: "exam", SyllabusID: f.syllabusID, QuestionCount: 1, DurationSeconds: 3600,
	}); err != ErrSessionConflict {
		t.Fatalf("second concurrent exam err = %v, want ErrSessionConflict", err)
	}

	// A different learner is never blocked by another learner's open exam.
	if _, err := f.store.CreateSession(f.ctx, "owner-b", CreateSessionInput{
		Mode: "exam", SyllabusID: f.syllabusID, QuestionCount: 1, DurationSeconds: 3600,
	}); err != nil {
		t.Fatalf("other learner's exam err = %v, want nil", err)
	}

	// Once the first exam is closed, the learner may open a new one.
	if _, err := f.store.SubmitSession(f.ctx, "owner-a", first.ID); err != nil {
		t.Fatalf("submit first exam: %v", err)
	}
	if _, err := f.store.CreateSession(f.ctx, "owner-a", CreateSessionInput{
		Mode: "exam", SyllabusID: f.syllabusID, QuestionCount: 1, DurationSeconds: 3600,
	}); err != nil {
		t.Fatalf("exam after prior closed err = %v, want nil", err)
	}
}

func TestPostgresStore_Integration_Session_CreateSessionRejectsInvalidInput(t *testing.T) {
	f, _ := newSessionFixture(t)
	cases := map[string]CreateSessionInput{
		"bad mode":              {Mode: "quiz", SyllabusID: f.syllabusID, QuestionCount: 1},
		"zero count":            {Mode: "practice", SyllabusID: f.syllabusID, QuestionCount: 0},
		"empty syllabus":        {Mode: "practice", SyllabusID: "", QuestionCount: 1},
		"exam duration too low": {Mode: "exam", SyllabusID: f.syllabusID, QuestionCount: 1, DurationSeconds: 1},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := f.store.CreateSession(f.ctx, "owner-a", in); err != ErrInvalidAnswer {
				t.Fatalf("err = %v, want ErrInvalidAnswer", err)
			}
		})
	}
}

func TestPostgresStore_Integration_Session_CreateSessionNoEligibleQuestions(t *testing.T) {
	f := newFixture(t)
	// f.syllabusID is the shared real "0610" syllabus other tests in this run seed questions
	// into; an empty, freshly seeded syllabus is required to actually have zero eligible
	// questions, matching TestPostgresStore_Integration_ListQuestions_EmptySyllabus_ReturnsEmptyList.
	empty := seedActiveTestSyllabus(t, f.db)
	if _, err := f.store.CreateSession(f.ctx, "owner-a", CreateSessionInput{
		Mode: "practice", SyllabusID: empty, QuestionCount: 1,
	}); err != ErrNoQuestions {
		t.Fatalf("err = %v, want ErrNoQuestions", err)
	}
}
