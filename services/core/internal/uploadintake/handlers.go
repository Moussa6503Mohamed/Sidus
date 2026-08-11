package uploadintake

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Register exposes only admin intake controls. No route serves uploaded bytes, including to an
// administrator. Review execution is intentionally absent until T-0032.
func Register(mux *http.ServeMux, store Store, storage *LocalQuarantineStore, v auth.Verifier) {
	h := &handler{store: store, storage: storage}
	mux.HandleFunc("POST /private-uploads", auth.Protect(v, PermManageUpload, h.create))
	mux.HandleFunc("GET /private-uploads", auth.Protect(v, PermManageUpload, h.list))
	mux.HandleFunc("POST /private-uploads/{id}/review-jobs", auth.Protect(v, PermManageUpload, h.queueReview))
	mux.HandleFunc("POST /private-uploads/{id}/deletion-request", auth.Protect(v, PermManageUpload, h.requestDeletion))
}

type handler struct {
	store   Store
	storage *LocalQuarantineStore
}

const internalError = "an internal error occurred"

func actor(r *http.Request) (string, bool) {
	c, ok := auth.ClaimsFromContext(r.Context())
	return c.Subject, ok && strings.TrimSpace(c.Subject) != ""
}
func errJSON(w http.ResponseWriter, s int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": msg})
}
func jsonOut(w http.ResponseWriter, s int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	_ = json.NewEncoder(w).Encode(v)
}
func safeID(v string) bool { return len(v) > 0 && len(v) <= 128 && !strings.ContainsAny(v, "/\\") }
func pdfContentType(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]), "application/pdf")
}
func safeFilename(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && utf8.RuneCountInString(v) <= 255 && filepath.Base(v) == v && strings.EqualFold(filepath.Ext(v), ".pdf") && !strings.ContainsAny(v, "\\/")
}
func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	if !pdfContentType(r) {
		errJSON(w, 400, "invalid_content_type", "upload must be application/pdf")
		return
	}
	filename := r.Header.Get("X-Sidus-Upload-Filename")
	if !safeFilename(filename) {
		errJSON(w, 400, "invalid_filename", "filename must be a safe .pdf filename")
		return
	}
	// Read only first signature bytes before writing. The complete stream is then copied directly
	// into private storage and never parsed/OCRed/logged/returned.
	prefix := make([]byte, 5)
	n, e := io.ReadFull(r.Body, prefix)
	if e != nil || n != 5 || !bytes.Equal(prefix, []byte("%PDF-")) {
		errJSON(w, 400, "invalid_pdf_signature", "upload is not a PDF")
		return
	}
	ref, hash, size, e := h.storage.Save(io.MultiReader(bytes.NewReader(prefix), io.LimitReader(r.Body, MaxPDFBytes+1)))
	if e != nil {
		if strings.Contains(e.Error(), "size limit") {
			errJSON(w, 413, "payload_too_large", "upload exceeds 25 MiB limit")
		} else {
			errJSON(w, 500, "storage_error", internalError)
		}
		return
	}
	subject, ok := actor(r)
	if !ok {
		errJSON(w, 401, "unauthorized", "authentication is required")
		return
	}
	v, e := h.store.Create(r.Context(), CreateInput{ObjectRef: ref, OriginalFilename: strings.TrimSpace(filename), MediaType: "application/pdf", ByteSize: size, SHA256: hash, ActorID: subject})
	if e != nil {
		_ = h.storage.Delete(ref)
		errJSON(w, 500, "internal_error", internalError)
		return
	}
	jsonOut(w, 201, v)
}
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.store.List(r.Context())
	if e != nil {
		errJSON(w, 500, "internal_error", internalError)
		return
	}
	jsonOut(w, 200, map[string]any{"items": v})
}
func (h *handler) requestDeletion(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, func(id, a string) (Upload, error) { return h.store.RequestDeletion(r.Context(), id, a) })
}
func (h *handler) transition(w http.ResponseWriter, r *http.Request, fn func(string, string) (Upload, error)) {
	id := r.PathValue("id")
	if !safeID(id) {
		errJSON(w, 404, "not_found", "private upload not found")
		return
	}
	a, ok := actor(r)
	if !ok {
		errJSON(w, 401, "unauthorized", "authentication is required")
		return
	}
	v, e := fn(id, a)
	if errors.Is(e, ErrNotFound) {
		errJSON(w, 404, "not_found", "private upload not found")
		return
	}
	if errors.Is(e, ErrInvalidTransition) {
		errJSON(w, 409, "invalid_transition", "upload lifecycle transition is not allowed")
		return
	}
	if e != nil {
		errJSON(w, 500, "internal_error", internalError)
		return
	}
	jsonOut(w, 200, v)
}
func (h *handler) queueReview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !safeID(id) {
		errJSON(w, 404, "not_found", "private upload not found")
		return
	}
	sourceID, ok := decodeQueueReviewBody(r)
	if !ok {
		errJSON(w, 400, "invalid_json", "request body is invalid")
		return
	}
	a, ok := actor(r)
	if !ok {
		errJSON(w, 401, "unauthorized", "authentication is required")
		return
	}
	j, e := h.store.QueueReview(r.Context(), id, sourceID, a)
	if errors.Is(e, ErrNotFound) {
		errJSON(w, 404, "not_found", "private upload not found")
		return
	}
	if errors.Is(e, ErrInvalidTransition) {
		errJSON(w, 409, "invalid_transition", "upload lifecycle transition is not allowed")
		return
	}
	if errors.Is(e, ErrSourceNotApproved) {
		errJSON(w, 400, "source_not_approved", "content source must be approved and catalogue-linked")
		return
	}
	if e != nil {
		errJSON(w, 500, "internal_error", internalError)
		return
	}
	jsonOut(w, 201, j)
}

// decodeQueueReviewBody token-parses exact key and rejects duplicate/case-variant/trailing JSON
// before database work. Struct decoding alone would accept ContentSourceId case-insensitively.
func decodeQueueReviewBody(r *http.Request) (string, bool) {
	dec := json.NewDecoder(io.LimitReader(r.Body, 4097))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return "", false
	}
	var value string
	count := 0
	for dec.More() {
		key, err := dec.Token()
		if err != nil || key != "contentSourceId" || count != 0 {
			return "", false
		}
		if dec.Decode(&value) != nil {
			return "", false
		}
		count++
	}
	if _, err = dec.Token(); err != nil || count != 1 {
		return "", false
	}
	if _, err = dec.Token(); err != io.EOF {
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, value != "" && len(value) <= 128
}
