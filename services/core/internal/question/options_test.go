package question

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseOptions_AcceptsExactSchemaAndOrder(t *testing.T) {
	options, err := ParseOptions(json.RawMessage(`[{"id":"first","label":"runtime one"},{"id":"second","label":"runtime two"}]`))
	if err != nil {
		t.Fatalf("ParseOptions: %v", err)
	}
	if len(options) != 2 || options[0].ID != "first" || options[1].ID != "second" {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseOptions_RejectsStrictValidationMatrix(t *testing.T) {
	valid := `{"id":"a","label":"runtime"}`
	tests := map[string]string{
		"null":             `null`,
		"not array":        `{}`,
		"too few":          `[` + valid + `]`,
		"too many":         `[` + strings.TrimSuffix(strings.Repeat(valid+`,`, 7), ",") + `]`,
		"unknown":          `[{"id":"a","label":"x","extra":1},{"id":"b","label":"y"}]`,
		"case variant":     `[{"ID":"a","label":"x"},{"id":"b","label":"y"}]`,
		"duplicate key":    `[{"id":"a","id":"b","label":"x"},{"id":"c","label":"y"}]`,
		"duplicate id":     `[{"id":"a","label":"x"},{"id":"a","label":"y"}]`,
		"blank id":         `[{"id":" ","label":"x"},{"id":"b","label":"y"}]`,
		"blank label":      `[{"id":"a","label":" "},{"id":"b","label":"y"}]`,
		"wrong id type":    `[{"id":1,"label":"x"},{"id":"b","label":"y"}]`,
		"wrong label type": `[{"id":"a","label":1},{"id":"b","label":"y"}]`,
		"id too long":      `[{"id":"` + strings.Repeat("x", 65) + `","label":"x"},{"id":"b","label":"y"}]`,
		"label too long":   `[{"id":"a","label":"` + strings.Repeat("x", 1001) + `"},{"id":"b","label":"y"}]`,
		"trailing json":    `[` + valid + `,{"id":"b","label":"y"}] {}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseOptions(json.RawMessage(raw)); !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("ParseOptions error = %v, want ErrInvalidOptions", err)
			}
		})
	}
}

func TestValidateOptionsForResponseType(t *testing.T) {
	options := []MultipleChoiceOption{{ID: "a", Label: "x"}, {ID: "b", Label: "y"}}
	if err := validateOptionsForResponseType(ResponseMultipleChoice, options); err != nil {
		t.Fatalf("MC options: %v", err)
	}
	if !errors.Is(validateOptionsForResponseType(ResponseMultipleChoice, nil), ErrInvalidOptions) {
		t.Fatal("MC missing options accepted")
	}
	if !errors.Is(validateOptionsForResponseType(ResponseShortAnswer, options), ErrInvalidOptions) {
		t.Fatal("non-MC options accepted")
	}
	if err := validateOptionsForResponseType(ResponseStructuredResponse, nil); err != nil {
		t.Fatalf("non-MC absent options: %v", err)
	}
}
