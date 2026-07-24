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

func TestUpdate_Success(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, err := store.Create(ctx, CreateInput{Title: "Bio syllabus", SourceURL: "https://example.org/upd-1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		ActorID:          strPtr("curator-1"),
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
		ActorID: strPtr("curator-1"),
		Owner:   strPtr("CAIE"),
		Title:   strPtr("Cambridge IGCSE Biology 0610 syllabus"),
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
	if ev.ActorID != "curator-1" {
		t.Fatalf("actorId = %q, want curator-1", ev.ActorID)
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
		ActorID: strPtr("curator-1"),
		Owner:   strPtr("   "),
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
		ActorID:      strPtr("curator-1"),
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
			ActorID:   strPtr("curator-1"),
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
		ActorID:   strPtr("curator-1"),
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
		ActorID: strPtr("curator-1"),
		Owner:   strPtr("CAIE"),
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "invalid_status_transition" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestUpdate_MissingActorID_Returns400(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	source, _ := store.Create(ctx, CreateInput{Title: "Bio", SourceURL: "https://example.org/upd-actor"})

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		Owner: strPtr("CAIE"),
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "missing_required_fields" {
		t.Fatalf("error = %v", body["error"])
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %d, want 0", len(store.events))
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
		ActorID:          strPtr("curator-1"),
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
		ActorID: strPtr("curator-1"),
		Title:   strPtr("Bio syllabus"),
		Owner:   strPtr("Cambridge Assessment International Education"),
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
		ActorID: strPtr("curator-1"),
		Owner:   strPtr("Cambridge Assessment International Education"),
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

	resp := doJSON(t, http.MethodPatch, srv.URL+"/content-sources/"+source.ID, updateRequest{
		ActorID: strPtr("curator-1"),
	})
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

	resp := doJSON(t, http.MethodPost, srv.URL+"/content-sources/"+source.ID+"/reject", reviewRequest{ReviewerID: "reviewer-1", Reason: "licence unclear"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	rejected := decodeJSON[Source](t, resp)
	if rejected.Status != StatusRejected {
		t.Fatalf("status = %q, want rejected", rejected.Status)
	}
}
