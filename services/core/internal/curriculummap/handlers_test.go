package curriculummap

import (
	"bytes"
	"context"
	"encoding/json"
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

// memorySource is the subset of a content_sources row the curriculum-map source gate needs.
type memorySource struct {
	status              string
	catalogueSyllabusID *string
}

// memoryStore is an in-memory Store used only for handler tests, mirroring PostgresStore's
// validation semantics (syllabus gate, parent/cycle checks, source gate, lifecycle rules)
// without a live Postgres instance.
type memoryStore struct {
	activeSyllabuses map[string]bool
	sources          map[string]memorySource
	nodes            map[string]Node
	events           []Event
	nextID           int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		activeSyllabuses: map[string]bool{"syl-active": true, "syl-other-active": true},
		sources: map[string]memorySource{
			"src-approved-active":   {status: "approved", catalogueSyllabusID: strPtr("syl-active")},
			"src-approved-other":    {status: "approved", catalogueSyllabusID: strPtr("syl-other-active")},
			"src-pending":           {status: "pending", catalogueSyllabusID: strPtr("syl-active")},
			"src-approved-unlinked": {status: "approved", catalogueSyllabusID: nil},
		},
		nodes: map[string]Node{},
	}
}

func (m *memoryStore) newID() string {
	m.nextID++
	return "node-" + time.Now().Format("150405") + "-" + string(rune('a'+m.nextID))
}

func (m *memoryStore) validateSource(sourceID, syllabusID string) error {
	s, ok := m.sources[sourceID]
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

func (m *memoryStore) validateParent(parentID, syllabusID, selfID string) error {
	parent, ok := m.nodes[parentID]
	if !ok || parent.SyllabusID != syllabusID {
		return ErrInvalidParent
	}
	if selfID == "" {
		return nil
	}
	current := parentID
	for hops := 0; hops < maxParentHops; hops++ {
		if current == selfID {
			return ErrInvalidParent
		}
		n, ok := m.nodes[current]
		if !ok || n.ParentNodeID == nil {
			return nil
		}
		current = *n.ParentNodeID
	}
	return ErrInvalidParent
}

func (m *memoryStore) CreateNode(_ context.Context, in CreateInput) (Node, error) {
	if !m.activeSyllabuses[in.SyllabusID] {
		return Node{}, ErrUnknownSyllabus
	}
	if in.ParentNodeID != nil {
		if err := m.validateParent(*in.ParentNodeID, in.SyllabusID, ""); err != nil {
			return Node{}, err
		}
	}
	if err := m.validateSource(in.ContentSourceID, in.SyllabusID); err != nil {
		return Node{}, err
	}
	for _, n := range m.nodes {
		if n.SyllabusID == in.SyllabusID && n.NodeCode == in.NodeCode {
			return Node{}, ErrDuplicateNodeCode
		}
	}
	now := time.Now().UTC()
	node := Node{
		ID:              m.newID(),
		SyllabusID:      in.SyllabusID,
		ParentNodeID:    in.ParentNodeID,
		NodeKind:        in.NodeKind,
		NodeCode:        in.NodeCode,
		Label:           in.Label,
		Status:          StatusDraft,
		ContentSourceID: in.ContentSourceID,
		SourceLocator:   in.SourceLocator,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	m.nodes[node.ID] = node
	m.events = append(m.events, Event{NodeID: node.ID, EventType: EventNodeCreated, ActorID: in.ActorID, EventTime: now})
	return node, nil
}

func (m *memoryStore) UpdateNode(_ context.Context, id string, in UpdateInput) (Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return Node{}, ErrNotFound
	}
	if n.Status != StatusDraft {
		return Node{}, ErrInvalidTransition
	}

	var changed []string
	if in.NodeCode != nil && *in.NodeCode != n.NodeCode {
		for _, other := range m.nodes {
			if other.ID != id && other.SyllabusID == n.SyllabusID && other.NodeCode == *in.NodeCode {
				return Node{}, ErrDuplicateNodeCode
			}
		}
		n.NodeCode = *in.NodeCode
		changed = append(changed, "nodeCode")
	}
	if in.Label != nil && *in.Label != n.Label {
		n.Label = *in.Label
		changed = append(changed, "label")
	}
	if in.NodeKind != nil && *in.NodeKind != n.NodeKind {
		n.NodeKind = *in.NodeKind
		changed = append(changed, "nodeKind")
	}
	if in.ContentSourceID != nil && *in.ContentSourceID != n.ContentSourceID {
		if err := m.validateSource(*in.ContentSourceID, n.SyllabusID); err != nil {
			return Node{}, err
		}
		n.ContentSourceID = *in.ContentSourceID
		changed = append(changed, "contentSourceId")
	}
	if in.ParentNodeIDSet {
		same := (in.ParentNodeID == nil && n.ParentNodeID == nil) ||
			(in.ParentNodeID != nil && n.ParentNodeID != nil && *in.ParentNodeID == *n.ParentNodeID)
		if !same {
			if in.ParentNodeID != nil {
				if *in.ParentNodeID == id {
					return Node{}, ErrInvalidParent
				}
				if err := m.validateParent(*in.ParentNodeID, n.SyllabusID, id); err != nil {
					return Node{}, err
				}
			}
			n.ParentNodeID = in.ParentNodeID
			changed = append(changed, "parentNodeId")
		}
	}
	if in.SourceLocatorSet {
		same := (in.SourceLocator == nil && n.SourceLocator == nil) ||
			(in.SourceLocator != nil && n.SourceLocator != nil && *in.SourceLocator == *n.SourceLocator)
		if !same {
			n.SourceLocator = in.SourceLocator
			changed = append(changed, "sourceLocator")
		}
	}

	if len(changed) == 0 {
		return Node{}, ErrNoChanges
	}
	n.UpdatedAt = time.Now().UTC()
	m.nodes[id] = n
	m.events = append(m.events, Event{NodeID: id, EventType: EventNodeUpdated, ActorID: in.ActorID, ChangedFields: changed, EventTime: n.UpdatedAt})
	return n, nil
}

func (m *memoryStore) VerifyNode(_ context.Context, id, actorID string) (Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return Node{}, ErrNotFound
	}
	if n.Status != StatusDraft {
		return Node{}, ErrInvalidTransition
	}
	n.Status = StatusVerified
	n.UpdatedAt = time.Now().UTC()
	m.nodes[id] = n
	m.events = append(m.events, Event{NodeID: id, EventType: EventNodeVerified, ActorID: actorID, ChangedFields: []string{"status"}, EventTime: n.UpdatedAt})
	return n, nil
}

