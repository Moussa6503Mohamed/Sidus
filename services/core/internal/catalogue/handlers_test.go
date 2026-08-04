package catalogue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// fakeVerifier maps opaque test tokens to verified claims so handler tests exercise the real
// auth middleware and role matrix without a live Clerk instance. Unknown tokens verify as
// invalid (401).
type fakeVerifier struct{}

const (
	adminToken    = "admin-token"
	editorToken   = "editor-token"
	reviewerToken = "reviewer-token"
	learnerToken  = "learner-token"
	noRoleToken   = "norole-token"

	adminSubject = "user_admin"
)

func (fakeVerifier) Verify(_ context.Context, token string) (auth.Claims, error) {
	switch token {
	case adminToken:
		return auth.Claims{Subject: adminSubject, Role: auth.RoleAdmin}, nil
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

// memoryStore is an in-memory Store for handler tests. It mirrors PostgresStore's validation
// semantics closely enough to exercise the handlers and role matrix without a live database.
type memoryStore struct {
	subjects      map[string]Subject
	syllabuses    map[string]Syllabus
	events        []Event
	subjectEvents []SubjectEvent
	nextID        int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{subjects: map[string]Subject{}, syllabuses: map[string]Syllabus{}}
}

func (m *memoryStore) id(prefix string) string {
	m.nextID++
	return prefix + "-" + time.Now().Format("150405.000000") + "-" + string(rune('a'+m.nextID))
}

func (m *memoryStore) CreateSubject(_ context.Context, in CreateSubjectInput) (Subject, error) {
	for _, s := range m.subjects {
		if s.Name == in.Name {
			return Subject{}, ErrDuplicateSubject
		}
	}
	now := time.Now().UTC()
	s := Subject{ID: m.id("sub"), Name: in.Name, CreatedAt: now, UpdatedAt: now}
	m.subjects[s.ID] = s
	m.subjectEvents = append(m.subjectEvents, SubjectEvent{
		SubjectID: s.ID, EventType: EventSubjectCreated, ActorID: in.ActorID, ChangedFields: []string{"name"},
	})
	return s, nil
}

func (m *memoryStore) ListSubjects(_ context.Context) ([]Subject, error) {
	out := []Subject{}
	for _, s := range m.subjects {
		out = append(out, s)
	}
	return out, nil
}

func (m *memoryStore) CreateSyllabus(_ context.Context, in CreateSyllabusInput) (Syllabus, error) {
	if _, ok := m.subjects[in.SubjectID]; !ok {
		return Syllabus{}, ErrSubjectNotFound
	}
	track := ""
	if in.Track != nil {
		track = *in.Track
	}
	for _, s := range m.syllabuses {
		st := ""
		if s.Track != nil {
			st = *s.Track
		}
		if s.Board == in.Board && s.SyllabusCode == in.SyllabusCode && st == track {
			return Syllabus{}, ErrDuplicateSyllabus
		}
	}
	status := in.Status
	if status == "" {
		status = StatusDraft
	}
	now := time.Now().UTC()
	s := Syllabus{
		ID: m.id("syl"), Board: in.Board, SyllabusCode: in.SyllabusCode, SubjectID: in.SubjectID,
		Qualification: in.Qualification, Track: in.Track, DisplayName: in.DisplayName,
		CurriculumYear: in.CurriculumYear, Status: status, CreatedAt: now, UpdatedAt: now,
	}
	m.syllabuses[s.ID] = s
	m.events = append(m.events, Event{SyllabusID: s.ID, EventType: EventSyllabusCreated, ActorID: in.ActorID})
	return s, nil
}

func (m *memoryStore) UpdateSyllabus(_ context.Context, id string, in UpdateSyllabusInput) (Syllabus, error) {
	s, ok := m.syllabuses[id]
	if !ok {
		return Syllabus{}, ErrNotFound
	}
	changed := false
	apply := func(cur *string, val *string, set func(string)) {
		if val == nil || (cur != nil && *cur == *val) {
			return
		}
		set(*val)
		changed = true
	}
	apply(&s.Board, in.Board, func(v string) { s.Board = v })
	apply(&s.SyllabusCode, in.SyllabusCode, func(v string) { s.SyllabusCode = v })
	apply(&s.Qualification, in.Qualification, func(v string) { s.Qualification = v })
	apply(&s.DisplayName, in.DisplayName, func(v string) { s.DisplayName = v })
	if in.Track != nil {
		s.Track = in.Track
		changed = true
	}
	if in.CurriculumYear != nil {
		s.CurriculumYear = in.CurriculumYear
		changed = true
	}
	if in.Status != nil && *in.Status != s.Status {
		s.Status = *in.Status
		changed = true
	}
	if !changed {
		return Syllabus{}, ErrNoChanges
	}
	s.UpdatedAt = time.Now().UTC()
	m.syllabuses[id] = s
	m.events = append(m.events, Event{SyllabusID: id, EventType: EventSyllabusUpdated, ActorID: in.ActorID})
	return s, nil
}

func (m *memoryStore) GetSyllabus(_ context.Context, id string, activeOnly bool) (Syllabus, error) {
	s, ok := m.syllabuses[id]
	if !ok || (activeOnly && s.Status != StatusActive) {
		return Syllabus{}, ErrNotFound
	}
	return s, nil
}

func (m *memoryStore) ListSyllabuses(_ context.Context, activeOnly bool) ([]Syllabus, error) {
	out := []Syllabus{}
	for _, s := range m.syllabuses {
		if !activeOnly || s.Status == StatusActive {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *memoryStore) ResolveActiveSyllabusByCode(_ context.Context, code string) (string, bool, error) {
	var ids []string
	for _, s := range m.syllabuses {
		if s.SyllabusCode == code && s.Status == StatusActive {
			ids = append(ids, s.ID)
		}
	}
	if len(ids) != 1 {
		return "", false, nil
	}
	return ids[0], true, nil
}

func newTestServer() (*httptest.Server, *memoryStore) {
	store := newMemoryStore()
	mux := http.NewServeMux()
	Register(mux, store, fakeVerifier{})
	return httptest.NewServer(mux), store
}

func strPtr(s string) *string { return &s }

func doJSONAs(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
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
		t.Fatalf("do: %v", err)
	}
	return resp
}

func doJSON(t *testing.T, method, url string, body any) *http.Response {
	return doJSONAs(t, method, url, adminToken, body)
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}

// seedActiveSyllabus creates a subject + an active syllabus directly in the store.
func seedActiveSyllabus(t *testing.T, store *memoryStore) (Subject, Syllabus) {
	t.Helper()
	ctx := context.Background()
	subj, err := store.CreateSubject(ctx, CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})
	if err != nil {
		t.Fatalf("seed subject: %v", err)
	}
	syl, err := store.CreateSyllabus(ctx, CreateSyllabusInput{
		ActorID: adminSubject, Board: "Cambridge International", SyllabusCode: "0610",
		SubjectID: subj.ID, Qualification: "Cambridge IGCSE", Track: strPtr("Extended"),
		DisplayName: "Cambridge IGCSE Biology 0610 (Extended)", Status: StatusActive,
	})
	if err != nil {
		t.Fatalf("seed syllabus: %v", err)
	}
	return subj, syl
}

func TestCreateSyllabus_Success(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	subj, err := store.CreateSubject(context.Background(), CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})
	if err != nil {
		t.Fatalf("subject: %v", err)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", map[string]any{
		"board": "Cambridge International", "syllabusCode": "0610", "subjectId": subj.ID,
		"qualification": "Cambridge IGCSE", "track": "Extended",
		"displayName": "Cambridge IGCSE Biology 0610 (Extended)", "status": "active",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	syl := decodeJSON[Syllabus](t, resp)
	if syl.ID == "" || syl.Status != StatusActive {
		t.Fatalf("unexpected syllabus %+v", syl)
	}
}

func TestCreateSyllabus_MissingFields(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", map[string]any{
		"board": "Cambridge International",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeJSON[map[string]any](t, resp)
	if body["error"] != "missing_required_fields" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestCreateSyllabus_UnknownSubject_400(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", map[string]any{
		"board": "Cambridge International", "syllabusCode": "0610", "subjectId": "does-not-exist",
		"qualification": "Cambridge IGCSE", "displayName": "X",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if decodeJSON[map[string]any](t, resp)["error"] != "unknown_subject" {
		t.Fatal("want unknown_subject error")
	}
}

func TestCreateSyllabus_Duplicate_409(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	_, _ = seedActiveSyllabus(t, store)
	subj := func() Subject {
		for _, s := range store.subjects {
			return s
		}
		return Subject{}
	}()

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", map[string]any{
		"board": "Cambridge International", "syllabusCode": "0610", "subjectId": subj.ID,
		"qualification": "Cambridge IGCSE", "track": "Extended", "displayName": "dup",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

func TestCreateSyllabus_InvalidStatus_400(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	subj, _ := store.CreateSubject(context.Background(), CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", map[string]any{
		"board": "Cambridge International", "syllabusCode": "0610", "subjectId": subj.ID,
		"qualification": "Cambridge IGCSE", "displayName": "X", "status": "bogus",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestListSyllabuses_ActiveOnly(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	subj, _ := store.CreateSubject(context.Background(), CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})
	_, _ = store.CreateSyllabus(context.Background(), CreateSyllabusInput{
		ActorID: adminSubject, Board: "Cambridge International", SyllabusCode: "0610", SubjectID: subj.ID,
		Qualification: "Cambridge IGCSE", DisplayName: "active one", Status: StatusActive,
	})
	_, _ = store.CreateSyllabus(context.Background(), CreateSyllabusInput{
		ActorID: adminSubject, Board: "Cambridge International", SyllabusCode: "9999", SubjectID: subj.ID,
		Qualification: "Cambridge IGCSE", DisplayName: "draft one", Status: StatusDraft,
	})

	resp := doJSONAs(t, http.MethodGet, srv.URL+"/catalogue/syllabuses", reviewerToken, nil)
	body := decodeJSON[map[string]json.RawMessage](t, resp)
	var items []Syllabus
	if err := json.Unmarshal(body["items"], &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) != 1 || items[0].Status != StatusActive {
		t.Fatalf("items = %+v, want exactly the active one", items)
	}
}

func TestGetSyllabus_DraftHiddenFromReader_404(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	subj, _ := store.CreateSubject(context.Background(), CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})
	draft, _ := store.CreateSyllabus(context.Background(), CreateSyllabusInput{
		ActorID: adminSubject, Board: "Cambridge International", SyllabusCode: "0610", SubjectID: subj.ID,
		Qualification: "Cambridge IGCSE", DisplayName: "draft", Status: StatusDraft,
	})

	resp := doJSONAs(t, http.MethodGet, srv.URL+"/catalogue/syllabuses/"+draft.ID, editorToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (draft not visible to reader)", resp.StatusCode)
	}
}

func TestUpdateSyllabus_Success_And_NoChanges(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	_, syl := seedActiveSyllabus(t, store)

	resp := doJSON(t, http.MethodPatch, srv.URL+"/catalogue/syllabuses/"+syl.ID, map[string]any{"status": "retired"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if decodeJSON[Syllabus](t, resp).Status != StatusRetired {
		t.Fatal("status not updated to retired")
	}

	// Re-applying the same value is a no-op → 400 no_changes.
	resp = doJSON(t, http.MethodPatch, srv.URL+"/catalogue/syllabuses/"+syl.ID, map[string]any{"status": "retired"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 no_changes", resp.StatusCode)
	}
}

func TestUpdateSyllabus_NotFound_404(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	resp := doJSON(t, http.MethodPatch, srv.URL+"/catalogue/syllabuses/missing", map[string]any{"status": "active"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Role matrix ---

func TestCatalogueRoleMatrix_Read(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	_, _ = seedActiveSyllabus(t, store)

	// editor/reviewer/admin can read; learner, unknown, and no-token are denied.
	for _, tc := range []struct {
		token string
		want  int
	}{
		{editorToken, http.StatusOK},
		{reviewerToken, http.StatusOK},
		{adminToken, http.StatusOK},
		{learnerToken, http.StatusForbidden},
		{noRoleToken, http.StatusForbidden},
		{"", http.StatusUnauthorized},
		{"bogus", http.StatusUnauthorized},
	} {
		resp := doJSONAs(t, http.MethodGet, srv.URL+"/catalogue/syllabuses", tc.token, nil)
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatalf("read as %q: got %d, want %d", tc.token, resp.StatusCode, tc.want)
		}
	}
}

func TestCatalogueRoleMatrix_ManageAdminOnly(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	subj, _ := store.CreateSubject(context.Background(), CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})

	body := map[string]any{
		"board": "Cambridge International", "syllabusCode": "0610", "subjectId": subj.ID,
		"qualification": "Cambridge IGCSE", "displayName": "X",
	}
	// Only admin may create; editor/reviewer/learner/unknown are forbidden; no token 401.
	for _, tc := range []struct {
		token string
		want  int
	}{
		{editorToken, http.StatusForbidden},
		{reviewerToken, http.StatusForbidden},
		{learnerToken, http.StatusForbidden},
		{noRoleToken, http.StatusForbidden},
		{"", http.StatusUnauthorized},
	} {
		resp := doJSONAs(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", tc.token, body)
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatalf("manage as %q: got %d, want %d", tc.token, resp.StatusCode, tc.want)
		}
	}
}

func TestCreateSyllabus_ActorFromVerifiedSubject(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	subj, _ := store.CreateSubject(context.Background(), CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", map[string]any{
		"board": "Cambridge International", "syllabusCode": "0610", "subjectId": subj.ID,
		"qualification": "Cambridge IGCSE", "displayName": "X",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	// The recorded event actor must be the verified admin subject, never a body field.
	found := false
	for _, e := range store.events {
		if e.EventType == EventSyllabusCreated {
			found = true
			if e.ActorID != adminSubject {
				t.Fatalf("event actor = %q, want %q", e.ActorID, adminSubject)
			}
		}
	}
	if !found {
		t.Fatal("no syllabus_created event recorded")
	}
}

// TestCreateSyllabus_RejectsUnknownField proves the strict decoder blocks a smuggled actor
// field (identity comes only from the verified subject).
func TestCreateSyllabus_RejectsUnknownField(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	subj, _ := store.CreateSubject(context.Background(), CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", map[string]any{
		"board": "Cambridge International", "syllabusCode": "0610", "subjectId": subj.ID,
		"qualification": "Cambridge IGCSE", "displayName": "X", "actorId": "attacker",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (unknown field rejected)", resp.StatusCode)
	}
}

// --- Subject audit ---

// TestCreateSubject_Success_RecordsVerifiedSubjectEvent proves admin subject creation records a
// subject_created event whose actor is the verified Clerk subject (never a body field), and
// that changed_fields carries only field names, never the submitted value.
func TestCreateSubject_Success_RecordsVerifiedSubjectEvent(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/subjects", map[string]any{"name": "Chemistry"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	subject := decodeJSON[Subject](t, resp)

	found := false
	for _, e := range store.subjectEvents {
		if e.SubjectID != subject.ID {
			continue
		}
		found = true
		if e.EventType != EventSubjectCreated {
			t.Fatalf("event type = %q, want %q", e.EventType, EventSubjectCreated)
		}
		if e.ActorID != adminSubject {
			t.Fatalf("event actor = %q, want %q (verified subject, not a body field)", e.ActorID, adminSubject)
		}
		if len(e.ChangedFields) != 1 || e.ChangedFields[0] != "name" {
			t.Fatalf("changed fields = %v, want [\"name\"] (names only, never the value)", e.ChangedFields)
		}
		for _, f := range e.ChangedFields {
			if f == "Chemistry" {
				t.Fatal("changed_fields must never contain the submitted field value")
			}
		}
	}
	if !found {
		t.Fatal("no subject_created event recorded for the created subject")
	}
}

// TestCreateSubject_Duplicate_NoNewEventRecorded proves a rejected (duplicate) subject creation
// records no new subject event.
func TestCreateSubject_Duplicate_NoNewEventRecorded(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	resp := doJSON(t, http.MethodPost, srv.URL+"/catalogue/subjects", map[string]any{"name": "Physics"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	before := len(store.subjectEvents)

	resp = doJSON(t, http.MethodPost, srv.URL+"/catalogue/subjects", map[string]any{"name": "Physics"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 duplicate_subject", resp.StatusCode)
	}
	if len(store.subjectEvents) != before {
		t.Fatalf("subject event count = %d, want %d (duplicate create must record no event)", len(store.subjectEvents), before)
	}
}

// TestCreateSubject_Unauthorized_NoEventRecorded proves an unauthenticated/unauthorized subject
// creation attempt never reaches the store and records no subject event.
func TestCreateSubject_Unauthorized_NoEventRecorded(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()

	for _, tc := range []struct {
		token string
		want  int
	}{
		{"", http.StatusUnauthorized},
		{"bogus", http.StatusUnauthorized},
		{learnerToken, http.StatusForbidden},
		{editorToken, http.StatusForbidden},
	} {
		resp := doJSONAs(t, http.MethodPost, srv.URL+"/catalogue/subjects", tc.token, map[string]any{"name": "Geology"})
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatalf("create as %q: got %d, want %d", tc.token, resp.StatusCode, tc.want)
		}
	}
	if len(store.subjectEvents) != 0 {
		t.Fatalf("subject event count = %d, want 0 (no unauthorized creation should reach the store)", len(store.subjectEvents))
	}
	if len(store.subjects) != 0 {
		t.Fatalf("subject count = %d, want 0", len(store.subjects))
	}
}

// --- internal_error must never leak raw infrastructure text ---

// sensitiveErrText simulates a raw driver/infrastructure error containing details (DSN-like
// text, table/column names) that must never reach an HTTP client.
const sensitiveErrText = "pq: password authentication failed for user \"sidus_admin\" on host db.internal:5432 (table subjects)"

// failingStore wraps memoryStore but forces every read/write to fail with a raw, detail-bearing
// error, proving the handlers never forward err.Error() for infrastructure failures.
type failingStore struct{ memoryStore }

func newFailingStore() *failingStore { return &failingStore{memoryStore: *newMemoryStore()} }

func (f *failingStore) ListSubjects(context.Context) ([]Subject, error) {
	return nil, errors.New(sensitiveErrText)
}
func (f *failingStore) CreateSubject(context.Context, CreateSubjectInput) (Subject, error) {
	return Subject{}, errors.New(sensitiveErrText)
}
func (f *failingStore) ListSyllabuses(context.Context, bool) ([]Syllabus, error) {
	return nil, errors.New(sensitiveErrText)
}
func (f *failingStore) GetSyllabus(context.Context, string, bool) (Syllabus, error) {
	return Syllabus{}, errors.New(sensitiveErrText)
}
func (f *failingStore) CreateSyllabus(context.Context, CreateSyllabusInput) (Syllabus, error) {
	return Syllabus{}, errors.New(sensitiveErrText)
}
func (f *failingStore) UpdateSyllabus(context.Context, string, UpdateSyllabusInput) (Syllabus, error) {
	return Syllabus{}, errors.New(sensitiveErrText)
}

func TestInternalError_NeverLeaksRawInfrastructureText(t *testing.T) {
	store := newFailingStore()
	// Seed one syllabus directly (bypassing the failing store) so update/get have a target id.
	store.memoryStore.subjects["seed-subject"] = Subject{ID: "seed-subject", Name: "Seed"}
	store.memoryStore.syllabuses["seed-syllabus"] = Syllabus{ID: "seed-syllabus", Status: StatusActive}

	mux := http.NewServeMux()
	Register(mux, store, fakeVerifier{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/catalogue/subjects", nil},
		{http.MethodPost, "/catalogue/subjects", map[string]any{"name": "Astronomy"}},
		{http.MethodGet, "/catalogue/syllabuses", nil},
		{http.MethodGet, "/catalogue/syllabuses/seed-syllabus", nil},
		{http.MethodPost, "/catalogue/syllabuses", map[string]any{
			"board": "X", "syllabusCode": "1", "subjectId": "seed-subject", "qualification": "Q", "displayName": "D",
		}},
		{http.MethodPatch, "/catalogue/syllabuses/seed-syllabus", map[string]any{"status": "retired"}},
	}
	for _, tc := range cases {
		resp := doJSON(t, tc.method, srv.URL+tc.path, tc.body)
		bodyBytes := decodeJSON[map[string]any](t, resp)
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("%s %s: status = %d, want 500", tc.method, tc.path, resp.StatusCode)
		}
		if bodyBytes["error"] != "internal_error" {
			t.Fatalf("%s %s: error = %v, want internal_error", tc.method, tc.path, bodyBytes["error"])
		}
		msg, _ := bodyBytes["message"].(string)
		if msg != internalErrorMessage {
			t.Fatalf("%s %s: message = %q, want stable generic message %q", tc.method, tc.path, msg, internalErrorMessage)
		}
		if strings.Contains(msg, "password") || strings.Contains(msg, "sidus_admin") || strings.Contains(msg, "db.internal") {
			t.Fatalf("%s %s: message leaked raw infrastructure text: %q", tc.method, tc.path, msg)
		}
	}
}

// --- Strict JSON: case-variant fields, identity fields, and trailing data (T-0008) ---
//
// Go's struct decoding matches JSON field names case-insensitively, so
// json.Decoder.DisallowUnknownFields alone accepts `{"SyllabusCode":...}` as `syllabusCode`.
// decodeStrict's allowlist pre-pass is the actual rejection point; these tests prove it fires
// before any store call.

// doRaw issues a request with a raw, pre-serialized body so tests can send malformed,
// case-variant, or multi-value payloads that json.Marshal could never produce.
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

func TestCreateSubject_StrictJSON(t *testing.T) {
	cases := map[string]string{
		"case variant":    `{"Name":"Astronomy"}`,
		"actorId":         `{"name":"Astronomy","actorId":"attacker-supplied"}`,
		"unknown field":   `{"name":"Astronomy","totallyUnknown":"x"}`,
		"trailing object": `{"name":"Astronomy"}{}`,
		"trailing junk":   `{"name":"Astronomy"}garbage`,
		"null body":       `null`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()

			resp := doRaw(t, http.MethodPost, srv.URL+"/catalogue/subjects", adminToken, body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if got := decodeJSON[map[string]any](t, resp); got["error"] != "invalid_json" {
				t.Fatalf("error = %v, want invalid_json", got["error"])
			}
			if len(store.subjects) != 0 {
				t.Fatalf("subjects = %d, want 0 (rejected body must not write)", len(store.subjects))
			}
		})
	}
}

func TestCreateSyllabus_StrictJSON(t *testing.T) {
	srv, store := newTestServer()
	defer srv.Close()
	subj, err := store.CreateSubject(context.Background(), CreateSubjectInput{ActorID: adminSubject, Name: "Biology"})
	if err != nil {
		t.Fatalf("subject: %v", err)
	}

	valid := `"board":"Cambridge International","syllabusCode":"0610","subjectId":"` + subj.ID + `","qualification":"Cambridge IGCSE","displayName":"D"`
	cases := map[string]string{
		"Board case variant": `{"Board":"Cambridge International","syllabusCode":"0610","subjectId":"` + subj.ID + `","qualification":"Cambridge IGCSE","displayName":"D"}`,
		"Label case variant": `{` + valid + `,"Label":"x"}`,
		"actorId":            `{` + valid + `,"actorId":"attacker-supplied"}`,
		"reviewerId":         `{` + valid + `,"reviewerId":"attacker-supplied"}`,
		"trailing object":    `{` + valid + `}{}`,
		"trailing junk":      `{` + valid + `}garbage`,
		"null body":          `null`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			before := len(store.syllabuses)
			resp := doRaw(t, http.MethodPost, srv.URL+"/catalogue/syllabuses", adminToken, body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if got := decodeJSON[map[string]any](t, resp); got["error"] != "invalid_json" {
				t.Fatalf("error = %v, want invalid_json", got["error"])
			}
			if len(store.syllabuses) != before {
				t.Fatalf("syllabuses = %d, want %d (rejected body must not write)", len(store.syllabuses), before)
			}
		})
	}
}

func TestUpdateSyllabus_StrictJSON(t *testing.T) {
	cases := map[string]string{
		"Board case variant":  `{"Board":"New Board"}`,
		"DisplayName variant": `{"DisplayName":"New Name"}`,
		"actorId":             `{"displayName":"New Name","actorId":"attacker-supplied"}`,
		"reviewerId":          `{"displayName":"New Name","reviewerId":"attacker-supplied"}`,
		"unknown field":       `{"displayName":"New Name","totallyUnknown":"x"}`,
		"trailing object":     `{"displayName":"New Name"}{}`,
		"trailing junk":       `{"displayName":"New Name"}garbage`,
		"null body":           `null`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, store := newTestServer()
			defer srv.Close()
			_, syl := seedActiveSyllabus(t, store)
			before := len(store.events)

			resp := doRaw(t, http.MethodPatch, srv.URL+"/catalogue/syllabuses/"+syl.ID, adminToken, body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if got := decodeJSON[map[string]any](t, resp); got["error"] != "invalid_json" {
				t.Fatalf("error = %v, want invalid_json", got["error"])
			}
			if len(store.events) != before {
				t.Fatalf("events = %d, want %d (rejected body must not audit)", len(store.events), before)
			}
		})
	}
}
