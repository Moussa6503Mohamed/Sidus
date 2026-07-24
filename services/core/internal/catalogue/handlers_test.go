package catalogue

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
	subjects   map[string]Subject
	syllabuses map[string]Syllabus
	events     []Event
	nextID     int
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
