package question

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// Register mounts the question endpoints on mux. Reads of verified questions require
// question:read; draft create/update and draft rubric-version create require question:create;
// verifying a rubric version, verifying a question, and retiring a question require
// question:verify; listing rubric versions requires question_rubric:read. Every endpoint is
// behind a verified Clerk session; the verified subject — never a body field — is the audit
// actor, the rubric creator, and the rubric reviewer.
func Register(mux *http.ServeMux, store Store, v auth.Verifier) {
	h := &handler{store: store}
	mux.HandleFunc("GET /questions", auth.Protect(v, auth.PermReadQuestion, h.listQuestions))
	mux.HandleFunc("GET /questions/{id}", auth.Protect(v, auth.PermReadQuestion, h.getQuestion))
	mux.HandleFunc("POST /questions", auth.Protect(v, auth.PermCreateQuestion, h.createQuestion))
	mux.HandleFunc("PATCH /questions/{id}", auth.Protect(v, auth.PermCreateQuestion, h.updateQuestion))
	mux.HandleFunc("POST /questions/{id}/verify", auth.Protect(v, auth.PermVerifyQuestion, h.verifyQuestion))
	mux.HandleFunc("POST /questions/{id}/retire", auth.Protect(v, auth.PermVerifyQuestion, h.retireQuestion))
	mux.HandleFunc("GET /questions/{id}/rubric-versions", auth.Protect(v, auth.PermReadQuestionRubric, h.listRubricVersions))
	mux.HandleFunc("POST /questions/{id}/rubric-versions", auth.Protect(v, auth.PermCreateQuestion, h.createRubricVersion))
	mux.HandleFunc("POST /questions/{id}/rubric-versions/{version}/verify", auth.Protect(v, auth.PermVerifyQuestion, h.verifyRubricVersion))
}

type handler struct {
	store Store
}

// internalErrorMessage is the only text ever returned for database, scan, transaction, or other
// infrastructure failures. Raw Go/driver error text must never reach an HTTP response.
const internalErrorMessage = "an internal error occurred"

func actorFromContext(r *http.Request) (string, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || blank(claims.Subject) {
		return "", false
	}
	return claims.Subject, true
}

// writeInvalidJSON writes the single, stable strict-decoding rejection. The message never echoes
// the offending field name back to the caller.
func writeInvalidJSON(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON with no unknown fields")
}

// decodeStrict decodes exactly one JSON object into dst, rejecting anything else with the same
// stable 400: a non-object body, trailing values or junk after the object, and any key outside
// allowed — which covers identity fields (`actorId`, `reviewerId`), immutable fields
// (`syllabusId` on PATCH, `id`, `createdAt`, `updatedAt`), the lifecycle field (`status`), typos,
// and case variants.
//
// The allowlist pre-pass exists because it is the only reliable rejection point. Go's struct
// decoding matches field names case-insensitively, so `json.Decoder.DisallowUnknownFields` alone
// accepts `{"SyllabusId":...}` as `syllabusId`; and it has no effect at all when the destination
// is a map (which PATCH needs, to tell "absent" from "present as null"). Rejection happens before
// any store call, so a rejected request can never mutate a row or write an event.
//
// It returns the decoded key set so a caller can tell which fields were supplied.
func decodeStrict(w http.ResponseWriter, r *http.Request, allowed map[string]struct{}, dst any) (map[string]json.RawMessage, bool) {
	raw := map[string]json.RawMessage{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		writeInvalidJSON(w)
		return nil, false
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		writeInvalidJSON(w)
		return nil, false
	}
	for name := range raw {
		if _, ok := allowed[name]; !ok {
			writeInvalidJSON(w)
			return nil, false
		}
	}

	strictDec := json.NewDecoder(bytes.NewReader(mustMarshal(raw)))
	strictDec.DisallowUnknownFields()
	if err := strictDec.Decode(dst); err != nil {
		writeInvalidJSON(w)
		return nil, false
	}
	return raw, true
}

// --- Reads ---

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

	questions, err := h.store.ListQuestions(r.Context(), syllabusID, nodeID, true)
	if mapped, ok := mapQuestionError(err); ok {
		writeQuestionError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": questions})
}

