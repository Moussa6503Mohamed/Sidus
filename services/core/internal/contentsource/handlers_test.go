package contentsource

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

// fakeVerifier maps opaque test tokens to verified claims so handler tests exercise the
// real auth middleware and role matrix without any live Clerk instance or cryptography.
// Any unknown token verifies as invalid (mirrors a missing/expired/forged token -> 401).
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

// memoryStore is an in-memory Store used only for handler tests, so they run without a
// live Postgres instance. It mirrors PostgresStore's validation semantics.
type memoryStore struct {
	sources map[string]Source
	reviews []Review
	events  []Event
	nextID  int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{sources: map[string]Source{}}
}

func (m *memoryStore) newID() string {
	m.nextID++
	return "id-" + time.Now().Format("150405") + "-" + string(rune('a'+m.nextID))
}

func (m *memoryStore) Create(_ context.Context, in CreateInput) (Source, error) {
	for _, s := range m.sources {
		if s.SourceURL == in.SourceURL {
			return Source{}, ErrDuplicateSourceURL
		}
	}
	now := time.Now().UTC()
	source := Source{
		ID:               m.newID(),
		Title:            in.Title,
		Owner:            in.Owner,
		SourceURL:        in.SourceURL,
		SourceHash:       in.SourceHash,
		LicenceReference: in.LicenceReference,
		PermittedUse:     in.PermittedUse,
		AllowedAudience:  in.AllowedAudience,
		SyllabusCode:     in.SyllabusCode,
		Status:           StatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	m.sources[source.ID] = source
	return source, nil
}

func (m *memoryStore) Get(_ context.Context, id string) (Source, error) {
	s, ok := m.sources[id]
	if !ok {
		return Source{}, ErrNotFound
	}
	return s, nil
}

func (m *memoryStore) List(_ context.Context, status *Status) ([]Source, error) {
	out := []Source{}
	for _, s := range m.sources {
		if status == nil || s.Status == *status {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *memoryStore) Approve(_ context.Context, id string, in ApproveInput) (Source, []string, error) {
	s, ok := m.sources[id]
	if !ok {
		return Source{}, nil, ErrNotFound
	}
	if s.Status != StatusPending {
		return Source{}, nil, ErrInvalidTransition
	}
	if missing := MissingApprovalFields(s); len(missing) > 0 {
		return s, missing, nil
	}
	s.Status = StatusApproved
	s.UpdatedAt = time.Now().UTC()
	m.sources[id] = s
	m.reviews = append(m.reviews, Review{ContentSourceID: id, Decision: StatusApproved, ReviewerID: in.ReviewerID, DecisionDate: in.DecisionDate})
	return s, nil, nil
}

func (m *memoryStore) Reject(_ context.Context, id string, in RejectInput) (Source, error) {
	s, ok := m.sources[id]
	if !ok {
		return Source{}, ErrNotFound
	}
	if s.Status != StatusPending {
		return Source{}, ErrInvalidTransition
	}
	s.Status = StatusRejected
	s.UpdatedAt = time.Now().UTC()
	m.sources[id] = s
	reason := in.Reason
	m.reviews = append(m.reviews, Review{ContentSourceID: id, Decision: StatusRejected, ReviewerID: in.ReviewerID, DecisionDate: in.DecisionDate, Reason: &reason})
	return s, nil
}

func (m *memoryStore) Update(_ context.Context, id string, in UpdateInput) (Source, []string, error) {
	s, ok := m.sources[id]
	if !ok {
		return Source{}, nil, ErrNotFound
	}
	if s.Status != StatusPending {
		return Source{}, nil, ErrInvalidTransition
	}

	cols := []struct {
		field   string
		value   *string
		current *string
		apply   func(v string)
	}{
		{"title", in.Title, &s.Title, func(v string) { s.Title = v }},
		{"owner", in.Owner, s.Owner, func(v string) { s.Owner = strPtr(v) }},
		{"sourceUrl", in.SourceURL, &s.SourceURL, func(v string) { s.SourceURL = v }},
		{"sourceHash", in.SourceHash, s.SourceHash, func(v string) { s.SourceHash = strPtr(v) }},
		{"licenceReference", in.LicenceReference, s.LicenceReference, func(v string) { s.LicenceReference = strPtr(v) }},
		{"permittedUse", in.PermittedUse, s.PermittedUse, func(v string) { s.PermittedUse = strPtr(v) }},
		{"allowedAudience", in.AllowedAudience, s.AllowedAudience, func(v string) { s.AllowedAudience = strPtr(v) }},
		{"syllabusCode", in.SyllabusCode, s.SyllabusCode, func(v string) { s.SyllabusCode = strPtr(v) }},
	}
	var changed []string
	suppliedCount := 0
	for _, c := range cols {
		if c.value == nil {
			continue
		}
		suppliedCount++
		if c.current != nil && *c.current == *c.value {
			continue // supplied value matches what is already stored: not a real change
		}
		if in.SourceURL != nil && c.field == "sourceUrl" {
			for oid, other := range m.sources {
				if oid != id && other.SourceURL == *in.SourceURL {
					return Source{}, nil, ErrDuplicateSourceURL
				}
			}
		}
		c.apply(*c.value)
		changed = append(changed, c.field)
	}
	if suppliedCount == 0 {
		return Source{}, nil, ErrNoUpdatableFields
	}
	if len(changed) == 0 {
		return Source{}, nil, ErrNoChanges
	}

	s.UpdatedAt = time.Now().UTC()
	m.sources[id] = s
	m.events = append(m.events, Event{ContentSourceID: id, EventType: EventMetadataUpdated, ActorID: in.ActorID, ChangedFields: changed, EventTime: s.UpdatedAt})
	return s, changed, nil
}

func newTestServer() (*httptest.Server, *memoryStore) {
	store := newMemoryStore()
	mux := http.NewServeMux()
	Register(mux, store, fakeVerifier{})
	return httptest.NewServer(mux), store
}

func strPtr(s string) *string { return &s }

// doJSON issues a request authenticated as an admin (full content-source permissions), so
// behavior tests focus on business rules rather than auth. Auth-specific tests use doJSONAs.
func doJSON(t *testing.T, method, url string, body any) *http.Response {
	return doJSONAs(t, method, url, adminToken, body)
}

// doJSONAs issues a request bearing the given token. An empty token sends no Authorization
// header (the missing-token case).
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

func TestCreate_Success(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources", createRequest{
		Title:     "Cambridge IGCSE Biology 0610 syllabus",
		SourceURL: "https://example.org/0610-syllabus",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	source := decodeJSON[Source](t, resp)
	if source.Status != StatusPending {
		t.Fatalf("status = %q, want pending", source.Status)
	}
	if source.ID == "" {
		t.Fatal("expected generated id")
	}
}

func TestCreate_MissingRequiredFields(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources", createRequest{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "missing_required_fields" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestGet_NotFound(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodGet, srv.URL+"/content-sources/missing", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestList_FiltersByStatus(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	pending, _ := store.Create(ctx, CreateInput{Title: "A", SourceURL: "https://example.org/a"})
	_, _ = store.Create(ctx, CreateInput{Title: "B", SourceURL: "https://example.org/b"})
	_, _, _ = store.Approve(ctx, pending.ID, ApproveInput{ReviewerID: "never-approved-without-fields", DecisionDate: time.Now()})

	resp := doJSON(t, http.MethodGet, srv.URL+"/content-sources?status=pending", nil)
	body := decodeJSON[map[string]json.RawMessage](t, resp)
	var items []Source
	if err := json.Unmarshal(body["items"], &items); err != nil {
		t.Fatalf("unmarshal items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2 (approve should have been blocked on missing fields)", len(items))
	}
}

func TestApprove_MissingRightsFields(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{Title: "Bio syllabus", SourceURL: "https://example.org/bio"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/approve", reviewRequest{})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "missing_required_fields" {
		t.Fatalf("error = %v", body["error"])
	}
	missing, ok := body["missing"].([]any)
	if !ok || len(missing) == 0 {
		t.Fatalf("missing = %v, want non-empty list", body["missing"])
	}
}

func TestApprove_SucceedsWhenAllRightsFieldsPresent(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{
		Title:            "Bio syllabus",
		SourceURL:        "https://example.org/bio-complete",
		Owner:            strPtr("Cambridge Assessment International Education"),
		SourceHash:       strPtr("sha256:abc123"),
		LicenceReference: strPtr("CAIE-PUBLIC-SYLLABUS-2026"),
		PermittedUse:     strPtr("Link, version metadata, human-reviewed topic/objective mapping only"),
		AllowedAudience:  strPtr("internal-editorial"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/approve", reviewRequest{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	approved := decodeJSON[Source](t, resp)
	if approved.Status != StatusApproved {
		t.Fatalf("status = %q, want approved", approved.Status)
	}
}

func TestApprove_NotPending(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{Title: "Bio syllabus", SourceURL: "https://example.org/bio-2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.Reject(ctx, source.ID, RejectInput{ReviewerID: "reviewer-1", Reason: "no licence", DecisionDate: time.Now()}); err != nil {
		t.Fatalf("reject: %v", err)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/approve", reviewRequest{})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

func TestReject_RequiresReasonAndReviewer(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{Title: "Bio syllabus", SourceURL: "https://example.org/bio-3"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Reviewer identity now comes from the verified session, so a reject with no reason is
	// the only missing-field case left at the HTTP boundary.
	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/reject", reviewRequest{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestApprove_WhitespaceOnlyRightsFieldsCannotPass(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{
		Title:            "Bio syllabus",
		SourceURL:        "https://example.org/bio-whitespace",
		Owner:            strPtr(" "),
		SourceHash:       strPtr(" "),
		LicenceReference: strPtr(" "),
		PermittedUse:     strPtr(" "),
		AllowedAudience:  strPtr(" "),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/approve", reviewRequest{})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	body := decodeJSON[map[string]any](t, resp)
	missing, ok := body["missing"].([]any)
	if !ok {
		t.Fatalf("missing = %v, want list", body["missing"])
	}
	wantMissing := []string{"owner", "sourceHash", "licenceReference", "permittedUse", "allowedAudience"}
	if len(missing) != len(wantMissing) {
		t.Fatalf("missing = %v, want %v", missing, wantMissing)
	}
	for i, field := range wantMissing {
		if missing[i] != field {
			t.Fatalf("missing[%d] = %v, want %q", i, missing[i], field)
		}
	}
}

func TestMissingApprovalFields_RejectsWhitespaceOnlyValues(t *testing.T) {
	blank := " "
	source := Source{
		Title:            blank,
		SourceURL:        blank,
		Owner:            &blank,
		SourceHash:       &blank,
		LicenceReference: &blank,
		PermittedUse:     &blank,
		AllowedAudience:  &blank,
	}
	missing := MissingApprovalFields(source)
	want := []string{"owner", "title", "sourceUrl", "sourceHash", "licenceReference", "permittedUse", "allowedAudience"}
	if len(missing) != len(want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
	for i, field := range want {
		if missing[i] != field {
			t.Fatalf("missing[%d] = %v, want %q", i, missing[i], field)
		}
	}
}

func TestList_InvalidStatus_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodGet, srv.URL+"/content-sources?status=bogus", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_status" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestList_ValidStatus_Returns200(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	for _, status := range []string{"pending", "approved", "rejected", "expired"} {
		resp := doJSON(t, http.MethodGet, srv.URL+"/content-sources?status="+status, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%q: got %d, want %d", status, resp.StatusCode, http.StatusOK)
		}
	}
}

func TestCreate_InvalidSyllabusCode_Returns400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources", createRequest{
		Title:        "Bio syllabus",
		SourceURL:    "https://example.org/bio-bad-syllabus",
		SyllabusCode: strPtr("9999"),
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_syllabus_code" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestCreate_ValidSyllabusCode_Returns201(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	for _, code := range []string{"0610", "5090"} {
		resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources", createRequest{
			Title:        "Bio syllabus " + code,
			SourceURL:    "https://example.org/bio-" + code,
			SyllabusCode: strPtr(code),
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("syllabusCode=%q: got %d, want %d", code, resp.StatusCode, http.StatusCreated)
		}
	}
}

func TestUpdate_Success(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{Title: "Bio syllabus", SourceURL: "https://example.org/upd-1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		Owner:            strPtr("Cambridge Assessment International Education"),
		LicenceReference: strPtr("CAIE-PUBLIC-SYLLABUS-2026"),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	updated := decodeJSON[Source](t, resp)
	if updated.Owner == nil || *updated.Owner != "Cambridge Assessment International Education" {
		t.Fatalf("owner = %v, want set", updated.Owner)
	}
	if updated.Status != StatusPending {
		t.Fatalf("status = %q, want still pending (PATCH never approves)", updated.Status)
	}
}

func TestUpdate_CreatesImmutableEvent(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{Title: "Bio syllabus", SourceURL: "https://example.org/upd-event"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		Owner: strPtr("CAIE"),
		Title: strPtr("Cambridge IGCSE Biology 0610 syllabus"),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(store.events) != 1 {
		t.Fatalf("events = %d, want 1", len(store.events))
	}
	ev := store.events[0]
	if ev.EventType != EventMetadataUpdated {
		t.Fatalf("eventType = %q, want %q", ev.EventType, EventMetadataUpdated)
	}
	// Actor is the verified Clerk subject of the request (admin token), never a body field.
	if ev.ActorID != adminSubject {
		t.Fatalf("actorId = %q, want %q (verified subject)", ev.ActorID, adminSubject)
	}
	want := []string{"title", "owner"}
	if len(ev.ChangedFields) != len(want) {
		t.Fatalf("changedFields = %v, want %v", ev.ChangedFields, want)
	}
	for i, f := range want {
		if ev.ChangedFields[i] != f {
			t.Fatalf("changedFields[%d] = %q, want %q", i, ev.ChangedFields[i], f)
		}
	}
}

func TestUpdate_WhitespaceOnlyValue_Returns400(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, _ := store.Create(ctx, CreateInput{Title: "Bio", SourceURL: "https://example.org/upd-ws"})

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		Owner: strPtr("   "),
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %d, want 0 (rejected update must not audit)", len(store.events))
	}
}

func TestUpdate_InvalidSyllabusCode_Returns400(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, _ := store.Create(ctx, CreateInput{Title: "Bio", SourceURL: "https://example.org/upd-syl"})

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		SyllabusCode: strPtr("9999"),
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_syllabus_code" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestUpdate_InvalidSourceURL_Returns400(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, _ := store.Create(ctx, CreateInput{Title: "Bio", SourceURL: "https://example.org/upd-url"})

	for _, bad := range []string{"not-a-url", "ftp://example.org/x", "//example.org/x", "example.org/x"} {
		resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
			SourceURL: strPtr(bad),
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("sourceUrl=%q: status = %d, want %d", bad, resp.StatusCode, http.StatusBadRequest)
		}
		body := decodeJSON[map[string]any](t, resp)
		if body["error"] != "invalid_source_url" {
			t.Fatalf("sourceUrl=%q: error = %v", bad, body["error"])
		}
	}
}

func TestUpdate_DuplicateURL_Returns409(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	_, _ = store.Create(ctx, CreateInput{Title: "A", SourceURL: "https://example.org/dup-a"})
	target, _ := store.Create(ctx, CreateInput{Title: "B", SourceURL: "https://example.org/dup-b"})

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+target.ID, updateRequest{
		SourceURL: strPtr("https://example.org/dup-a"),
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "duplicate_source_url" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestUpdate_NonPending_Returns409(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, _ := store.Create(ctx, CreateInput{Title: "Bio", SourceURL: "https://example.org/upd-np"})
	if _, err := store.Reject(ctx, source.ID, RejectInput{ReviewerID: "r1", Reason: "no licence", DecisionDate: time.Now()}); err != nil {
		t.Fatalf("reject: %v", err)
	}

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		Owner: strPtr("CAIE"),
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_status_transition" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestUpdate_MixedSameAndNewValues_RecordsOnlyChangedFields(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{
		Title:     "Bio syllabus",
		SourceURL: "https://example.org/upd-mixed",
		Owner:     strPtr("Cambridge Assessment International Education"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		Owner:            strPtr("Cambridge Assessment International Education"), // same as stored
		LicenceReference: strPtr("CAIE-PUBLIC-SYLLABUS-2026"),                    // new
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(store.events) != 1 {
		t.Fatalf("events = %d, want 1", len(store.events))
	}
	want := []string{"licenceReference"}
	got := store.events[0].ChangedFields
	if len(got) != len(want) {
		t.Fatalf("changedFields = %v, want %v", got, want)
	}
	for i, f := range want {
		if got[i] != f {
			t.Fatalf("changedFields[%d] = %q, want %q", i, got[i], f)
		}
	}
}

func TestUpdate_AllSameValues_Returns400NoChanges(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{
		Title:     "Bio syllabus",
		SourceURL: "https://example.org/upd-nochange",
		Owner:     strPtr("Cambridge Assessment International Education"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		Title: strPtr("Bio syllabus"),
		Owner: strPtr("Cambridge Assessment International Education"),
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "no_changes" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestUpdate_NoChangeRequest_NoEventAndNoUpdatedAtChange(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{
		Title:     "Bio syllabus",
		SourceURL: "https://example.org/upd-noop",
		Owner:     strPtr("Cambridge Assessment International Education"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	before, err := store.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		Owner: strPtr("Cambridge Assessment International Education"),
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %d, want 0 (no-change request must not audit)", len(store.events))
	}
	after, err := store.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("updatedAt changed: before=%v after=%v, want unchanged", before.UpdatedAt, after.UpdatedAt)
	}
}

func TestUpdate_NoUpdatableFields_Returns400(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, _ := store.Create(ctx, CreateInput{Title: "Bio", SourceURL: "https://example.org/upd-empty"})

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "no_updatable_fields" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestReject_Success(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{Title: "Bio syllabus", SourceURL: "https://example.org/bio-4"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/reject", reviewRequest{Reason: "licence unclear"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	rejected := decodeJSON[Source](t, resp)
	if rejected.Status != StatusRejected {
		t.Fatalf("status = %q, want rejected", rejected.Status)
	}
}

// --- Authentication and authorization (T-0003) ---

// TestAuth_MissingToken_401 covers every content-source endpoint refusing an unauthenticated
// request before any handler logic runs.
func TestAuth_MissingToken_401(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/content-sources", createRequest{Title: "x", SourceURL: "https://example.org/x"}},
		{http.MethodGet, "/content-sources", nil},
		{http.MethodGet, "/content-sources/some-id", nil},
		{http.MethodPatch, "/content-sources/some-id", updateRequest{Owner: strPtr("x")}},
		{http.MethodPost, "/content-sources/some-id/approve", reviewRequest{}},
		{http.MethodPost, "/content-sources/some-id/reject", reviewRequest{Reason: "x"}},
	}
	for _, c := range cases {
		resp := doJSONAs(t, c.method, srv.URL+c.path, "", c.body)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s: status = %d, want 401", c.method, c.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestAuth_InvalidToken_401 covers an unrecognized/forged token being rejected.
func TestAuth_InvalidToken_401(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSONAs(t, http.MethodGet, srv.URL+"/content-sources", "forged-token", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_token" {
		t.Fatalf("error = %v, want invalid_token", body["error"])
	}
}

// TestAuth_LearnerForbidden_403 covers a valid session whose role has no content-source
// access being denied on every endpoint.
func TestAuth_LearnerForbidden_403(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/learner"})

	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/content-sources", createRequest{Title: "x", SourceURL: "https://example.org/lx"}},
		{http.MethodGet, "/content-sources", nil},
		{http.MethodGet, "/content-sources/" + src.ID, nil},
		{http.MethodPatch, "/content-sources/" + src.ID, updateRequest{Owner: strPtr("x")}},
		{http.MethodPost, "/content-sources/" + src.ID + "/approve", reviewRequest{}},
		{http.MethodPost, "/content-sources/" + src.ID + "/reject", reviewRequest{Reason: "x"}},
	}
	for _, c := range cases {
		resp := doJSONAs(t, c.method, srv.URL+c.path, learnerToken, c.body)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s: status = %d, want 403", c.method, c.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestAuth_MissingRole_403 covers a valid session with a missing/unknown role being denied
// by default.
func TestAuth_MissingRole_403(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSONAs(t, http.MethodGet, srv.URL+"/content-sources", noRoleToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

// TestAuth_EditorPermissions covers editor being able to read/create/update but not
// approve/reject.
func TestAuth_EditorPermissions(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/editor"})

	// Allowed: list.
	if resp := doJSONAs(t, http.MethodGet, srv.URL+"/content-sources", editorToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("editor list: status = %d, want 200", resp.StatusCode)
	}
	// Allowed: create.
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources", editorToken, createRequest{Title: "New", SourceURL: "https://example.org/editor-new"}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("editor create: status = %d, want 201", resp.StatusCode)
	}
	// Allowed: update.
	if resp := doJSONAs(t, http.MethodPatch, srv.URL+"/content-sources/"+src.ID, editorToken, updateRequest{Owner: strPtr("CAIE")}); resp.StatusCode != http.StatusOK {
		t.Fatalf("editor update: status = %d, want 200", resp.StatusCode)
	}
	// Forbidden: approve and reject.
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/approve", editorToken, reviewRequest{}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("editor approve: status = %d, want 403", resp.StatusCode)
	}
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/reject", editorToken, reviewRequest{Reason: "x"}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("editor reject: status = %d, want 403", resp.StatusCode)
	}
}

// TestAuth_ReviewerPermissions covers reviewer having editor permissions plus reject/approve.
func TestAuth_ReviewerPermissions(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/reviewer"})

	// Allowed: update (editor permission).
	if resp := doJSONAs(t, http.MethodPatch, srv.URL+"/content-sources/"+src.ID, reviewerToken, updateRequest{Owner: strPtr("CAIE")}); resp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer update: status = %d, want 200", resp.StatusCode)
	}
	// Allowed: reject.
	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/reject", reviewerToken, reviewRequest{Reason: "licence unclear"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer reject: status = %d, want 200", resp.StatusCode)
	}
}

// TestAuth_AdminPermissions covers admin being able to approve (all permissions).
func TestAuth_AdminPermissions(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{
		Title:            "Bio",
		SourceURL:        "https://example.org/admin",
		Owner:            strPtr("CAIE"),
		SourceHash:       strPtr("sha256:abc"),
		LicenceReference: strPtr("REF"),
		PermittedUse:     strPtr("metadata only"),
		AllowedAudience:  strPtr("internal"),
	})

	if resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/approve", adminToken, reviewRequest{}); resp.StatusCode != http.StatusOK {
		t.Fatalf("admin approve: status = %d, want 200", resp.StatusCode)
	}
}

// TestUpdate_ActorFromVerifiedSubject proves the audit actor is the verified subject and can
// never be spoofed from the request body.
func TestUpdate_ActorFromVerifiedSubject(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/actor-subject"})

	resp := doJSONAs(t, http.MethodPatch, srv.URL+"/content-sources/"+src.ID, editorToken, updateRequest{Owner: strPtr("CAIE")})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(store.events) != 1 {
		t.Fatalf("events = %d, want 1", len(store.events))
	}
	if store.events[0].ActorID != editorSubject {
		t.Fatalf("actorId = %q, want %q (verified subject)", store.events[0].ActorID, editorSubject)
	}
}

// TestReject_ReviewerFromVerifiedSubject proves the review reviewer is the verified subject.
func TestReject_ReviewerFromVerifiedSubject(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/reviewer-subject"})

	resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/reject", reviewerToken, reviewRequest{Reason: "licence unclear"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(store.reviews) != 1 {
		t.Fatalf("reviews = %d, want 1", len(store.reviews))
	}
	if store.reviews[0].ReviewerID != reviewerSubject {
		t.Fatalf("reviewerId = %q, want %q (verified subject)", store.reviews[0].ReviewerID, reviewerSubject)
	}
}

// --- Legacy caller-identity fields must be rejected (T-0003 hardening) ---

// TestUpdate_RejectsUnknownActorId proves a body carrying the legacy `actorId` field is
// rejected with a stable 400 invalid_json, cannot influence the audited actor, and produces
// no event. Identity comes only from the verified subject.
func TestUpdate_RejectsUnknownActorId(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/actorid-reject"})

	resp := doJSONAs(t, http.MethodPatch, srv.URL+"/content-sources/"+src.ID, editorToken, map[string]any{
		"owner":   "CAIE",
		"actorId": "attacker-supplied",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_json" {
		t.Fatalf("error = %v, want invalid_json", body["error"])
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %d, want 0 (rejected body must not audit)", len(store.events))
	}
}

// TestApprove_RejectsUnknownReviewerId proves a body carrying the legacy `reviewerId` field is
// rejected with 400 invalid_json before any state change; the reviewer can never be spoofed.
func TestApprove_RejectsUnknownReviewerId(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{
		Title:            "Bio",
		SourceURL:        "https://example.org/reviewerid-approve",
		Owner:            strPtr("CAIE"),
		SourceHash:       strPtr("sha256:abc"),
		LicenceReference: strPtr("REF"),
		PermittedUse:     strPtr("metadata only"),
		AllowedAudience:  strPtr("internal"),
	})

	resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/approve", adminToken, map[string]any{
		"reviewerId": "attacker-supplied",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_json" {
		t.Fatalf("error = %v, want invalid_json", body["error"])
	}
	if len(store.reviews) != 0 {
		t.Fatalf("reviews = %d, want 0 (rejected body must not record a review)", len(store.reviews))
	}
	got := decodeJSON[Source](t, doJSONAs(t, http.MethodGet, srv.URL+"/content-sources/"+src.ID, adminToken, nil))
	if got.Status != StatusPending {
		t.Fatalf("status = %q, want still pending (approve was rejected)", got.Status)
	}
}

// TestReject_RejectsUnknownReviewerId mirrors the approve case for the reject endpoint.
func TestReject_RejectsUnknownReviewerId(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/reviewerid-reject"})

	resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/reject", reviewerToken, map[string]any{
		"reason":     "licence unclear",
		"reviewerId": "attacker-supplied",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_json" {
		t.Fatalf("error = %v, want invalid_json", body["error"])
	}
	if len(store.reviews) != 0 {
		t.Fatalf("reviews = %d, want 0 (rejected body must not record a review)", len(store.reviews))
	}
}

// TestCreate_RejectsUnknownField proves the create endpoint also rejects unknown fields, so no
// caller-controlled identity or stray field can slip through.
func TestCreate_RejectsUnknownField(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSONAs(t, http.MethodPost, srv.URL+"/content-sources", adminToken, map[string]any{
		"title":     "Bio",
		"sourceUrl": "https://example.org/create-unknown",
		"actorId":   "attacker-supplied",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_json" {
		t.Fatalf("error = %v, want invalid_json", body["error"])
	}
}

// --- decodeStrict must accept exactly one JSON value (T-0003 final hardening) ---

// doRaw issues a request with a raw, pre-serialized body so tests can send malformed or
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

// TestCreate_RejectsTrailingJSONValue proves a body consisting of a valid JSON object followed
// by a second valid JSON value is rejected: json.Decoder.Decode alone happily reads just the
// first value and silently ignores the rest.
func TestCreate_RejectsTrailingJSONValue(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	body := `{"title":"Bio","sourceUrl":"https://example.org/trailing-json"}{"actorId":"attacker-supplied"}`
	resp := doRaw(t, http.MethodPost, srv.URL+"/content-sources", adminToken, body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	got := decodeJSON[map[string]any](t, resp)
	if got["error"] != "invalid_json" {
		t.Fatalf("error = %v, want invalid_json", got["error"])
	}
	if len(store.sources) != 0 {
		t.Fatalf("sources = %d, want 0 (trailing JSON must not create a source)", len(store.sources))
	}
}

// TestUpdate_RejectsTrailingJunk proves a body with valid JSON followed by non-whitespace junk
// (not even valid JSON) is rejected and produces no audit event.
func TestUpdate_RejectsTrailingJunk(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/trailing-junk"})

	body := `{"owner":"CAIE"}garbage`
	resp := doRaw(t, http.MethodPatch, srv.URL+"/content-sources/"+src.ID, editorToken, body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	got := decodeJSON[map[string]any](t, resp)
	if got["error"] != "invalid_json" {
		t.Fatalf("error = %v, want invalid_json", got["error"])
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %d, want 0 (rejected trailing-body request must not audit)", len(store.events))
	}
}

// TestReject_RejectsTrailingJSONValue mirrors the trailing-value case for the reject endpoint,
// proving no review is recorded when the body carries a second JSON value.
func TestReject_RejectsTrailingJSONValue(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/trailing-reject"})

	body := `{"reason":"licence unclear"} {"reviewerId":"attacker-supplied"}`
	resp := doRaw(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/reject", reviewerToken, body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	got := decodeJSON[map[string]any](t, resp)
	if got["error"] != "invalid_json" {
		t.Fatalf("error = %v, want invalid_json", got["error"])
	}
	if len(store.reviews) != 0 {
		t.Fatalf("reviews = %d, want 0 (trailing JSON must not record a review)", len(store.reviews))
	}
}

// TestApprove_TrailingWhitespace_Accepted proves trailing whitespace after the single JSON
// value is accepted (this is not "trailing data" — only non-whitespace/extra-value bytes are).
func TestApprove_TrailingWhitespace_Accepted(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{
		Title:            "Bio",
		SourceURL:        "https://example.org/trailing-whitespace",
		Owner:            strPtr("CAIE"),
		SourceHash:       strPtr("sha256:abc"),
		LicenceReference: strPtr("REF"),
		PermittedUse:     strPtr("metadata only"),
		AllowedAudience:  strPtr("internal"),
	})

	body := "{}\n\t \n"
	resp := doRaw(t, http.MethodPost, srv.URL+"/content-sources/"+src.ID+"/approve", adminToken, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// TestUpdate_RejectsUnknownActorId_WithTrailingWhitespace proves whitespace-only trailing bytes
// after an otherwise-rejected body still surface the original unknown-field error, not a
// different failure mode.
func TestUpdate_RejectsUnknownActorId_WithTrailingWhitespace(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	src, _ := store.Create(context.Background(), CreateInput{Title: "Bio", SourceURL: "https://example.org/actorid-ws"})

	body := `{"owner":"CAIE","actorId":"attacker-supplied"}` + "\n"
	resp := doRaw(t, http.MethodPatch, srv.URL+"/content-sources/"+src.ID, editorToken, body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	got := decodeJSON[map[string]any](t, resp)
	if got["error"] != "invalid_json" {
		t.Fatalf("error = %v, want invalid_json", got["error"])
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %d, want 0", len(store.events))
	}
}
