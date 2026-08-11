package question

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strings"
)

// maxRubricCriteria bounds a rubric so a pathological payload cannot be stored or walked.
const maxRubricCriteria = 200

// rubricDocumentFields and rubricCriterionFields are the EXACT, case-sensitive key sets accepted
// at each level of a rubric. They are the whole schema: anything else — an unknown key, a nested
// key at the wrong level, or a case variant such as `Criteria`, `ID`, `Marks`, or `Descriptor` —
// is rejected.
//
// A case-sensitive check is load-bearing rather than pedantic. Go's struct decoding matches JSON
// field names case-insensitively, so decoding into a struct (even with DisallowUnknownFields)
// silently accepts `{"Criteria":[{"ID":"c1","Marks":2}]}` — which would have let a payload that
// does not match the documented schema, and does not match what the TypeScript contract or a
// future marking consumer expects, be stored as a verified rubric.
var (
	rubricDocumentFields  = map[string]struct{}{"criteria": {}, "answerKey": {}, "feedback": {}}
	rubricCriterionFields = map[string]struct{}{"id": {}, "marks": {}, "descriptor": {}}
	rubricAnswerKeyFields = map[string]struct{}{"correctOptionId": {}, "correctOptionIds": {}, "acceptedAnswers": {}, "numericValue": {}, "tolerance": {}}
	rubricFeedbackFields  = map[string]struct{}{"correctExplanation": {}, "incorrectExplanations": {}}
	rubricIncorrectFields = map[string]struct{}{"optionId": {}, "explanation": {}}
)

// rubricDocument and rubricCriterion document the accepted shape and are used to construct
// payloads. Validation deliberately does NOT decode into them — see rubricDocumentFields.
//
// The descriptor is original editorial text supplied at runtime by a private, approved workflow.
// It is never seeded, never copied from a mark scheme, and never recorded in the audit trail.
type rubricDocument struct {
	Criteria  []rubricCriterion `json:"criteria"`
	AnswerKey *rubricAnswerKey  `json:"answerKey,omitempty"`
	Feedback  *rubricFeedback   `json:"feedback,omitempty"`
}

type rubricAnswerKey struct {
	CorrectOptionID string `json:"correctOptionId"`
}

type rubricFeedback struct {
	CorrectExplanation    string                       `json:"correctExplanation"`
	IncorrectExplanations []rubricIncorrectExplanation `json:"incorrectExplanations"`
}

type rubricIncorrectExplanation struct {
	OptionID    string `json:"optionId"`
	Explanation string `json:"explanation"`
}

type rubricCriterion struct {
	ID         string  `json:"id"`
	Marks      *int    `json:"marks"`
	Descriptor *string `json:"descriptor"`
}

// ValidateRubric checks a rubric payload against the validation-safe schema and the declared
// maximum marks. It returns ErrInvalidRubric or ErrInvalidMaxMarks — never a raw JSON decoding
// error, whose text could echo caller content back in a response.
//
// Rules:
//   - maxMarks must be a positive integer.
//   - The payload must be exactly one JSON object, with no trailing data.
//   - Every key, at every level, must match the schema EXACTLY: `criteria` on the document, and
//     `id`/`marks`/`descriptor` on a criterion. Unknown keys, misplaced keys, case variants, and
//     duplicate keys are all rejected.
//   - criteria must be a non-empty array (bounded by maxRubricCriteria) of objects.
//   - Each criterion needs a non-blank, unique id and a positive integer marks; descriptor is
//     optional but must be a non-blank string when present (explicit null means absent).
//   - Criterion marks must sum exactly to maxMarks, so a rubric can never award more or fewer
//     marks than the question is worth.
func ValidateRubric(raw json.RawMessage, maxMarks int) error {
	if maxMarks <= 0 {
		return ErrInvalidMaxMarks
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return ErrInvalidRubric
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	fields, err := decodeExactObject(dec, rubricDocumentFields)
	if err != nil {
		return err
	}
	if err := requireEOF(dec); err != nil {
		return err
	}

	criteriaRaw, ok := fields["criteria"]
	if !ok {
		return ErrInvalidRubric
	}
	var items []json.RawMessage
	if err := json.Unmarshal(criteriaRaw, &items); err != nil {
		return ErrInvalidRubric
	}
	if len(items) == 0 || len(items) > maxRubricCriteria {
		return ErrInvalidRubric
	}

	seen := make(map[string]struct{}, len(items))
	total := 0
	for _, item := range items {
		id, marks, err := validateCriterion(item)
		if err != nil {
			return err
		}
		if _, dup := seen[id]; dup {
			return ErrInvalidRubric
		}
		seen[id] = struct{}{}
		// Bounding each criterion by maxMarks keeps the running total from overflowing on a
		// hostile payload, and a single over-large criterion is a marks error either way.
		if marks > maxMarks {
			return ErrInvalidMaxMarks
		}
		total += marks
	}
	if total != maxMarks {
		return ErrInvalidMaxMarks
	}
	if answerKeyRaw, ok := fields["answerKey"]; ok {
		if !validateAnswerKeyShape(answerKeyRaw) {
			return ErrInvalidRubric
		}
	}
	if feedbackRaw, ok := fields["feedback"]; ok {
		if _, err := validateFeedback(feedbackRaw); err != nil {
			return err
		}
	}
	return nil
}

func validateAnswerKeyShape(raw json.RawMessage) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	fields, err := decodeExactObject(dec, rubricAnswerKeyFields)
	if err != nil || requireEOF(dec) != nil || len(fields) != 1 && len(fields) != 2 {
		return false
	}
	if raw, ok := fields["correctOptionId"]; ok {
		var v string
		return len(fields) == 1 && json.Unmarshal(raw, &v) == nil && !blank(v)
	}
	if raw, ok := fields["correctOptionIds"]; ok {
		var v []string
		return len(fields) == 1 && json.Unmarshal(raw, &v) == nil && len(v) > 0
	}
	if raw, ok := fields["acceptedAnswers"]; ok {
		var v []string
		return len(fields) == 1 && json.Unmarshal(raw, &v) == nil && len(v) > 0
	}
	if fields["numericValue"] != nil && fields["tolerance"] != nil {
		return len(fields) == 2 && isJSONNumber(fields["numericValue"]) && isJSONNumber(fields["tolerance"])
	}
	return false
}