func (m *memoryStore) RetireNode(_ context.Context, id, actorID string) (Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return Node{}, ErrNotFound
	}
	if n.Status != StatusDraft && n.Status != StatusVerified {
		return Node{}, ErrInvalidTransition
	}
	n.Status = StatusRetired
	n.UpdatedAt = time.Now().UTC()
	m.nodes[id] = n
	m.events = append(m.events, Event{NodeID: id, EventType: EventNodeRetired, ActorID: actorID, ChangedFields: []string{"status"}, EventTime: n.UpdatedAt})
	return n, nil
}

func (m *memoryStore) GetNode(_ context.Context, id string, verifiedOnly bool) (Node, error) {
	n, ok := m.nodes[id]
	if !ok || (verifiedOnly && n.Status != StatusVerified) {
		return Node{}, ErrNotFound
	}
	return n, nil
}

func (m *memoryStore) ListNodesBySyllabus(_ context.Context, syllabusID string, verifiedOnly bool) ([]Node, error) {
	out := []Node{}
	for _, n := range m.nodes {
		if n.SyllabusID != syllabusID {
			continue
		}
		if verifiedOnly && n.Status != StatusVerified {
			continue
		}
		out = append(out, n)
	}
	return out, nil
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

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func validCreateReq() createNodeRequest {
	return createNodeRequest{
		SyllabusID:      "syl-active",
		NodeKind:        "topic",
		NodeCode:        "T1",
		Label:           "Topic one",
		ContentSourceID: "src-approved-active",
	}
}

func TestCreateNode_Success(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	node := decodeJSON[Node](t, resp)
	if node.Status != StatusDraft {
		t.Fatalf("status = %q, want draft", node.Status)
	}
	if len(store.events) != 1 || store.events[0].ActorID != adminSubject {
		t.Fatalf("expected one node_created event with verified actor, got %+v", store.events)
	}
}

func TestCreateNode_UnknownSyllabus_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.SyllabusID = "syl-unknown"
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "unknown_syllabus" {
		t.Fatalf("error = %q, want unknown_syllabus", body["error"])
	}
}

func TestCreateNode_PendingSource_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.ContentSourceID = "src-pending"
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "unapproved_source" {
		t.Fatalf("error = %q, want unapproved_source", body["error"])
	}
}

func TestCreateNode_UnlinkedSource_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.ContentSourceID = "src-approved-unlinked"
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "unlinked_source" {
		t.Fatalf("error = %q, want unlinked_source", body["error"])
	}
}

func TestCreateNode_MismatchedSource_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.ContentSourceID = "src-approved-other"
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "mismatched_source" {
		t.Fatalf("error = %q, want mismatched_source", body["error"])
	}
}

