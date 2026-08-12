package teacher

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrInviteUnavailable = errors.New("invite unavailable")
var ErrInvalidAssignment = errors.New("invalid assignment")

type Store interface {
	CreateClass(context.Context, string, string) (Class, error)
	ListClasses(context.Context, string) ([]Class, error)
	CreateInvite(context.Context, string, string, time.Duration, int) (Invite, error)
	Roster(context.Context, string, string) ([]RosterMember, error)
	AcceptInvite(context.Context, string, string) (Class, error)
	RevokeMembership(context.Context, string, string) error
	CreateAssignment(context.Context, string, string, CreateAssignmentInput) (Assignment, error)
	ListAssignments(context.Context, string, string) ([]Assignment, error)
	ListLearnerAssignments(context.Context, string) ([]LearnerAssignment, error)
	StartAssignment(context.Context, string, string) (StartedAssignment, error)
}