// ValidateRubricForQuestion adds response-type and option-reference rules to structural rubric
// validation. Call while holding question row lock so answer key and options are one revision.
func ValidateRubricForQuestion(raw json.RawMessage, maxMarks int, responseType ResponseType, options []MultipleChoiceOption) error {
	if err := ValidateRubric(raw, maxMarks); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	fields, err := decodeExactObject(dec, rubricDocumentFields)
	if err != nil || requireEOF(dec) != nil {
		return ErrInvalidRubric
	}
	answerKeyRaw, hasAnswerKey := fields["answerKey"]
	feedbackRaw, hasFeedback := fields["feedback"]
	if responseType == ResponseShortAnswer || responseType == ResponseStructuredResponse || responseType == ResponseEssay {
		if hasAnswerKey || hasFeedback {
			return ErrInvalidRubric
		}
		return nil
	}
	if !hasAnswerKey {
		return ErrInvalidRubric
	}
	if responseType == ResponseExactShortAnswer || responseType == ResponseNumericAnswer {
		if hasFeedback || !validateDeterministicAnswerKey(answerKeyRaw, responseType, nil) {
			return ErrInvalidRubric
		}
		return nil
	}
	if !hasFeedback {
		return ErrInvalidRubric
	}
	correctOptionID, err := validateAnswerKey(answerKeyRaw)
	if responseType == ResponseMultiSelect {
		return validateMultiSelectRubric(answerKeyRaw, feedbackRaw, options)
	}
	if err != nil {
		return err
	}
	feedback, err := validateFeedback(feedbackRaw)
	if err != nil {
		return err
	}
	current := make(map[string]struct{}, len(options))
	for _, option := range options {
		current[option.ID] = struct{}{}
	}
	if _, ok := current[correctOptionID]; !ok {
		return ErrInvalidRubric
	}
	seen := make(map[string]struct{}, len(feedback.IncorrectExplanations))
	for _, item := range feedback.IncorrectExplanations {
		if item.OptionID == correctOptionID {
			return ErrInvalidRubric
		}
		if _, ok := current[item.OptionID]; !ok {
			return ErrInvalidRubric
		}
		if _, duplicate := seen[item.OptionID]; duplicate {
			return ErrInvalidRubric
		}
		seen[item.OptionID] = struct{}{}
	}
	if len(seen) != len(options)-1 {
		return ErrInvalidRubric
	}
	for id := range current {
		if id == correctOptionID {
			continue
		}
		if _, ok := seen[id]; !ok {
			return ErrInvalidRubric
		}
	}
	return nil
}

