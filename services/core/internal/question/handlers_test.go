package question

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// fakeVerifier maps opaque test tokens to verified claims so handler tests exercise the real
// auth middleware and role matrix without any live Clerk instance or cryptography.
type fakeVerifier struct{}

const (
	adminToken    = "admin-token"
	editorToken   = "editor-token"
	reviewerToken = "reviewer-token"
	learnerToken  = "learner-token"
	noRoleToken   = "norole-token"

	adminSubject    = "user_admin"
	editorSubject   = "user_editor"
	reviewerSubject = "user_reviewer"
)

func (fakeVerifier) Verify(_ context.Context, token string) (auth.Claims, error) {
	switch token {
	case adminToken:
		return auth.Claims{Subject: adminSubject, Role: auth.RoleAdmin}, nil
	case editorToken:
		return auth.Claims{Subject: editorSubject, Role: auth.RoleEditor}, nil
	case reviewerToken:
		return auth.Claims{Subject: reviewerSubject, Role: auth.RoleReviewer}, nil
	case learnerToken:
		return auth.Claims{Subject: "user_learner", Role: auth.RoleLearner}, nil
	case noRoleToken:
		return auth.Claims{Subject: "user_norole", Role: auth.RoleUnknown}, nil
	default:
		return auth.Claims{}, auth.ErrInvalidToken
	}
}

// memorySource is the subset of a content_sources row the grounding gate needs.
type memorySource struct {
	status              string
	catalogueSyllabusID *string
}

// memoryNode is the subset of a curriculum_map_nodes row the grounding gate needs.
type memoryNode struct {
	syllabusID string
	status     string
	sourceID   string
}

// memoryStore is an in-memory Store used only for handler tests, mirroring PostgresStore's
// validation semantics (syllabus gate, verified-node + source grounding gate, lifecycle rules,
// rubric version allocation and immutability) without a live Postgres instance.
type memoryStore struct {
	activeSyllabuses map[string]bool
	sources          map[string]memorySource
	nodes            map[string]memoryNode
	questions        map[string]Question
	rubrics          map[string][]RubricVersion
	events           []Event
	nextID           int
}

func statusPointer(status Status) *Status { return &status }

func newMemoryStore() *memoryStore {
	return &memoryStore{
		activeSyllabuses: map[string]bool{"syl-active": true, "syl-other-active": true},
		sources: map[string]memorySource{
			"src-approved-active": {status: "approved", catalogueSyllabusID: strPtr("syl-active")},
			"src-approved-other":  {status: "approved", catalogueSyllabusID: strPtr("syl-other-active")},
		},
		nodes: map[string]memoryNode{
			"node-verified":       {syllabusID: "syl-active", status: "verified", sourceID: "src-approved-active"},
			"node-verified-2":     {syllabusID: "syl-active", status: "verified", sourceID: "src-approved-active"},
			"node-draft":          {syllabusID: "syl-active", status: "draft", sourceID: "src-approved-active"},
			"node-retired":        {syllabusID: "syl-active", status: "retired", sourceID: "src-approved-active"},
			"node-other-syllabus": {syllabusID: "syl-other-active", status: "verified", sourceID: "src-approved-other"},
		},
		questions: map[string]Question{},
		rubrics:   map[string][]RubricVersion{},
	}
}

// setSourceStatus rewrites a seeded source's gate-relevant fields so tests can simulate a source
// that was approved and linked when a question was created but has since regressed.
func (m *memoryStore) setSourceStatus(id, status string, catalogueSyllabusID *string) {
	m.sources[id] = memorySource{status: status, catalogueSyllabusID: catalogueSyllabusID}
}

func (m *memoryStore) deleteSource(id string) { delete(m.sources, id) }

// setNodeStatus simulates a node that has been retired (or reverted to draft) after a question
// was grounded in it.
func (m *memoryStore) setNodeStatus(id, status string) {
	n := m.nodes[id]
	n.status = status
	m.nodes[id] = n
}

func (m *memoryStore) newID() string {
	m.nextID++
	return fmt.Sprintf("question-%d", m.nextID)
}

func (m *memoryStore) validateGrounding(nodeID, syllabusID string) error {
	n, ok := m.nodes[nodeID]
	if !ok {
		return ErrUnknownNode
	}
	if n.syllabusID != syllabusID {
		return ErrMismatchedNode
	}
	if n.status != "verified" {
		return ErrUnverifiedNode
	}
	s, ok := m.sources[n.sourceID]
	if !ok {
		return ErrUnknownSource
	}
	if s.status != "approved" {
		return ErrUnapprovedSource
	}
	if s.catalogueSyllabusID == nil {
		return ErrUnlinkedSource
	}
	if *s.catalogueSyllabusID != syllabusID {
		return ErrMismatchedSource
	}
	return nil
}

