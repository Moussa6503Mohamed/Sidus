package catalogue

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a subject or syllabus does not exist.
var ErrNotFound = errors.New("catalogue record not found")

// ErrDuplicateSubject is returned when creating a subject whose name already exists.
var ErrDuplicateSubject = errors.New("subject with this name already exists")

// ErrDuplicateSyllabus is returned when creating/updating a syllabus that collides with an
// existing (board, syllabus_code, track) identity.
var ErrDuplicateSyllabus = errors.New("syllabus with this board, code, and track already exists")

// ErrSubjectNotFound is returned when a syllabus references a subject ID that does not exist.
var ErrSubjectNotFound = errors.New("referenced subject does not exist")

// ErrNoChanges is returned when an update supplies no field that actually differs.
var ErrNoChanges = errors.New("supplied fields match current values")

// Store persists catalogue subjects and syllabuses and their immutable audit events.
type Store interface {
	CreateSubject(ctx context.Context, in CreateSubjectInput) (Subject, error)
	ListSubjects(ctx context.Context) ([]Subject, error)

	CreateSyllabus(ctx context.Context, in CreateSyllabusInput) (Syllabus, error)
	UpdateSyllabus(ctx context.Context, id string, in UpdateSyllabusInput) (Syllabus, error)

	// GetSyllabus returns a syllabus by ID. When activeOnly is true, a non-active syllabus
	// is reported as ErrNotFound (readers only see active syllabuses).
	GetSyllabus(ctx context.Context, id string, activeOnly bool) (Syllabus, error)

	// ListSyllabuses returns syllabuses. When activeOnly is true, only active ones.
	ListSyllabuses(ctx context.Context, activeOnly bool) ([]Syllabus, error)

	// ResolveActiveSyllabusByCode resolves a syllabus code to the ID of the single active
	// syllabus with that code. found is false when zero or more than one active syllabus
	// carries the code (unknown or ambiguous); err is reserved for infrastructure failures.
	// This backs registry-based content-source validation without exposing catalogue error
	// types across package boundaries.
	ResolveActiveSyllabusByCode(ctx context.Context, code string) (id string, found bool, err error)
}
