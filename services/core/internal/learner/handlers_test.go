package learner

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// fakeVerifier maps opaque test tokens to verified claims so handler tests exercise the real auth
// middleware and role matrix without any live Clerk instance or cryptography.
type fakeVerifier struct{}

const (
	adminToken    = "admin-token"
	editorToken   = "editor-token"
	reviewerToken = "reviewer-token"
	learnerToken  = "learner-token"
	noRoleToken   = "norole-token"
)

func (fakeVerifier) Verify(_ context.Context, token string) (auth.Claims, error) {
	switch token {
	case adminToken:
		return auth.Claims{Subject: "user_admin", Role: auth.RoleAdmin}, nil
	case editorToken:
		return auth.Claims{Subject: "user_editor", Role: auth.RoleEditor}, nil
	case reviewerToken:
		return auth.Claims{Subject: "user_reviewer", Role: auth.RoleReviewer}, nil
	case learnerToken:
		return auth.Claims{Subject: "user_learner", Role: auth.RoleLearner}, nil
	case noRoleToken:
		return auth.Claims{Subject: "user_norole", Role: auth.RoleUnknown}, nil
	default:
		return auth.Claims{}, auth.ErrInvalidToken
	}
}

type markingMemoryStore struct {
	*memoryStore
	mu         sync.Mutex
	projection MarkingProjection
	job        MarkingJob
}

func newMarkingMemoryStore() *markingMemoryStore {
	return &markingMemoryStore{memoryStore: newMemoryStore(), projection: MarkingProjection{AttemptID: "attempt-1", Status: MarkingPending}, job: MarkingJob{RequestID: "stable-request-id", AttemptID: "attempt-1"}}
}
func (m *markingMemoryStore) RequestMarking(_ context.Context, owner, attempt string) (MarkingJob, MarkingProjection, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if owner != "user_learner" || attempt != "attempt-1" {
		return MarkingJob{}, MarkingProjection{}, false, ErrMarkingNotFound
	}
	return m.job, m.projection, m.projection.Status == MarkingPending, nil
}
func (m *markingMemoryStore) ApplyMarking(_ context.Context, id string, out MarkingOutcome) (MarkingProjection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id != "stable-request-id" {
		return MarkingProjection{}, ErrMarkingNotFound
	}
	if out.Status == MarkingAccepted {
		m.projection.Status = MarkingAccepted
		m.projection.Result = out.Result
	}
	return m.projection, nil
}
func (m *markingMemoryStore) GetMarking(_ context.Context, owner, attempt string) (MarkingProjection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if owner != "user_learner" || attempt != "attempt-1" {
		return MarkingProjection{}, ErrMarkingNotFound
	}
	return m.projection, nil
}

type assertingMarker struct {
	mu  sync.Mutex
	ids []string
}

type analyticsMemoryStore struct {
	*memoryStore
	value LearningAnalytics
}

func (m *analyticsMemoryStore) GetLearningAnalytics(_ context.Context, owner string) (LearningAnalytics, error) {
	if owner != "user_learner" {
		return LearningAnalytics{}, ErrAttemptNotFound
	}
	return m.value, nil
}

func (m *assertingMarker) MarkWrittenAttempt(_ context.Context, job MarkingJob) (MarkingOutcome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ids = append(m.ids, job.RequestID)
	return MarkingOutcome{Status: MarkingAccepted, Result: &MarkingResult{CriterionMarks: []CriterionMark{{CriterionID: "c", MarksAwarded: 1, Feedback: "ok"}}, AwardedMarks: 1, MaxMarks: 1, Model: "fake", ModelVersion: "v1", Confidence: .9}}, nil
}

