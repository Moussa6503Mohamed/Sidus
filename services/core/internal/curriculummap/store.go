package curriculummap

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a node does not exist.
var ErrNotFound = errors.New("curriculum map node not found")

// ErrDuplicateNodeCode is returned when a node's code collides with another node's code
// within the same syllabus.
var ErrDuplicateNodeCode = errors.New("node code already exists for this syllabus")

// ErrUnknownSyllabus is returned when the target syllabus does not resolve to a known, active
// catalogue syllabus.
var ErrUnknownSyllabus = errors.New("syllabus is unknown or not active")

// ErrInvalidParent is returned when a supplied parent node does not exist, belongs to a
// different syllabus, or would create a cycle in the hierarchy.
var ErrInvalidParent = errors.New("parent node is invalid")

// ErrUnknownSource is returned when the referenced content source does not exist.
var ErrUnknownSource = errors.New("content source is unknown")

// ErrUnapprovedSource is returned when the referenced content source is not approved.
var ErrUnapprovedSource = errors.New("content source is not approved")

// ErrUnlinkedSource is returned when the referenced content source has no linked catalogue
// syllabus.
var ErrUnlinkedSource = errors.New("content source is not linked to a catalogue syllabus")

// ErrMismatchedSource is returned when the referenced content source's linked catalogue
// syllabus does not match the node's syllabus.
var ErrMismatchedSource = errors.New("content source is linked to a different syllabus")

// ErrNoChanges is returned when an update supplies no field that actually differs.
var ErrNoChanges = errors.New("supplied fields match current values")

// ErrInvalidTransition is returned when a lifecycle action (update/verify/retire) is attempted
// from a status that does not permit it.
var ErrInvalidTransition = errors.New("curriculum map node lifecycle transition is not allowed")

// Store persists curriculum-map nodes and their immutable audit events.
type Store interface {
	// CreateNode creates a new draft node. It validates, before any write: the syllabus is a
	// known active catalogue syllabus; the parent (if supplied) exists and belongs to the same
	// syllabus, without creating a cycle; and the content source is approved, linked, and
	// matches the syllabus.
	CreateNode(ctx context.Context, in CreateInput) (Node, error)

	// UpdateNode applies a partial update to a draft node. Only a node whose status is draft
	// may be updated (ErrInvalidTransition otherwise). The parent gate applies to any changed
	// parent, and the source gate is re-run on every update — against the supplied source if
	// any, otherwise the node's stored source — because a source can be un-approved, unlinked,
	// or re-linked after the node was created. A failed gate writes nothing.
	UpdateNode(ctx context.Context, id string, in UpdateInput) (Node, error)

	// VerifyNode transitions a draft node to verified. The node's stored source must still pass
	// the source gate; otherwise nothing is written.
	VerifyNode(ctx context.Context, id string, actorID string) (Node, error)

	// RetireNode transitions a draft or verified node to retired. The node's stored source must
	// still pass the source gate; otherwise nothing is written.
	RetireNode(ctx context.Context, id string, actorID string) (Node, error)

	// GetNode returns a node by ID, regardless of lifecycle status. Every caller of this method
	// is gated by curriculum_map:read, which only editor/reviewer/admin hold — no non-editorial
	// consumer of this route exists — so there is no "reader" population to hide a draft or
	// retired node from (see D-0008 "Update (T-0010)").
	GetNode(ctx context.Context, id string) (Node, error)

	// ListNodesBySyllabus returns nodes for a syllabus, optionally filtered to one lifecycle
	// status. The syllabus must resolve to a known, active catalogue syllabus
	// (ErrUnknownSyllabus otherwise) — an unknown or inactive syllabus is never reported as an
	// empty list. When status is nil, nodes of every status are returned; a known active
	// syllabus with no matching nodes yields an empty slice.
	ListNodesBySyllabus(ctx context.Context, syllabusID string, status *Status) ([]Node, error)
}
