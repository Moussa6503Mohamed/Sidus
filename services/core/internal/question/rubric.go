package question

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// maxRubricCriteria bounds a rubric so a pathological payload cannot be stored or walked.
const maxRubricCriteria = 200

// rubricDocument is the only accepted rubric shape. It is deliberately narrow: Core must be able
// to validate a rubric completely (structure, marks arithmetic) before writing it, and an
// open-ended blob could not be checked at all.
//
// The descriptor is original editorial text supplied at runtime by a private, approved workflow.
// It is never seeded, never copied from a mark scheme, and never recorded in the audit trail.
type rubricDocument struct {
	Criteria []rubricCriterion `json:"criteria"`
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
//   - The payload must be exactly one JSON object with no unknown fields and no trailing data.
//   - criteria must be a non-empty array (bounded by maxRubricCriteria).
//   - Each criterion needs a non-blank, unique id and a positive integer marks; descriptor is
//     optional but must be non-blank when present.
//   - Criterion marks must sum exactly to maxMarks, so a rubric can never award more or fewer
//     marks than the question is worth.
func ValidateRubric(raw json.RawMessage, maxMarks int) error {
	if maxMarks <= 0 {
		return ErrInvalidMaxMarks
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return ErrInvalidRubric
	}

	var doc rubricDocument
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return ErrInvalidRubric
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		return ErrInvalidRubric
	}

	if len(doc.Criteria) == 0 || len(doc.Criteria) > maxRubricCriteria {
		return ErrInvalidRubric
	}

	seen := make(map[string]struct{}, len(doc.Criteria))
	total := 0
	for _, c := range doc.Criteria {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			return ErrInvalidRubric
		}
		if _, dup := seen[id]; dup {
			return ErrInvalidRubric
		}
		seen[id] = struct{}{}
		if c.Marks == nil || *c.Marks <= 0 {
			return ErrInvalidRubric
		}
		if c.Descriptor != nil && blank(*c.Descriptor) {
			return ErrInvalidRubric
		}
		total += *c.Marks
	}
	if total != maxMarks {
		return ErrInvalidMaxMarks
	}
	return nil
}
