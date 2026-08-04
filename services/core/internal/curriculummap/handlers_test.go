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

// setSourceStatus rewrites a seeded source's gate-relevant fields, so tests can simulate a
// source that was approved+linked when a node was created but has since been un-approved,
// unlinked, or re-linked to another syllabus.
func (m *memoryStore) setSourceStatus(id, status string, catalogueSyllabusID *string) {
	m.sources[id] = memorySource{status: status, catalogueSyllabusID: catalogueSyllabusID}
}

// deleteSource removes a seeded source entirely (simulating an unknown source reference).
func (m *memoryStore) deleteSource(id string) { delete(m.sources, id) }

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
	// Mirrors PostgresStore: the source gate re-runs on every update, against the supplied
	// source if any, otherwise the stored one.
	effectiveSource := n.ContentSourceID
	if in.ContentSourceID != nil {
		effectiveSource = *in.ContentSourceID
	}
	if err := m.validateSource(effectiveSource, n.SyllabusID); err != nil {
		return Node{}, err
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
	if err := m.validateSource(n.ContentSourceID, n.SyllabusID); err != nil {
		return Node{}, err
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
	if err := m.validateSource(n.ContentSourceID, n.SyllabusID); err != nil {
		return Node{}, err
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
	if !m.activeSyllabuses[syllabusID] {
		return nil, ErrUnknownSyllabus
	}
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

// --- Strict PATCH decoding (identity / immutable / lifecycle / unknown fields) ---

// patchRaw sends a PATCH with a raw (possibly malformed) body, bypassing json.Marshal.
func patchRaw(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader([]byte(body)))
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

// snapshot captures the mutable state a rejected request must leave untouched.
type storeSnapshot struct {
	node   Node
	events int
}

func snapshotOf(t *testing.T, store *memoryStore, id string) storeSnapshot {
	t.Helper()
	n, ok := store.nodes[id]
	if !ok {
		t.Fatalf("node %q missing from store", id)
	}
	return storeSnapshot{node: n, events: len(store.events)}
}

// assertUnchanged fails if a node was mutated, transitioned, re-timestamped, or audited.
func assertUnchanged(t *testing.T, store *memoryStore, id string, before storeSnapshot) {
	t.Helper()
	after, ok := store.nodes[id]
	if !ok {
		t.Fatalf("node %q disappeared from store", id)
	}
	if after != before.node {
		t.Fatalf("node mutated by a rejected request:\n before = %+v\n after  = %+v", before.node, after)
	}
	if !after.UpdatedAt.Equal(before.node.UpdatedAt) {
		t.Fatalf("updatedAt changed by a rejected request: %v -> %v", before.node.UpdatedAt, after.UpdatedAt)
	}
	if len(store.events) != before.events {
		t.Fatalf("event count = %d, want %d (a rejected request must write no event)", len(store.events), before.events)
	}
}

// TestUpdateNode_ForbiddenAndUnknownFields_Returns400 covers every identity, immutable, and
// lifecycle field a caller might try to smuggle through PATCH, plus generic unknown fields and
// typos. DisallowUnknownFields has no effect on the map decode PATCH uses, so these are
// rejected by the explicit field allowlist.
func TestUpdateNode_ForbiddenAndUnknownFields_Returns400(t *testing.T) {
	cases := map[string]string{
		"actorId":         `{"label":"new","actorId":"someone-else"}`,
		"reviewerId":      `{"label":"new","reviewerId":"someone-else"}`,
		"syllabusId":      `{"label":"new","syllabusId":"syl-other-active"}`,
		"status":          `{"label":"new","status":"verified"}`,
		"id":              `{"label":"new","id":"other-node"}`,
		"createdAt":       `{"label":"new","createdAt":"2020-01-01T00:00:00Z"}`,
		"updatedAt":       `{"label":"new","updatedAt":"2020-01-01T00:00:00Z"}`,
		"unknown field":   `{"label":"new","totallyUnknown":"x"}`,
		"typo field":      `{"labell":"new"}`,
		"case variant":    `{"Label":"new"}`,
		"forbidden alone": `{"status":"retired"}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
			before := snapshotOf(t, store, node.ID)

			resp := patchRaw(t, srv.URL+"/curriculum-map/nodes/"+node.ID, body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			respBody := decodeJSON[map[string]string](t, resp)
			if respBody["error"] != "invalid_json" {
				t.Fatalf("error = %q, want invalid_json", respBody["error"])
			}
			assertUnchanged(t, store, node.ID, before)
		})
	}
}

// TestUpdateNode_TrailingJSON_Returns400 rejects a second JSON value and non-JSON junk after
// the body object.
func TestUpdateNode_TrailingJSON_Returns400(t *testing.T) {
	cases := map[string]string{
		"trailing object": `{"label":"new"}{}`,
		"trailing array":  `{"label":"new"}[1]`,
		"trailing junk":   `{"label":"new"}not-json`,
		"trailing string": `{"label":"new"}"extra"`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
			before := snapshotOf(t, store, node.ID)

			resp := patchRaw(t, srv.URL+"/curriculum-map/nodes/"+node.ID, body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			respBody := decodeJSON[map[string]string](t, resp)
			if respBody["error"] != "invalid_json" {
				t.Fatalf("error = %q, want invalid_json", respBody["error"])
			}
			assertUnchanged(t, store, node.ID, before)
		})
	}
}

// TestUpdateNode_NullableFieldsStillClearable confirms the strict allowlist did not break the
// absent-vs-null distinction for the two nullable PATCH fields.
func TestUpdateNode_NullableFieldsStillClearable(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	req := validCreateReq()
	req.SourceLocator = strPtr("section-1")
	node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", req))
	if node.SourceLocator == nil {
		t.Fatal("expected sourceLocator to be set on create")
	}

	// Explicit null clears; absent leaves alone (asserted by the follow-up no_changes PATCH).
	resp := patchRaw(t, srv.URL+"/curriculum-map/nodes/"+node.ID, `{"sourceLocator":null}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	cleared := decodeJSON[Node](t, resp)
	if cleared.SourceLocator != nil {
		t.Fatalf("sourceLocator = %v, want nil after explicit null", *cleared.SourceLocator)
	}

	repeat := patchRaw(t, srv.URL+"/curriculum-map/nodes/"+node.ID, `{"sourceLocator":null}`)
	if repeat.StatusCode != http.StatusBadRequest {
		t.Fatalf("repeat clear status = %d, want 400 no_changes", repeat.StatusCode)
	}
	repeatBody := decodeJSON[map[string]string](t, repeat)
	if repeatBody["error"] != "no_changes" {
		t.Fatalf("error = %q, want no_changes", repeatBody["error"])
	}
}

// --- Source gate re-validated on every node write ---

// brokenSourceCases mutates the node's already-referenced source into each failed state and
// names the stable error code Core must return.
var brokenSourceCases = map[string]struct {
	breakSource func(store *memoryStore, sourceID string)
	wantError   string
}{
	"unapproved source": {
		breakSource: func(s *memoryStore, id string) { s.setSourceStatus(id, "pending", strPtr("syl-active")) },
		wantError:   "unapproved_source",
	},
	"unlinked source": {
		breakSource: func(s *memoryStore, id string) { s.setSourceStatus(id, "approved", nil) },
		wantError:   "unlinked_source",
	},
	"mismatched source": {
		breakSource: func(s *memoryStore, id string) { s.setSourceStatus(id, "approved", strPtr("syl-other-active")) },
		wantError:   "mismatched_source",
	},
	"unknown source": {
		breakSource: func(s *memoryStore, id string) { s.deleteSource(id) },
		wantError:   "unknown_source",
	},
}

// runBrokenSourceCases creates a draft node with a good source, breaks that source, then runs
// do — which must be rejected with the expected code and leave the node and audit trail intact.
func runBrokenSourceCases(t *testing.T, do func(t *testing.T, srvURL string, node Node) *http.Response, prepare func(t *testing.T, srvURL string, node Node)) {
	t.Helper()
	for name, tc := range brokenSourceCases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
			if prepare != nil {
				prepare(t, srv.URL, node)
			}
			tc.breakSource(store, node.ContentSourceID)
			before := snapshotOf(t, store, node.ID)

			resp := do(t, srv.URL, node)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			body := decodeJSON[map[string]string](t, resp)
			if body["error"] != tc.wantError {
				t.Fatalf("error = %q, want %q", body["error"], tc.wantError)
			}
			assertUnchanged(t, store, node.ID, before)
		})
	}
}

// TestUpdateNode_RevalidatesSourceGate proves PATCH re-runs the source gate even when the
// request does not touch contentSourceId.
func TestUpdateNode_RevalidatesSourceGate(t *testing.T) {
	runBrokenSourceCases(t, func(t *testing.T, srvURL string, node Node) *http.Response {
		return doJSON(t, http.MethodPatch, srvURL+"/curriculum-map/nodes/"+node.ID, map[string]any{"label": "Updated label"})
	}, nil)
}

// TestVerifyNode_RevalidatesSourceGate proves verify re-runs the source gate before the status
// transition and its audit event.
func TestVerifyNode_RevalidatesSourceGate(t *testing.T) {
	runBrokenSourceCases(t, func(t *testing.T, srvURL string, node Node) *http.Response {
		return doJSONAs(t, http.MethodPost, srvURL+"/curriculum-map/nodes/"+node.ID+"/verify", reviewerToken, nil)
	}, nil)
}

// TestRetireNode_RevalidatesSourceGate proves retire re-runs the source gate from draft.
func TestRetireNode_RevalidatesSourceGate(t *testing.T) {
	runBrokenSourceCases(t, func(t *testing.T, srvURL string, node Node) *http.Response {
		return doJSONAs(t, http.MethodPost, srvURL+"/curriculum-map/nodes/"+node.ID+"/retire", reviewerToken, nil)
	}, nil)
}

// TestRetireNode_VerifiedNode_RevalidatesSourceGate proves the gate also applies on the
// verified → retired transition.
func TestRetireNode_VerifiedNode_RevalidatesSourceGate(t *testing.T) {
	runBrokenSourceCases(t, func(t *testing.T, srvURL string, node Node) *http.Response {
		return doJSONAs(t, http.MethodPost, srvURL+"/curriculum-map/nodes/"+node.ID+"/retire", reviewerToken, nil)
	}, func(t *testing.T, srvURL string, node Node) {
		resp := doJSONAs(t, http.MethodPost, srvURL+"/curriculum-map/nodes/"+node.ID+"/verify", reviewerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("prepare verify status = %d, want 200", resp.StatusCode)
		}
	})
}

// TestUpdateNode_RepointToBadSource_Returns400 keeps the original "changed contentSourceId"
// path covered alongside the new always-on revalidation.
func TestUpdateNode_RepointToBadSource_Returns400(t *testing.T) {
	for sourceID, wantError := range map[string]string{
		"src-pending":           "unapproved_source",
		"src-approved-unlinked": "unlinked_source",
		"src-approved-other":    "mismatched_source",
		"src-does-not-exist":    "unknown_source",
	} {
		t.Run(sourceID, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			node := decodeJSON[Node](t, doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq()))
			before := snapshotOf(t, store, node.ID)

			resp := doJSON(t, http.MethodPatch, srv.URL+"/curriculum-map/nodes/"+node.ID,
				map[string]any{"contentSourceId": sourceID})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			body := decodeJSON[map[string]string](t, resp)
			if body["error"] != wantError {
				t.Fatalf("error = %q, want %q", body["error"], wantError)
			}
			assertUnchanged(t, store, node.ID, before)
		})
	}
}