func (m *memoryStore) CreateQuestion(_ context.Context, in CreateInput) (Question, error) {
	if !m.activeSyllabuses[in.SyllabusID] {
		return Question{}, ErrUnknownSyllabus
	}
	if err := m.validateGrounding(in.CurriculumMapNodeID, in.SyllabusID); err != nil {
		return Question{}, err
	}
	now := time.Now().UTC()
	q := Question{
		ID:                  m.newID(),
		SyllabusID:          in.SyllabusID,
		CurriculumMapNodeID: in.CurriculumMapNodeID,
		ResponseType:        in.ResponseType,
		Language:            in.Language,
		Prompt:              in.Prompt,
		Status:              StatusDraft,
		ContentRevision:     1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	m.questions[q.ID] = q
	m.events = append(m.events, Event{QuestionID: q.ID, EventType: EventQuestionCreated, ActorID: in.ActorID, EventTime: now})
	return q, nil
}

func (m *memoryStore) UpdateQuestion(_ context.Context, id string, in UpdateInput) (Question, error) {
	q, ok := m.questions[id]
	if !ok {
		return Question{}, ErrNotFound
	}
	if q.Status != StatusDraft {
		return Question{}, ErrInvalidTransition
	}
	effectiveNode := q.CurriculumMapNodeID
	if in.CurriculumMapNodeID != nil {
		effectiveNode = *in.CurriculumMapNodeID
	}
	if err := m.validateGrounding(effectiveNode, q.SyllabusID); err != nil {
		return Question{}, err
	}

	var changed []string
	if in.CurriculumMapNodeID != nil && *in.CurriculumMapNodeID != q.CurriculumMapNodeID {
		q.CurriculumMapNodeID = *in.CurriculumMapNodeID
		changed = append(changed, "curriculumMapNodeId")
	}
	if in.Language != nil && *in.Language != q.Language {
		q.Language = *in.Language
		changed = append(changed, "language")
	}
	if in.Prompt != nil && *in.Prompt != q.Prompt {
		q.Prompt = *in.Prompt
		changed = append(changed, "prompt")
	}
	if in.ResponseType != nil && *in.ResponseType != q.ResponseType {
		q.ResponseType = *in.ResponseType
		changed = append(changed, "responseType")
	}
	if len(changed) == 0 {
		return Question{}, ErrNoChanges
	}
	// Exactly one increment per successful content update — mirrors PostgresStore.
	q.ContentRevision++
	changed = append(changed, "contentRevision")
	q.UpdatedAt = time.Now().UTC()
	m.questions[id] = q
	m.events = append(m.events, Event{QuestionID: id, EventType: EventQuestionUpdated, ActorID: in.ActorID, ChangedFields: changed, EventTime: q.UpdatedAt})
	return q, nil
}

func (m *memoryStore) VerifyQuestion(_ context.Context, id, actorID string) (Question, error) {
	q, ok := m.questions[id]
	if !ok {
		return Question{}, ErrNotFound
	}
	if q.Status != StatusDraft {
		return Question{}, ErrInvalidTransition
	}
	if err := m.validateGrounding(q.CurriculumMapNodeID, q.SyllabusID); err != nil {
		return Question{}, err
	}
	verifiedTotal, verifiedCurrent := 0, 0
	for _, v := range m.rubrics[id] {
		if v.Status != RubricVerified {
			continue
		}
		verifiedTotal++
		if v.QuestionRevision == q.ContentRevision {
			verifiedCurrent++
		}
	}
	if verifiedCurrent == 0 {
		if verifiedTotal == 0 {
			return Question{}, ErrMissingVerifiedRubric
		}
		return Question{}, ErrStaleVerifiedRubric
	}
	q.Status = StatusVerified
	q.UpdatedAt = time.Now().UTC()
	m.questions[id] = q
	m.events = append(m.events, Event{QuestionID: id, EventType: EventQuestionVerified, ActorID: actorID, ChangedFields: []string{"status"}, EventTime: q.UpdatedAt})
	return q, nil
}

func (m *memoryStore) RetireQuestion(_ context.Context, id, actorID string) (Question, error) {
	q, ok := m.questions[id]
	if !ok {
		return Question{}, ErrNotFound
	}
	if q.Status != StatusDraft && q.Status != StatusVerified {
		return Question{}, ErrInvalidTransition
	}
	if err := m.validateGrounding(q.CurriculumMapNodeID, q.SyllabusID); err != nil {
		return Question{}, err
	}
	q.Status = StatusRetired
	q.UpdatedAt = time.Now().UTC()
	m.questions[id] = q
	m.events = append(m.events, Event{QuestionID: id, EventType: EventQuestionRetired, ActorID: actorID, ChangedFields: []string{"status"}, EventTime: q.UpdatedAt})
	return q, nil
}

func (m *memoryStore) GetQuestion(_ context.Context, id string) (Question, error) {
	q, ok := m.questions[id]
	if !ok {
		return Question{}, ErrNotFound
	}
	return q, nil
}

func (m *memoryStore) ListQuestions(_ context.Context, syllabusID string, nodeID *string, status *Status) ([]Question, error) {
	if !m.activeSyllabuses[syllabusID] {
		return nil, ErrUnknownSyllabus
	}
	// Mirrors PostgresStore: an unknown or foreign node filter is a stable 400, never an empty
	// 200 that a caller cannot distinguish from "no questions authored yet".
	if nodeID != nil {
		n, ok := m.nodes[*nodeID]
		if !ok {
			return nil, ErrUnknownNode
		}
		if n.syllabusID != syllabusID {
			return nil, ErrMismatchedNode
		}
	}
	out := []Question{}
	for _, q := range m.questions {
		if q.SyllabusID != syllabusID {
			continue
		}
		if nodeID != nil && q.CurriculumMapNodeID != *nodeID {
			continue
		}
		if status != nil && q.Status != *status {
			continue
		}
		out = append(out, q)
	}
	return out, nil
}

func (m *memoryStore) CreateRubricVersion(_ context.Context, questionID string, in CreateRubricVersionInput) (RubricVersion, error) {
	if err := ValidateRubric(in.Rubric, in.MaxMarks); err != nil {
		return RubricVersion{}, err
	}
	q, ok := m.questions[questionID]
	if !ok {
		return RubricVersion{}, ErrNotFound
	}
	if q.Status != StatusDraft {
		return RubricVersion{}, ErrInvalidTransition
	}
	if err := m.validateGrounding(q.CurriculumMapNodeID, q.SyllabusID); err != nil {
		return RubricVersion{}, err
	}
	next := len(m.rubrics[questionID]) + 1
	now := time.Now().UTC()
	v := RubricVersion{
		ID:               fmt.Sprintf("%s-rubric-%d", questionID, next),
		QuestionID:       questionID,
		Version:          next,
		QuestionRevision: q.ContentRevision,
		Rubric:           in.Rubric,
		MaxMarks:         in.MaxMarks,
		Status:           RubricDraft,
		CreatedBy:        in.ActorID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	m.rubrics[questionID] = append(m.rubrics[questionID], v)
	m.events = append(m.events, Event{QuestionID: questionID, EventType: EventRubricVersionCreated, ActorID: in.ActorID, ChangedFields: []string{"rubricVersion", "questionRevision"}, EventTime: now})
	return v, nil
}

func (m *memoryStore) ListRubricVersions(_ context.Context, questionID string) ([]RubricVersion, error) {
	if _, ok := m.questions[questionID]; !ok {
		return nil, ErrNotFound
	}
	out := []RubricVersion{}
	out = append(out, m.rubrics[questionID]...)
	return out, nil
}

func (m *memoryStore) VerifyRubricVersion(_ context.Context, questionID string, version int, actorID string) (RubricVersion, error) {
	q, ok := m.questions[questionID]
	if !ok {
		return RubricVersion{}, ErrNotFound
	}
	if q.Status != StatusDraft && q.Status != StatusVerified {
		return RubricVersion{}, ErrInvalidTransition
	}
	if err := m.validateGrounding(q.CurriculumMapNodeID, q.SyllabusID); err != nil {
		return RubricVersion{}, err
	}
	for i, v := range m.rubrics[questionID] {
		if v.Version != version {
			continue
		}
		if v.Status != RubricDraft {
			return RubricVersion{}, ErrInvalidTransition
		}
		if err := ValidateRubric(v.Rubric, v.MaxMarks); err != nil {
			return RubricVersion{}, err
		}
		v.Status = RubricVerified
		v.ReviewedBy = &actorID
		v.UpdatedAt = time.Now().UTC()
		m.rubrics[questionID][i] = v
		m.events = append(m.events, Event{QuestionID: questionID, EventType: EventRubricVersionVerify, ActorID: actorID, ChangedFields: []string{"rubricVersionStatus"}, EventTime: v.UpdatedAt})
		return v, nil
	}
	return RubricVersion{}, ErrRubricVersionNotFound
}

func strPtr(s string) *string { return &s }

func newTestServer() (*httptest.Server, *memoryStore) {
	store := newMemoryStore()
	mux := http.NewServeMux()
	Register(mux, store, fakeVerifier{})
	return httptest.NewServer(mux), store
}

func doJSON(t *testing.T, method, url string, body any) *http.Response {
	return doJSONAs(t, method, url, adminToken, body)
}

func doJSONAs(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// doRaw sends a request with a raw (possibly malformed) body, bypassing json.Marshal.
func doRaw(t *testing.T, method, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func validCreateReq() createQuestionRequest {
	return createQuestionRequest{
		SyllabusID:          "syl-active",
		CurriculumMapNodeID: "node-verified",
		ResponseType:        "short_answer",
		Language:            "en",
		Prompt:              "Original question body written by an editor.",
	}
}

// validRubric is an original, minimal rubric structure. It carries no mark-scheme wording.
func validRubric() map[string]any {
	return map[string]any{
		"rubric": map[string]any{
			"criteria": []map[string]any{
				{"id": "c1", "marks": 2, "descriptor": "first original criterion"},
				{"id": "c2", "marks": 1},
			},
		},
		"maxMarks": 3,
	}
}

func createQuestion(t *testing.T, srvURL string) Question {
	t.Helper()
	resp := doJSON(t, http.MethodPost, srvURL+"/questions", validCreateReq())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create question status = %d, want 201", resp.StatusCode)
	}
	return decodeJSON[Question](t, resp)
}

// createVerifiedRubric appends a rubric version and verifies it, the precondition for verifying
// a question.
func createVerifiedRubric(t *testing.T, srvURL string, q Question) RubricVersion {
	t.Helper()
	resp := doJSON(t, http.MethodPost, srvURL+"/questions/"+q.ID+"/rubric-versions", validRubric())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create rubric version status = %d, want 201", resp.StatusCode)
	}
	v := decodeJSON[RubricVersion](t, resp)
	verifyResp := doJSONAs(t, http.MethodPost,
		fmt.Sprintf("%s/questions/%s/rubric-versions/%d/verify", srvURL, q.ID, v.Version), reviewerToken, nil)
	if verifyResp.StatusCode != http.StatusOK {
		t.Fatalf("verify rubric version status = %d, want 200", verifyResp.StatusCode)
	}
	return decodeJSON[RubricVersion](t, verifyResp)
}

// --- Create + node/syllabus matching ---

func TestCreateQuestion_Success(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	if q.Status != StatusDraft {
		t.Fatalf("status = %q, want draft", q.Status)
	}
	if len(store.events) != 1 || store.events[0].ActorID != adminSubject {
		t.Fatalf("expected one question_created event with the verified actor, got %+v", store.events)
	}
}

func TestCreateQuestion_NodeGate(t *testing.T) {
	cases := map[string]struct {
		nodeID    string
		wantError string
	}{
		"unknown node":            {"node-does-not-exist", "unknown_node"},
		"draft node":              {"node-draft", "unverified_node"},
		"retired node":            {"node-retired", "unverified_node"},
		"node in other syllabus":  {"node-other-syllabus", "mismatched_node"},
		"node id is not a string": {" ", "missing_required_fields"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			req := validCreateReq()
			req.CurriculumMapNodeID = tc.nodeID
			resp := doJSON(t, http.MethodPost, srv.URL+"/questions", req)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			body := decodeJSON[map[string]any](t, resp)
			if body["error"] != tc.wantError {
				t.Fatalf("error = %v, want %q", body["error"], tc.wantError)
			}
			if len(store.questions) != 0 || len(store.events) != 0 {
				t.Fatalf("a rejected create wrote state: questions=%d events=%d", len(store.questions), len(store.events))
			}
		})
	}
}

func TestCreateQuestion_UnknownSyllabus_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.SyllabusID = "syl-unknown"
	resp := doJSON(t, http.MethodPost, srv.URL+"/questions", req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if body := decodeJSON[map[string]string](t, resp); body["error"] != "unknown_syllabus" {
		t.Fatalf("error = %q, want unknown_syllabus", body["error"])
	}
}

func TestCreateQuestion_InvalidResponseType_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.ResponseType = "essay"
	resp := doJSON(t, http.MethodPost, srv.URL+"/questions", req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if body := decodeJSON[map[string]string](t, resp); body["error"] != "invalid_response_type" {
		t.Fatalf("error = %q, want invalid_response_type", body["error"])
	}
}

func TestUpdateQuestion_RepointToInvalidNode_Returns400(t *testing.T) {
	for nodeID, wantError := range map[string]string{
		"node-does-not-exist": "unknown_node",
		"node-draft":          "unverified_node",
		"node-retired":        "unverified_node",
		"node-other-syllabus": "mismatched_node",
	} {
		t.Run(nodeID, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			q := createQuestion(t, srv.URL)
			before := snapshotOf(t, store, q.ID)

			resp := doJSON(t, http.MethodPatch, srv.URL+"/questions/"+q.ID,
				map[string]any{"curriculumMapNodeId": nodeID})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if body := decodeJSON[map[string]string](t, resp); body["error"] != wantError {
				t.Fatalf("error = %q, want %q", body["error"], wantError)
			}
			assertUnchanged(t, store, q.ID, before)
		})
	}
}

