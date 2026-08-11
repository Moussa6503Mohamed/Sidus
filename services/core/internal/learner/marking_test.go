package learner

import "testing"

func TestValidMarkingResultRequiresExactPinnedCriteria(t *testing.T) {
	rubric := []byte(`{"criteria":[{"id":"one","marks":2},{"id":"two","marks":1}]}`)
	valid := MarkingResult{CriterionMarks: []CriterionMark{{CriterionID: "one", MarksAwarded: 2, Feedback: "ok"}, {CriterionID: "two", MarksAwarded: 1, Feedback: "ok"}}, AwardedMarks: 3, MaxMarks: 3, Model: "fake", ModelVersion: "v1", Confidence: .9}
	if !validMarkingResult(valid, rubric) {
		t.Fatal("expected exact result to pass")
	}
	valid.CriterionMarks[1] = CriterionMark{CriterionID: "foreign", MarksAwarded: 0, Feedback: "ok"}
	if validMarkingResult(valid, rubric) {
		t.Fatal("foreign zero-mark criterion must not replace a pinned criterion")
	}
}
