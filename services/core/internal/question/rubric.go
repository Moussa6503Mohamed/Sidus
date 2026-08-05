package question

import (
	"bytes"
	"encoding/json"
	"io"
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
	rubricDocumentFields  = map[string]struct{}{"criteria": {}, "answerKey": {}}
	rubricCriterionFields = map[string]struct{}{"id": {}, "marks": {}, "descriptor": {}}
	rubricAnswerKeyFields = map[string]struct{}{"correctOptionId": {}}
)

// rubricDocument and rubricCriterion document the accepted shape and are used to construct
// payloads. Validation deliberately does NOT decode into them — see rubricDocumentFields.
//
// The descriptor is original editorial text supplied at runtime by a private, approved workflow.
// It is never seeded, never copied from a mark scheme, and never recorded in the audit trail.
type rubricDocument struct {
	Criteria  []rubricCriterion `json:"criteria"`
	AnswerKey *rubricAnswerKey  `json:"answerKey,omitempty"`
}

type rubricAnswerKey struct {
	CorrectOptionID string `json:"correctOptionId"`
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
		if _, err := validateAnswerKey(answerKeyRaw); err != nil {
			return err
		}
	}
	return nil
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
	if responseType != ResponseMultipleChoice {
		if hasAnswerKey {
			return ErrInvalidRubric
		}
		return nil
	}
	if !hasAnswerKey {
		return ErrInvalidRubric
	}
	correctOptionID, err := validateAnswerKey(answerKeyRaw)
	if err != nil {
		return err
	}
	for _, option := range options {
		if option.ID == correctOptionID {
			return nil
		}
	}
	return ErrInvalidRubric
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