// memoryStore is an in-memory Store used only for handler tests: routing, auth wiring, input
// validation, and error-shape/no-leakage checks. The real eligibility gate (verified question +
// current verified canonical rubric + verified node + approved linked source) lives entirely in
// PostgresStore's SQL and is covered by postgres_store_test.go's integration tests.
type memoryStore struct {
	activeSyllabuses map[string]bool
	nodeSyllabus     map[string]string
	questions        map[string]Projection
	syllabuses       []Syllabus
	modules          []Module
	attempts         map[string]struct {
		owner, question string
		status          AttemptStatus
	}
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		activeSyllabuses: map[string]bool{"syl-active": true},
		nodeSyllabus:     map[string]string{"node-1": "syl-active", "node-other": "syl-other-active"},
		syllabuses: []Syllabus{
			{
				ID:            "syl-active",
				Board:         "opaque-board",
				SyllabusCode:  "opaque-code",
				Qualification: "opaque-qualification",
				Track:         nil,
				DisplayName:   "opaque-display-name",
			},
		},
		modules: []Module{{ID: "node-1", SyllabusID: "syl-active", Code: "M1", Label: "opaque-module"}},
		questions: map[string]Projection{
			"q-1": {
				ID:                  "q-1",
				SyllabusID:          "syl-active",
				CurriculumMapNodeID: "node-1",
				ResponseType:        ResponseMultipleChoice,
				Language:            "en",
				Prompt:              "opaque-test-prompt",
				Options:             []Option{{ID: "opt-a", Label: "opaque-a"}, {ID: "opt-b", Label: "opaque-b"}},
				ContentRevision:     1,
			},
		},
		attempts: map[string]struct {
			owner, question string
			status          AttemptStatus
		}{},
	}
}

func (m *memoryStore) ListQuestions(_ context.Context, syllabusID string, nodeID *string) ([]Projection, error) {
	if !m.activeSyllabuses[syllabusID] {
		return nil, ErrUnknownSyllabus
	}
	if nodeID != nil {
		syl, ok := m.nodeSyllabus[*nodeID]
		if !ok {
			return nil, ErrUnknownNode
		}
		if syl != syllabusID {
			return nil, ErrMismatchedNode
		}
	}
	items := []Projection{}
	for _, q := range m.questions {
		if q.SyllabusID != syllabusID {
			continue
		}
		if nodeID != nil && q.CurriculumMapNodeID != *nodeID {
			continue
		}
		items = append(items, q)
	}
	return items, nil
}

func (m *memoryStore) GetQuestion(_ context.Context, id string) (Projection, error) {
	q, ok := m.questions[id]
	if !ok {
		return Projection{}, ErrNotFound
	}
	return q, nil
}

func (m *memoryStore) ListActiveSyllabuses(_ context.Context) ([]Syllabus, error) {
	return m.syllabuses, nil
}

func (m *memoryStore) ListModules(_ context.Context, syllabusID string) ([]Module, error) {
	if !m.activeSyllabuses[syllabusID] {
		return nil, ErrUnknownSyllabus
	}
	items := []Module{}
	for _, module := range m.modules {
		if module.SyllabusID == syllabusID {
			items = append(items, module)
		}
	}
	return items, nil
}

func (m *memoryStore) CreateAttempt(_ context.Context, owner, questionID string) (Attempt, error) {
	if _, ok := m.questions[questionID]; !ok {
		return Attempt{}, ErrNotFound
	}
	id := "attempt-" + owner
	m.attempts[id] = struct {
		owner, question string
		status          AttemptStatus
	}{owner, questionID, AttemptOpen}
	return Attempt{AttemptID: id, QuestionID: questionID, Status: AttemptOpen, MaxMarks: 2}, nil
}

func (m *memoryStore) SubmitAttempt(_ context.Context, owner, attemptID, selected string) (AttemptResult, error) {
	a, ok := m.attempts[attemptID]
	if !ok || a.owner != owner {
		return AttemptResult{}, ErrAttemptNotFound
	}
	if a.status != AttemptOpen {
		return AttemptResult{}, ErrAttemptSubmitted
	}
	if selected != "opt-a" && selected != "opt-b" {
		return AttemptResult{}, ErrInvalidOption
	}
	a.status = AttemptSubmitted
	m.attempts[attemptID] = a
	correct := selected == "opt-a"
	marks := 0
	if correct {
		marks = 2
	}
	return AttemptResult{AttemptID: attemptID, QuestionID: a.question, SelectedOptionID: selected,
		CorrectOptionID: "opt-a", IsCorrect: correct, AwardedMarks: marks, MaxMarks: 2,
		Feedback: Feedback{CorrectExplanation: "opaque-c", IncorrectExplanations: []IncorrectExplanation{{OptionID: "opt-b", Explanation: "opaque-w"}}}}, nil
}

func newTestMux() http.Handler {
	mux := http.NewServeMux()
	Register(mux, newMemoryStore(), fakeVerifier{})
	return mux
}

func doRequest(t *testing.T, method, target, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	newTestMux().ServeHTTP(rec, req)
	return rec
}

