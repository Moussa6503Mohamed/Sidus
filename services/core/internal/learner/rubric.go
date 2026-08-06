package learner

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

var (
	learnerRubricFields    = map[string]struct{}{"criteria": {}, "answerKey": {}, "feedback": {}}
	learnerAnswerKeyFields = map[string]struct{}{"correctOptionId": {}}
	learnerFeedbackFields  = map[string]struct{}{"correctExplanation": {}, "incorrectExplanations": {}}
	learnerIncorrectFields = map[string]struct{}{"optionId": {}, "explanation": {}}
)

type markingRubric struct {
	CorrectOptionID string
	Feedback        Feedback
}

// parseMarkingRubric accepts only T-0016's exact verified MCQ feedback shape. Old historical
// rubrics remain stored but cannot silently become Practice marking data.
func parseMarkingRubric(raw json.RawMessage) (markingRubric, error) {
	fields, ok := exactObject(raw, learnerRubricFields)
	if !ok || fields["answerKey"] == nil || fields["feedback"] == nil {
		return markingRubric{}, ErrNotFound
	}
	answer, ok := exactObject(fields["answerKey"], learnerAnswerKeyFields)
	if !ok || len(answer) != 1 {
		return markingRubric{}, ErrNotFound
	}
	var correct string
	if json.Unmarshal(answer["correctOptionId"], &correct) != nil || strings.TrimSpace(correct) == "" {
		return markingRubric{}, ErrNotFound
	}
	correct = strings.TrimSpace(correct)
	feedbackFields, ok := exactObject(fields["feedback"], learnerFeedbackFields)
	if !ok || len(feedbackFields) != 2 {
		return markingRubric{}, ErrNotFound
	}
	var feedback Feedback
	if json.Unmarshal(feedbackFields["correctExplanation"], &feedback.CorrectExplanation) != nil || strings.TrimSpace(feedback.CorrectExplanation) == "" {
		return markingRubric{}, ErrNotFound
	}
	var items []json.RawMessage
	if json.Unmarshal(feedbackFields["incorrectExplanations"], &items) != nil || len(items) == 0 {
		return markingRubric{}, ErrNotFound
	}
	seen := map[string]struct{}{}
	for _, rawItem := range items {
		itemFields, ok := exactObject(rawItem, learnerIncorrectFields)
		if !ok || len(itemFields) != 2 {
			return markingRubric{}, ErrNotFound
		}
		var item IncorrectExplanation
		if json.Unmarshal(itemFields["optionId"], &item.OptionID) != nil || json.Unmarshal(itemFields["explanation"], &item.Explanation) != nil {
			return markingRubric{}, ErrNotFound
		}
		item.OptionID = strings.TrimSpace(item.OptionID)
		if item.OptionID == "" || item.OptionID == correct || strings.TrimSpace(item.Explanation) == "" {
			return markingRubric{}, ErrNotFound
		}
		if _, duplicate := seen[item.OptionID]; duplicate {
			return markingRubric{}, ErrNotFound
		}
		seen[item.OptionID] = struct{}{}
		feedback.IncorrectExplanations = append(feedback.IncorrectExplanations, item)
	}
	return markingRubric{CorrectOptionID: correct, Feedback: feedback}, nil
}

func exactObject(raw json.RawMessage, allowed map[string]struct{}) (map[string]json.RawMessage, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, false
	}
	fields := map[string]json.RawMessage{}
	for dec.More() {
		keyToken, err := dec.Token()
		key, isString := keyToken.(string)
		if err != nil || !isString {
			return nil, false
		}
		if _, allowedKey := allowed[key]; !allowedKey {
			return nil, false
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, false
		}
		var value json.RawMessage
		if dec.Decode(&value) != nil {
			return nil, false
		}
		fields[key] = value
	}
	if _, err := dec.Token(); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return fields, true
}