func (h *handler) getQuestion(w http.ResponseWriter, r *http.Request) {
	q, err := h.store.GetQuestion(r.Context(), r.PathValue("id"), true)
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

// --- Question writes ---

// createQuestionFields is the exact, case-sensitive set of accepted POST /questions field names.
var createQuestionFields = map[string]struct{}{
	"syllabusId":          {},
	"curriculumMapNodeId": {},
	"responseType":        {},
	"language":            {},
	"prompt":              {},
}

type createQuestionRequest struct {
	SyllabusID          string `json:"syllabusId"`
	CurriculumMapNodeID string `json:"curriculumMapNodeId"`
	ResponseType        string `json:"responseType"`
	Language            string `json:"language"`
	Prompt              string `json:"prompt"`
}

func (h *handler) createQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	var req createQuestionRequest
	if _, ok := decodeStrict(w, r, createQuestionFields, &req); !ok {
		return
	}

	var missing []string
	for _, f := range []struct {
		name  string
		value string
	}{
		{"syllabusId", req.SyllabusID},
		{"curriculumMapNodeId", req.CurriculumMapNodeID},
		{"responseType", req.ResponseType},
		{"language", req.Language},
		{"prompt", req.Prompt},
	} {
		if blank(f.value) {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		writeMissingFields(w, missing)
		return
	}

	responseType := ResponseType(strings.TrimSpace(req.ResponseType))
	if !IsValidResponseType(responseType) {
		writeInvalidResponseType(w)
		return
	}

	q, err := h.store.CreateQuestion(r.Context(), CreateInput{
		ActorID:             actor,
		SyllabusID:          strings.TrimSpace(req.SyllabusID),
		CurriculumMapNodeID: strings.TrimSpace(req.CurriculumMapNodeID),
		ResponseType:        responseType,
		Language:            strings.TrimSpace(req.Language),
		Prompt:              strings.TrimSpace(req.Prompt),
	})
	if mapped, ok := mapQuestionError(err); ok {
		writeQuestionError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusCreated, q)
}

type updateQuestionRequest struct {
	CurriculumMapNodeID *string `json:"curriculumMapNodeId"`
	ResponseType        *string `json:"responseType"`
	Language            *string `json:"language"`
	Prompt              *string `json:"prompt"`
}

// updatablePatchFields is the exact, case-sensitive set of PATCH-able JSON field names.
//
// PATCH decodes into a map[string]json.RawMessage first so a caller cannot smuggle a field past
// the struct decode (json.Decoder.DisallowUnknownFields has NO effect on a map destination). Any
// key outside this allowlist — identity fields (`actorId`, `reviewerId`), immutable fields
// (`syllabusId`, `id`, `createdAt`, `updatedAt`), the lifecycle field (`status`), typos, and case
// variants — is rejected as a stable 400 `invalid_json` before any store call, so a rejected
// request can never mutate a question or write an event.
var updatablePatchFields = map[string]struct{}{
	"curriculumMapNodeId": {},
	"responseType":        {},
	"language":            {},
	"prompt":              {},
}

func (h *handler) updateQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}

	var req updateQuestionRequest
	raw, ok := decodeStrict(w, r, updatablePatchFields, &req)
	if !ok {
		return
	}

	if len(raw) == 0 {
		writeError(w, http.StatusBadRequest, "no_updatable_fields", "supply at least one field to update")
		return
	}

	// No PATCH field is nullable: a supplied field must carry a non-blank value.
	var blankFields []string
	for _, f := range []struct {
		name  string
		value *string
	}{
		{"curriculumMapNodeId", req.CurriculumMapNodeID},
		{"responseType", req.ResponseType},
		{"language", req.Language},
		{"prompt", req.Prompt},
	} {
		if f.value != nil && blank(*f.value) {
			blankFields = append(blankFields, f.name)
		}
	}
	if len(blankFields) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "blank_fields", "fields": blankFields})
		return
	}

	in := UpdateInput{
		ActorID:             actor,
		CurriculumMapNodeID: trimPtr(req.CurriculumMapNodeID),
		Language:            trimPtr(req.Language),
		Prompt:              trimPtr(req.Prompt),
	}
	if req.ResponseType != nil {
		responseType := ResponseType(strings.TrimSpace(*req.ResponseType))
		if !IsValidResponseType(responseType) {
			writeInvalidResponseType(w)
			return
		}
		in.ResponseType = &responseType
	}

	q, err := h.store.UpdateQuestion(r.Context(), r.PathValue("id"), in)
	if mapped, ok := mapQuestionError(err); ok {
		writeQuestionError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (h *handler) verifyQuestion(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.store.VerifyQuestion)
}

func (h *handler) retireQuestion(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.store.RetireQuestion)
}