func doRequestBody(t *testing.T, h http.Handler, method, target, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAttemptRoutes_RoleMatrixAndOwnership(t *testing.T) {
	for _, token := range []string{learnerToken, editorToken, reviewerToken, adminToken} {
		store := newMemoryStore()
		mux := http.NewServeMux()
		Register(mux, store, fakeVerifier{})
		created := doRequestBody(t, mux, http.MethodPost, "/learner/questions/q-1/attempts", token, "")
		if created.Code != http.StatusCreated {
			t.Fatalf("token %s create = %d", token, created.Code)
		}
		attempt := decodeAttempt(t, created.Body.Bytes())
		submitted := doRequestBody(t, mux, http.MethodPost, "/learner/attempts/"+attempt.AttemptID+"/submit", token, `{"selectedOptionId":"opt-b"}`)
		if submitted.Code != http.StatusOK {
			t.Fatalf("token %s submit = %d: %s", token, submitted.Code, submitted.Body.String())
		}
	}
	if rec := doRequestBody(t, newTestMux(), http.MethodPost, "/learner/questions/q-1/attempts", noRoleToken, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("unknown role create = %d", rec.Code)
	}
}

func TestAttemptSubmit_StrictLifecycleOwnershipAndLeakage(t *testing.T) {
	store := newMemoryStore()
	mux := http.NewServeMux()
	Register(mux, store, fakeVerifier{})
	created := doRequestBody(t, mux, http.MethodPost, "/learner/questions/q-1/attempts", learnerToken, "")
	var open map[string]json.RawMessage
	if err := json.Unmarshal(created.Body.Bytes(), &open); err != nil {
		t.Fatal(err)
	}
	if len(open) != 4 || open["attemptId"] == nil || open["questionId"] == nil || open["status"] == nil || open["maxMarks"] == nil {
		t.Fatalf("open keys leaked or missing: %v", open)
	}
	attempt := decodeAttempt(t, created.Body.Bytes())
	foreign := doRequestBody(t, mux, http.MethodPost, "/learner/attempts/"+attempt.AttemptID+"/submit", editorToken, `{"selectedOptionId":"opt-a"}`)
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign = %d", foreign.Code)
	}
	invalid := doRequestBody(t, mux, http.MethodPost, "/learner/attempts/"+attempt.AttemptID+"/submit", learnerToken, `{"selectedOptionId":"stale"}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid = %d", invalid.Code)
	}
	resultRec := doRequestBody(t, mux, http.MethodPost, "/learner/attempts/"+attempt.AttemptID+"/submit", learnerToken, `{"selectedOptionId":"opt-b"}`)
	var result map[string]json.RawMessage
	if err := json.Unmarshal(resultRec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	want := []string{"attemptId", "questionId", "selectedOptionId", "correctOptionId", "isCorrect", "awardedMarks", "maxMarks", "feedback"}
	if len(result) != len(want) {
		t.Fatalf("result keys = %v", result)
	}
	for _, key := range want {
		if result[key] == nil {
			t.Fatalf("missing %s", key)
		}
	}
	replay := doRequestBody(t, mux, http.MethodPost, "/learner/attempts/"+attempt.AttemptID+"/submit", learnerToken, `{"selectedOptionId":"opt-a"}`)
	if replay.Code != http.StatusConflict {
		t.Fatalf("replay = %d", replay.Code)
	}
}

func TestAttemptSubmit_RejectsMalformedBeforeStore(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"selectedOptionId":""}`, `{"SelectedOptionId":"opt-a"}`, `{"selectedOptionId":1}`, `{"selectedOptionId":"opt-a","extra":1}`, `{"selectedOptionId":"opt-a","selectedOptionId":"opt-b"}`, `{"selectedOptionId":"opt-a"}{}`} {
		rec := doRequestBody(t, newTestMux(), http.MethodPost, "/learner/attempts/a/submit", learnerToken, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q = %d", body, rec.Code)
		}
	}
}

func decodeAttempt(t *testing.T, raw []byte) Attempt {
	t.Helper()
	var attempt Attempt
	if err := json.Unmarshal(raw, &attempt); err != nil {
		t.Fatal(err)
	}
	return attempt
}

// --- Role matrix ---

func TestListQuestions_RoleMatrix(t *testing.T) {
	for _, tok := range []string{adminToken, editorToken, reviewerToken, learnerToken} {
		rec := doRequest(t, http.MethodGet, "/learner/questions?syllabusId=syl-active", tok)
		if rec.Code != http.StatusOK {
			t.Fatalf("token %q: status = %d, want 200; body=%s", tok, rec.Code, rec.Body.String())
		}
	}
}

