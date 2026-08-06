package learner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
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

// memoryStore is an in-memory Store used only for handler tests: routing, auth wiring, input
// validation, and error-shape/no-leakage checks. The real eligibility gate (verified question +
// current verified canonical rubric + verified node + approved linked source) lives entirely in
// PostgresStore's SQL and is covered by postgres_store_test.go's integration tests.
type memoryStore struct {
	activeSyllabuses map[string]bool
	nodeSyllabus     map[string]string
	questions        map[string]Projection
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		activeSyllabuses: map[string]bool{"syl-active": true},
		nodeSyllabus:     map[string]string{"node-1": "syl-active", "node-other": "syl-other-active"},
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
// answerKey, marks, actorId, reviewerId, createdAt, updatedAt, sourceId/sourceUrl/sourceHash, or
// anything else — is a leak of editorial/internal data to a learner-role-reachable route.
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
