package question

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const (
	minMultipleChoiceOptions = 2
	maxMultipleChoiceOptions = 6
	maxOptionIDLength        = 64
	maxOptionLabelLength     = 1000
)

var optionFields = map[string]struct{}{"id": {}, "label": {}}

// ParseOptions validates exact, case-sensitive option JSON and returns normalized values.
func ParseOptions(raw json.RawMessage) ([]MultipleChoiceOption, error) {
	var items []json.RawMessage
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &items) != nil ||
		len(items) < minMultipleChoiceOptions || len(items) > maxMultipleChoiceOptions {
		return nil, ErrInvalidOptions
	}
	options := make([]MultipleChoiceOption, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		dec := json.NewDecoder(bytes.NewReader(item))
		fields, err := decodeExactObject(dec, optionFields)
		if err != nil || requireEOF(dec) != nil || len(fields) != len(optionFields) {
			return nil, ErrInvalidOptions
		}
		var option MultipleChoiceOption
		if json.Unmarshal(fields["id"], &option.ID) != nil || json.Unmarshal(fields["label"], &option.Label) != nil {
			return nil, ErrInvalidOptions
		}
		option.ID = strings.TrimSpace(option.ID)
		option.Label = strings.TrimSpace(option.Label)
		if option.ID == "" || option.Label == "" || !utf8.ValidString(option.ID) || !utf8.ValidString(option.Label) ||
			utf8.RuneCountInString(option.ID) > maxOptionIDLength || utf8.RuneCountInString(option.Label) > maxOptionLabelLength {
			return nil, ErrInvalidOptions
		}
		if _, duplicate := seen[option.ID]; duplicate {
			return nil, ErrInvalidOptions
		}
		seen[option.ID] = struct{}{}
		options = append(options, option)
	}
	return options, nil
}

func validateOptionsForResponseType(responseType ResponseType, options []MultipleChoiceOption) error {
	if responseType != ResponseMultipleChoice && responseType != ResponseMultiSelect {
		if options != nil {
			return ErrInvalidOptions
		}
		return nil
	}
	if options == nil {
		return ErrInvalidOptions
	}
	raw, err := json.Marshal(options)
	if err != nil {
		return ErrInvalidOptions
	}
	_, err = ParseOptions(raw)
	return err
}
