package question

import (
	"encoding/json"
	"errors"
	"testing"
)

// errUnexpected stands in for an infrastructure failure (a driver/scan error) that must never be
// mapped to a stable domain code.
var errUnexpected = errors.New("pq: relation \"questions\" does not exist")

func TestValidateRubric_Accepts(t *testing.T) {
	cases := map[string]struct {
		rubric   string
		maxMarks int
	}{
		"single criterion":            {`{"criteria":[{"id":"c1","marks":3}]}`, 3},
		"multiple criteria":           {`{"criteria":[{"id":"c1","marks":2},{"id":"c2","marks":1}]}`, 3},
		"criterion with descriptor":   {`{"criteria":[{"id":"c1","marks":1,"descriptor":"original wording"}]}`, 1},
		"explicit null descriptor":    {`{"criteria":[{"id":"c1","marks":1,"descriptor":null}]}`, 1},
		"ids trimmed but not blanked": {`{"criteria":[{"id":" c1 ","marks":1}]}`, 1},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateRubric(json.RawMessage(tc.rubric), tc.maxMarks); err != nil {
				t.Fatalf("ValidateRubric = %v, want nil", err)
			}
		})
	}
}

func TestValidateRubric_Rejects(t *testing.T) {
	cases := map[string]struct {
		rubric   string
		maxMarks int
		wantErr  error
	}{
		"empty payload":            {``, 3, ErrInvalidRubric},
		"not an object":            {`[1,2,3]`, 3, ErrInvalidRubric},
		"no criteria key":          {`{}`, 3, ErrInvalidRubric},
		"empty criteria":           {`{"criteria":[]}`, 3, ErrInvalidRubric},
		"unknown document field":   {`{"criteria":[{"id":"c1","marks":3}],"answerKey":"x"}`, 3, ErrInvalidRubric},
		"unknown criterion field":  {`{"criteria":[{"id":"c1","marks":3,"answer":"x"}]}`, 3, ErrInvalidRubric},
		"blank id":                 {`{"criteria":[{"id":"","marks":3}]}`, 3, ErrInvalidRubric},
		"duplicate id":             {`{"criteria":[{"id":"c1","marks":2},{"id":"c1","marks":1}]}`, 3, ErrInvalidRubric},
		"missing marks":            {`{"criteria":[{"id":"c1"}]}`, 3, ErrInvalidRubric},
		"zero marks":               {`{"criteria":[{"id":"c1","marks":0},{"id":"c2","marks":3}]}`, 3, ErrInvalidRubric},
		"negative marks":           {`{"criteria":[{"id":"c1","marks":-1},{"id":"c2","marks":4}]}`, 3, ErrInvalidRubric},
		"blank descriptor":         {`{"criteria":[{"id":"c1","marks":3,"descriptor":"  "}]}`, 3, ErrInvalidRubric},
		"trailing json":            {`{"criteria":[{"id":"c1","marks":3}]}{}`, 3, ErrInvalidRubric},
		"marks below maxMarks":     {`{"criteria":[{"id":"c1","marks":2}]}`, 3, ErrInvalidMaxMarks},
		"marks above maxMarks":     {`{"criteria":[{"id":"c1","marks":5}]}`, 3, ErrInvalidMaxMarks},
		"zero maxMarks":            {`{"criteria":[{"id":"c1","marks":3}]}`, 0, ErrInvalidMaxMarks},
		"negative maxMarks":        {`{"criteria":[{"id":"c1","marks":3}]}`, -3, ErrInvalidMaxMarks},
		"criteria is not an array": {`{"criteria":{"id":"c1"}}`, 3, ErrInvalidRubric},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateRubric(json.RawMessage(tc.rubric), tc.maxMarks)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateRubric = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestValidateRubric_BoundsCriteria keeps a pathological payload from being stored or walked.
func TestValidateRubric_BoundsCriteria(t *testing.T) {
	doc := rubricDocument{}
	marks := 1
	for i := 0; i <= maxRubricCriteria; i++ {
		id := "c" + string(rune('A'+i%26)) + string(rune('a'+i/26))
		doc.Criteria = append(doc.Criteria, rubricCriterion{ID: id, Marks: &marks})
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal oversized rubric: %v", err)
	}
	if err := ValidateRubric(raw, len(doc.Criteria)); !errors.Is(err, ErrInvalidRubric) {
		t.Fatalf("ValidateRubric = %v, want ErrInvalidRubric for %d criteria", err, len(doc.Criteria))
	}
}