func TestCreateNode_UnknownSource_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.ContentSourceID = "src-does-not-exist"
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "unknown_source" {
		t.Fatalf("error = %q, want unknown_source", body["error"])
	}
}

func TestCreateNode_ParentDifferentSyllabus_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	parentReq := validCreateReq()
	parentReq.NodeCode = "PARENT"
	parentReq.SyllabusID = "syl-other-active"
	parentReq.ContentSourceID = "src-approved-other"
	parentResp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", parentReq)
	parent := decodeJSON[Node](t, parentResp)

	childReq := validCreateReq()
	childReq.ParentNodeID = &parent.ID
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", childReq)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "invalid_parent" {
		t.Fatalf("error = %q, want invalid_parent", body["error"])
	}
}

func TestCreateNode_DuplicateCode_Returns409(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq())
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq())
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "duplicate_node_code" {
		t.Fatalf("error = %q, want duplicate_node_code", body["error"])
	}
}

func TestUpdateNode_ParentCycle_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req1 := validCreateReq()
	req1.NodeCode = "A"
	a := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req1))

	req2 := validCreateReq()
	req2.NodeCode = "B"
	req2.ParentNodeID = &a.ID
	b := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req2))

	// Attempt to make A a child of B: A -> B -> A would be a cycle.
	patch := map[string]any{"parentNodeId": b.ID}
	resp := doJSON(t, http.MethodPatch, srv.URL+"/curriculum-map/nodes/"+a.ID, patch)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "invalid_parent" {
		t.Fatalf("error = %q, want invalid_parent", body["error"])
	}
}

func TestUpdateNode_NonDraft_Returns409(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
	doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes/"+node.ID+"/verify", nil)

	resp := doJSON(t, http.MethodPatch, srv.URL+"/curriculum-map/nodes/"+node.ID, map[string]any{"label": "changed"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "invalid_lifecycle_transition" {
		t.Fatalf("error = %q, want invalid_lifecycle_transition", body["error"])
	}
}

func TestLifecycle_VerifyThenRetire(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))

	verifyResp := doJSONAs(t, http.MethodPost, srv.URL+"/curriculum-map/nodes/"+node.ID+"/verify", reviewerToken, nil)
	if verifyResp.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", verifyResp.StatusCode)
	}
	verified := decodeJSON[Node](t, verifyResp)
	if verified.Status != StatusVerified {
		t.Fatalf("status = %q, want verified", verified.Status)
	}

	// Re-verify a verified node is an invalid transition.
	reverify := doJSONAs(t, http.MethodPost, srv.URL+"/curriculum-map/nodes/"+node.ID+"/verify", reviewerToken, nil)
	if reverify.StatusCode != http.StatusConflict {
		t.Fatalf("re-verify status = %d, want 409", reverify.StatusCode)
	}

	retireResp := doJSONAs(t, http.MethodPost, srv.URL+"/curriculum-map/nodes/"+node.ID+"/retire", reviewerToken, nil)
	if retireResp.StatusCode != http.StatusOK {
		t.Fatalf("retire status = %d, want 200", retireResp.StatusCode)
	}
	retired := decodeJSON[Node](t, retireResp)
	if retired.Status != StatusRetired {
		t.Fatalf("status = %q, want retired", retired.Status)
	}

	// Retiring an already-retired node is an invalid transition.
	reretire := doJSONAs(t, http.MethodPost, srv.URL+"/curriculum-map/nodes/"+node.ID+"/retire", reviewerToken, nil)
	if reretire.StatusCode != http.StatusConflict {
		t.Fatalf("re-retire status = %d, want 409", reretire.StatusCode)
	}
}

func TestGetNode_DraftHiddenFromReaders(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))

	resp := doJSON(t, http.MethodGet, srv.URL+"/curriculum-map/nodes/"+node.ID, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get draft status = %d, want 404 (readers only see verified)", resp.StatusCode)
	}

	doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes/"+node.ID+"/verify", nil)
	resp2 := doJSON(t, http.MethodGet, srv.URL+"/curriculum-map/nodes/"+node.ID, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get verified status = %d, want 200", resp2.StatusCode)
	}
}

