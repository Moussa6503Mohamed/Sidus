// Package contentsource implements the content rights/provenance gate: sources start
// pending, require a documented set of rights fields before they can be approved, and
// every approve/reject decision is recorded as an immutable review.
package contentsource

import "time"

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
	if s.Owner == nil || *s.Owner == "" {
		missing = append(missing, "owner")
	}
	if s.Title == "" {
		missing = append(missing, "title")
	}
	if s.SourceURL == "" {
		missing = append(missing, "sourceUrl")
	}
	if s.SourceHash == nil || *s.SourceHash == "" {
		missing = append(missing, "sourceHash")
	}
	if s.LicenceReference == nil || *s.LicenceReference == "" {
		missing = append(missing, "licenceReference")
	}
	if s.PermittedUse == nil || *s.PermittedUse == "" {
		missing = append(missing, "permittedUse")
	}
	if s.AllowedAudience == nil || *s.AllowedAudience == "" {
		missing = append(missing, "allowedAudience")
	}
	return missing
}