func TestUpdateQuestion_RepointToValidNode_Succeeds(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	resp := doJSON(t, http.MethodPatch, srv.URL+"/questions/"+q.ID,
		map[string]any{"curriculumMapNodeId": "node-verified-2"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if updated := decodeJSON[Question](t, resp); updated.CurriculumMapNodeID != "node-verified-2" {
		t.Fatalf("node = %q, want node-verified-2", updated.CurriculumMapNodeID)
	}
}

// --- Grounding gate re-validated on every write ---

// snapshot captures the mutable state a rejected request must leave untouched.
type storeSnapshot struct {
	question Question
	events   int
	rubrics  int
}

func snapshotOf(t *testing.T, store *memoryStore, id string) storeSnapshot {
	t.Helper()
	q, ok := store.questions[id]
	if !ok {
		t.Fatalf("question %q missing from store", id)
	}
	return storeSnapshot{question: q, events: len(store.events), rubrics: len(store.rubrics[id])}
}

func assertUnchanged(t *testing.T, store *memoryStore, id string, before storeSnapshot) {
	t.Helper()
	after, ok := store.questions[id]
	if !ok {
		t.Fatalf("question %q disappeared from store", id)
	}
	if after != before.question {
		t.Fatalf("question mutated by a rejected request:\n before = %+v\n after  = %+v", before.question, after)
	}
	if !after.UpdatedAt.Equal(before.question.UpdatedAt) {
		t.Fatalf("updatedAt changed by a rejected request: %v -> %v", before.question.UpdatedAt, after.UpdatedAt)
	}
	if len(store.events) != before.events {
		t.Fatalf("event count = %d, want %d (a rejected request must write no event)", len(store.events), before.events)
	}
	if len(store.rubrics[id]) != before.rubrics {
		t.Fatalf("rubric count = %d, want %d (a rejected request must write no rubric version)", len(store.rubrics[id]), before.rubrics)
	}
}

// brokenGroundingCases mutates the question's node or its source into each failed state and names
// the stable error code Core must return.
var brokenGroundingCases = map[string]struct {
	breakIt   func(store *memoryStore, q Question)
	wantError string
}{
	"node retired after creation": {
		breakIt:   func(s *memoryStore, q Question) { s.setNodeStatus(q.CurriculumMapNodeID, "retired") },
		wantError: "unverified_node",
	},
	"node reverted to draft": {
		breakIt:   func(s *memoryStore, q Question) { s.setNodeStatus(q.CurriculumMapNodeID, "draft") },
		wantError: "unverified_node",
	},
	"source un-approved": {
		breakIt: func(s *memoryStore, _ Question) {
			s.setSourceStatus("src-approved-active", "pending", strPtr("syl-active"))
		},
		wantError: "unapproved_source",
	},
	"source unlinked": {
		breakIt:   func(s *memoryStore, _ Question) { s.setSourceStatus("src-approved-active", "approved", nil) },
		wantError: "unlinked_source",
	},
	"source re-linked elsewhere": {
		breakIt: func(s *memoryStore, _ Question) {
			s.setSourceStatus("src-approved-active", "approved", strPtr("syl-other-active"))
		},
		wantError: "mismatched_source",
	},
	"source deleted": {
		breakIt:   func(s *memoryStore, _ Question) { s.deleteSource("src-approved-active") },
		wantError: "unknown_source",
	},
}

// runBrokenGroundingCases creates a question with good grounding, optionally prepares it, breaks
// the grounding, then runs do — which must be rejected and leave the question and audit trail
// intact.
func runBrokenGroundingCases(t *testing.T, do func(t *testing.T, srvURL string, q Question) *http.Response, prepare func(t *testing.T, srvURL string, q Question)) {
	t.Helper()
	for name, tc := range brokenGroundingCases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			q := createQuestion(t, srv.URL)
			if prepare != nil {
				prepare(t, srv.URL, q)
			}
			tc.breakIt(store, q)
			before := snapshotOf(t, store, q.ID)

			resp := do(t, srv.URL, q)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if body := decodeJSON[map[string]string](t, resp); body["error"] != tc.wantError {
				t.Fatalf("error = %q, want %q", body["error"], tc.wantError)
			}
			assertUnchanged(t, store, q.ID, before)
		})
	}
}

