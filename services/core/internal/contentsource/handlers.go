package contentsource

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Register mounts the content source HTTP endpoints on mux.
func Register(mux *http.ServeMux, store Store) {
	h := &handler{store: store}
	mux.HandleFunc("POST /content-sources", h.create)
	mux.HandleFunc("GET /content-sources", h.list)
	mux.HandleFunc("GET /content-sources/{id}", h.get)
	mux.HandleFunc("POST /content-sources/{id}/approve", h.approve)
	mux.HandleFunc("POST /content-sources/{id}/reject", h.reject)
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

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
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

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	var status *Status
	if v := r.URL.Query().Get("status"); v != "" {
		s := Status(v)
		status = &s
	}

	sources, err := h.store.List(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": sources})
}

type reviewRequest struct {
	ReviewerID   string  `json:"reviewerId"`
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

	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if strings.TrimSpace(req.ReviewerID) == "" {
		writeMissingFields(w, http.StatusBadRequest, []string{"reviewerId"})
		return
	}
	decisionDate, err := req.decisionDate()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_decision_date", "decisionDate must be RFC3339")
		return
	}

	source, missing, err := h.store.Approve(r.Context(), id, ApproveInput{
		ReviewerID:   req.ReviewerID,
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

	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	var missing []string
	if strings.TrimSpace(req.ReviewerID) == "" {
		missing = append(missing, "reviewerId")
	}
	if strings.TrimSpace(req.Reason) == "" {
		missing = append(missing, "reason")
	}
	if len(missing) > 0 {
		writeMissingFields(w, http.StatusBadRequest, missing)
		return
	}

	decisionDate, err := req.decisionDate()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_decision_date", "decisionDate must be RFC3339")
		return
	}

	source, err := h.store.Reject(r.Context(), id, RejectInput{
		ReviewerID:   req.ReviewerID,
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
