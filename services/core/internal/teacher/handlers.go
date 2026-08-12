package teacher

import (
	"encoding/json"
	"errors"
	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
	"io"
	"net/http"
	"strings"
	"time"
)

const internalError = "an internal error occurred"

func Register(mux *http.ServeMux, s Store, v auth.Verifier) {
	h := handler{s}
	mux.HandleFunc("GET /teacher/classes", auth.Protect(v, auth.PermManageTeacherClasses, h.listClasses))
	mux.HandleFunc("POST /teacher/classes", auth.Protect(v, auth.PermManageTeacherClasses, h.createClass))
	mux.HandleFunc("POST /teacher/classes/{id}/invites", auth.Protect(v, auth.PermManageTeacherClasses, h.createInvite))
	mux.HandleFunc("GET /teacher/classes/{id}/roster", auth.Protect(v, auth.PermManageTeacherClasses, h.roster))
	mux.HandleFunc("GET /teacher/classes/{id}/assignments", auth.Protect(v, auth.PermManageTeacherClasses, h.listAssignments))
	mux.HandleFunc("POST /teacher/classes/{id}/assignments", auth.Protect(v, auth.PermManageTeacherClasses, h.createAssignment))
	mux.HandleFunc("POST /learner/class-invitations/accept", auth.Protect(v, auth.PermUseLearnerAssignment, h.acceptInvite))
	mux.HandleFunc("POST /learner/classes/{id}/revoke", auth.Protect(v, auth.PermUseLearnerAssignment, h.revoke))
	mux.HandleFunc("GET /learner/assignments", auth.Protect(v, auth.PermUseLearnerAssignment, h.learnerAssignments))
	mux.HandleFunc("POST /learner/assignments/{id}/start", auth.Protect(v, auth.PermUseLearnerAssignment, h.startAssignment))
}

type handler struct{ s Store }

func subject(r *http.Request) (string, bool) {
	c, ok := auth.ClaimsFromContext(r.Context())
	return c.Subject, ok && strings.TrimSpace(c.Subject) != ""
}
func decode(r *http.Request, target any) bool {
	d := json.NewDecoder(io.LimitReader(r.Body, 8193))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return false
	}
	return d.Decode(&struct{}{}) == io.EOF
}
func empty(r *http.Request) bool {
	b, e := io.ReadAll(io.LimitReader(r.Body, 4097))
	return e == nil && len(strings.TrimSpace(string(b))) == 0
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code string) {
	write(w, status, map[string]string{"error": code, "message": map[string]string{"not_found": "resource not found", "invalid_json": "request body is invalid", "invalid_input": "request is invalid", "invite_unavailable": "invite is unavailable"}[code]})
}
func mapErr(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, ErrNotFound), errors.Is(e, ErrForbidden):
		fail(w, 404, "not_found")
	case errors.Is(e, ErrInviteUnavailable):
		fail(w, 404, "invite_unavailable")
	case errors.Is(e, ErrInvalidAssignment):
		fail(w, 400, "invalid_input")
	default:
		write(w, 500, map[string]string{"error": "internal_error", "message": internalError})
	}
}
func (h handler) createClass(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(r, &in) {
		fail(w, 400, "invalid_json")
		return
	}
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.CreateClass(r.Context(), s, in.Name)
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 201, o)
}
func (h handler) listClasses(w http.ResponseWriter, r *http.Request) {
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.ListClasses(r.Context(), s)
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 200, map[string]any{"items": o})
}
func (h handler) createInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TTLSeconds int `json:"ttlSeconds"`
		MaxUses    int `json:"maxUses"`
	}
	if !decode(r, &in) {
		fail(w, 400, "invalid_json")
		return
	}
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.CreateInvite(r.Context(), s, r.PathValue("id"), time.Duration(in.TTLSeconds)*time.Second, in.MaxUses)
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 201, o)
}
func (h handler) roster(w http.ResponseWriter, r *http.Request) {
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.Roster(r.Context(), s, r.PathValue("id"))
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 200, map[string]any{"items": o})
}
func (h handler) createAssignment(w http.ResponseWriter, r *http.Request) {
	var in CreateAssignmentInput
	if !decode(r, &in) {
		fail(w, 400, "invalid_json")
		return
	}
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.CreateAssignment(r.Context(), s, r.PathValue("id"), in)
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 201, o)
}
func (h handler) listAssignments(w http.ResponseWriter, r *http.Request) {
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.ListAssignments(r.Context(), s, r.PathValue("id"))
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 200, map[string]any{"items": o})
}
func (h handler) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !decode(r, &in) {
		fail(w, 400, "invalid_json")
		return
	}
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.AcceptInvite(r.Context(), s, in.Token)
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 200, o)
}
func (h handler) revoke(w http.ResponseWriter, r *http.Request) {
	if !empty(r) {
		fail(w, 400, "invalid_json")
		return
	}
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	if e := h.s.RevokeMembership(r.Context(), s, r.PathValue("id")); e != nil {
		mapErr(w, e)
		return
	}
	write(w, 204, nil)
}
func (h handler) learnerAssignments(w http.ResponseWriter, r *http.Request) {
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.ListLearnerAssignments(r.Context(), s)
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 200, map[string]any{"items": o})
}
func (h handler) startAssignment(w http.ResponseWriter, r *http.Request) {
	if !empty(r) {
		fail(w, 400, "invalid_json")
		return
	}
	s, ok := subject(r)
	if !ok {
		fail(w, 401, "not_found")
		return
	}
	o, e := h.s.StartAssignment(r.Context(), s, r.PathValue("id"))
	if e != nil {
		mapErr(w, e)
		return
	}
	write(w, 200, o)
}
