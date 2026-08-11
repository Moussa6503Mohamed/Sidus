package uploadintake

import (
	"bytes"
	"context"
	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeVerifier struct{ claims auth.Claims }

func (f fakeVerifier) Verify(context.Context, string) (auth.Claims, error) { return f.claims, nil }

type memoryStore struct{ created int }

func (m *memoryStore) Create(_ context.Context, in CreateInput) (Upload, error) {
	m.created++
	return Upload{ID: "u", OriginalFilename: in.OriginalFilename, Status: StatusQuarantined}, nil
}
func (m *memoryStore) List(context.Context) ([]Upload, error) { return []Upload{}, nil }
func (m *memoryStore) MarkScanClean(context.Context, string, string) (Upload, error) {
	return Upload{}, ErrNotFound
}
func (m *memoryStore) RequestDeletion(context.Context, string, string) (Upload, error) {
	return Upload{}, ErrNotFound
}
func (m *memoryStore) QueueReview(context.Context, string, string, string) (ReviewJob, error) {
	return ReviewJob{}, ErrNotFound
}
func testServer(t *testing.T) (*memoryStore, *httptest.Server) {
	t.Helper()
	s := &memoryStore{}
	q, e := NewLocalQuarantineStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	Register(mux, s, q, fakeVerifier{auth.Claims{Subject: "admin", Role: auth.RoleAdmin}})
	return s, httptest.NewServer(mux)
}
func req(t *testing.T, url, ct, name, body string) *http.Response {
	t.Helper()
	r, e := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	r.Header.Set("Authorization", "Bearer x")
	r.Header.Set("Content-Type", ct)
	r.Header.Set("X-Sidus-Upload-Filename", name)
	out, e := http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestCreateRejectsNonPDFBeforeStore(t *testing.T) {
	s, h := testServer(t)
	defer h.Close()
	for _, tc := range []struct{ ct, n, b string }{{"text/plain", "x.pdf", "%PDF-x"}, {"application/pdf", "x.txt", "%PDF-x"}, {"application/pdf", "x.pdf", "hello"}} {
		r := req(t, h.URL+"/private-uploads", tc.ct, tc.n, tc.b)
		if r.StatusCode != 400 {
			t.Fatalf("status %d", r.StatusCode)
		}
		_ = r.Body.Close()
	}
	if s.created != 0 {
		t.Fatal("store called")
	}
}
func TestCreateAcceptsPDF(t *testing.T) {
	s, h := testServer(t)
	defer h.Close()
	r := req(t, h.URL+"/private-uploads", "application/pdf", "safe.pdf", "%PDF-test")
	defer r.Body.Close()
	if r.StatusCode != 201 {
		t.Fatalf("status %d", r.StatusCode)
	}
	if s.created != 1 {
		t.Fatal("missing create")
	}
}

func TestDecodeQueueReviewBodyIsExact(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"ContentSourceId":"x"}`, `{"contentSourceId":"x","x":1}`,
		`{"contentSourceId":"x","contentSourceId":"y"}`, `{"contentSourceId":""}`, `{"contentSourceId":"x"}{}`,
	} {
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(raw))
		if _, ok := decodeQueueReviewBody(r); ok {
			t.Fatalf("accepted %s", raw)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"contentSourceId":" x "}`))
	v, ok := decodeQueueReviewBody(r)
	if !ok || v != "x" {
		t.Fatalf("got %q %v", v, ok)
	}
}
