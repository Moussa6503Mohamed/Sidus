package curriculummap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
)

// Register mounts the curriculum-map endpoints on mux. Reads (list/get nodes of any lifecycle
// status; list accepts an optional status filter) require curriculum_map:read; draft create/
// update require curriculum_map:create; verify/retire require curriculum_map:verify. Every
// endpoint is behind a verified Clerk session; the verified subject — never a body field — is
// the audit actor.
func Register(mux *http.ServeMux, store Store, v auth.Verifier) {
	h := &handler{store: store}
	mux.HandleFunc("GET /curriculum-map/nodes", auth.Protect(v, auth.PermReadCurriculumMap, h.listNodes))
	mux.HandleFunc("GET /curriculum-map/nodes/{id}", auth.Protect(v, auth.PermReadCurriculumMap, h.getNode))
	mux.HandleFunc("POST /curriculum-map/nodes", auth.Protect(v, auth.PermCreateCurriculumMap, h.createNode))
	mux.HandleFunc("PATCH /curriculum-map/nodes/{id}", auth.Protect(v, auth.PermCreateCurriculumMap, h.updateNode))
	mux.HandleFunc("POST /curriculum-map/nodes/{id}/verify", auth.Protect(v, auth.PermVerifyCurriculumMap, h.verifyNode))
	mux.HandleFunc("POST /curriculum-map/nodes/{id}/retire", auth.Protect(v, auth.PermVerifyCurriculumMap, h.retireNode))
}

type handler struct {
	store Store
}

// internalErrorMessage is the only text ever returned for database, scan, transaction, or
// other infrastructure failures. Raw Go/driver error text must never reach an HTTP response.
const internalErrorMessage = "an internal error occurred"

func actorFromContext(r *http.Request) (string, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return "", false
	}
	return claims.Subject, true
}

// writeInvalidJSON writes the single, stable strict-decoding rejection. The message never
// echoes the offending field name back to the caller.
func writeInvalidJSON(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON with no unknown fields")
}

// createNodeFields is the exact, case-sensitive set of accepted POST /curriculum-map/nodes
// field names.
var createNodeFields = map[string]struct{}{
	"syllabusId":      {},
	"parentNodeId":    {},
	"nodeKind":        {},
	"nodeCode":        {},
	"label":           {},
	"contentSourceId": {},
	"sourceLocator":   {},
}

// decodeStrict decodes exactly one JSON object into dst, rejecting anything else with the same
// stable 400: a non-object body, trailing values or junk after the object, and any key outside
// allowed — which covers identity fields (`actorId`, `reviewerId`), typos, and case variants
// (e.g. `Label`, `SourceUrl`).
//
// The allowlist pre-pass exists because it is the only reliable rejection point. Go's struct
// decoding matches field names case-insensitively, so `json.Decoder.DisallowUnknownFields` alone
// accepts a case-variant key as the declared field. Rejection happens before any store call, so
// a rejected request can never mutate a row or write an event.
func decodeStrict(w http.ResponseWriter, r *http.Request, allowed map[string]struct{}, dst any) bool {
	raw := map[string]json.RawMessage{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		writeInvalidJSON(w)
		return false
	}
	if raw == nil {
		writeInvalidJSON(w)
		return false
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		writeInvalidJSON(w)
		return false
	}
	for name := range raw {
		if _, ok := allowed[name]; !ok {
			writeInvalidJSON(w)
			return false
		}
	}

	strictDec := json.NewDecoder(bytes.NewReader(mustMarshal(raw)))
	strictDec.DisallowUnknownFields()
	if err := strictDec.Decode(dst); err != nil {
		writeInvalidJSON(w)
		return false
	}
	return true
}

func (h *handler) listNodes(w http.ResponseWriter, r *http.Request) {
	syllabusID := strings.TrimSpace(r.URL.Query().Get("syllabusId"))
	if syllabusID == "" {
		writeMissingFields(w, []string{"syllabusId"})
		return
	}

	var statusFilter *Status
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		s := Status(raw)
		if !IsValidStatus(s) {
			writeError(w, http.StatusBadRequest, "invalid_status", "status must be one of: draft, verified, retired")
			return
		}
		statusFilter = &s
	}

	nodes, err := h.store.ListNodesBySyllabus(r.Context(), syllabusID, statusFilter)
	if mapped, ok := mapNodeError(err); ok {
		writeNodeError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": nodes})
}

