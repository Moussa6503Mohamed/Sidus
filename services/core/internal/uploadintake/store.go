package uploadintake

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("private upload not found")
var ErrInvalidTransition = errors.New("upload lifecycle transition is not allowed")
var ErrSourceNotApproved = errors.New("content source must be approved and catalogue-linked")

type Store interface {
	Create(context.Context, CreateInput) (Upload, error)
	List(context.Context) ([]Upload, error)
	MarkScanClean(context.Context, string, string) (Upload, error)
	RequestDeletion(context.Context, string, string) (Upload, error)
	QueueReview(context.Context, string, string, string) (ReviewJob, error)
}
