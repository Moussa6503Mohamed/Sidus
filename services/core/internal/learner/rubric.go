package learner

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
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

// validatePinnedAnswerSchema validates only marking data inside an already canonical, verified
// rubric. It never returns that key on learner reads; malformed pinned data makes a question
// unavailable rather than guessing at marking.
func validatePinnedAnswerSchema(responseType ResponseType, raw, optionsRaw json.RawMessage) error {
	if responseType == ResponseMultipleChoice {
		marking, err := parseMarkingRubric(raw)
		if err != nil {
			return err
		}
		var options []Option
		if json.Unmarshal(optionsRaw, &options) != nil || !markingCoversOptions(marking, options) {
			return ErrNotFound
		}
		return nil
	}
	fields, ok := exactObject(raw, learnerRubricFields)
	if !ok {
		return ErrNotFound
	}
	if responseType == ResponseShortAnswer || responseType == ResponseStructuredResponse || responseType == ResponseEssay {
		if fields["answerKey"] != nil || fields["feedback"] != nil {
			return ErrNotFound
		}
		return nil
	}
	answerRaw := fields["answerKey"]
	if answerRaw == nil || fields["feedback"] != nil {
		return ErrNotFound
	}
	answer, ok := exactObject(answerRaw, map[string]struct{}{"correctOptionIds": {}, "acceptedAnswers": {}, "numericValue": {}, "tolerance": {}})
	if !ok {
		return ErrNotFound
	}
	switch responseType {
	case ResponseMultiSelect:
		var ids []string
		if len(answer) != 1 || json.Unmarshal(answer["correctOptionIds"], &ids) != nil || len(ids) < 2 {
			return ErrNotFound
		}
		var options []Option
		if json.Unmarshal(optionsRaw, &options) != nil {
			return ErrNotFound
		}
		valid := map[string]struct{}{}
		for _, opt := range options {
			valid[opt.ID] = struct{}{}
		}
		seen := map[string]struct{}{}
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				return ErrNotFound
			}
			if _, ok := valid[id]; !ok {
				return ErrNotFound
			}
			if _, dup := seen[id]; dup {
				return ErrNotFound
			}
			seen[id] = struct{}{}
		}
		return nil
	case ResponseExactShortAnswer:
		var answers []string
		if len(answer) != 1 || json.Unmarshal(answer["acceptedAnswers"], &answers) != nil || len(answers) == 0 {
			return ErrNotFound
		}
		for _, v := range answers {
			if normalizeAnswer(v) == "" {
				return ErrNotFound
			}
		}
		return nil
	case ResponseNumericAnswer:
		var value, tolerance float64
		if len(answer) != 2 || json.Unmarshal(answer["numericValue"], &value) != nil || json.Unmarshal(answer["tolerance"], &tolerance) != nil || math.IsNaN(value) || math.IsInf(value, 0) || math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance < 0 {
			return ErrNotFound
		}
		return nil
	}
	return ErrNotFound
}

func normalizeAnswer(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
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
