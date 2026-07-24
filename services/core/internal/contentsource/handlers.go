package contentsource

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// Register mounts the content source HTTP endpoints on mux. Every endpoint requires a valid
// Clerk session (verified by v) and the role permission listed against it; there is no
// unauthenticated content-source access. The authenticated Clerk subject — never a
// request-body field — is used as the audit actor/reviewer.
func Register(mux *http.ServeMux, store Store, v auth.Verifier) {
	h := &handler{store: store}
	mux.HandleFunc("POST /content-sources", auth.Protect(v, auth.PermCreateSource, h.create))
	mux.HandleFunc("GET /content-sources", auth.Protect(v, auth.PermReadSource, h.list))
	mux.HandleFunc("GET /content-sources/{id}", auth.Protect(v, auth.PermReadSource, h.get))
	mux.HandleFunc("PATCH /content-sources/{id}", auth.Protect(v, auth.PermUpdateSource, h.update))
	mux.HandleFunc("POST /content-sources/{id}/approve", auth.Protect(v, auth.PermReviewSource, h.approve))
	mux.HandleFunc("POST /content-sources/{id}/reject", auth.Protect(v, auth.PermReviewSource, h.reject))
}

// actorFromContext returns the verified Clerk subject that Protect placed on the request
// context. Handlers use it as the audit actor/reviewer; it is never taken from the body.
func actorFromContext(r *http.Request) (string, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return "", false
	}
	return claims.Subject, true
}

type handler struct {
	store Store
}

type createRequest struct {
	Title            string  `json:"title"`
	SourceURL        string  `json:"sourceUrl"`
	Owner            *string `json:"owner"`
	SourceHash       *string `json:"sourceHash"`
	LicenceReference *string `json:"licenceReference"`
	PermittedUse     *string `json:"permittedUse"`
	AllowedAudience  *string `json:"allowedAudience"`
	SyllabusCode     *string `json:"syllabusCode"`
}

// decodeStrict decodes the JSON request body into dst, rejecting any unknown field. Legacy
// caller-controlled identity fields (e.g. actorId, reviewerId) — and any other unrecognized
// field — therefore fail with a stable 400 invalid_json response rather than being silently
// ignored. Returns false (and writes the error) on any decode failure.
func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON with no unknown fields")
		return false
	}
	return true
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !decodeStrict(w, r, &req) {
		return
	}

	var missing []string
	if strings.TrimSpace(req.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(req.SourceURL) == "" {
		missing = append(missing, "sourceUrl")
	}
	if len(missing) > 0 {
		writeMissingFields(w, http.StatusBadRequest, missing)
		return
	}
	if req.SyllabusCode != nil && !isValidSyllabusCode(*req.SyllabusCode) {
		writeError(w, http.StatusBadRequest, "invalid_syllabus_code", "syllabusCode must be one of: 0610, 5090")
		return
	}

	source, err := h.store.Create(r.Context(), CreateInput{
		Title:            req.Title,
		SourceURL:        req.SourceURL,
		Owner:            req.Owner,
		SourceHash:       req.SourceHash,
		LicenceReference: req.LicenceReference,
		PermittedUse:     req.PermittedUse,
		AllowedAudience:  req.AllowedAudience,
		SyllabusCode:     req.SyllabusCode,
	})
	if errors.Is(err, ErrDuplicateSourceURL) {
		writeError(w, http.StatusConflict, "duplicate_source_url", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, source)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	source, err := h.store.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "content source not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, source)
}

// updateRequest carries only metadata fields. The actor is never accepted from the body; it
// is derived from the verified Clerk session subject.
type updateRequest struct {
	Title            *string `json:"title"`
	Owner            *string `json:"owner"`
	SourceURL        *string `json:"sourceUrl"`
	SourceHash       *string `json:"sourceHash"`
	LicenceReference *string `json:"licenceReference"`
	PermittedUse     *string `json:"permittedUse"`
	AllowedAudience  *string `json:"allowedAudience"`
	SyllabusCode     *string `json:"syllabusCode"`
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}

	var req updateRequest
	if !decodeStrict(w, r, &req) {
		return
	}

	// Reject empty/whitespace-only values on any supplied field: a PATCH never clears a
	// rights field, it only fills one in.
	supplied := []struct {
		name  string
		value *string
	}{
		{"title", req.Title},
		{"owner", req.Owner},
		{"sourceUrl", req.SourceURL},
		{"sourceHash", req.SourceHash},
		{"licenceReference", req.LicenceReference},
		{"permittedUse", req.PermittedUse},
		{"allowedAudience", req.AllowedAudience},
		{"syllabusCode", req.SyllabusCode},
	}
	var blank []string
	suppliedCount := 0
	for _, f := range supplied {
		if f.value == nil {
			continue
		}
		suppliedCount++
		if strings.TrimSpace(*f.value) == "" {
			blank = append(blank, f.name)
		}
	}
	if len(blank) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "blank_fields", "fields": blank})
		return
	}
	if suppliedCount == 0 {
		writeError(w, http.StatusBadRequest, "no_updatable_fields", "supply at least one field to update")
		return
	}

	if req.SyllabusCode != nil && !isValidSyllabusCode(*req.SyllabusCode) {
		writeError(w, http.StatusBadRequest, "invalid_syllabus_code", "syllabusCode must be one of: 0610, 5090")
		return
	}
	if req.SourceURL != nil && !isValidHTTPURL(*req.SourceURL) {
		writeError(w, http.StatusBadRequest, "invalid_source_url", "sourceUrl must be an absolute http or https URL")
		return
	}

	source, _, err := h.store.Update(r.Context(), id, UpdateInput{
		ActorID:          actor,
		Title:            req.Title,
		Owner:            req.Owner,
		SourceURL:        req.SourceURL,
		SourceHash:       req.SourceHash,
		LicenceReference: req.LicenceReference,
		PermittedUse:     req.PermittedUse,
		AllowedAudience:  req.AllowedAudience,
		SyllabusCode:     req.SyllabusCode,
	})
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "content source not found")
		return
	case errors.Is(err, ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_status_transition", "content source is not pending")
		return
	case errors.Is(err, ErrDuplicateSourceURL):
		writeError(w, http.StatusConflict, "duplicate_source_url", err.Error())
		return
	case errors.Is(err, ErrNoUpdatableFields):
		writeError(w, http.StatusBadRequest, "no_updatable_fields", "supply at least one field to update")
		return
	case errors.Is(err, ErrNoChanges):
		writeError(w, http.StatusBadRequest, "no_changes", "supplied values match the current stored values; nothing to update")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, source)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	var status *Status
	if v := r.URL.Query().Get("status"); v != "" {
		s := Status(v)
		if !isValidStatus(s) {
			writeError(w, http.StatusBadRequest, "invalid_status", "status must be one of: pending, approved, rejected, expired")
			return
		}
		status = &s
	}

	sources, err := h.store.List(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": sources})
}