func TestUpdateQuestion_RevalidatesGrounding(t *testing.T) {
	runBrokenGroundingCases(t, func(t *testing.T, srvURL string, q Question) *http.Response {
		return doJSON(t, http.MethodPatch, srvURL+"/questions/"+q.ID, map[string]any{"prompt": "Reworded original prompt."})
	}, nil)
}

func TestCreateRubricVersion_RevalidatesGrounding(t *testing.T) {
	runBrokenGroundingCases(t, func(t *testing.T, srvURL string, q Question) *http.Response {
		return doJSON(t, http.MethodPost, srvURL+"/questions/"+q.ID+"/rubric-versions", validRubric())
	}, nil)
}

func TestVerifyRubricVersion_RevalidatesGrounding(t *testing.T) {
	runBrokenGroundingCases(t, func(t *testing.T, srvURL string, q Question) *http.Response {
		return doJSONAs(t, http.MethodPost, srvURL+"/questions/"+q.ID+"/rubric-versions/1/verify", reviewerToken, nil)
	}, func(t *testing.T, srvURL string, q Question) {
		resp := doJSON(t, http.MethodPost, srvURL+"/questions/"+q.ID+"/rubric-versions", validRubric())
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("prepare rubric status = %d, want 201", resp.StatusCode)
		}
	})
}

func TestVerifyQuestion_RevalidatesGrounding(t *testing.T) {
	runBrokenGroundingCases(t, func(t *testing.T, srvURL string, q Question) *http.Response {
		return doJSONAs(t, http.MethodPost, srvURL+"/questions/"+q.ID+"/verify", reviewerToken, nil)
	}, func(t *testing.T, srvURL string, q Question) {
		createVerifiedRubric(t, srvURL, q)
	})
}

func TestRetireQuestion_RevalidatesGrounding(t *testing.T) {
	runBrokenGroundingCases(t, func(t *testing.T, srvURL string, q Question) *http.Response {
		return doJSONAs(t, http.MethodPost, srvURL+"/questions/"+q.ID+"/retire", reviewerToken, nil)
	}, nil)
}

// --- Rubric versions ---

func TestCreateRubricVersion_MonotonicAndUnique(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	for want := 1; want <= 3; want++ {
		resp := doJSON(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", validRubric())
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201", resp.StatusCode)
		}
		v := decodeJSON[RubricVersion](t, resp)
		if v.Version != want {
			t.Fatalf("version = %d, want %d", v.Version, want)
		}
		if v.Status != RubricDraft {
			t.Fatalf("status = %q, want draft", v.Status)
		}
		if v.CreatedBy != adminSubject {
			t.Fatalf("createdBy = %q, want the verified subject %q", v.CreatedBy, adminSubject)
		}
	}

	listResp := doJSON(t, http.MethodGet, srv.URL+"/questions/"+q.ID+"/rubric-versions", nil)
	versions := decodeJSON[map[string][]RubricVersion](t, listResp)["items"]
	if len(versions) != 3 {
		t.Fatalf("versions = %d, want 3", len(versions))
	}
	for i, v := range versions {
		if v.Version != i+1 {
			t.Fatalf("versions[%d].version = %d, want %d (must be monotonic)", i, v.Version, i+1)
		}
	}
}

func TestVerifyRubricVersion_RecordsReviewerAndIsSingleUse(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	v := createVerifiedRubric(t, srv.URL, q)
	if v.Status != RubricVerified {
		t.Fatalf("status = %q, want verified", v.Status)
	}
	if v.ReviewedBy == nil || *v.ReviewedBy != reviewerSubject {
		t.Fatalf("reviewedBy = %v, want the verified subject %q", v.ReviewedBy, reviewerSubject)
	}

	// Re-verifying an already-verified version is an invalid transition — a version never
	// changes content or status twice.
	repeat := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions/1/verify", reviewerToken, nil)
	if repeat.StatusCode != http.StatusConflict {
		t.Fatalf("re-verify status = %d, want 409", repeat.StatusCode)
	}
}