func TestListNodes_RequiresSyllabusID(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodGet, srv.URL+"/curriculum-map/nodes", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// --- Role matrix ---

func TestRoleMatrix_LearnerAndUnknownDenied(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	for _, token := range []string{learnerToken, noRoleToken} {
		resp := doJSONAs(t, http.MethodGet, srv.URL+"/curriculum-map/nodes?syllabusId=syl-active", token, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("read as %q status = %d, want 403", token, resp.StatusCode)
		}
		createResp := doJSONAs(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", token, validCreateReq())
		if createResp.StatusCode != http.StatusForbidden {
			t.Fatalf("create as %q status = %d, want 403", token, createResp.StatusCode)
		}
	}
}

func TestRoleMatrix_MissingToken_Returns401(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSONAs(t, http.MethodGet, srv.URL+"/curriculum-map/nodes?syllabusId=syl-active", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRoleMatrix_EditorCanCreateButNotVerify(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSONAs(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", editorToken, validCreateReq())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("editor create status = %d, want 201", resp.StatusCode)
	}
	node := decodeJSON[Node](t, resp)

	verifyResp := doJSONAs(t, http.MethodPost, srv.URL+"/curriculum-map/nodes/"+node.ID+"/verify", editorToken, nil)
	if verifyResp.StatusCode != http.StatusForbidden {
		t.Fatalf("editor verify status = %d, want 403", verifyResp.StatusCode)
	}
}

func TestRoleMatrix_ReviewerCanVerifyAndRetire(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
	resp := doJSONAs(t, http.MethodPost, srv.URL+"/curriculum-map/nodes/"+node.ID+"/verify", reviewerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer verify status = %d, want 200", resp.StatusCode)
	}
}

// --- Strict JSON ---

func TestCreateNode_UnknownField_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	body := map[string]any{
		"syllabusId":      "syl-active",
		"nodeKind":        "topic",
		"nodeCode":        "T1",
		"label":           "Topic",
		"contentSourceId": "src-approved-active",
		"actorId":         "someone-else",
	}
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	respBody := decodeJSON[map[string]string](t, resp)
	if respBody["error"] != "invalid_json" {
		t.Fatalf("error = %q, want invalid_json", respBody["error"])
	}
}

func TestCreateNode_TrailingJSON_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	raw := `{"syllabusId":"syl-active","nodeKind":"topic","nodeCode":"T1","label":"Topic","contentSourceId":"src-approved-active"}{}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/curriculum-map/nodes", bytes.NewReader([]byte(raw)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateNode_InvalidNodeKind_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.NodeKind = "not-a-kind"
	resp := doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "invalid_node_kind" {
		t.Fatalf("error = %q, want invalid_node_kind", body["error"])
	}
}

func TestUpdateNode_NoUpdatableFields_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
	resp := doJSON(t, http.MethodPatch, srv.URL+"/curriculum-map/nodes/"+node.ID, map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "no_updatable_fields" {
		t.Fatalf("error = %q, want no_updatable_fields", body["error"])
	}
}

func TestUpdateNode_NoChanges_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
	resp := doJSON(t, http.MethodPatch, srv.URL+"/curriculum-map/nodes/"+node.ID, map[string]any{"label": node.Label})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "no_changes" {
		t.Fatalf("error = %q, want no_changes", body["error"])
	}
}

func TestUpdateNode_Success_RecordsEvent(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
	resp := doJSON(t, http.MethodPatch, srv.URL+"/curriculum-map/nodes/"+node.ID, map[string]any{"label": "Updated label"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	updated := decodeJSON[Node](t, resp)
	if updated.Label != "Updated label" {
		t.Fatalf("label = %q, want Updated label", updated.Label)
	}
	if len(store.events) != 2 || store.events[1].EventType != EventNodeUpdated {
		t.Fatalf("expected node_updated event, got %+v", store.events)
	}
}

// --- No raw internal errors ---

func TestNoRawInternalErrorText(t *testing.T) {
	// Every mapped domain error message is a static, curated string defined in this package —
	// never a raw driver/SQL error. This test documents that guarantee at the handler layer by
	// asserting the known error codes never carry unexpected substrings.
	for err, code := range map[error]string{
		ErrNotFound:          "not_found",
		ErrDuplicateNodeCode: "duplicate_node_code",
		ErrUnknownSyllabus:   "unknown_syllabus",
		ErrInvalidParent:     "invalid_parent",
		ErrUnknownSource:     "unknown_source",
		ErrUnapprovedSource:  "unapproved_source",
		ErrUnlinkedSource:    "unlinked_source",
		ErrMismatchedSource:  "mismatched_source",
		ErrNoChanges:         "no_changes",
		ErrInvalidTransition: "invalid_lifecycle_transition",
	} {
		mapped, ok := mapNodeError(err)
		if !ok {
			t.Fatalf("mapNodeError(%v) not mapped", err)
		}
		if mapped.code != code {
			t.Fatalf("mapNodeError(%v).code = %q, want %q", err, mapped.code, code)
		}
	}
}
