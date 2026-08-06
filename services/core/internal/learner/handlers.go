package learner

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// Register mounts the learner-facing question delivery routes on mux. Both routes require
// learner_question:read, held by every recognized role (learner, editor, reviewer, admin);
// auth.Protect denies an unknown/missing role before the handler ever runs. This package never
// widens or reuses the editorial /questions routes.
func Register(mux *http.ServeMux, store Store, v auth.Verifier) {
	h := &handler{store: store}
	mux.HandleFunc("GET /learner/questions", auth.Protect(v, auth.PermReadLearnerQuestion, h.listQuestions))
	mux.HandleFunc("GET /learner/questions/{id}", auth.Protect(v, auth.PermReadLearnerQuestion, h.getQuestion))
	mux.HandleFunc("GET /learner/syllabuses", auth.Protect(v, auth.PermReadLearnerQuestion, h.listSyllabuses))
}

type handler struct {
	store Store
}

// internalErrorMessage is the only text ever returned for database, scan, or other
// infrastructure failures. Raw Go/driver error text must never reach an HTTP response.
const internalErrorMessage = "an internal error occurred"

func (h *handler) listQuestions(w http.ResponseWriter, r *http.Request) {
	syllabusID := strings.TrimSpace(r.URL.Query().Get("syllabusId"))
	if syllabusID == "" {
		writeMissingFields(w, []string{"syllabusId"})
		return
	}
	var nodeID *string
	if raw := strings.TrimSpace(r.URL.Query().Get("curriculumMapNodeId")); raw != "" {
		nodeID = &raw
	}

	items, err := h.store.ListQuestions(r.Context(), syllabusID, nodeID)
	if mapped, ok := mapLearnerError(err); ok {
		writeLearnerError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) getQuestion(w http.ResponseWriter, r *http.Request) {
	q, err := h.store.GetQuestion(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "question not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (h *handler) listSyllabuses(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListActiveSyllabuses(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// --- Error mapping ---

type learnerErrorMapping struct {
	status  int
	code    string
	message string
}

func mapLearnerError(err error) (learnerErrorMapping, bool) {
	switch {
	case errors.Is(err, ErrUnknownSyllabus):
		return learnerErrorMapping{http.StatusBadRequest, "unknown_syllabus", err.Error()}, true
	case errors.Is(err, ErrUnknownNode):
		return learnerErrorMapping{http.StatusBadRequest, "unknown_node", err.Error()}, true
	case errors.Is(err, ErrMismatchedNode):
		return learnerErrorMapping{http.StatusBadRequest, "mismatched_node", err.Error()}, true
	default:
		return learnerErrorMapping{}, false
	}
}

func writeLearnerError(w http.ResponseWriter, m learnerErrorMapping) {
	writeError(w, m.status, m.code, m.message)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func writeMissingFields(w http.ResponseWriter, missing []string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_required_fields", "missing": missing})
}