// reviewRequest carries only the decision metadata. The reviewer is never accepted from the
// body; it is derived from the verified Clerk session subject.
type reviewRequest struct {
	Reason       string  `json:"reason"`
	DecisionDate *string `json:"decisionDate"`
}

func (req reviewRequest) decisionDate() (time.Time, error) {
	if req.DecisionDate == nil || *req.DecisionDate == "" {
		return time.Now().UTC(), nil
	}
	return time.Parse(time.RFC3339, *req.DecisionDate)
}

func (h *handler) approve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	reviewer, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}

	var req reviewRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	decisionDate, err := req.decisionDate()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_decision_date", "decisionDate must be RFC3339")
		return
	}

	source, missing, err := h.store.Approve(r.Context(), id, ApproveInput{
		ReviewerID:   reviewer,
		DecisionDate: decisionDate,
	})
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "content source not found")
		return
	case errors.Is(err, ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_status_transition", "content source is not pending")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if len(missing) > 0 {
		writeMissingFields(w, http.StatusUnprocessableEntity, missing)
		return
	}

	writeJSON(w, http.StatusOK, source)
}

func (h *handler) reject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	reviewer, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}

	var req reviewRequest
	if !decodeStrict(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.Reason) == "" {
		writeMissingFields(w, http.StatusBadRequest, []string{"reason"})
		return
	}

	decisionDate, err := req.decisionDate()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_decision_date", "decisionDate must be RFC3339")
		return
	}

	source, err := h.store.Reject(r.Context(), id, RejectInput{
		ReviewerID:   reviewer,
		Reason:       req.Reason,
		DecisionDate: decisionDate,
	})
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "content source not found")
		return
	case errors.Is(err, ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_status_transition", "content source is not pending")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, source)
}

func isValidStatus(s Status) bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected, StatusExpired:
		return true
	default:
		return false
	}
}

func isValidSyllabusCode(code string) bool {
	return code == "0610" || code == "5090"
}

// isValidHTTPURL reports whether s is an absolute URL with an http/https scheme and a host.
func isValidHTTPURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func writeMissingFields(w http.ResponseWriter, status int, missing []string) {
	writeJSON(w, status, map[string]any{"error": "missing_required_fields", "missing": missing})
}