func validateDeterministicAnswerKey(raw json.RawMessage, typ ResponseType, options []MultipleChoiceOption) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	fields, err := decodeExactObject(dec, rubricAnswerKeyFields)
	if err != nil || requireEOF(dec) != nil {
		return false
	}
	switch typ {
	case ResponseExactShortAnswer:
		if len(fields) != 1 || fields["acceptedAnswers"] == nil {
			return false
		}
		var answers []string
		if json.Unmarshal(fields["acceptedAnswers"], &answers) != nil || len(answers) == 0 || len(answers) > 20 {
			return false
		}
		seen := map[string]struct{}{}
		for _, answer := range answers {
			normalized := strings.Join(strings.Fields(strings.ToLower(answer)), " ")
			if normalized == "" || len([]rune(normalized)) > 500 {
				return false
			}
			if _, ok := seen[normalized]; ok {
				return false
			}
			seen[normalized] = struct{}{}
		}
		return true
	case ResponseNumericAnswer:
		if len(fields) != 2 || fields["numericValue"] == nil || fields["tolerance"] == nil || !isJSONNumber(fields["numericValue"]) || !isJSONNumber(fields["tolerance"]) {
			return false
		}
		var value, tolerance float64
		return json.Unmarshal(fields["numericValue"], &value) == nil && json.Unmarshal(fields["tolerance"], &tolerance) == nil && !math.IsNaN(value) && !math.IsInf(value, 0) && !math.IsNaN(tolerance) && !math.IsInf(tolerance, 0) && tolerance >= 0
	}
	return false
}

func validateMultiSelectRubric(answerRaw, feedbackRaw json.RawMessage, options []MultipleChoiceOption) error {
	dec := json.NewDecoder(bytes.NewReader(answerRaw))
	fields, err := decodeExactObject(dec, rubricAnswerKeyFields)
	if err != nil || requireEOF(dec) != nil || len(fields) != 1 || fields["correctOptionIds"] == nil {
		return ErrInvalidRubric
	}
	var ids []string
	if json.Unmarshal(fields["correctOptionIds"], &ids) != nil || len(ids) < 2 || len(ids) >= len(options) {
		return ErrInvalidRubric
	}
	current := map[string]struct{}{}
	for _, option := range options {
		current[option.ID] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return ErrInvalidRubric
		}
		if _, ok := current[id]; !ok {
			return ErrInvalidRubric
		}
		if _, duplicate := seen[id]; duplicate {
			return ErrInvalidRubric
		}
		seen[id] = struct{}{}
	}
	feedback, err := validateFeedback(feedbackRaw)
	if err != nil {
		return err
	}
	if len(feedback.IncorrectExplanations) != len(options)-len(ids) {
		return ErrInvalidRubric
	}
	wrong := map[string]struct{}{}
	for _, item := range feedback.IncorrectExplanations {
		if _, correct := seen[item.OptionID]; correct {
			return ErrInvalidRubric
		}
		if _, ok := current[item.OptionID]; !ok {
			return ErrInvalidRubric
		}
		if _, dup := wrong[item.OptionID]; dup {
			return ErrInvalidRubric
		}
		wrong[item.OptionID] = struct{}{}
	}
	return nil
}

func validateFeedback(raw json.RawMessage) (rubricFeedback, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	fields, err := decodeExactObject(dec, rubricFeedbackFields)
	if err != nil || requireEOF(dec) != nil || len(fields) != 2 {
		return rubricFeedback{}, ErrInvalidRubric
	}
	var correct string
	if json.Unmarshal(fields["correctExplanation"], &correct) != nil || blank(correct) {
		return rubricFeedback{}, ErrInvalidRubric
	}
	var raws []json.RawMessage
	if json.Unmarshal(fields["incorrectExplanations"], &raws) != nil || len(raws) == 0 || len(raws) > maxMultipleChoiceOptions-1 {
		return rubricFeedback{}, ErrInvalidRubric
	}
	result := rubricFeedback{CorrectExplanation: correct, IncorrectExplanations: make([]rubricIncorrectExplanation, 0, len(raws))}
	for _, itemRaw := range raws {
		itemDec := json.NewDecoder(bytes.NewReader(itemRaw))
		itemFields, err := decodeExactObject(itemDec, rubricIncorrectFields)
		if err != nil || requireEOF(itemDec) != nil || len(itemFields) != 2 {
			return rubricFeedback{}, ErrInvalidRubric
		}
		var item rubricIncorrectExplanation
		if json.Unmarshal(itemFields["optionId"], &item.OptionID) != nil || blank(item.OptionID) || len([]rune(strings.TrimSpace(item.OptionID))) > maxOptionIDLength {
			return rubricFeedback{}, ErrInvalidRubric
		}
		item.OptionID = strings.TrimSpace(item.OptionID)
		if json.Unmarshal(itemFields["explanation"], &item.Explanation) != nil || blank(item.Explanation) {
			return rubricFeedback{}, ErrInvalidRubric
		}
		result.IncorrectExplanations = append(result.IncorrectExplanations, item)
	}
	return result, nil
}

