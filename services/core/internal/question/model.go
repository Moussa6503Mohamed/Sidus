// Package question implements private question, licensed-provenance, and versioned-rubric
// infrastructure. It stores runtime question prompts, original rubric structures, and
// metadata-only provenance for licensed adaptations. Runtime records are written by a private
// approved editorial workflow; source material is never stored (no past papers, mark schemes,
// syllabus text, extracted text, diagrams, or OCR output), and no content or licence facts are
// seeded.
//
// Every question is grounded in exactly one curriculum-map node (T-0006). Core is the sole
// authority for that grounding: on every question write it re-validates that the node exists, is
// verified, belongs to the question's syllabus, and that the node's content source still passes
// the T-0006 source gate (approved and linked to the same syllabus). No other package — and no
// AI service — may create, update, verify, or retire a question or rubric version.
//
// This package performs no AI generation, no Anthropic call, no OCR, no ingestion, and no
// question derivation.
package question

import (
	"encoding/json"
	"strings"
	"time"
)

// ResponseType is the shape of answer a question expects.
type ResponseType string

const (
	ResponseMultipleChoice     ResponseType = "multiple_choice"
	ResponseMultiSelect        ResponseType = "multi_select"
	ResponseExactShortAnswer   ResponseType = "exact_short_answer"
	ResponseNumericAnswer      ResponseType = "numeric_answer"
	ResponseShortAnswer        ResponseType = "short_answer"
	ResponseStructuredResponse ResponseType = "structured_response"
	ResponseEssay              ResponseType = "essay"
)

// IsValidResponseType reports whether t is a known response type.
func IsValidResponseType(t ResponseType) bool {
	switch t {
	case ResponseMultipleChoice, ResponseMultiSelect, ResponseExactShortAnswer, ResponseNumericAnswer, ResponseShortAnswer, ResponseStructuredResponse, ResponseEssay:
		return true
	default:
		return false
	}
}

// OriginType declares how a newly-created question was authored. Historical questions created
// before T-0018 retain a nil origin and are never silently classified.
type OriginType string

const (
	OriginOriginal           OriginType = "original"
	OriginLicensedAdaptation OriginType = "licensed_adaptation"
)

func IsValidOriginType(t OriginType) bool {
	return t == OriginOriginal || t == OriginLicensedAdaptation
}

// Status is the lifecycle state of a question.
type Status string

const (
	// StatusDraft is a question being authored: editable, invisible to reader endpoints.
	StatusDraft Status = "draft"
	// StatusVerified is a reviewer/admin-confirmed question, visible to reader endpoints. It
	// requires at least one verified rubric version.
	StatusVerified Status = "verified"
	// StatusRetired is a withdrawn question: kept for history and visible to authorized editorial
	// readers, but unavailable to any future learner-facing surface.
	StatusRetired Status = "retired"
)

// RubricStatus is the lifecycle state of a rubric version. A rubric version is never retired:
// superseding it means creating a new version.
type RubricStatus string

const (
	RubricDraft    RubricStatus = "draft"
	RubricVerified RubricStatus = "verified"
)

