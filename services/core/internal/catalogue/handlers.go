package catalogue

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// Register mounts the curriculum catalogue endpoints on mux. Reads (list/get active
// syllabuses and list subjects) require content_catalogue:read; mutations (create/update)
// require content_catalogue:manage (admin only). Every endpoint is behind a verified Clerk
// session; the verified subject — never a body field — is the audit actor.
func Register(mux *http.ServeMux, store Store, v auth.Verifier) {
	h := &handler{store: store}
	mux.HandleFunc("GET /catalogue/subjects", auth.Protect(v, auth.PermReadCatalogue, h.listSubjects))
	mux.HandleFunc("POST /catalogue/subjects", auth.Protect(v, auth.PermManageCatalogue, h.createSubject))
	mux.HandleFunc("GET /catalogue/syllabuses", auth.Protect(v, auth.PermReadCatalogue, h.listSyllabuses))
	mux.HandleFunc("GET /catalogue/syllabuses/{id}", auth.Protect(v, auth.PermReadCatalogue, h.getSyllabus))
	mux.HandleFunc("POST /catalogue/syllabuses", auth.Protect(v, auth.PermManageCatalogue, h.createSyllabus))
	mux.HandleFunc("PATCH /catalogue/syllabuses/{id}", auth.Protect(v, auth.PermManageCatalogue, h.updateSyllabus))
}

type handler struct {
	store Store
}

func actorFromContext(r *http.Request) (string, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return "", false
	}
	return claims.Subject, true
}

// decodeStrict decodes exactly one JSON value into dst, rejecting unknown fields and trailing
// data. Mirrors the content-source strict decoder so caller-controlled identity fields cannot
// be smuggled in.
func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON with no unknown fields")
		return false
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON with no unknown fields")
		return false
	}
	return true
}

func (h *handler) listSubjects(w http.ResponseWriter, r *http.Request) {
	subjects, err := h.store.ListSubjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": subjects})
}

type createSubjectRequest struct {
	Name string `json:"name"`
}

func (h *handler) createSubject(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	var req createSubjectRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeMissingFields(w, []string{"name"})
		return
	}

	subject, err := h.store.CreateSubject(r.Context(), CreateSubjectInput{ActorID: actor, Name: strings.TrimSpace(req.Name)})
	if errors.Is(err, ErrDuplicateSubject) {
		writeError(w, http.StatusConflict, "duplicate_subject", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, subject)
}

