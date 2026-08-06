// Package learner implements the learner-safe, verified-question delivery projection (T-0015).
// It is a strictly read-only surface over the private editorial infrastructure built in
// T-0007/T-0013/T-0014 (services/core/internal/question): a structurally reduced view that can
// never carry a question's lifecycle status, canonical rubric id, rubric structure, answer key,
// marks, event data, actor identity, timestamps, or internal source metadata.
//
// Every type in this package is independent of the question package by design — it does not
// import it — so the learner projection can never gain an editorial field by accident (e.g. by
// someone widening a shared struct). The eligibility gate that decides which questions are
// reachable through this package is re-validated on every read; nothing is cached or trusted
// from a prior write.
package learner

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
