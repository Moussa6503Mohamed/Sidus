// Package contentsource implements the content rights/provenance gate: sources start
// pending, require a documented set of rights fields before they can be approved, and
// every approve/reject decision is recorded as an immutable review.
package contentsource

import (
	"strings"
	"time"
)

// Status is the lifecycle state of a content source.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusExpired  Status = "expired"
)

// Source is a rights/provenance record for external material considered for ingestion.
// It never stores the source material itself, only metadata about it.
type Source struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Owner            *string   `json:"owner"`
	SourceURL        string    `json:"sourceUrl"`
	SourceHash       *string   `json:"sourceHash"`
	LicenceReference *string   `json:"licenceReference"`
	PermittedUse     *string   `json:"permittedUse"`
	AllowedAudience  *string   `json:"allowedAudience"`
	SyllabusCode     *string   `json:"syllabusCode"`
	Status           Status    `json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// Review is an immutable record of an approve/reject decision on a Source.
type Review struct {
	ID              string    `json:"id"`
	ContentSourceID string    `json:"contentSourceId"`
	Decision        Status    `json:"decision"`
	ReviewerID      string    `json:"reviewerId"`
	DecisionDate    time.Time `json:"decisionDate"`
	Reason          *string   `json:"reason,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// EventType is the kind of change recorded in a source event.
type EventType string

// EventMetadataUpdated records a successful update of a pending source's metadata.
const EventMetadataUpdated EventType = "metadata_updated"

// Event is an immutable audit record of a metadata change to a Source. It records which
// fields changed (names only) and who changed them — never the field values themselves,
// and never any source material.
type Event struct {
	ID              string    `json:"id"`
	ContentSourceID string    `json:"contentSourceId"`
	EventType       EventType `json:"eventType"`
	ActorID         string    `json:"actorId"`
	EventTime       time.Time `json:"eventTime"`
	ChangedFields   []string  `json:"changedFields"`
	CreatedAt       time.Time `json:"createdAt"`
}

// CreateInput is the payload for creating a new pending Source. All rights fields are
// optional at creation time; there is no separate update endpoint in this task's scope,
// so any field required for approval must be supplied here.
type CreateInput struct {
	Title            string
	Owner            *string
	SourceURL        string
	SourceHash       *string
	LicenceReference *string
	PermittedUse     *string
	AllowedAudience  *string
	SyllabusCode     *string
}

// UpdateInput is the payload for updating a pending Source's metadata. Every field is an
// optional pointer: a nil pointer means "leave unchanged", a non-nil pointer means the
// caller supplied that field and it should be applied. ActorID identifies who made the
// change and is required. Values are applied but never stored in the audit trail.
type UpdateInput struct {
	ActorID          string
	Title            *string
	Owner            *string
	SourceURL        *string
	SourceHash       *string
	LicenceReference *string
	PermittedUse     *string
	AllowedAudience  *string
	SyllabusCode     *string
}

// UpdatableFields lists the JSON field names a PATCH may change, in a stable order used
// for building SQL and for recording changed-field names in audit events.
var UpdatableFields = []string{
	"title",
	"owner",
	"sourceUrl",
	"sourceHash",
	"licenceReference",
	"permittedUse",
	"allowedAudience",
	"syllabusCode",
}

// RequiredApprovalFields lists the field names checked before a source may be approved.
var RequiredApprovalFields = []string{
	"owner",
	"title",
	"sourceUrl",
	"sourceHash",
	"licenceReference",
	"permittedUse",
	"allowedAudience",
}

// MissingApprovalFields returns the names (in RequiredApprovalFields order) of any
// required rights field that is absent or empty on s.
func MissingApprovalFields(s Source) []string {
	var missing []string
	if isBlank(s.Owner) {
		missing = append(missing, "owner")
	}
	if strings.TrimSpace(s.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(s.SourceURL) == "" {
		missing = append(missing, "sourceUrl")
	}
	if isBlank(s.SourceHash) {
		missing = append(missing, "sourceHash")
	}
	if isBlank(s.LicenceReference) {
		missing = append(missing, "licenceReference")
	}
	if isBlank(s.PermittedUse) {
		missing = append(missing, "permittedUse")
	}
	if isBlank(s.AllowedAudience) {
		missing = append(missing, "allowedAudience")
	}
	return missing
}

// isBlank reports whether an optional string field is nil or contains only whitespace.
func isBlank(v *string) bool {
	return v == nil || strings.TrimSpace(*v) == ""
}
