// Package catalogue implements the curriculum catalogue: a metadata-only registry of
// subjects and syllabuses. It is the single authority for which syllabuses exist, so future
// approved Cambridge syllabuses can be onboarded as data without code changes. It never
// stores source material, questions, objectives, assessment rules, or rights claims.
package catalogue

import (
	"strings"
	"time"
)

// SyllabusStatus is the lifecycle state of a catalogue syllabus.
type SyllabusStatus string

const (
	// StatusDraft is a syllabus being prepared; not yet visible to readers or resolvable.
	StatusDraft SyllabusStatus = "draft"
	// StatusActive is a live syllabus: readable and resolvable for content-source association.
	StatusActive SyllabusStatus = "active"
	// StatusRetired is a withdrawn syllabus: kept for history, not resolvable for new writes.
	StatusRetired SyllabusStatus = "retired"
)

// Subject is a normalized subject (e.g. "Biology"). Requests reference subjects by ID, never
// by free-text, so board/subject/level values are never trusted from content-source callers.
type Subject struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Syllabus is a curriculum-catalogue record: identity metadata only.
type Syllabus struct {
	ID             string         `json:"id"`
	Board          string         `json:"board"`
	SyllabusCode   string         `json:"syllabusCode"`
	SubjectID      string         `json:"subjectId"`
	Qualification  string         `json:"qualification"`
	Track          *string        `json:"track"`
	DisplayName    string         `json:"displayName"`
	CurriculumYear *string        `json:"curriculumYear"`
	Status         SyllabusStatus `json:"status"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// EventType is the kind of change recorded in a syllabus audit event.
type EventType string

const (
	// EventSyllabusCreated records the creation of a catalogue syllabus.
	EventSyllabusCreated EventType = "syllabus_created"
	// EventSyllabusUpdated records a metadata change to a catalogue syllabus.
	EventSyllabusUpdated EventType = "syllabus_updated"
)

// Event is an immutable audit record of a catalogue mutation. It records which fields were
// set/changed (names only) and who changed them (the verified Clerk subject) — never the
// field values, and never any source material.
type Event struct {
	ID            string    `json:"id"`
	SyllabusID    string    `json:"syllabusId"`
	EventType     EventType `json:"eventType"`
	ActorID       string    `json:"actorId"`
	EventTime     time.Time `json:"eventTime"`
	ChangedFields []string  `json:"changedFields"`
	CreatedAt     time.Time `json:"createdAt"`
}

// CreateSubjectInput is the payload for registering a subject.
type CreateSubjectInput struct {
	ActorID string
	Name    string
}

// CreateSyllabusInput is the payload for registering a syllabus. Track and CurriculumYear are
// optional (nil = not applicable / not known). Status defaults to draft when empty.
type CreateSyllabusInput struct {
	ActorID        string
	Board          string
	SyllabusCode   string
	SubjectID      string
	Qualification  string
	Track          *string
	DisplayName    string
	CurriculumYear *string
	Status         SyllabusStatus
}

// UpdateSyllabusInput is the payload for changing catalogue metadata on a syllabus. Every
// field is an optional pointer: nil means "leave unchanged". ActorID is required.
type UpdateSyllabusInput struct {
	ActorID        string
	Board          *string
	SyllabusCode   *string
	SubjectID      *string
	Qualification  *string
	Track          *string
	DisplayName    *string
	CurriculumYear *string
	Status         *SyllabusStatus
}

// IsValidStatus reports whether s is a known lifecycle status.
func IsValidStatus(s SyllabusStatus) bool {
	switch s {
	case StatusDraft, StatusActive, StatusRetired:
		return true
	default:
		return false
	}
}

// blank reports whether a string is empty or whitespace-only.
func blank(s string) bool { return strings.TrimSpace(s) == "" }