// Question is an original question record: curriculum grounding, response shape, language,
// original prompt, lifecycle status, and the revision of its content.
type Question struct {
	ID                  string       `json:"id"`
	SyllabusID          string       `json:"syllabusId"`
	CurriculumMapNodeID string       `json:"curriculumMapNodeId"`
	ResponseType        ResponseType `json:"responseType"`
	Language            string       `json:"language"`
	Prompt              string       `json:"prompt"`
	// Options is question content. It is non-nil only for multiple-choice questions; nil is
	// serialized as null so pre-T-0013 rows remain explicit and backward compatible.
	Options []MultipleChoiceOption `json:"options"`
	// OriginType is nil only for historical pre-T-0018 rows. New creates require an explicit
	// value. Provenance is present exactly for licensed adaptations and is editorial-only.
	OriginType *OriginType         `json:"originType"`
	Provenance *QuestionProvenance `json:"provenance"`
	Status     Status              `json:"status"`
	// CanonicalRubricVersionID is selected explicitly by a reviewer/admin. Historical and draft
	// questions may remain nil; Core never derives or replaces it automatically.
	CanonicalRubricVersionID *string `json:"canonicalRubricVersionId"`
	// ContentRevision starts at 1 and increases by exactly one on every successful draft-content
	// update. It is never caller-settable. A rubric version is only current — and so only counts
	// towards question verification — while its QuestionRevision equals this value.
	ContentRevision int       `json:"contentRevision"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// QuestionProvenance is immutable metadata for one licensed adaptation. It never stores source
// text, copied wording, images, diagrams, answers, mark schemes, or licence-field values.
type QuestionProvenance struct {
	QuestionID      string     `json:"questionId"`
	ContentSourceID string     `json:"contentSourceId"`
	SourceLocator   string     `json:"sourceLocator"`
	OriginType      OriginType `json:"originType"`
	VerifiedActorID string     `json:"verifiedActorId"`
	VerifiedAt      time.Time  `json:"verifiedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// RubricVersion is one immutable, numbered marking rubric for a question. Its content
// (question, question revision, version, rubric structure, maximum marks, creator) never changes
// after creation; only verification metadata does.
type RubricVersion struct {
	ID         string `json:"id"`
	QuestionID string `json:"questionId"`
	Version    int    `json:"version"`
	// QuestionRevision is the question's ContentRevision at the moment this version was created:
	// the exact question content the reviewer marked against. When the question is edited, this
	// version becomes stale for question verification but remains readable and immutable.
	QuestionRevision int             `json:"questionRevision"`
	Rubric           json.RawMessage `json:"rubric"`
	MaxMarks         int             `json:"maxMarks"`
	Status           RubricStatus    `json:"status"`
	CreatedBy        string          `json:"createdBy"`
	ReviewedBy       *string         `json:"reviewedBy"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

// EventType is the kind of change recorded in a question audit event.
type EventType string

const (
	EventQuestionCreated      EventType = "question_created"
	EventQuestionUpdated      EventType = "question_updated"
	EventQuestionVerified     EventType = "question_verified"
	EventQuestionRetired      EventType = "question_retired"
	EventRubricVersionCreated EventType = "rubric_version_created"
	EventRubricVersionVerify  EventType = "rubric_version_verified"
	EventCanonicalRubricSet   EventType = "canonical_rubric_selected"
)

// Event is an immutable audit record of a question or rubric-version mutation. It records which
// fields changed (names only) and who changed them (the verified Clerk subject) — never field
// values, never a prompt, never rubric structure.
type Event struct {
	ID            string    `json:"id"`
	QuestionID    string    `json:"questionId"`
	EventType     EventType `json:"eventType"`
	ActorID       string    `json:"actorId"`
	EventTime     time.Time `json:"eventTime"`
	ChangedFields []string  `json:"changedFields"`
	CreatedAt     time.Time `json:"createdAt"`
}

// CreateInput is the payload for creating a draft Question. The syllabus must equal the mapped
// node's syllabus; status is never caller-settable.
type CreateInput struct {
	ActorID             string
	SyllabusID          string
	CurriculumMapNodeID string
	ResponseType        ResponseType
	Language            string
	Prompt              string
	Options             []MultipleChoiceOption
	OriginType          OriginType
	ContentSourceID     *string
	SourceLocator       *string
}

// UpdateInput is the payload for PATCHing a draft Question. Every field is an optional pointer:
// nil means "leave unchanged". SyllabusID is not patchable — re-point the curriculum-map node
// instead, which re-validates the syllabus match.
type UpdateInput struct {
	ActorID             string
	CurriculumMapNodeID *string
	ResponseType        *ResponseType
	Language            *string
	Prompt              *string
	Options             *[]MultipleChoiceOption
}

// MultipleChoiceOption is original editorial question content. ID is stable across label edits
// and ordering changes so rubric answer keys can refer to an option without copying its label.
type MultipleChoiceOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CreateRubricVersionInput is the payload for appending a draft rubric version to a question.
// The version number is allocated server-side inside the write transaction — it is never
// caller-supplied.
type CreateRubricVersionInput struct {
	ActorID  string
	Rubric   json.RawMessage
	MaxMarks int
}

// blank reports whether a string is empty or whitespace-only.
func blank(s string) bool { return strings.TrimSpace(s) == "" }