// transition runs a lifecycle action with the verified subject as actor. Verify and retire carry
// no request body, so there is nothing to decode and no way to supply an actor.
func (h *handler) transition(w http.ResponseWriter, r *http.Request, action func(ctx context.Context, id, actorID string) (Question, error)) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	q, err := action(r.Context(), r.PathValue("id"), actor)
	if mapped, ok := mapQuestionError(err); ok {
		writeQuestionError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

// --- Rubric versions ---

// createRubricVersionFields is the exact, case-sensitive set of accepted rubric-version field
// names. The version number is allocated server-side and is deliberately absent.
var createRubricVersionFields = map[string]struct{}{
	"rubric":   {},
	"maxMarks": {},
}

type createRubricVersionRequest struct {
	Rubric   json.RawMessage `json:"rubric"`
	MaxMarks *int            `json:"maxMarks"`
}

func (h *handler) createRubricVersion(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	var req createRubricVersionRequest
	if _, ok := decodeStrict(w, r, createRubricVersionFields, &req); !ok {
		return
	}

	var missing []string
	if len(bytes.TrimSpace(req.Rubric)) == 0 {
		missing = append(missing, "rubric")
	}
	if req.MaxMarks == nil {
		missing = append(missing, "maxMarks")
	}
	if len(missing) > 0 {
		writeMissingFields(w, missing)
		return
	}

	version, err := h.store.CreateRubricVersion(r.Context(), r.PathValue("id"), CreateRubricVersionInput{
		ActorID:  actor,
		Rubric:   req.Rubric,
		MaxMarks: *req.MaxMarks,
	})
	if mapped, ok := mapQuestionError(err); ok {
		writeQuestionError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusCreated, version)
}

func (h *handler) listRubricVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.store.ListRubricVersions(r.Context(), r.PathValue("id"))
	if mapped, ok := mapQuestionError(err); ok {
		writeQuestionError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": versions})
}

func (h *handler) verifyRubricVersion(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version <= 0 {
		writeError(w, http.StatusNotFound, "not_found", "rubric version not found")
		return
	}

	updated, storeErr := h.store.VerifyRubricVersion(r.Context(), r.PathValue("id"), version, actor)
	if mapped, ok := mapQuestionError(storeErr); ok {
		writeQuestionError(w, mapped)
		return
	}
	if storeErr != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// --- Error mapping ---

// questionErrorMapping pairs a domain error with its stable HTTP status/code/message. Every
// message is a static string defined in this package — never raw driver or scan text.
type questionErrorMapping struct {
	status  int
	code    string
	message string
}

func mapQuestionError(err error) (questionErrorMapping, bool) {
	switch {
	case errors.Is(err, ErrNotFound):
		return questionErrorMapping{http.StatusNotFound, "not_found", "question not found"}, true
	case errors.Is(err, ErrRubricVersionNotFound):
		return questionErrorMapping{http.StatusNotFound, "not_found", "rubric version not found"}, true
	case errors.Is(err, ErrUnknownSyllabus):
		return questionErrorMapping{http.StatusBadRequest, "unknown_syllabus", err.Error()}, true
	case errors.Is(err, ErrUnknownNode):
		return questionErrorMapping{http.StatusBadRequest, "unknown_node", err.Error()}, true
	case errors.Is(err, ErrUnverifiedNode):
		return questionErrorMapping{http.StatusBadRequest, "unverified_node", err.Error()}, true
	case errors.Is(err, ErrMismatchedNode):
		return questionErrorMapping{http.StatusBadRequest, "mismatched_node", err.Error()}, true
	case errors.Is(err, ErrUnknownSource):
		return questionErrorMapping{http.StatusBadRequest, "unknown_source", err.Error()}, true
	case errors.Is(err, ErrUnapprovedSource):
		return questionErrorMapping{http.StatusBadRequest, "unapproved_source", err.Error()}, true
	case errors.Is(err, ErrUnlinkedSource):
		return questionErrorMapping{http.StatusBadRequest, "unlinked_source", err.Error()}, true
	case errors.Is(err, ErrMismatchedSource):
		return questionErrorMapping{http.StatusBadRequest, "mismatched_source", err.Error()}, true
	case errors.Is(err, ErrInvalidRubric):
		return questionErrorMapping{http.StatusBadRequest, "invalid_rubric", err.Error()}, true
	case errors.Is(err, ErrInvalidMaxMarks):
		return questionErrorMapping{http.StatusBadRequest, "invalid_max_marks", err.Error()}, true
	case errors.Is(err, ErrNoChanges):
		return questionErrorMapping{http.StatusBadRequest, "no_changes", err.Error()}, true
	case errors.Is(err, ErrMissingVerifiedRubric):
		return questionErrorMapping{http.StatusConflict, "missing_verified_rubric", err.Error()}, true
	case errors.Is(err, ErrInvalidTransition):
		return questionErrorMapping{http.StatusConflict, "invalid_lifecycle_transition", err.Error()}, true
	case errors.Is(err, ErrDuplicateRubricVersion):
		return questionErrorMapping{http.StatusConflict, "duplicate_rubric_version", err.Error()}, true
	default:
		return questionErrorMapping{}, false
	}
}

func writeQuestionError(w http.ResponseWriter, m questionErrorMapping) {
	writeError(w, m.status, m.code, m.message)
}

func writeInvalidResponseType(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid_response_type",
		"responseType must be one of: multiple_choice, short_answer, structured_response")
}

// trimPtr returns a pointer to the trimmed value, or nil if v is nil.
func trimPtr(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	return &t
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
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