func (h *handler) getNode(w http.ResponseWriter, r *http.Request) {
	node, err := h.store.GetNode(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "curriculum map node not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

type createNodeRequest struct {
	SyllabusID      string  `json:"syllabusId"`
	ParentNodeID    *string `json:"parentNodeId"`
	NodeKind        string  `json:"nodeKind"`
	NodeCode        string  `json:"nodeCode"`
	Label           string  `json:"label"`
	ContentSourceID string  `json:"contentSourceId"`
	SourceLocator   *string `json:"sourceLocator"`
}

func (h *handler) createNode(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	var req createNodeRequest
	if !decodeStrict(w, r, createNodeFields, &req) {
		return
	}

	var missing []string
	for _, f := range []struct {
		name  string
		value string
	}{
		{"syllabusId", req.SyllabusID},
		{"nodeKind", req.NodeKind},
		{"nodeCode", req.NodeCode},
		{"label", req.Label},
		{"contentSourceId", req.ContentSourceID},
	} {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		writeMissingFields(w, missing)
		return
	}
	if blankPtr(req.ParentNodeID) || blankPtr(req.SourceLocator) {
		writeError(w, http.StatusBadRequest, "blank_field", "parentNodeId and sourceLocator must be non-empty when supplied")
		return
	}
	kind := NodeKind(strings.TrimSpace(req.NodeKind))
	if !IsValidKind(kind) {
		writeError(w, http.StatusBadRequest, "invalid_node_kind", "nodeKind must be one of: topic, objective, practical_skill, assessment_rule")
		return
	}

	node, err := h.store.CreateNode(r.Context(), CreateInput{
		ActorID:         actor,
		SyllabusID:      strings.TrimSpace(req.SyllabusID),
		ParentNodeID:    trimPtr(req.ParentNodeID),
		NodeKind:        kind,
		NodeCode:        strings.TrimSpace(req.NodeCode),
		Label:           strings.TrimSpace(req.Label),
		ContentSourceID: strings.TrimSpace(req.ContentSourceID),
		SourceLocator:   trimPtr(req.SourceLocator),
	})
	if mapped, ok := mapNodeError(err); ok {
		writeNodeError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

type updateNodeRequest struct {
	ParentNodeID    *string `json:"parentNodeId"`
	NodeKind        *string `json:"nodeKind"`
	NodeCode        *string `json:"nodeCode"`
	Label           *string `json:"label"`
	ContentSourceID *string `json:"contentSourceId"`
	SourceLocator   *string `json:"sourceLocator"`
}

// updatablePatchFields is the exact, case-sensitive set of PATCH-able JSON field names.
//
// PATCH decodes into a map[string]json.RawMessage to tell "field absent" from "field present as
// null" for the nullable fields, and json.Decoder.DisallowUnknownFields has NO effect when the
// destination is a map. This allowlist is therefore the strict-JSON enforcement point for
// PATCH: any other key — identity fields (`actorId`, `reviewerId`), immutable fields
// (`syllabusId`, `id`), lifecycle fields (`status`, only settable via verify/retire), field-name
// typos, and case variants — is rejected as a stable 400 `invalid_json` before any store call,
// so no rejected request can mutate a node or write an event.
var updatablePatchFields = map[string]struct{}{
	"parentNodeId":    {},
	"nodeKind":        {},
	"nodeCode":        {},
	"label":           {},
	"contentSourceId": {},
	"sourceLocator":   {},
}

func (h *handler) updateNode(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}

	// Distinguish "field absent" from "field present" (incl. present-null) using RawMessage,
	// since ParentNodeID/SourceLocator are nullable fields that can be explicitly cleared.
	raw := map[string]json.RawMessage{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		writeInvalidJSON(w)
		return
	}
	if raw == nil {
		writeInvalidJSON(w)
		return
	}
	// Trailing values/junk after the single JSON object are rejected.
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		writeInvalidJSON(w)
		return
	}
	// DisallowUnknownFields does nothing for a map destination, so unknown/forbidden keys are
	// rejected here — before any store call, so a rejected PATCH never mutates a node or event.
	for name := range raw {
		if _, ok := updatablePatchFields[name]; !ok {
			writeInvalidJSON(w)
			return
		}
	}

	var req updateNodeRequest
	strictDec := json.NewDecoder(bytes.NewReader(mustMarshal(raw)))
	strictDec.DisallowUnknownFields()
	if err := strictDec.Decode(&req); err != nil {
		writeInvalidJSON(w)
		return
	}

	_, parentSupplied := raw["parentNodeId"]
	_, locatorSupplied := raw["sourceLocator"]

	var blank []string
	for _, f := range []struct {
		name  string
		value *string
	}{
		{"nodeKind", req.NodeKind},
		{"nodeCode", req.NodeCode},
		{"label", req.Label},
		{"contentSourceId", req.ContentSourceID},
	} {
		if f.value != nil && strings.TrimSpace(*f.value) == "" {
			blank = append(blank, f.name)
		}
	}
	if blankPtr(req.ParentNodeID) || blankPtr(req.SourceLocator) {
		blank = append(blank, "parentNodeId/sourceLocator (must be non-empty when non-null)")
	}
	if len(blank) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "blank_fields", "fields": blank})
		return
	}

	suppliedCount := len(raw)
	if suppliedCount == 0 {
		writeError(w, http.StatusBadRequest, "no_updatable_fields", "supply at least one field to update")
		return
	}

	in := UpdateInput{
		ActorID:          actor,
		NodeCode:         trimPtr(req.NodeCode),
		Label:            trimPtr(req.Label),
		ContentSourceID:  trimPtr(req.ContentSourceID),
		ParentNodeIDSet:  parentSupplied,
		ParentNodeID:     trimPtr(req.ParentNodeID),
		SourceLocatorSet: locatorSupplied,
		SourceLocator:    trimPtr(req.SourceLocator),
	}
	if req.NodeKind != nil {
		kind := NodeKind(strings.TrimSpace(*req.NodeKind))
		if !IsValidKind(kind) {
			writeError(w, http.StatusBadRequest, "invalid_node_kind", "nodeKind must be one of: topic, objective, practical_skill, assessment_rule")
			return
		}
		in.NodeKind = &kind
	}

	node, err := h.store.UpdateNode(r.Context(), r.PathValue("id"), in)
	if mapped, ok := mapNodeError(err); ok {
		writeNodeError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (h *handler) verifyNode(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	node, err := h.store.VerifyNode(r.Context(), r.PathValue("id"), actor)
	if mapped, ok := mapNodeError(err); ok {
		writeNodeError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (h *handler) retireNode(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_token", "authenticated subject required")
		return
	}
	node, err := h.store.RetireNode(r.Context(), r.PathValue("id"), actor)
	if mapped, ok := mapNodeError(err); ok {
		writeNodeError(w, mapped)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// nodeErrorMapping pairs a domain error with its stable HTTP status/code/message.
type nodeErrorMapping struct {
	status  int
	code    string
	message string
}

func mapNodeError(err error) (nodeErrorMapping, bool) {
	switch {
	case errors.Is(err, ErrNotFound):
		return nodeErrorMapping{http.StatusNotFound, "not_found", "curriculum map node not found"}, true
	case errors.Is(err, ErrDuplicateNodeCode):
		return nodeErrorMapping{http.StatusConflict, "duplicate_node_code", err.Error()}, true
	case errors.Is(err, ErrUnknownSyllabus):
		return nodeErrorMapping{http.StatusBadRequest, "unknown_syllabus", err.Error()}, true
	case errors.Is(err, ErrInvalidParent):
		return nodeErrorMapping{http.StatusBadRequest, "invalid_parent", err.Error()}, true
	case errors.Is(err, ErrUnknownSource):
		return nodeErrorMapping{http.StatusBadRequest, "unknown_source", err.Error()}, true
	case errors.Is(err, ErrUnapprovedSource):
		return nodeErrorMapping{http.StatusBadRequest, "unapproved_source", err.Error()}, true
	case errors.Is(err, ErrUnlinkedSource):
		return nodeErrorMapping{http.StatusBadRequest, "unlinked_source", err.Error()}, true
	case errors.Is(err, ErrMismatchedSource):
		return nodeErrorMapping{http.StatusBadRequest, "mismatched_source", err.Error()}, true
	case errors.Is(err, ErrNoChanges):
		return nodeErrorMapping{http.StatusBadRequest, "no_changes", err.Error()}, true
	case errors.Is(err, ErrInvalidTransition):
		return nodeErrorMapping{http.StatusConflict, "invalid_lifecycle_transition", err.Error()}, true
	default:
		return nodeErrorMapping{}, false
	}
}

func writeNodeError(w http.ResponseWriter, m nodeErrorMapping) {
	writeError(w, m.status, m.code, m.message)
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
