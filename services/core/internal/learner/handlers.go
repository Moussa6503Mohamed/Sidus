package learner

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// Register mounts the learner-facing question delivery routes on mux. Both routes require
// learner_question:read, held by every recognized role (learner, editor, reviewer, admin);
// auth.Protect denies an unknown/missing role before the handler ever runs. This package never
// widens or reuses the editorial /questions routes.
func Register(mux *http.ServeMux, store Store, v auth.Verifier, markers ...SonnetMarker) {
	h := &handler{store: store}
	mux.HandleFunc("GET /learner/questions", auth.Protect(v, auth.PermReadLearnerQuestion, h.listQuestions))
	mux.HandleFunc("GET /learner/questions/{id}", auth.Protect(v, auth.PermReadLearnerQuestion, h.getQuestion))
	mux.HandleFunc("GET /learner/syllabuses", auth.Protect(v, auth.PermReadLearnerQuestion, h.listSyllabuses))
	mux.HandleFunc("GET /learner/modules", auth.Protect(v, auth.PermReadLearnerQuestion, h.listModules))
	mux.HandleFunc("POST /learner/questions/{id}/attempts", auth.Protect(v, auth.PermUseLearnerAttempt, h.createAttempt))
	mux.HandleFunc("POST /learner/attempts/{id}/submit", auth.Protect(v, auth.PermUseLearnerAttempt, h.submitAttempt))
	if markingStore, ok := store.(MarkingStore); ok {
		var marker SonnetMarker
		if len(markers) == 1 {
			marker = markers[0]
		}
		mh := &markingHandler{store: markingStore, marker: marker}
		mux.HandleFunc("POST /learner/attempts/{id}/marking", auth.Protect(v, auth.PermUseLearnerAttempt, mh.request))
		mux.HandleFunc("GET /learner/attempts/{id}/marking", auth.Protect(v, auth.PermUseLearnerAttempt, mh.get))
	}
	if sessions, ok := store.(SessionStore); ok {
		sh := &sessionHandler{store: sessions}
		mux.HandleFunc("POST /learner/assessment-sessions", auth.Protect(v, auth.PermUseLearnerAttempt, sh.create))
		mux.HandleFunc("GET /learner/assessment-sessions/{id}", auth.Protect(v, auth.PermUseLearnerAttempt, sh.get))
		mux.HandleFunc("PATCH /learner/assessment-sessions/{id}/responses", auth.Protect(v, auth.PermUseLearnerAttempt, sh.save))
		mux.HandleFunc("POST /learner/assessment-sessions/{id}/submit", auth.Protect(v, auth.PermUseLearnerAttempt, sh.submit))
		mux.HandleFunc("GET /learner/assessment-sessions/{id}/result", auth.Protect(v, auth.PermUseLearnerAttempt, sh.result))
	}
}

func learnerSubject(r *http.Request) (string, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	return claims.Subject, ok && strings.TrimSpace(claims.Subject) != ""
}

func (h *handler) createAttempt(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 4097))
	if err != nil || len(data) > 4096 || len(bytes.TrimSpace(data)) != 0 {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be empty")
		return
	}
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}
	attempt, err := h.store.CreateAttempt(r.Context(), subject, r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "question not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusCreated, attempt)
}

var submitFields = map[string]struct{}{"selectedOptionId": {}, "selectedOptionIds": {}, "text": {}, "value": {}}