func TestVerifyRubricVersion_UnknownVersion_Returns404(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	resp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions/9/verify", reviewerToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestCreateRubricVersion_InvalidRubricOrMarks_Returns400(t *testing.T) {
	cases := map[string]struct {
		body      string
		wantError string
	}{
		"marks do not sum to maxMarks": {`{"rubric":{"criteria":[{"id":"c1","marks":2}]},"maxMarks":5}`, "invalid_max_marks"},
		"zero maxMarks":                {`{"rubric":{"criteria":[{"id":"c1","marks":2}]},"maxMarks":0}`, "invalid_max_marks"},
		"negative maxMarks":            {`{"rubric":{"criteria":[{"id":"c1","marks":2}]},"maxMarks":-3}`, "invalid_max_marks"},
		"empty criteria":               {`{"rubric":{"criteria":[]},"maxMarks":3}`, "invalid_rubric"},
		"missing criteria":             {`{"rubric":{},"maxMarks":3}`, "invalid_rubric"},
		"unknown rubric field":         {`{"rubric":{"criteria":[{"id":"c1","marks":3}],"answers":"x"},"maxMarks":3}`, "invalid_rubric"},
		"unknown criterion field":      {`{"rubric":{"criteria":[{"id":"c1","marks":3,"answer":"x"}]},"maxMarks":3}`, "invalid_rubric"},
		"blank criterion id":           {`{"rubric":{"criteria":[{"id":"  ","marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"duplicate criterion id":       {`{"rubric":{"criteria":[{"id":"c1","marks":2},{"id":"c1","marks":1}]},"maxMarks":3}`, "invalid_rubric"},
		"non-positive criterion marks": {`{"rubric":{"criteria":[{"id":"c1","marks":0},{"id":"c2","marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"missing criterion marks":      {`{"rubric":{"criteria":[{"id":"c1"}]},"maxMarks":3}`, "invalid_rubric"},
		"blank descriptor":             {`{"rubric":{"criteria":[{"id":"c1","marks":3,"descriptor":" "}]},"maxMarks":3}`, "invalid_rubric"},
		"rubric is not an object":      {`{"rubric":[1,2,3],"maxMarks":3}`, "invalid_rubric"},

		// Case variants of every rubric key. Go's struct decoding matches JSON names
		// case-insensitively, so these were previously ACCEPTED despite the documented schema.
		"Criteria case variant":   {`{"rubric":{"Criteria":[{"id":"c1","marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"CRITERIA case variant":   {`{"rubric":{"CRITERIA":[{"id":"c1","marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"ID case variant":         {`{"rubric":{"criteria":[{"ID":"c1","marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"Marks case variant":      {`{"rubric":{"criteria":[{"id":"c1","Marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"Descriptor case variant": {`{"rubric":{"criteria":[{"id":"c1","marks":3,"Descriptor":"x"}]},"maxMarks":3}`, "invalid_rubric"},

		// Structural abuse the schema must not admit.
		"duplicate criteria key":   {`{"rubric":{"criteria":[{"id":"c1","marks":3}],"criteria":[{"id":"c2","marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"duplicate criterion key":  {`{"rubric":{"criteria":[{"id":"c1","marks":1,"marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"criterion is not object":  {`{"rubric":{"criteria":["c1"]},"maxMarks":3}`, "invalid_rubric"},
		"criterion is null":        {`{"rubric":{"criteria":[null]},"maxMarks":3}`, "invalid_rubric"},
		"criteria is null":         {`{"rubric":{"criteria":null},"maxMarks":3}`, "invalid_rubric"},
		"id is not a string":       {`{"rubric":{"criteria":[{"id":7,"marks":3}]},"maxMarks":3}`, "invalid_rubric"},
		"marks is a string":        {`{"rubric":{"criteria":[{"id":"c1","marks":"3"}]},"maxMarks":3}`, "invalid_rubric"},
		"marks is fractional":      {`{"rubric":{"criteria":[{"id":"c1","marks":1.5},{"id":"c2","marks":1.5}]},"maxMarks":3}`, "invalid_rubric"},
		"descriptor is not string": {`{"rubric":{"criteria":[{"id":"c1","marks":3,"descriptor":7}]},"maxMarks":3}`, "invalid_rubric"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			q := createQuestion(t, srv.URL)
			before := snapshotOf(t, store, q.ID)

			resp := doRaw(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", tc.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if body := decodeJSON[map[string]string](t, resp); body["error"] != tc.wantError {
				t.Fatalf("error = %q, want %q", body["error"], tc.wantError)
			}
			assertUnchanged(t, store, q.ID, before)
		})
	}
}

func TestCreateRubricVersion_MissingFields_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	resp := doRaw(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", `{}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if body := decodeJSON[map[string]any](t, resp); body["error"] != "missing_required_fields" {
		t.Fatalf("error = %v, want missing_required_fields", body["error"])
	}
}

func TestCreateRubricVersion_NonDraftQuestion_Returns409(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	createVerifiedRubric(t, srv.URL, q)
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("verify question status = %d, want 200", resp.StatusCode)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", validRubric())
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	if body := decodeJSON[map[string]string](t, resp); body["error"] != "invalid_lifecycle_transition" {
		t.Fatalf("error = %q, want invalid_lifecycle_transition", body["error"])
	}
}

// --- Question lifecycle ---

func TestVerifyQuestion_WithoutVerifiedRubric_Returns409(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)

	// No rubric at all.
	before := snapshotOf(t, store, q.ID)
	resp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	if body := decodeJSON[map[string]string](t, resp); body["error"] != "missing_verified_rubric" {
		t.Fatalf("error = %q, want missing_verified_rubric", body["error"])
	}
	assertUnchanged(t, store, q.ID, before)

	// A draft rubric version is not enough either.
	if r := doJSON(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", validRubric()); r.StatusCode != http.StatusCreated {
		t.Fatalf("create rubric status = %d, want 201", r.StatusCode)
	}
	resp2 := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil)
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("draft-rubric verify status = %d, want 409", resp2.StatusCode)
	}
}

func TestQuestionLifecycle_VerifyThenRetire(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	createVerifiedRubric(t, srv.URL, q)

	verifyResp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil)
	if verifyResp.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", verifyResp.StatusCode)
	}
	if verified := decodeJSON[Question](t, verifyResp); verified.Status != StatusVerified {
		t.Fatalf("status = %q, want verified", verified.Status)
	}

	if reverify := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil); reverify.StatusCode != http.StatusConflict {
		t.Fatalf("re-verify status = %d, want 409", reverify.StatusCode)
	}

	// A verified question is no longer PATCHable.
	if patch := doJSON(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, map[string]any{"prompt": "changed"}); patch.StatusCode != http.StatusConflict {
		t.Fatalf("patch verified status = %d, want 409", patch.StatusCode)
	}

	retireResp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/retire", reviewerToken, nil)
	if retireResp.StatusCode != http.StatusOK {
		t.Fatalf("retire status = %d, want 200", retireResp.StatusCode)
	}
	if retired := decodeJSON[Question](t, retireResp); retired.Status != StatusRetired {
		t.Fatalf("status = %q, want retired", retired.Status)
	}

	if reretire := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/retire", reviewerToken, nil); reretire.StatusCode != http.StatusConflict {
		t.Fatalf("re-retire status = %d, want 409", reretire.StatusCode)
	}
}

func TestQuestionReads_ReturnAllStatusesAndSupportFilters(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	createVerifiedRubric(t, srv.URL, q)
	doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil)

	// Visible while verified.
	if resp := doJSON(t, http.MethodGet, srv.URL+"/questions/"+q.ID, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("get verified status = %d, want 200", resp.StatusCode)
	}
	listed := decodeJSON[map[string][]Question](t, doJSON(t, http.MethodGet, srv.URL+"/questions?syllabusId=syl-active", nil))
	if len(listed["items"]) != 1 {
		t.Fatalf("verified list items = %d, want 1", len(listed["items"]))
	}

	doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/retire", reviewerToken, nil)

	if resp := doJSON(t, http.MethodGet, srv.URL+"/questions/"+q.ID, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("get retired status = %d, want 200", resp.StatusCode)
	}
	afterRetire := decodeJSON[map[string][]Question](t, doJSON(t, http.MethodGet, srv.URL+"/questions?syllabusId=syl-active", nil))
	if len(afterRetire["items"]) != 1 || afterRetire["items"][0].Status != StatusRetired {
		t.Fatalf("all-status list = %+v, want retired question", afterRetire["items"])
	}
	verifiedOnly := decodeJSON[map[string][]Question](t, doJSON(t, http.MethodGet, srv.URL+"/questions?syllabusId=syl-active&status=verified", nil))
	if len(verifiedOnly["items"]) != 0 {
		t.Fatalf("verified filter returned %d retired questions, want 0", len(verifiedOnly["items"]))
	}
	retiredOnly := decodeJSON[map[string][]Question](t, doJSON(t, http.MethodGet, srv.URL+"/questions?syllabusId=syl-active&status=retired", nil))
	if len(retiredOnly["items"]) != 1 {
		t.Fatalf("retired filter items = %d, want 1", len(retiredOnly["items"]))
	}
}

func TestGetQuestion_DraftVisibleToEditorialReaders(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	if resp := doJSON(t, http.MethodGet, srv.URL+"/questions/"+q.ID, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("get draft status = %d, want 200", resp.StatusCode)
	}
	for _, query := range []string{"", "&status=all", "&status=draft"} {
		items := decodeJSON[map[string][]Question](t, doJSON(t, http.MethodGet,
			srv.URL+"/questions?syllabusId=syl-active"+query, nil))["items"]
		if len(items) != 1 || items[0].Status != StatusDraft {
			t.Fatalf("query %q items = %+v, want draft question", query, items)
		}
	}
}

func TestListQuestions_InvalidStatus_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	for _, status := range []string{"bogus", "Draft"} {
		resp := doJSON(t, http.MethodGet, srv.URL+"/questions?syllabusId=syl-active&status="+status, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %q response = %d, want 400", status, resp.StatusCode)
		}
		if body := decodeJSON[map[string]string](t, resp); body["error"] != "invalid_status" {
			t.Fatalf("error = %q, want invalid_status", body["error"])
		}
	}
}

// --- Listing ---

func TestListQuestions_RequiresSyllabusID(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodGet, srv.URL+"/questions", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestListQuestions_UnknownSyllabus_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodGet, srv.URL+"/questions?syllabusId=syl-does-not-exist", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if body := decodeJSON[map[string]string](t, resp); body["error"] != "unknown_syllabus" {
		t.Fatalf("error = %q, want unknown_syllabus", body["error"])
	}
}

func TestListQuestions_OptionalNodeFilter(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	createVerifiedRubric(t, srv.URL, q)
	doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil)

	matching := decodeJSON[map[string][]Question](t, doJSON(t, http.MethodGet,
		srv.URL+"/questions?syllabusId=syl-active&curriculumMapNodeId=node-verified", nil))
	if len(matching["items"]) != 1 {
		t.Fatalf("matching node filter items = %d, want 1", len(matching["items"]))
	}

	other := decodeJSON[map[string][]Question](t, doJSON(t, http.MethodGet,
		srv.URL+"/questions?syllabusId=syl-active&curriculumMapNodeId=node-verified-2", nil))
	if len(other["items"]) != 0 {
		t.Fatalf("non-matching node filter items = %d, want 0", len(other["items"]))
	}
}

// --- Role matrix ---

func TestRoleMatrix_LearnerAndUnknownDenied(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	for _, token := range []string{learnerToken, noRoleToken} {
		for _, call := range []struct {
			method string
			path   string
			body   any
		}{
			{http.MethodGet, "/questions?syllabusId=syl-active", nil},
			{http.MethodGet, "/questions/any-id", nil},
			{http.MethodPost, "/questions", validCreateReq()},
			{http.MethodPatch, "/questions/any-id", map[string]any{"prompt": "x"}},
			{http.MethodPost, "/questions/any-id/verify", nil},
			{http.MethodPost, "/questions/any-id/retire", nil},
			{http.MethodGet, "/questions/any-id/rubric-versions", nil},
			{http.MethodPost, "/questions/any-id/rubric-versions", validRubric()},
			{http.MethodPost, "/questions/any-id/rubric-versions/1/verify", nil},
		} {
			resp := doJSONAs(t, call.method, srv.URL+call.path, token, call.body)
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("%s %s as %q status = %d, want 403", call.method, call.path, token, resp.StatusCode)
			}
		}
	}
}

func TestRoleMatrix_MissingToken_Returns401(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSONAs(t, http.MethodGet, srv.URL+"/questions?syllabusId=syl-active", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRoleMatrix_EditorDrafts_ReviewerVerifies(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	// Editor: create draft question + draft rubric version + list rubric versions.
	createResp := doJSONAs(t, http.MethodPost, srv.URL+"/questions", editorToken, validCreateReq())
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("editor create status = %d, want 201", createResp.StatusCode)
	}
	q := decodeJSON[Question](t, createResp)

	if resp := doJSONAs(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, editorToken, map[string]any{"prompt": "Reworded original prompt."}); resp.StatusCode != http.StatusOK {
		t.Fatalf("editor patch status = %d, want 200", resp.StatusCode)
	}
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", editorToken, validRubric()); resp.StatusCode != http.StatusCreated {
		t.Fatalf("editor rubric create status = %d, want 201", resp.StatusCode)
	}
	if resp := doJSONAs(t, http.MethodGet, srv.URL+"/questions/"+q.ID+"/rubric-versions", editorToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("editor rubric list status = %d, want 200", resp.StatusCode)
	}

	// Editor must not verify or retire anything.
	for _, path := range []string{
		"/questions/" + q.ID + "/rubric-versions/1/verify",
		"/questions/" + q.ID + "/verify",
		"/questions/" + q.ID + "/retire",
	} {
		if resp := doJSONAs(t, http.MethodPost, srv.URL+path, editorToken, nil); resp.StatusCode != http.StatusForbidden {
			t.Fatalf("editor POST %s status = %d, want 403", path, resp.StatusCode)
		}
	}

	// Reviewer: verify rubric, verify question, retire question.
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions/1/verify", reviewerToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer rubric verify status = %d, want 200", resp.StatusCode)
	}
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer question verify status = %d, want 200", resp.StatusCode)
	}
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/retire", reviewerToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer retire status = %d, want 200", resp.StatusCode)
	}
}

