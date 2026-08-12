package teacher

import (
	"context"
	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeVerifier struct{}

func (fakeVerifier) Verify(context.Context, string) (auth.Claims, error) {
	return auth.Claims{Subject: "teacher-a", Role: auth.RoleTeacher}, nil
}

type fakeStore struct {
	gotOwner string
	accept   bool
}

func (s *fakeStore) CreateClass(_ context.Context, o, n string) (Class, error) {
	s.gotOwner = o
	return Class{ID: "class-1", Name: n, Status: "active"}, nil
}
func (s *fakeStore) ListClasses(context.Context, string) ([]Class, error) { return nil, nil }
func (s *fakeStore) CreateInvite(context.Context, string, string, time.Duration, int) (Invite, error) {
	return Invite{}, nil
}
func (s *fakeStore) Roster(context.Context, string, string) ([]RosterMember, error) { return nil, nil }
func (s *fakeStore) AcceptInvite(_ context.Context, o, _ string) (Class, error) {
	s.gotOwner = o
	s.accept = true
	return Class{ID: "class-1"}, nil
}
func (s *fakeStore) RevokeMembership(context.Context, string, string) error { return nil }
func (s *fakeStore) CreateAssignment(context.Context, string, string, CreateAssignmentInput) (Assignment, error) {
	return Assignment{}, nil
}
func (s *fakeStore) ListAssignments(context.Context, string, string) ([]Assignment, error) {
	return nil, nil
}
func (s *fakeStore) ListLearnerAssignments(context.Context, string) ([]LearnerAssignment, error) {
	return nil, nil
}
func (s *fakeStore) StartAssignment(context.Context, string, string) (StartedAssignment, error) {
	return StartedAssignment{}, nil
}
func TestTeacherCreateUsesVerifiedOwner(t *testing.T) {
	s := &fakeStore{}
	m := http.NewServeMux()
	Register(m, s, fakeVerifier{})
	r := httptest.NewRequest(http.MethodPost, "/teacher/classes", http.NoBody)
	r.Header.Set("Authorization", "Bearer x")
	w := httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest(http.MethodPost, "/teacher/classes", strings.NewReader(`{"name":"Private class"}`))
	r.Header.Set("Authorization", "Bearer x")
	w = httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 201 || s.gotOwner != "teacher-a" {
		t.Fatalf("%d %q", w.Code, s.gotOwner)
	}
}
func TestInviteAcceptIsLearnerScoped(t *testing.T) {
	s := &fakeStore{}
	m := http.NewServeMux()
	Register(m, s, fakeVerifier{})
	r := httptest.NewRequest(http.MethodPost, "/learner/class-invitations/accept", strings.NewReader(`{"token":"abcdefghijklmnopqrstuvwxyz"}`))
	r.Header.Set("Authorization", "Bearer x")
	w := httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 200 || !s.accept || s.gotOwner != "teacher-a" {
		t.Fatalf("%d %#v %q", w.Code, s.accept, s.gotOwner)
	}
}