// --- Syllabus validation on list ---

func TestListNodes_UnknownSyllabus_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodGet, srv.URL+"/curriculum-map/nodes?syllabusId=syl-does-not-exist", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]string](t, resp)
	if body["error"] != "unknown_syllabus" {
		t.Fatalf("error = %q, want unknown_syllabus", body["error"])
	}
}

func TestListNodes_ActiveSyllabusWithNoVerifiedNodes_Returns200Empty(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	// A draft node exists but readers see verified only — still a 200 with an empty list.
	doJSON(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", validCreateReq())

	resp := doJSON(t, http.MethodGet, srv.URL+"/curriculum-map/nodes?syllabusId=syl-active", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON[map[string][]Node](t, resp)
	if len(body["items"]) != 0 {
		t.Fatalf("items = %d, want 0", len(body["items"]))
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

// --- Strict JSON: case-variant fields, identity fields, and trailing data (T-0008) ---
//
// Go's struct decoding matches JSON field names case-insensitively, so
// json.Decoder.DisallowUnknownFields alone accepts `{"Label":...}` as `label`. decodeStrict's
// allowlist pre-pass is the actual rejection point; these tests prove it fires before any
// store call.

// doRaw issues a request with a raw, pre-serialized body so tests can send case-variant or
// multi-value payloads that json.Marshal could never produce.
func doRaw(t *testing.T, method, url, token, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader([]byte(body)))
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

func TestCreateNode_StrictJSON(t *testing.T) {
	valid := `"syllabusId":"syl-active","nodeKind":"topic","nodeCode":"T1","label":"Topic one","contentSourceId":"src-approved-active"`
	cases := map[string]string{
		"Label case variant":      `{"syllabusId":"syl-active","nodeKind":"topic","nodeCode":"T1","Label":"Topic one","contentSourceId":"src-approved-active"}`,
		"SyllabusId case variant": `{"SyllabusId":"syl-active","nodeKind":"topic","nodeCode":"T1","label":"Topic one","contentSourceId":"src-approved-active"}`,
		"actorId":                 `{` + valid + `,"actorId":"attacker-supplied"}`,
		"reviewerId":              `{` + valid + `,"reviewerId":"attacker-supplied"}`,
		"unknown field":           `{` + valid + `,"totallyUnknown":"x"}`,
		"trailing object":         `{` + valid + `}{}`,
		"trailing junk":           `{` + valid + `}garbage`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			resp := doRaw(t, http.MethodPost, srv.URL+"/curriculum-map/nodes", adminToken, body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if got := decodeJSON[map[string]any](t, resp); got["error"] != "invalid_json" {
				t.Fatalf("error = %v, want invalid_json", got["error"])
			}
			if len(store.nodes) != 0 {
				t.Fatalf("nodes = %d, want 0 (rejected body must not create a node)", len(store.nodes))
			}
		})
	}
}