func TestAuditIdentity_IsVerifiedSubjectOnly(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	createResp := doJSONAs(t, http.MethodPost, srv.URL+"/questions", editorToken, validCreateReq())
	q := decodeJSON[Question](t, createResp)
	rubric := decodeJSON[RubricVersion](t, doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", editorToken, validRubric()))
	if rubric.CreatedBy != editorSubject {
		t.Fatalf("rubric createdBy = %q, want %q", rubric.CreatedBy, editorSubject)
	}
	doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions/1/verify", reviewerToken, nil)
	doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil)

	wantActors := []string{editorSubject, editorSubject, reviewerSubject, reviewerSubject}
	if len(store.events) != len(wantActors) {
		t.Fatalf("events = %d, want %d: %+v", len(store.events), len(wantActors), store.events)
	}
	for i, want := range wantActors {
		if store.events[i].ActorID != want {
			t.Fatalf("events[%d].actorId = %q, want %q", i, store.events[i].ActorID, want)
		}
	}
}

// TestAuditNeverStoresContent asserts the audit trail records field NAMES only — never a prompt
// or rubric value.
func TestAuditNeverStoresContent(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	doJSON(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, map[string]any{"prompt": "A different original prompt."})
	createVerifiedRubric(t, srv.URL, q)

	for _, e := range store.events {
		for _, f := range e.ChangedFields {
			switch f {
			case "curriculumMapNodeId", "responseType", "language", "prompt", "status",
				"syllabusId", "contentRevision", "rubricVersion", "questionRevision",
				"rubricVersionStatus":
			default:
				t.Fatalf("changed field %q is not a known field name — audit must never carry values", f)
			}
		}
	}
}

// --- Question content revision: a rubric must match the question it was reviewed against ---

// patchPrompt applies a prompt edit and returns the updated question.
func patchPrompt(t *testing.T, srvURL, id, prompt string) Question {
	t.Helper()
	resp := doJSON(t, http.MethodPatch, srvURL+"/questions/"+id, map[string]any{"prompt": prompt})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d, want 200", resp.StatusCode)
	}
	return decodeJSON[Question](t, resp)
}

func TestCreateQuestion_StartsAtRevisionOne(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	if q := createQuestion(t, srv.URL); q.ContentRevision != 1 {
		t.Fatalf("contentRevision = %d, want 1", q.ContentRevision)
	}
}

// TestUpdateQuestion_IncrementsRevisionExactlyOnce covers each editable content field: one
// successful update must move the revision by exactly one, never more.
func TestUpdateQuestion_IncrementsRevisionExactlyOnce(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	want := q.ContentRevision

	for _, patch := range []map[string]any{
		{"prompt": "A second original prompt."},
		{"language": "fr"},
		{"responseType": "structured_response"},
		{"curriculumMapNodeId": "node-verified-2"},
		// A multi-field update is still one revision.
		{"prompt": "A third original prompt.", "language": "de"},
	} {
		want++
		resp := doJSON(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, patch)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("patch %v status = %d, want 200", patch, resp.StatusCode)
		}
		if updated := decodeJSON[Question](t, resp); updated.ContentRevision != want {
			t.Fatalf("contentRevision after %v = %d, want %d", patch, updated.ContentRevision, want)
		}
	}
}

