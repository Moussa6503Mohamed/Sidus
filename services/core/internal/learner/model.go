// Package learner implements learner-safe verified-question delivery and owned Practice attempts.
// Question reads remain a strictly reduced surface over private editorial infrastructure built in
// T-0007/T-0013/T-0014 (services/core/internal/question): a structurally reduced view that can
// never carry a question's lifecycle status, canonical rubric id, rubric structure, answer key,
// marks, event data, actor identity, timestamps, question origin, provenance, licence facts, or
// internal source metadata.
//
// Every type in this package is independent of the question package by design — it does not
// import it — so the learner projection can never gain an editorial field by accident (e.g. by
// someone widening a shared struct). The eligibility gate that decides which questions are
// reachable through this package is re-validated on every read; nothing is cached or trusted
// from a prior write.
package learner

import "time"

// ResponseType mirrors question.ResponseType's wire values for the learner projection.
type ResponseType string

const (
	ResponseMultipleChoice     ResponseType = "multiple_choice"
	ResponseShortAnswer        ResponseType = "short_answer"
	ResponseStructuredResponse ResponseType = "structured_response"
)

// Option is one original, learner-visible multiple-choice option: a stable id and its label. It
// never carries a correctness flag — that lives only in the rubric answer key, which this
// package never reads or returns.
type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Projection is the explicit, exhaustive learner-safe question contract. Every field here is
// intentionally safe to reveal to any authenticated recognized-role caller. Nothing else may be
// added to this struct without updating this comment and docs/learner-question-delivery.md's
// exclusion list.
type Projection struct {
	ID                  string       `json:"id"`
	SyllabusID          string       `json:"syllabusId"`
	CurriculumMapNodeID string       `json:"curriculumMapNodeId"`
	ResponseType        ResponseType `json:"responseType"`
	Language            string       `json:"language"`
	Prompt              string       `json:"prompt"`
	// Options is present only for multiple_choice questions; nil serializes as null.
	Options         []Option `json:"options"`
	ContentRevision int      `json:"contentRevision"`
}

// Syllabus is the learner-safe active-syllabus discovery projection (review fix on T-0015): the
// minimal identity fields a practice screen needs to populate a syllabus picker. It deliberately
// does not reuse or extend catalogue.Syllabus — this package never imports catalogue, for the
// same reason it never imports question — so a future field added to the editorial catalogue
// type (e.g. rights/source metadata) can never widen this response by accident. It carries no
// source metadata, rights data, timestamps, actor ids, events, or curriculum objective content.
type Syllabus struct {
	ID            string  `json:"id"`
	Board         string  `json:"board"`
	SyllabusCode  string  `json:"syllabusCode"`
	Qualification string  `json:"qualification"`
	Track         *string `json:"track"`
	DisplayName   string  `json:"displayName"`
}

// Module is the learner-safe label for one verified curriculum-map entry that currently has at
// least one eligible learner question. It intentionally contains no source, provenance, status,
// locator, parent, or editorial lifecycle data. The internal database term remains
// curriculum_map_nodes; learner surfaces call these Modules.
type Module struct {
	ID         string `json:"id"`
	SyllabusID string `json:"syllabusId"`
	Code       string `json:"code"`
	Label      string `json:"label"`
}

type AttemptStatus string

const (
	AttemptOpen      AttemptStatus = "open"
	AttemptSubmitted AttemptStatus = "submitted"
)

// Attempt is learner-safe open-attempt acknowledgement. Pins remain internal.
type Attempt struct {
	AttemptID  string        `json:"attemptId"`
	QuestionID string        `json:"questionId"`
	Status     AttemptStatus `json:"status"`
	MaxMarks   int           `json:"maxMarks"`
}

type IncorrectExplanation struct {
	OptionID    string `json:"optionId"`
	Explanation string `json:"explanation"`
}

type Feedback struct {
	CorrectExplanation    string                 `json:"correctExplanation"`
	IncorrectExplanations []IncorrectExplanation `json:"incorrectExplanations"`
}

// AttemptResult is exhaustive learner-safe post-submit projection.
type AttemptResult struct {
	AttemptID        string   `json:"attemptId"`
	QuestionID       string   `json:"questionId"`
	SelectedOptionID string   `json:"selectedOptionId"`
	CorrectOptionID  string   `json:"correctOptionId"`
	IsCorrect        bool     `json:"isCorrect"`
	AwardedMarks     int      `json:"awardedMarks"`
	MaxMarks         int      `json:"maxMarks"`
	Feedback         Feedback `json:"feedback"`
}

type attemptRecord struct {
	ID                       string
	LearnerSubjectID         string
	QuestionID               string
	QuestionContentRevision  int
	CanonicalRubricVersionID string
	Status                   AttemptStatus
	MaxMarks                 int
	CreatedAt                time.Time
}