func validateAnswerKey(raw json.RawMessage) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	fields, err := decodeExactObject(dec, rubricAnswerKeyFields)
	if err != nil || requireEOF(dec) != nil || len(fields) != 1 {
		return "", ErrInvalidRubric
	}
	var id string
	if json.Unmarshal(fields["correctOptionId"], &id) != nil {
		return "", ErrInvalidRubric
	}
	id = strings.TrimSpace(id)
	if id == "" || len([]rune(id)) > maxOptionIDLength {
		return "", ErrInvalidRubric
	}
	return id, nil
}

// validateCriterion checks one criterion object and returns its trimmed id and marks.
func validateCriterion(raw json.RawMessage) (string, int, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	fields, err := decodeExactObject(dec, rubricCriterionFields)
	if err != nil {
		return "", 0, err
	}
	if err := requireEOF(dec); err != nil {
		return "", 0, err
	}

	idRaw, ok := fields["id"]
	if !ok {
		return "", 0, ErrInvalidRubric
	}
	var id string
	if err := json.Unmarshal(idRaw, &id); err != nil {
		return "", 0, ErrInvalidRubric
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", 0, ErrInvalidRubric
	}

	marksRaw, ok := fields["marks"]
	if !ok {
		return "", 0, ErrInvalidRubric
	}
	// encoding/json happily unmarshals a JSON *string* into a json.Number, so the raw value has
	// to be checked first: marks is a number, and "3" is not one.
	if !isJSONNumber(marksRaw) {
		return "", 0, ErrInvalidRubric
	}
	var marks json.Number
	if err := json.Unmarshal(marksRaw, &marks); err != nil {
		return "", 0, ErrInvalidRubric
	}
	// Int64 rejects 2.5 and 2e3-style values: marks are whole marks.
	marksValue, err := marks.Int64()
	if err != nil || marksValue <= 0 || marksValue > int64(maxRubricMarksPerCriterion) {
		return "", 0, ErrInvalidRubric
	}

	// descriptor is optional. An explicit null is treated as absent; anything other than a
	// non-blank string is rejected.
	if descriptorRaw, ok := fields["descriptor"]; ok && !isJSONNull(descriptorRaw) {
		var descriptor string
		if err := json.Unmarshal(descriptorRaw, &descriptor); err != nil {
			return "", 0, ErrInvalidRubric
		}
		if blank(descriptor) {
			return "", 0, ErrInvalidRubric
		}
	}

	return id, int(marksValue), nil
}

// maxRubricMarksPerCriterion bounds a single criterion so an absurd value cannot be stored. It is
// far above any realistic question and exists only to keep arithmetic and storage sane.
const maxRubricMarksPerCriterion = 1000

// decodeExactObject reads exactly one JSON object from dec, requiring every key to appear in
// allowed with EXACTLY that spelling and casing, and rejecting duplicate keys. Values are
// returned undecoded so each caller can type-check them itself.
//
// It is written against the token API rather than a struct or a map because neither of those can
// do this job: a struct decode matches names case-insensitively, and both a struct and a map
// silently keep the last value of a duplicated key instead of rejecting it.
func decodeExactObject(dec *json.Decoder, allowed map[string]struct{}) (map[string]json.RawMessage, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, ErrInvalidRubric
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, ErrInvalidRubric
	}

	fields := make(map[string]json.RawMessage, len(allowed))
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, ErrInvalidRubric
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, ErrInvalidRubric
		}
		if _, ok := allowed[key]; !ok {
			return nil, ErrInvalidRubric
		}
		if _, dup := fields[key]; dup {
			return nil, ErrInvalidRubric
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, ErrInvalidRubric
		}
		fields[key] = value
	}
	// Consume the closing brace.
	if _, err := dec.Token(); err != nil {
		return nil, ErrInvalidRubric
	}
	return fields, nil
}

// requireEOF rejects anything after the decoded value: a second object, an array, or junk.
func requireEOF(dec *json.Decoder) error {
	if _, err := dec.Token(); err != io.EOF {
		return ErrInvalidRubric
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

// isJSONNumber reports whether raw is a JSON number literal (not a quoted numeric string, and not
// a boolean, null, object, or array).
func isJSONNumber(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	first := trimmed[0]
	return first == '-' || (first >= '0' && first <= '9')
}