// TestUpdateQuestion_RejectedWritesLeaveRevisionUnchanged pins the other half of the invariant: a
// no-op or rejected update must not burn a revision, which would silently stale every rubric.
func TestUpdateQuestion_RejectedWritesLeaveRevisionUnchanged(t *testing.T) {
	cases := map[string]struct {
		body       string
		wantStatus int
	}{
		"no changes":         {`{"prompt":"Original question body written by an editor."}`, http.StatusBadRequest},
		"no updatable field": {`{}`, http.StatusBadRequest},
		"blank field":        {`{"prompt":"   "}`, http.StatusBadRequest},
		"unknown field":      {`{"prompt":"new","totallyUnknown":1}`, http.StatusBadRequest},
		"immutable field":    {`{"prompt":"new","syllabusId":"syl-other-active"}`, http.StatusBadRequest},
		"invalid node":       {`{"curriculumMapNodeId":"node-draft"}`, http.StatusBadRequest},
		"malformed json":     {`{"prompt":`, http.StatusBadRequest},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			q := createQuestion(t, srv.URL)
			before := snapshotOf(t, store, q.ID)

			resp := doRaw(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, tc.body)
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			resp.Body.Close()
			// assertUnchanged compares the whole question, contentRevision included.
			assertUnchanged(t, store, q.ID, before)
			if store.questions[q.ID].ContentRevision != before.question.ContentRevision {
				t.Fatalf("contentRevision = %d, want %d (a rejected write must not consume a revision)",
					store.questions[q.ID].ContentRevision, before.question.ContentRevision)
			}
		})
	}
}

func TestCreateRubricVersion_StampsCurrentQuestionRevision(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	first := decodeJSON[RubricVersion](t, doJSON(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", validRubric()))
	if first.QuestionRevision != 1 {
		t.Fatalf("first version questionRevision = %d, want 1", first.QuestionRevision)
	}

	patchPrompt(t, srv.URL, q.ID, "An edited original prompt.")

	second := decodeJSON[RubricVersion](t, doJSON(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/rubric-versions", validRubric()))
	if second.QuestionRevision != 2 {
		t.Fatalf("post-edit version questionRevision = %d, want 2", second.QuestionRevision)
	}
	if second.Version != 2 {
		t.Fatalf("version = %d, want 2", second.Version)
	}
}

// TestVerifyQuestion_StaleRubricAfterEdit_Returns409 is the review finding itself: a rubric
// verified against revision 1 must not authorise verifying a question whose content has changed.
func TestVerifyQuestion_StaleRubricAfterEdit_Returns409(t *testing.T) {
	for name, patch := range map[string]map[string]any{
		"prompt edited":       {"prompt": "A materially different original prompt."},
		"responseType edited": {"responseType": "structured_response"},
		"language edited":     {"language": "fr"},
		"node repointed":      {"curriculumMapNodeId": "node-verified-2"},
		"two fields at once":  {"prompt": "Another original prompt.", "language": "fr"},
	} {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			q := createQuestion(t, srv.URL)
			createVerifiedRubric(t, srv.URL, q)

			resp := doJSON(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, patch)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("patch status = %d, want 200", resp.StatusCode)
			}
			resp.Body.Close()
			before := snapshotOf(t, store, q.ID)

			verify := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil)
			if verify.StatusCode != http.StatusConflict {
				t.Fatalf("verify status = %d, want 409", verify.StatusCode)
			}
			if body := decodeJSON[map[string]string](t, verify); body["error"] != "missing_current_verified_rubric" {
				t.Fatalf("error = %q, want missing_current_verified_rubric", body["error"])
			}
			assertUnchanged(t, store, q.ID, before)
		})
	}
}

// TestVerifyQuestion_NewRubricAfterEdit_Succeeds shows the intended remedy: append and verify a
// rubric at the current revision.
func TestVerifyQuestion_NewRubricAfterEdit_Succeeds(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	createVerifiedRubric(t, srv.URL, q)
	edited := patchPrompt(t, srv.URL, q.ID, "An edited original prompt.")
	if edited.ContentRevision != 2 {
		t.Fatalf("contentRevision = %d, want 2", edited.ContentRevision)
	}

	second := createVerifiedRubric(t, srv.URL, q)
	if second.Version != 2 || second.QuestionRevision != 2 {
		t.Fatalf("second version = %+v, want version 2 at questionRevision 2", second)
	}

	verify := doJSONAs(t, http.MethodPost, srv.URL+"/questions/"+q.ID+"/verify", reviewerToken, nil)
	if verify.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", verify.StatusCode)
	}
	if verified := decodeJSON[Question](t, verify); verified.Status != StatusVerified {
		t.Fatalf("status = %q, want verified", verified.Status)
	}
}

// TestStaleRubricVersion_RemainsReadable confirms an edit stales a version for verification only:
// it is neither deleted nor downgraded, and editorial roles can still read it.
func TestStaleRubricVersion_RemainsReadable(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	original := createVerifiedRubric(t, srv.URL, q)
	patchPrompt(t, srv.URL, q.ID, "An edited original prompt.")

	for _, token := range []string{editorToken, reviewerToken, adminToken} {
		listed := decodeJSON[map[string][]RubricVersion](t,
			doJSONAs(t, http.MethodGet, srv.URL+"/questions/"+q.ID+"/rubric-versions", token, nil))["items"]
		if len(listed) != 1 {
			t.Fatalf("as %q: versions = %d, want 1", token, len(listed))
		}
		got := listed[0]
		if got.Status != RubricVerified {
			t.Fatalf("as %q: stale version status = %q, want verified (staleness must not downgrade it)", token, got.Status)
		}
		if got.Version != original.Version || got.QuestionRevision != 1 || got.MaxMarks != original.MaxMarks {
			t.Fatalf("as %q: stale version mutated: %+v", token, got)
		}
	}
}

// --- Listing: the optional node filter is validated ---

func TestListQuestions_NodeFilterValidated(t *testing.T) {
	cases := map[string]struct {
		nodeID    string
		wantError string
	}{
		"unknown node":        {"node-does-not-exist", "unknown_node"},
		"malformed node id":   {"not-a-uuid", "unknown_node"},
		"node other syllabus": {"node-other-syllabus", "mismatched_node"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv, _ := newTestServer()
			defer srv.Close()

			resp := doJSON(t, http.MethodGet,
				srv.URL+"/questions?syllabusId=syl-active&curriculumMapNodeId="+tc.nodeID, nil)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if body := decodeJSON[map[string]string](t, resp); body["error"] != tc.wantError {
				t.Fatalf("error = %q, want %q", body["error"], tc.wantError)
			}
		})
	}
}

// TestListQuestions_ValidNodeWithNoQuestions_ReturnsEmpty keeps the other half honest: a real
// node of the right syllabus is 200 with an empty list, not an error.
func TestListQuestions_ValidNodeWithNoQuestions_ReturnsEmpty(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodGet,
		srv.URL+"/questions?syllabusId=syl-active&curriculumMapNodeId=node-verified-2", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if items := decodeJSON[map[string][]Question](t, resp)["items"]; len(items) != 0 {
		t.Fatalf("items = %d, want 0", len(items))
	}
}

