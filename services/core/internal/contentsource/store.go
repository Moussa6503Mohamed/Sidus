package contentsource

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a Source does not exist.
var ErrNotFound = errors.New("content source not found")

// ErrInvalidTransition is returned when approve/reject is attempted on a Source that is
// not currently pending.
var ErrInvalidTransition = errors.New("content source is not pending")

// ErrDuplicateSourceURL is returned when creating a Source whose sourceUrl already exists.
var ErrDuplicateSourceURL = errors.New("content source with this sourceUrl already exists")

// ErrNoUpdatableFields is returned when an update supplies no updatable field.
var ErrNoUpdatableFields = errors.New("no updatable fields supplied")

// ApproveInput is the payload for approving a Source.
type ApproveInput struct {
	ReviewerID   string
	DecisionDate time.Time
}

// RejectInput is the payload for rejecting a Source.
type RejectInput struct {
	ReviewerID   string
	Reason       string
	DecisionDate time.Time
}

// Store persists content sources and their reviews.
type Store interface {
	Create(ctx context.Context, in CreateInput) (Source, error)
	Get(ctx context.Context, id string) (Source, error)
	List(ctx context.Context, status *Status) ([]Source, error)

	// Approve validates rights fields before transitioning a source to approved. If
	// required fields are missing, it returns the unchanged source, the list of missing
	// field names, and a nil error: this is an expected/handled outcome, not a fault.
	Approve(ctx context.Context, id string, in ApproveInput) (source Source, missing []string, err error)

	// Reject transitions a source to rejected and records the reason.
	Reject(ctx context.Context, id string, in RejectInput) (Source, error)

	// Update applies supplied metadata fields to a pending source, bumps updated_at, and
	// appends an immutable metadata_updated event listing the changed field names. It
	// returns ErrInvalidTransition if the source is not pending and ErrDuplicateSourceURL
	// if the new sourceUrl collides. changed lists the names of fields actually applied.
	Update(ctx context.Context, id string, in UpdateInput) (source Source, changed []string, err error)
}
