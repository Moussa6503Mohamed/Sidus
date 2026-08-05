package question

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a question does not exist (or is not visible to the caller).
var ErrNotFound = errors.New("question not found")

// ErrRubricVersionNotFound is returned when a rubric version does not exist for a question.
var ErrRubricVersionNotFound = errors.New("rubric version not found")

// ErrUnknownSyllabus is returned when the target syllabus does not resolve to a known, active
// catalogue syllabus.
var ErrUnknownSyllabus = errors.New("syllabus is unknown or not active")

// ErrUnknownNode is returned when the referenced curriculum-map node does not exist.
var ErrUnknownNode = errors.New("curriculum map node is unknown")

// ErrUnverifiedNode is returned when the referenced curriculum-map node is not verified (it is
// still a draft, or it has been retired).
var ErrUnverifiedNode = errors.New("curriculum map node is not verified")

// ErrMismatchedNode is returned when the referenced curriculum-map node belongs to a different
// syllabus than the question.
var ErrMismatchedNode = errors.New("curriculum map node belongs to a different syllabus")

// ErrUnknownSource is returned when the node's content source does not exist.
var ErrUnknownSource = errors.New("content source is unknown")

// ErrUnapprovedSource is returned when the node's content source is not approved.
var ErrUnapprovedSource = errors.New("content source is not approved")

// ErrUnlinkedSource is returned when the node's content source has no linked catalogue syllabus.
var ErrUnlinkedSource = errors.New("content source is not linked to a catalogue syllabus")

// ErrMismatchedSource is returned when the node's content source is linked to a different
// syllabus than the question.
var ErrMismatchedSource = errors.New("content source is linked to a different syllabus")

// ErrNoChanges is returned when an update supplies no field that actually differs.
var ErrNoChanges = errors.New("supplied fields match current values")

// ErrInvalidTransition is returned when a lifecycle action is attempted from a status that does
// not permit it.
var ErrInvalidTransition = errors.New("question lifecycle transition is not allowed")

// ErrMissingVerifiedRubric is returned when a question is verified while it has no verified
// rubric version at all.
var ErrMissingVerifiedRubric = errors.New("question requires at least one verified rubric version")

// ErrStaleVerifiedRubric is returned when a question has verified rubric versions, but every one
// of them was reviewed against an older revision of the question's content. It is deliberately
// distinct from ErrMissingVerifiedRubric: "you reviewed a rubric, then edited the question" needs
// a different fix (append and verify a new rubric version) from "you never reviewed a rubric".
var ErrStaleVerifiedRubric = errors.New("every verified rubric version was reviewed against an older question revision")

// ErrInvalidRubric is returned when a rubric payload does not match the validation-safe schema.
var ErrInvalidRubric = errors.New("rubric structure is invalid")

// ErrInvalidMaxMarks is returned when maximum marks are not positive, or do not match the sum of
// the rubric's criterion marks.
var ErrInvalidMaxMarks = errors.New("maximum marks are invalid for this rubric")

// ErrDuplicateRubricVersion is returned when a rubric version number collides for a question.
// Version numbers are allocated server-side under a row lock, so this indicates a concurrent
// allocation that lost the race, never a caller-supplied number.
var ErrDuplicateRubricVersion = errors.New("rubric version already exists for this question")

// Store persists questions, their rubric versions, and their immutable audit events.
//
// Every mutating method re-validates, inside its write transaction and before any row is
// written: the question's curriculum-map node exists, is verified, belongs to the question's
// syllabus, and that node's content source still passes the T-0006 source gate. A question whose
// node or source has regressed is frozen — it cannot be edited, given a rubric version, verified,
// or retired — until the node/source is restored. Freezing is preferred over writing against
// grounding that no longer passes the rights gate.
type Store interface {
	// CreateQuestion creates a draft question. Status is never caller-settable.
	CreateQuestion(ctx context.Context, in CreateInput) (Question, error)

	// UpdateQuestion applies a partial update to a draft question (ErrInvalidTransition
	// otherwise). A supplied node is validated as the effective node; otherwise the stored one
	// is re-validated. A successful update increments the question's ContentRevision by exactly
	// one; ErrNoChanges and any rejected write leave it untouched.
	UpdateQuestion(ctx context.Context, id string, in UpdateInput) (Question, error)

	// VerifyQuestion transitions a draft question to verified. It requires at least one verified
	// rubric version whose QuestionRevision equals the question's current ContentRevision —
	// otherwise ErrMissingVerifiedRubric (none verified at all) or ErrStaleVerifiedRubric (all
	// verified versions predate the question's current content).
	VerifyQuestion(ctx context.Context, id string, actorID string) (Question, error)

	// RetireQuestion transitions a draft or verified question to retired, hiding it from every
	// reader endpoint.
	RetireQuestion(ctx context.Context, id string, actorID string) (Question, error)

	// GetQuestion returns a question by ID to an authorized editorial reader, regardless of
	// lifecycle status.
	GetQuestion(ctx context.Context, id string) (Question, error)

	// ListQuestions returns questions for a syllabus, optionally narrowed to one curriculum-map
	// node. The syllabus must resolve to a known active catalogue syllabus (ErrUnknownSyllabus
	// otherwise) and a supplied node filter must exist (ErrUnknownNode) and belong to that
	// syllabus (ErrMismatchedNode) — neither an unknown syllabus nor an unknown or foreign node
	// is ever reported as an empty list. A nil status returns all lifecycle states.
	ListQuestions(ctx context.Context, syllabusID string, nodeID *string, status *Status) ([]Question, error)

	// CreateRubricVersion appends a draft rubric version to a draft question, allocating the next
	// positive version number under a row lock on the question and stamping the question's
	// current ContentRevision onto the version.
	CreateRubricVersion(ctx context.Context, questionID string, in CreateRubricVersionInput) (RubricVersion, error)

	// ListRubricVersions returns every rubric version for a question, oldest first. It is an
	// editorial read: it exposes draft rubric structure and is permissioned separately.
	ListRubricVersions(ctx context.Context, questionID string) ([]RubricVersion, error)

	// VerifyRubricVersion transitions a draft rubric version to verified. The parent question
	// must be draft or verified, and the stored rubric must still validate against its maximum
	// marks.
	VerifyRubricVersion(ctx context.Context, questionID string, version int, actorID string) (RubricVersion, error)
}