func decodeSubmit(r *http.Request) (string, bool) {
	dec := json.NewDecoder(io.LimitReader(r.Body, 4097))
	fields, ok := exactObjectFromDecoder(dec, submitFields)
	if !ok || len(fields) != 1 {
		return "", false
	}
	if selectedRaw, ok := fields["selectedOptionId"]; ok {
		var selected string
		if json.Unmarshal(selectedRaw, &selected) != nil {
			return "", false
		}
		selected = strings.TrimSpace(selected)
		return selected, selected != "" && len([]rune(selected)) <= 64
	}
	if selectedRaw, ok := fields["selectedOptionIds"]; ok {
		var ids []string
		if json.Unmarshal(selectedRaw, &ids) != nil || len(ids) == 0 || len(ids) > 6 {
			return "", false
		}
		seen := map[string]struct{}{}
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" || len([]rune(id)) > 64 {
				return "", false
			}
			if _, dup := seen[id]; dup {
				return "", false
			}
			seen[id] = struct{}{}
		}
		normalized, _ := json.Marshal(map[string]any{"selectedOptionIds": ids})
		return string(normalized), true
	}
	if textRaw, ok := fields["text"]; ok {
		var text string
		if json.Unmarshal(textRaw, &text) != nil {
			return "", false
		}
		text = strings.TrimSpace(text)
		if text == "" || len([]rune(text)) > 5000 {
			return "", false
		}
		normalized, _ := json.Marshal(map[string]string{"text": text})
		return string(normalized), true
	}
	if valueRaw, ok := fields["value"]; ok {
		var value float64
		if json.Unmarshal(valueRaw, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return "", false
		}
		normalized, _ := json.Marshal(map[string]float64{"value": value})
		return string(normalized), true
	}
	return "", false
}

func exactObjectFromDecoder(dec *json.Decoder, allowed map[string]struct{}) (map[string]json.RawMessage, bool) {
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return nil, false
	}
	fields := map[string]json.RawMessage{}
	for dec.More() {
		keyToken, err := dec.Token()
		key, stringKey := keyToken.(string)
		if err != nil || !stringKey {
			return nil, false
		}
		if _, allowedKey := allowed[key]; !allowedKey {
			return nil, false
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, false
		}
		var raw json.RawMessage
		if dec.Decode(&raw) != nil {
			return nil, false
		}
		fields[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return fields, true
}

func (h *handler) submitAttempt(w http.ResponseWriter, r *http.Request) {
	selected, ok := decodeSubmit(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is invalid")
		return
	}
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}
	result, err := h.store.SubmitAttempt(r.Context(), subject, r.PathValue("id"), selected)
	switch {
	case errors.Is(err, ErrAttemptNotFound):
		writeError(w, http.StatusNotFound, "not_found", "attempt not found")
	case errors.Is(err, ErrAttemptSubmitted):
		writeError(w, http.StatusConflict, "attempt_already_submitted", "attempt already submitted")
	case errors.Is(err, ErrInvalidOption):
		writeError(w, http.StatusBadRequest, "invalid_option", "selected option is invalid")
	case errors.Is(err, ErrInvalidAnswer):
		writeError(w, http.StatusBadRequest, "invalid_answer", "answer is invalid")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
	default:
		writeJSON(w, http.StatusOK, result)
	}
}

type handler struct {
	store Store
}

type markingHandler struct {
	store  MarkingStore
	marker SonnetMarker
}