func (h *handler) listSyllabuses(w http.ResponseWriter, r *http.Request) {
	// Readers only ever see active syllabuses.
	syllabuses, err := h.store.ListSyllabuses(r.Context(), true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": syllabuses})
}

func (h *handler) getSyllabus(w http.ResponseWriter, r *http.Request) {
	syllabus, err := h.store.GetSyllabus(r.Context(), r.PathValue("id"), true)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "syllabus not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, syllabus)
}

type createSyllabusRequest struct {
	Board          string  `json:"board"`
	SyllabusCode   string  `json:"syllabusCode"`
	SubjectID      string  `json:"subjectId"`
	Qualification  string  `json:"qualification"`
	Track          *string `json:"track"`
	DisplayName    string  `json:"displayName"`
	CurriculumYear *string `json:"curriculumYear"`
	Status         *string `json:"status"`
}

func (h *handler) createSyllabus(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	var req createSyllabusRequest
	if !decodeStrict(w, r, &req) {
		return
	}

	var missing []string
	for _, f := range []struct {
		name  string
		value string
	}{
		{"board", req.Board},
		{"syllabusCode", req.SyllabusCode},
		{"subjectId", req.SubjectID},
		{"qualification", req.Qualification},
		{"displayName", req.DisplayName},
	} {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		writeMissingFields(w, missing)
		return
	}
	// Optional pointer fields, if present, must not be blank (present-null clears/omits;
	// present-"" is a client error).
	if blankPtr(req.Track) || blankPtr(req.CurriculumYear) {
		writeError(w, http.StatusBadRequest, "blank_field", "track and curriculumYear must be non-empty when supplied")
		return
	}

	status := StatusDraft
	if req.Status != nil {
		status = SyllabusStatus(strings.TrimSpace(*req.Status))
		if !IsValidStatus(status) {
			writeError(w, http.StatusBadRequest, "invalid_status", "status must be one of: draft, active, retired")
			return
		}
	}

	syllabus, err := h.store.CreateSyllabus(r.Context(), CreateSyllabusInput{
		ActorID:        actor,
		Board:          strings.TrimSpace(req.Board),
		SyllabusCode:   strings.TrimSpace(req.SyllabusCode),
		SubjectID:      strings.TrimSpace(req.SubjectID),
		Qualification:  strings.TrimSpace(req.Qualification),
		Track:          req.Track,
		DisplayName:    strings.TrimSpace(req.DisplayName),
		CurriculumYear: req.CurriculumYear,
		Status:         status,
	})
	switch {
	case errors.Is(err, ErrDuplicateSyllabus):
		writeError(w, http.StatusConflict, "duplicate_syllabus", err.Error())
		return
	case errors.Is(err, ErrSubjectNotFound):
		writeError(w, http.StatusBadRequest, "unknown_subject", "subjectId does not reference an existing subject")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, syllabus)
}

type updateSyllabusRequest struct {
	Board          *string `json:"board"`
	SyllabusCode   *string `json:"syllabusCode"`
	SubjectID      *string `json:"subjectId"`
	Qualification  *string `json:"qualification"`
	Track          *string `json:"track"`
	DisplayName    *string `json:"displayName"`
	CurriculumYear *string `json:"curriculumYear"`
	Status         *string `json:"status"`
}

func (h *handler) updateSyllabus(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	var req updateSyllabusRequest
	if !decodeStrict(w, r, &req) {
		return
	}

	// Every supplied field must be non-blank: an update fills or changes metadata, it never
	// clears a required field to empty.
	supplied := []struct {
		name  string
		value *string
	}{
		{"board", req.Board},
		{"syllabusCode", req.SyllabusCode},
		{"subjectId", req.SubjectID},
		{"qualification", req.Qualification},
		{"track", req.Track},
		{"displayName", req.DisplayName},
		{"curriculumYear", req.CurriculumYear},
		{"status", req.Status},
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

	in := UpdateSyllabusInput{
		ActorID:        actor,
		Board:          trimPtr(req.Board),
		SyllabusCode:   trimPtr(req.SyllabusCode),
		SubjectID:      trimPtr(req.SubjectID),
		Qualification:  trimPtr(req.Qualification),
		Track:          trimPtr(req.Track),
		DisplayName:    trimPtr(req.DisplayName),
		CurriculumYear: trimPtr(req.CurriculumYear),
	}
	if req.Status != nil {
		status := SyllabusStatus(strings.TrimSpace(*req.Status))
		if !IsValidStatus(status) {
			writeError(w, http.StatusBadRequest, "invalid_status", "status must be one of: draft, active, retired")
			return
		}
		in.Status = &status
	}

	syllabus, err := h.store.UpdateSyllabus(r.Context(), r.PathValue("id"), in)
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "syllabus not found")
		return
	case errors.Is(err, ErrDuplicateSyllabus):
		writeError(w, http.StatusConflict, "duplicate_syllabus", err.Error())
		return
	case errors.Is(err, ErrSubjectNotFound):
		writeError(w, http.StatusBadRequest, "unknown_subject", "subjectId does not reference an existing subject")
		return
	case errors.Is(err, ErrNoChanges):
		writeError(w, http.StatusBadRequest, "no_changes", "supplied values match the current stored values; nothing to update")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, syllabus)
}

// blankPtr reports whether an optional string field is present but whitespace-only.
func blankPtr(v *string) bool { return v != nil && strings.TrimSpace(*v) == "" }

// trimPtr returns a pointer to the trimmed value, or nil if v is nil.
func trimPtr(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	return &t
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
