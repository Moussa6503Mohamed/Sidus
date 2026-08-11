package learner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubSessionStore is a handler-level fake: each method delegates to a settable func so a test can
// inject any SessionStore error/response without touching PostgresStore or a real database. It
// embeds *memoryStore so Register's `store.(SessionStore)` type assertion still sees a full Store,
// exactly like PostgresStore does in production, while session behavior stays fully test-controlled.
// Deep store logic (deadline math, ownership SQL, autosave conflicts) is already covered against a
// real Postgres in sessions_integration_test.go; this file only proves the HTTP layer (decode,
// routing, error-code mapping) forwards and reacts to SessionStore correctly.
type stubSessionStore struct {
	*memoryStore
	createFn func(context.Context, string, CreateSessionInput) (Session, error)
	getFn    func(context.Context, string, string) (Session, error)
	saveFn   func(context.Context, string, string, SaveResponseInput) (SessionItem, error)
	submitFn func(context.Context, string, string) (SessionResult, error)
	resultFn func(context.Context, string, string) (SessionResult, error)
}

func newStubSessionStore() *stubSessionStore {
	return &stubSessionStore{memoryStore: newMemoryStore()}
}

func (s *stubSessionStore) CreateSession(ctx context.Context, subject string, in CreateSessionInput) (Session, error) {
	if s.createFn == nil {
		return Session{}, ErrSessionNotFound
	}
	return s.createFn(ctx, subject, in)
}

func (s *stubSessionStore) GetSession(ctx context.Context, subject, id string) (Session, error) {
	if s.getFn == nil {
		return Session{}, ErrSessionNotFound
	}
	return s.getFn(ctx, subject, id)
}

func (s *stubSessionStore) SaveSessionResponse(ctx context.Context, subject, id string, in SaveResponseInput) (SessionItem, error) {
	if s.saveFn == nil {
		return SessionItem{}, ErrSessionNotFound
	}
	return s.saveFn(ctx, subject, id, in)
}

func (s *stubSessionStore) SubmitSession(ctx context.Context, subject, id string) (SessionResult, error) {
	if s.submitFn == nil {
		return SessionResult{}, ErrSessionNotFound
	}
	return s.submitFn(ctx, subject, id)
}

func (s *stubSessionStore) GetSessionResult(ctx context.Context, subject, id string) (SessionResult, error) {
	if s.resultFn == nil {
		return SessionResult{}, ErrSessionNotFound
	}
	return s.resultFn(ctx, subject, id)
}

func newSessionTestMux(store *stubSessionStore) http.Handler {
	mux := http.NewServeMux()
	Register(mux, store, fakeVerifier{})
	return mux
}

// assertErrorCode (handlers_test.go) hardcodes 400; session routes also return 404/409, so this
// checks only the error-code body field against whatever status the caller already asserted.
func assertErrorField(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != want {
		t.Fatalf("error = %v, want %v", body["error"], want)
	}
}

