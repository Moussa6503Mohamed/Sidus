package learner

import (
	"context"
	"errors"
)

// MarkingStatus is deliberately separate from learner_attempts: submitted attempt pins stay immutable.
type MarkingStatus string

const (
	MarkingPending  MarkingStatus = "pending"
	MarkingAccepted MarkingStatus = "accepted"
	MarkingWithheld MarkingStatus = "withheld"
)

type CriterionMark struct {
	CriterionID  string `json:"criterionId"`
	MarksAwarded int    `json:"marksAwarded"`
	Feedback     string `json:"feedback"`
}
type MarkingResult struct {
	CriterionMarks []CriterionMark `json:"criterionMarks"`
	AwardedMarks   int             `json:"awardedMarks"`
	MaxMarks       int             `json:"maxMarks"`
	Model          string          `json:"model"`
	ModelVersion   string          `json:"modelVersion"`
	CostUSDMicros  int64           `json:"costUsdMicros"`
	Confidence     float64         `json:"confidence"`
}

// MarkingProjection is learner-safe: no answer, rubric, canonical pin, provenance or source data.
type MarkingProjection struct {
	AttemptID      string         `json:"attemptId"`
	Status         MarkingStatus  `json:"status"`
	RetryCount     int            `json:"retryCount"`
	WithheldReason string         `json:"withheldReason,omitempty"`
	Result         *MarkingResult `json:"result,omitempty"`
}
type MarkingJob struct {
	RequestID       string
	AttemptID       string
	QuestionID      string
	SyllabusID      string
	RubricVersionID string
	Criteria        []RubricCriterion
}
type RubricCriterion struct {
	ID       string
	MaxMarks int
}
type MarkingOutcome struct {
	Status MarkingStatus
	Result *MarkingResult
	Reason string
}

type SonnetMarker interface {
	MarkWrittenAttempt(context.Context, MarkingJob) (MarkingOutcome, error)
}
type MarkingStore interface {
	RequestMarking(context.Context, string, string) (MarkingJob, MarkingProjection, bool, error)
	ApplyMarking(context.Context, string, MarkingOutcome) (MarkingProjection, error)
	GetMarking(context.Context, string, string) (MarkingProjection, error)
}

var ErrMarkingNotFound = errors.New("written marking request not found")
var ErrMarkingIneligible = errors.New("written attempt is not eligible for marking")
var ErrInvalidMarkingOutcome = errors.New("marking result failed validation")

func isWrittenResponse(t ResponseType) bool {
	return t == ResponseShortAnswer || t == ResponseStructuredResponse || t == ResponseEssay
}