// TestListQuestions_UnknownSyllabusBeatsNodeFilter preserves the existing syllabus behaviour: the
// syllabus is still resolved first, so a bad syllabus is unknown_syllabus regardless of the node.
func TestListQuestions_UnknownSyllabusBeatsNodeFilter(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodGet,
		srv.URL+"/questions?syllabusId=syl-does-not-exist&curriculumMapNodeId=node-verified", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if body := decodeJSON[map[string]string](t, resp); body["error"] != "unknown_syllabus" {
		t.Fatalf("error = %q, want unknown_syllabus", body["error"])
	}
}

// --- Strict JSON ---

func TestCreateQuestion_StrictJSON(t *testing.T) {
	cases := map[string]string{
		"actorId":          `{"syllabusId":"syl-active","curriculumMapNodeId":"node-verified","responseType":"short_answer","language":"en","prompt":"p","actorId":"someone-else"}`,
		"reviewerId":       `{"syllabusId":"syl-active","curriculumMapNodeId":"node-verified","responseType":"short_answer","language":"en","prompt":"p","reviewerId":"someone-else"}`,
		"status spoofing":  `{"syllabusId":"syl-active","curriculumMapNodeId":"node-verified","responseType":"short_answer","language":"en","prompt":"p","status":"verified"}`,
		"unknown field":    `{"syllabusId":"syl-active","curriculumMapNodeId":"node-verified","responseType":"short_answer","language":"en","prompt":"p","totallyUnknown":"x"}`,
		"case variant":     `{"SyllabusId":"syl-active","curriculumMapNodeId":"node-verified","responseType":"short_answer","language":"en","prompt":"p"}`,
		"trailing object":  `{"syllabusId":"syl-active","curriculumMapNodeId":"node-verified","responseType":"short_answer","language":"en","prompt":"p"}{}`,
		"trailing array":   `{"syllabusId":"syl-active","curriculumMapNodeId":"node-verified","responseType":"short_answer","language":"en","prompt":"p"}[1]`,
		"trailing junk":    `{"syllabusId":"syl-active","curriculumMapNodeId":"node-verified","responseType":"short_answer","language":"en","prompt":"p"}not-json`,
		"not an object":    `[1,2,3]`,
		"malformed object": `{"syllabusId":`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			resp := doRaw(t, http.MethodPost, srv.URL+"/questions", body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if respBody := decodeJSON[map[string]string](t, resp); respBody["error"] != "invalid_json" {
				t.Fatalf("error = %q, want invalid_json", respBody["error"])
			}
			if len(store.questions) != 0 || len(store.events) != 0 {
				t.Fatalf("a rejected create wrote state: questions=%d events=%d", len(store.questions), len(store.events))
			}
		})
	}
}

// TestUpdateQuestion_ForbiddenAndUnknownFields_Returns400 covers every identity, immutable, and
// lifecycle field a caller might try to smuggle through PATCH. DisallowUnknownFields has no
// effect on the map decode PATCH uses, so these are rejected by the explicit field allowlist.
func TestUpdateQuestion_ForbiddenAndUnknownFields_Returns400(t *testing.T) {
	cases := map[string]string{
		"actorId":          `{"prompt":"new","actorId":"someone-else"}`,
		"reviewerId":       `{"prompt":"new","reviewerId":"someone-else"}`,
		"syllabusId":       `{"prompt":"new","syllabusId":"syl-other-active"}`,
		"status":           `{"prompt":"new","status":"verified"}`,
		"id":               `{"prompt":"new","id":"other-question"}`,
		"createdAt":        `{"prompt":"new","createdAt":"2020-01-01T00:00:00Z"}`,
		"updatedAt":        `{"prompt":"new","updatedAt":"2020-01-01T00:00:00Z"}`,
		"unknown field":    `{"prompt":"new","totallyUnknown":"x"}`,
		"typo field":       `{"promptt":"new"}`,
		"case variant":     `{"Prompt":"new"}`,
		"forbidden alone":  `{"status":"retired"}`,
		"trailing object":  `{"prompt":"new"}{}`,
		"trailing array":   `{"prompt":"new"}[1]`,
		"trailing junk":    `{"prompt":"new"}not-json`,
		"trailing string":  `{"prompt":"new"}"extra"`,
		"malformed object": `{"prompt":`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			q := createQuestion(t, srv.URL)
			before := snapshotOf(t, store, q.ID)

			resp := doRaw(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if respBody := decodeJSON[map[string]string](t, resp); respBody["error"] != "invalid_json" {
				t.Fatalf("error = %q, want invalid_json", respBody["error"])
			}
			assertUnchanged(t, store, q.ID, before)
		})
	}
}

func TestUpdateQuestion_NoUpdatableFieldsAndNoChanges(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)

	empty := doRaw(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, `{}`)
	if empty.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty patch status = %d, want 400", empty.StatusCode)
	}
	if body := decodeJSON[map[string]string](t, empty); body["error"] != "no_updatable_fields" {
		t.Fatalf("error = %q, want no_updatable_fields", body["error"])
	}

	same := doJSON(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, map[string]any{"prompt": q.Prompt})
	if same.StatusCode != http.StatusBadRequest {
		t.Fatalf("same-value patch status = %d, want 400", same.StatusCode)
	}
	if body := decodeJSON[map[string]string](t, same); body["error"] != "no_changes" {
		t.Fatalf("error = %q, want no_changes", body["error"])
	}
}

func TestUpdateQuestion_BlankField_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	q := createQuestion(t, srv.URL)
	resp := doRaw(t, http.MethodPatch, srv.URL+"/questions/"+q.ID, `{"prompt":"   "}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if body := decodeJSON[map[string]any](t, resp); body["error"] != "blank_fields" {
		t.Fatalf("error = %v, want blank_fields", body["error"])
	}
}

// --- No raw internal errors ---

func TestNoRawInternalErrorText(t *testing.T) {
	// Every mapped domain error message is a static, curated string defined in this package —
	// never a raw driver/SQL error. This table documents that guarantee at the handler layer.
	for err, code := range map[error]string{
		ErrNotFound:               "not_found",
		ErrRubricVersionNotFound:  "not_found",
		ErrUnknownSyllabus:        "unknown_syllabus",
		ErrUnknownNode:            "unknown_node",
		ErrUnverifiedNode:         "unverified_node",
		ErrMismatchedNode:         "mismatched_node",
		ErrUnknownSource:          "unknown_source",
		ErrUnapprovedSource:       "unapproved_source",
		ErrUnlinkedSource:         "unlinked_source",
		ErrMismatchedSource:       "mismatched_source",
		ErrInvalidRubric:          "invalid_rubric",
		ErrInvalidMaxMarks:        "invalid_max_marks",
		ErrNoChanges:              "no_changes",
		ErrMissingVerifiedRubric:  "missing_verified_rubric",
		ErrStaleVerifiedRubric:    "missing_current_verified_rubric",
		ErrInvalidTransition:      "invalid_lifecycle_transition",
		ErrDuplicateRubricVersion: "duplicate_rubric_version",
	} {
		mapped, ok := mapQuestionError(err)
		if !ok {
			t.Fatalf("mapQuestionError(%v) not mapped", err)
		}
		if mapped.code != code {
			t.Fatalf("mapQuestionError(%v).code = %q, want %q", err, mapped.code, code)
		}
	}

	// An unmapped (infrastructure) error must never be mapped to a domain code — the handler
	// falls through to the single generic internal_error message.
	if _, ok := mapQuestionError(errUnexpected); ok {
		t.Fatal("an infrastructure error was mapped to a domain code")
	}
}