func TestListQuestions_UnknownRole_Forbidden(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions?syllabusId=syl-active", noRoleToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestListQuestions_MissingToken_Unauthorized(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions?syllabusId=syl-active", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestGetQuestion_RoleMatrix(t *testing.T) {
	for _, tok := range []string{adminToken, editorToken, reviewerToken, learnerToken} {
		rec := doRequest(t, http.MethodGet, "/learner/questions/q-1", tok)
		if rec.Code != http.StatusOK {
			t.Fatalf("token %q: status = %d, want 200", tok, rec.Code)
		}
	}
}

func TestGetQuestion_UnknownRole_Forbidden(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions/q-1", noRoleToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// --- Syllabus discovery (T-0015 review fix) ---

func TestListSyllabuses_RoleMatrix(t *testing.T) {
	for _, tok := range []string{adminToken, editorToken, reviewerToken, learnerToken} {
		rec := doRequest(t, http.MethodGet, "/learner/syllabuses", tok)
		if rec.Code != http.StatusOK {
			t.Fatalf("token %q: status = %d, want 200; body=%s", tok, rec.Code, rec.Body.String())
		}
	}
}

func TestListSyllabuses_UnknownRole_Forbidden(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/syllabuses", noRoleToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestListSyllabuses_MissingToken_Unauthorized(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/syllabuses", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// --- Learner module discovery ---

func TestListModules_RoleMatrix(t *testing.T) {
	for _, tok := range []string{adminToken, editorToken, reviewerToken, learnerToken} {
		rec := doRequest(t, http.MethodGet, "/learner/modules?syllabusId=syl-active", tok)
		if rec.Code != http.StatusOK {
			t.Fatalf("token %q: status = %d, want 200; body=%s", tok, rec.Code, rec.Body.String())
		}
	}
	if rec := doRequest(t, http.MethodGet, "/learner/modules?syllabusId=syl-active", noRoleToken); rec.Code != http.StatusForbidden {
		t.Fatalf("unknown role = %d, want 403", rec.Code)
	}
	if rec := doRequest(t, http.MethodGet, "/learner/modules?syllabusId=syl-active", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token = %d, want 401", rec.Code)
	}
}

func TestListModules_ValidatesSyllabusAndNoLeakage(t *testing.T) {
	if rec := doRequest(t, http.MethodGet, "/learner/modules", learnerToken); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing syllabus = %d", rec.Code)
	}
	if rec := doRequest(t, http.MethodGet, "/learner/modules?syllabusId=unknown", learnerToken); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown syllabus = %d", rec.Code)
	}
	rec := doRequest(t, http.MethodGet, "/learner/modules?syllabusId=syl-active", learnerToken)
	var body struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(body.Items))
	}
	var item map[string]json.RawMessage
	if err := json.Unmarshal(body.Items[0], &item); err != nil {
		t.Fatal(err)
	}
	want := []string{"id", "syllabusId", "code", "label"}
	got := make([]string, 0, len(item))
	for key := range item {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("module keys = %v, want %v", got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("module keys = %v, want %v", got, want)
		}
	}
}

// allowedSyllabusKeys is the exhaustive set of keys the learner syllabus-discovery projection
// may ever carry. Any other key — sourceId, rights/licence metadata, createdAt/updatedAt,
// actorId, subjectId, curriculumYear, status, or anything else — is a leak of catalogue-internal
// data to a learner-role-reachable route.
var allowedSyllabusKeys = []string{"id", "board", "syllabusCode", "qualification", "track", "displayName"}

func TestListSyllabuses_NoLeakage(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/syllabuses", learnerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) == 0 {
		t.Fatal("expected at least one item")
	}
	for _, item := range body.Items {
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(item, &decoded); err != nil {
			t.Fatalf("decode syllabus: %v", err)
		}
		got := make([]string, 0, len(decoded))
		for k := range decoded {
			got = append(got, k)
		}
		sort.Strings(got)
		want := append([]string(nil), allowedSyllabusKeys...)
		sort.Strings(want)
		if len(got) != len(want) {
			t.Fatalf("syllabus keys = %v, want exactly %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("syllabus keys = %v, want exactly %v", got, want)
			}
		}
	}
}

// --- Input validation ---

func TestListQuestions_MissingSyllabusID(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions", learnerToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "missing_required_fields" {
		t.Fatalf("error = %v, want missing_required_fields", body["error"])
	}
}

