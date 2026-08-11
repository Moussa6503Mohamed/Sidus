package learner

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a question does not exist, or exists but does not currently meet
// every eligibility gate. The two cases are deliberately indistinguishable to a caller: reporting
// "exists but ineligible" separately would leak the existence of a draft/retired/ungrounded
// question to a role that must never see it.
var ErrNotFound = errors.New("question not found")

// ErrUnknownSyllabus is returned when the target syllabus does not resolve to a known, active
// catalogue syllabus.
var ErrUnknownSyllabus = errors.New("syllabus is unknown or not active")

// ErrUnknownNode is returned when an optional curriculum-map node filter does not exist.
var ErrUnknownNode = errors.New("curriculum map node is unknown")

// ErrMismatchedNode is returned when an optional curriculum-map node filter belongs to a
// different syllabus than the one supplied.
var ErrMismatchedNode = errors.New("curriculum map node belongs to a different syllabus")

var ErrAttemptNotFound = errors.New("attempt not found")
var ErrAttemptSubmitted = errors.New("attempt already submitted")
var ErrInvalidOption = errors.New("selected option is invalid")
var ErrInvalidAnswer = errors.New("answer is invalid")

// Store serves the learner-safe question projection. Every method re-validates the full
// eligibility gate at read time: verified question status, an existing verified canonical
// rubric version stamped at the question's current content revision, a verified curriculum-map
// node, and an approved content source still linked to the question's syllabus. Core never falls
// back to "latest rubric" — only the explicit canonical selection (D-0016) counts.
type Store interface {
	// ListQuestions returns eligible questions for an active catalogue syllabus, optionally
	// narrowed to one curriculum-map node belonging to that syllabus. An unknown/inactive
	// syllabus or an unknown/foreign node filter is a stable error, never an empty list a caller
	// cannot distinguish from "no eligible questions yet".
	ListQuestions(ctx context.Context, syllabusID string, nodeID *string) ([]Projection, error)

	// GetQuestion returns one eligible question projection by id. A question that exists but is
	// draft, retired, has no current verified canonical rubric, or whose grounding node/source
	// has regressed is reported identically to a question that does not exist: ErrNotFound.
	GetQuestion(ctx context.Context, id string) (Projection, error)

	// ListActiveSyllabuses returns the learner-safe discovery projection for every catalogue
	// syllabus currently active. There is no filter and no error case beyond infrastructure
	// failure — an empty result is a valid, non-error response (no active syllabuses yet).
	ListActiveSyllabuses(ctx context.Context) ([]Syllabus, error)

	// ListModules returns learner-safe module picker entries for one active syllabus. Only verified
	// modules with at least one question passing the same live eligibility gate as ListQuestions
	// are discoverable; this prevents empty/draft/source-only curriculum structure from leaking.
	ListModules(ctx context.Context, syllabusID string) ([]Module, error)

	CreateAttempt(ctx context.Context, learnerSubjectID, questionID string) (Attempt, error)
	SubmitAttempt(ctx context.Context, learnerSubjectID, attemptID, selectedOptionID string) (AttemptResult, error)
}
