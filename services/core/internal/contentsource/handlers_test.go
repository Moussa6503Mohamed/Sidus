package contentsource

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// memoryStore is an in-memory Store used only for handler tests, so they run without a
// live Postgres instance. It mirrors PostgresStore's validation semantics.
type memoryStore struct {
	sources map[string]Source
	reviews []Review
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

func newTestServer() (*httptest.Server, *memoryStore) {
	store := newMemoryStore()
	mux := http.NewServeMux()
	Register(mux, store)
	return httptest.NewServer(mux), store
}

func strPtr(s string) *string { return &s }

func doJSON(t *testing.T, method, url string, body any) *http.Response {
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

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/approve", reviewRequest{ReviewerID: "reviewer-1"})
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

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/approve", reviewRequest{ReviewerID: "reviewer-1"})
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

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/approve", reviewRequest{ReviewerID: "reviewer-1"})
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

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/approve", reviewRequest{ReviewerID: "reviewer-1"})
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

func TestReject_Success(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{Title: "Bio syllabus", SourceURL: "https://example.org/bio-4"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/reject", reviewRequest{ReviewerID: "reviewer-1", Reason: "licence unclear"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	rejected := decodeJSON[Source](t, resp)
	if rejected.Status != StatusRejected {
		t.Fatalf("status = %q, want rejected", rejected.Status)
	}
}