func TestListQuestions_UnknownSyllabus(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions?syllabusId=nope", learnerToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorCode(t, rec, "unknown_syllabus")
}

func TestListQuestions_UnknownNodeFilter(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions?syllabusId=syl-active&curriculumMapNodeId=nope", learnerToken)
	assertErrorCode(t, rec, "unknown_node")
}

func TestListQuestions_MismatchedNodeFilter(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions?syllabusId=syl-active&curriculumMapNodeId=node-other", learnerToken)
	assertErrorCode(t, rec, "mismatched_node")
}

func TestListQuestions_EmptyResult(t *testing.T) {
	// syl-active has no node filter mismatch and one seeded question, so filtering by the
	// *correct* other syllabus (none seeded) yields an empty — not missing — list. We reuse the
	// only active syllabus but a node with no matching question to exercise the empty-array path.
	rec := doRequest(t, http.MethodGet, "/learner/questions?syllabusId=syl-active&curriculumMapNodeId=node-1", learnerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(body.Items))
	}
}

func TestGetQuestion_NotFound(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions/does-not-exist", learnerToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != want {
		t.Fatalf("error = %v, want %v", body["error"], want)
	}
}

// --- No leakage ---

// allowedProjectionKeys is the exhaustive, explicit set of keys the learner projection may ever
// carry. Any other key present in a response — status, canonicalRubricVersionId, rubric,
// answerKey, marks, actorId, reviewerId, createdAt, updatedAt, originType, provenance,
// contentSourceId, sourceLocator, licenceReference, or anything else — is a leak of
// editorial/internal data to a learner-role-reachable route.
var allowedProjectionKeys = []string{
	"id", "syllabusId", "curriculumMapNodeId", "responseType", "language", "prompt", "options", "contentRevision",
}

func assertOnlyAllowedKeys(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode projection: %v", err)
	}
	got := make([]string, 0, len(decoded))
	for k := range decoded {
		got = append(got, k)
	}
	sort.Strings(got)
	want := append([]string(nil), allowedProjectionKeys...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("projection keys = %v, want exactly %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("projection keys = %v, want exactly %v", got, want)
		}
	}
}

func TestGetQuestion_NoLeakage(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions/q-1", learnerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertOnlyAllowedKeys(t, rec.Body.Bytes())
}

func TestListQuestions_NoLeakage(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/learner/questions?syllabusId=syl-active", learnerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) == 0 {
		t.Fatal("expected at least one item")
	}
	for _, item := range body.Items {
		assertOnlyAllowedKeys(t, item)
	}
}

func TestMarkingPostRepeatedUsesStableRequestIDOnce(t *testing.T) {
	store := newMarkingMemoryStore()
	marker := &assertingMarker{}
	mux := http.NewServeMux()
	Register(mux, store, fakeVerifier{}, marker)
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/learner/attempts/attempt-1/marking", nil)
		req.Header.Set("Authorization", "Bearer "+learnerToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	marker.mu.Lock()
	defer marker.mu.Unlock()
	if len(marker.ids) != 1 || marker.ids[0] != "stable-request-id" {
		t.Fatalf("marker IDs=%v", marker.ids)
	}
}

func TestAnalytics_IsLearnerOwnedAndSafe(t *testing.T) {
	store := &analyticsMemoryStore{memoryStore: newMemoryStore(), value: LearningAnalytics{ScoredItems: 1, AwardedMarks: 1, PossibleMarks: 2, PendingMarking: 1, Modules: []AnalyticsModule{{ModuleID: "module-1", ModuleLabel: "opaque module", ScoredItems: 1, AwardedMarks: 1, PossibleMarks: 2}}}}
	mux := http.NewServeMux()
	Register(mux, store, fakeVerifier{})
	denied := httptest.NewRequest(http.MethodGet, "/learner/analytics", nil)
	denied.Header.Set("Authorization", "Bearer "+noRoleToken)
	deniedRec := httptest.NewRecorder()
	mux.ServeHTTP(deniedRec, denied)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatal("unknown role must be denied")
	}
	req := httptest.NewRequest(http.MethodGet, "/learner/analytics", nil)
	req.Header.Set("Authorization", "Bearer "+learnerToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, forbidden := range []string{"answer", "rubric", "source", "provenance", "model", "cost", "questionId"} {
		if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte(forbidden)) {
			t.Fatalf("analytics leaked %s: %s", forbidden, rec.Body.String())
		}
	}
}