func emptyRequestBody(r *http.Request) bool {
	data, err := io.ReadAll(io.LimitReader(r.Body, 4097))
	return err == nil && len(data) <= 4096 && len(bytes.TrimSpace(data)) == 0
}
func (h *markingHandler) request(w http.ResponseWriter, r *http.Request) {
	if !emptyRequestBody(r) {
		writeError(w, 400, "invalid_json", "request body must be empty")
		return
	}
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, 401, "unauthorized", "authentication is required")
		return
	}
	job, projection, created, err := h.store.RequestMarking(r.Context(), subject, r.PathValue("id"))
	if errors.Is(err, ErrMarkingIneligible) {
		writeError(w, 409, "marking_ineligible", "attempt is not eligible for written marking")
		return
	}
	if errors.Is(err, ErrMarkingNotFound) {
		writeError(w, 404, "not_found", "attempt not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal_error", internalErrorMessage)
		return
	}
	// A missing provider is deliberately a pending result, not a fabricated mark or an auth bypass.
	if created && h.marker != nil {
		outcome, callErr := h.marker.MarkWrittenAttempt(r.Context(), job)
		if callErr != nil {
			outcome = MarkingOutcome{Status: MarkingPending, Reason: "provider_unavailable"}
		}
		projection, err = h.store.ApplyMarking(r.Context(), job.RequestID, outcome)
		if err != nil {
			writeError(w, 500, "internal_error", internalErrorMessage)
			return
		}
	}
	writeJSON(w, http.StatusAccepted, projection)
}
func (h *markingHandler) get(w http.ResponseWriter, r *http.Request) {
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, 401, "unauthorized", "authentication is required")
		return
	}
	p, err := h.store.GetMarking(r.Context(), subject, r.PathValue("id"))
	if errors.Is(err, ErrMarkingNotFound) {
		writeError(w, 404, "not_found", "attempt not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, 200, p)
}

type sessionHandler struct{ store SessionStore }

func decodeSessionBody(r *http.Request, target any) bool {
	data, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	if err != nil || len(data) == 0 || len(data) > 8192 {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(target) != nil {
		return false
	}
	return dec.Decode(&struct{}{}) == io.EOF
}

func (h *sessionHandler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateSessionInput
	if !decodeSessionBody(r, &in) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is invalid")
		return
	}
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}
	s, err := h.store.CreateSession(r.Context(), subject, in)
	writeSessionError(w, err)
	if err == nil {
		writeJSON(w, http.StatusCreated, s)
	}
}
func (h *sessionHandler) get(w http.ResponseWriter, r *http.Request) {
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, 401, "unauthorized", "authentication is required")
		return
	}
	s, err := h.store.GetSession(r.Context(), subject, r.PathValue("id"))
	writeSessionError(w, err)
	if err == nil {
		writeJSON(w, 200, s)
	}
}
func (h *sessionHandler) save(w http.ResponseWriter, r *http.Request) {
	var in SaveResponseInput
	if !decodeSessionBody(r, &in) {
		writeError(w, 400, "invalid_json", "request body is invalid")
		return
	}
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, 401, "unauthorized", "authentication is required")
		return
	}
	item, err := h.store.SaveSessionResponse(r.Context(), subject, r.PathValue("id"), in)
	writeSessionError(w, err)
	if err == nil {
		writeJSON(w, 200, item)
	}
}
func (h *sessionHandler) submit(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 4097))
	if err != nil || len(data) > 4096 || len(bytes.TrimSpace(data)) != 0 {
		writeError(w, 400, "invalid_json", "request body must be empty")
		return
	}
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, 401, "unauthorized", "authentication is required")
		return
	}
	result, err := h.store.SubmitSession(r.Context(), subject, r.PathValue("id"))
	writeSessionError(w, err)
	if err == nil {
		writeJSON(w, 200, result)
	}
}
func (h *sessionHandler) result(w http.ResponseWriter, r *http.Request) {
	subject, ok := learnerSubject(r)
	if !ok {
		writeError(w, 401, "unauthorized", "authentication is required")
		return
	}
	result, err := h.store.GetSessionResult(r.Context(), subject, r.PathValue("id"))
	writeSessionError(w, err)
	if err == nil {
		writeJSON(w, 200, result)
	}
}
func writeSessionError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, ErrSessionNotFound):
		writeError(w, 404, "not_found", "assessment session not found")
	case errors.Is(err, ErrSessionConflict):
		writeError(w, 409, "session_conflict", "assessment session conflict")
	case errors.Is(err, ErrSessionExpired):
		writeError(w, 409, "session_expired", "assessment session deadline elapsed")
	case errors.Is(err, ErrSessionClosed):
		writeError(w, 409, "session_closed", "assessment session is closed")
	case errors.Is(err, ErrNoQuestions):
		writeError(w, 409, "no_questions", "no eligible questions")
	case errors.Is(err, ErrInvalidAnswer):
		writeError(w, 400, "invalid_input", "request is invalid")
	default:
		writeError(w, 500, "internal_error", internalErrorMessage)
	}
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

func (h *handler) listModules(w http.ResponseWriter, r *http.Request) {
	syllabusID := strings.TrimSpace(r.URL.Query().Get("syllabusId"))
	if syllabusID == "" {
		writeMissingFields(w, []string{"syllabusId"})
		return
	}
	items, err := h.store.ListModules(r.Context(), syllabusID)
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
