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

		// Case variants. Go's struct decoding matches JSON field names case-insensitively, so
		// every one of these was accepted before the review fix even though the documented
		// schema — and the TypeScript contract — spell the keys exactly one way.
		"Criteria":             {`{"Criteria":[{"id":"c1","marks":3}]}`, 3, ErrInvalidRubric},
		"CRITERIA":             {`{"CRITERIA":[{"id":"c1","marks":3}]}`, 3, ErrInvalidRubric},
		"criterion ID":         {`{"criteria":[{"ID":"c1","marks":3}]}`, 3, ErrInvalidRubric},
		"criterion Id":         {`{"criteria":[{"Id":"c1","marks":3}]}`, 3, ErrInvalidRubric},
		"criterion Marks":      {`{"criteria":[{"id":"c1","Marks":3}]}`, 3, ErrInvalidRubric},
		"criterion MARKS":      {`{"criteria":[{"id":"c1","MARKS":3}]}`, 3, ErrInvalidRubric},
		"criterion Descriptor": {`{"criteria":[{"id":"c1","marks":3,"Descriptor":"x"}]}`, 3, ErrInvalidRubric},
		"all keys capitalised": {`{"Criteria":[{"ID":"c1","Marks":3,"Descriptor":"x"}]}`, 3, ErrInvalidRubric},
		"leading space in key": {`{" criteria":[{"id":"c1","marks":3}]}`, 3, ErrInvalidRubric},

		// Duplicate keys: a map or struct decode silently keeps the last one.
		"duplicate criteria key":     {`{"criteria":[{"id":"c1","marks":3}],"criteria":[{"id":"c2","marks":3}]}`, 3, ErrInvalidRubric},
		"duplicate criterion id key": {`{"criteria":[{"id":"c1","id":"c2","marks":3}]}`, 3, ErrInvalidRubric},
		"duplicate marks key":        {`{"criteria":[{"id":"c1","marks":1,"marks":3}]}`, 3, ErrInvalidRubric},

		// Non-object / wrongly-typed values.
		"criteria is null":        {`{"criteria":null}`, 3, ErrInvalidRubric},
		"criterion is null":       {`{"criteria":[null]}`, 3, ErrInvalidRubric},
		"criterion is a string":   {`{"criteria":["c1"]}`, 3, ErrInvalidRubric},
		"criterion is a number":   {`{"criteria":[1]}`, 3, ErrInvalidRubric},
		"criterion is an array":   {`{"criteria":[[]]}`, 3, ErrInvalidRubric},
		"id is a number":          {`{"criteria":[{"id":7,"marks":3}]}`, 3, ErrInvalidRubric},
		"id is null":              {`{"criteria":[{"id":null,"marks":3}]}`, 3, ErrInvalidRubric},
		"marks is a string":       {`{"criteria":[{"id":"c1","marks":"3"}]}`, 3, ErrInvalidRubric},
		"marks is null":           {`{"criteria":[{"id":"c1","marks":null}]}`, 3, ErrInvalidRubric},
		"marks is fractional":     {`{"criteria":[{"id":"c1","marks":1.5},{"id":"c2","marks":1.5}]}`, 3, ErrInvalidRubric},
		"marks is boolean":        {`{"criteria":[{"id":"c1","marks":true}]}`, 3, ErrInvalidRubric},
		"descriptor is a number":  {`{"criteria":[{"id":"c1","marks":3,"descriptor":7}]}`, 3, ErrInvalidRubric},
		"descriptor is an object": {`{"criteria":[{"id":"c1","marks":3,"descriptor":{}}]}`, 3, ErrInvalidRubric},
		"document is a string":    {`"criteria"`, 3, ErrInvalidRubric},
		"document is null":        {`null`, 3, ErrInvalidRubric},
		"trailing array":          {`{"criteria":[{"id":"c1","marks":3}]}[1]`, 3, ErrInvalidRubric},
		"trailing junk":           {`{"criteria":[{"id":"c1","marks":3}]}not-json`, 3, ErrInvalidRubric},
		"criterion marks too big": {`{"criteria":[{"id":"c1","marks":100000}]}`, 100000, ErrInvalidRubric},
		"nested criteria key":     {`{"criteria":[{"id":"c1","marks":3,"criteria":[]}]}`, 3, ErrInvalidRubric},
		"document-level id key":   {`{"criteria":[{"id":"c1","marks":3}],"id":"x"}`, 3, ErrInvalidRubric},
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