func httpGet(t *testing.T, h http.Handler, target, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// --- invalid JSON / unknown fields (create + save share decodeSessionBody) ---

func TestSessionCreate_RejectsMalformedAndUnknownFields(t *testing.T) {
	malformed := []string{
		"",         // empty body
		"not-json", // malformed
		"{",        // truncated
		`{"mode":"practice","syllabusId":"syl-active","questionCount":1,"durationSeconds":0,"extra":1}`,        // unknown field
		`{"mode":"practice","syllabusId":"syl-active","questionCount":1,"durationSeconds":0}{"trailing":true}`, // trailing data
	}
	mux := newSessionTestMux(newStubSessionStore())
	for _, body := range malformed {
		rec := doRequestBody(t, mux, http.MethodPost, "/learner/assessment-sessions", learnerToken, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q = %d, want 400: %s", body, rec.Code, rec.Body.String())
		}
		assertErrorCode(t, rec, "invalid_json")
	}
}

// A well-formed body missing fields decodeSessionBody doesn't itself require (business validation
// is the store's job) must reach the store, not be rejected as invalid_json.
func TestSessionCreate_WellFormedIncompleteBodyReachesStore(t *testing.T) {
	mux := newSessionTestMux(newStubSessionStore()) // createFn unset -> ErrSessionNotFound -> 404
	rec := doRequestBody(t, mux, http.MethodPost, "/learner/assessment-sessions", learnerToken, `{"mode":"practice"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("body reached decode ok, want store call (404 from unset stub), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSessionSave_RejectsMalformedInvalidAndUnknownFields(t *testing.T) {
	mux := newSessionTestMux(newStubSessionStore())
	bodies := []string{
		"",
		"not-json",
		"{",
		`{"ordinal":1,"expectedVersion":0,"answer":{"selectedOptionId":"opt-a"},"extra":1}`,
		`{"ordinal":1,"expectedVersion":0,"answer":{"selectedOptionId":"opt-a"}}{"trailing":true}`,
	}
	for _, body := range bodies {
		rec := doRequestBody(t, mux, http.MethodPatch, "/learner/assessment-sessions/sess-1/responses", learnerToken, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q = %d, want 400: %s", body, rec.Code, rec.Body.String())
		}
		assertErrorCode(t, rec, "invalid_json")
	}
}

// --- foreign owner: store enforces ownership; handler must surface it as an opaque not_found ---

func TestSessionGet_ForeignOwner_NotFound(t *testing.T) {
	store := newStubSessionStore()
	store.getFn = func(_ context.Context, subject, id string) (Session, error) {
		if subject != "user_learner" {
			return Session{}, ErrSessionNotFound
		}
		return Session{ID: id, Mode: "practice", SyllabusID: "syl-active", Status: "open"}, nil
	}
	mux := newSessionTestMux(store)
	rec := httpGet(t, mux, "/learner/assessment-sessions/sess-1", learnerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner get = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	foreign := httpGet(t, mux, "/learner/assessment-sessions/sess-1", editorToken)
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign owner get = %d, want 404", foreign.Code)
	}
	assertErrorField(t, foreign, "not_found")
}

func TestSessionSave_ForeignOwner_NotFound(t *testing.T) {
	store := newStubSessionStore()
	store.saveFn = func(_ context.Context, subject, id string, in SaveResponseInput) (SessionItem, error) {
		if subject != "user_learner" {
			return SessionItem{}, ErrSessionNotFound
		}
		return SessionItem{Ordinal: in.Ordinal, ResponseVersion: in.ExpectedVersion + 1}, nil
	}
	mux := newSessionTestMux(store)
	body := `{"ordinal":1,"expectedVersion":0,"answer":{"selectedOptionId":"opt-a"}}`
	owner := doRequestBody(t, mux, http.MethodPatch, "/learner/assessment-sessions/sess-1/responses", learnerToken, body)
	if owner.Code != http.StatusOK {
		t.Fatalf("owner save = %d, want 200: %s", owner.Code, owner.Body.String())
	}
	foreign := doRequestBody(t, mux, http.MethodPatch, "/learner/assessment-sessions/sess-1/responses", editorToken, body)
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign owner save = %d, want 404", foreign.Code)
	}
	assertErrorField(t, foreign, "not_found")
}

func TestSessionSubmit_ForeignOwner_NotFound(t *testing.T) {
	store := newStubSessionStore()
	store.submitFn = func(_ context.Context, subject, id string) (SessionResult, error) {
		if subject != "user_learner" {
			return SessionResult{}, ErrSessionNotFound
		}
		return SessionResult{Session: Session{ID: id, Status: "submitted"}}, nil
	}
	mux := newSessionTestMux(store)
	foreign := doRequestBody(t, mux, http.MethodPost, "/learner/assessment-sessions/sess-1/submit", editorToken, "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign owner submit = %d, want 404", foreign.Code)
	}
	assertErrorField(t, foreign, "not_found")
}

func TestSessionResult_ForeignOwner_NotFound(t *testing.T) {
	store := newStubSessionStore()
	store.resultFn = func(_ context.Context, subject, id string) (SessionResult, error) {
		if subject != "user_learner" {
			return SessionResult{}, ErrSessionNotFound
		}
		return SessionResult{Session: Session{ID: id, Status: "submitted"}}, nil
	}
	mux := newSessionTestMux(store)
	foreign := httpGet(t, mux, "/learner/assessment-sessions/sess-1/result", editorToken)
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign owner result = %d, want 404", foreign.Code)
	}
	assertErrorField(t, foreign, "not_found")
}

// --- error-code mapping: conflict, expired, closed ---

func TestSessionSave_ConflictMapsTo409(t *testing.T) {
	store := newStubSessionStore()
	store.saveFn = func(context.Context, string, string, SaveResponseInput) (SessionItem, error) {
		return SessionItem{}, ErrSessionConflict
	}
	mux := newSessionTestMux(store)
	rec := doRequestBody(t, mux, http.MethodPatch, "/learner/assessment-sessions/sess-1/responses", learnerToken,
		`{"ordinal":1,"expectedVersion":0,"answer":{"selectedOptionId":"opt-a"}}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("conflict save = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	assertErrorField(t, rec, "session_conflict")
}

func TestSessionSave_ExpiredMapsTo409(t *testing.T) {
	store := newStubSessionStore()
	store.saveFn = func(context.Context, string, string, SaveResponseInput) (SessionItem, error) {
		return SessionItem{}, ErrSessionExpired
	}
	mux := newSessionTestMux(store)
	rec := doRequestBody(t, mux, http.MethodPatch, "/learner/assessment-sessions/sess-1/responses", learnerToken,
		`{"ordinal":1,"expectedVersion":0,"answer":{"selectedOptionId":"opt-a"}}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expired save = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	assertErrorField(t, rec, "session_expired")
}

func TestSessionSubmit_ExpiredMapsTo409(t *testing.T) {
	store := newStubSessionStore()
	store.submitFn = func(context.Context, string, string) (SessionResult, error) {
		return SessionResult{}, ErrSessionExpired
	}
	mux := newSessionTestMux(store)
	rec := doRequestBody(t, mux, http.MethodPost, "/learner/assessment-sessions/sess-1/submit", learnerToken, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expired submit = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	assertErrorField(t, rec, "session_expired")
}

func TestSessionSubmit_ClosedMapsTo409(t *testing.T) {
	store := newStubSessionStore()
	store.submitFn = func(context.Context, string, string) (SessionResult, error) {
		return SessionResult{}, ErrSessionClosed
	}
	mux := newSessionTestMux(store)
	rec := doRequestBody(t, mux, http.MethodPost, "/learner/assessment-sessions/sess-1/submit", learnerToken, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("closed submit = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	assertErrorField(t, rec, "session_closed")
}

func TestSessionCreate_NoQuestionsMapsTo409(t *testing.T) {
	store := newStubSessionStore()
	store.createFn = func(context.Context, string, CreateSessionInput) (Session, error) {
		return Session{}, ErrNoQuestions
	}
	mux := newSessionTestMux(store)
	rec := doRequestBody(t, mux, http.MethodPost, "/learner/assessment-sessions", learnerToken,
		`{"mode":"practice","syllabusId":"syl-active","questionCount":1,"durationSeconds":0}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("no-questions create = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	assertErrorField(t, rec, "no_questions")
}

// --- submit's dedicated empty-body handling (submit is the only session route with no schema:
// its body must be exactly empty, unlike create/save which run through decodeSessionBody) ---

func TestSessionSubmit_NonEmptyBodyRejectedBeforeStoreCall(t *testing.T) {
	store := newStubSessionStore()
	called := false
	store.submitFn = func(context.Context, string, string) (SessionResult, error) {
		called = true
		return SessionResult{Session: Session{ID: "sess-1", Status: "submitted"}}, nil
	}
	mux := newSessionTestMux(store)
	for _, body := range []string{"{}", " x", "null", "   \n\t  x"} {
		rec := doRequestBody(t, mux, http.MethodPost, "/learner/assessment-sessions/sess-1/submit", learnerToken, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q = %d, want 400: %s", body, rec.Code, rec.Body.String())
		}
		assertErrorCode(t, rec, "invalid_json")
	}
	if called {
		t.Fatal("store must not be called when submit body is rejected")
	}
}

func TestSessionSubmit_WhitespaceOnlyBodyAcceptedAndCallsStore(t *testing.T) {
	store := newStubSessionStore()
	called := false
	store.submitFn = func(_ context.Context, subject, id string) (SessionResult, error) {
		called = true
		if subject != "user_learner" || id != "sess-1" {
			t.Fatalf("unexpected subject/id: %s/%s", subject, id)
		}
		return SessionResult{Session: Session{ID: id, Status: "submitted"}}, nil
	}
	mux := newSessionTestMux(store)
	rec := doRequestBody(t, mux, http.MethodPost, "/learner/assessment-sessions/sess-1/submit", learnerToken, "   \n  ")
	if rec.Code != http.StatusOK {
		t.Fatalf("whitespace-only submit = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatal("store must be called for a whitespace-only (effectively empty) submit body")
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] == nil {
		t.Fatalf("missing status in submit result: %s", rec.Body.String())
	}
}
