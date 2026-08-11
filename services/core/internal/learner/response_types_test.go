package learner

import "testing"

func TestMarkAnswer_DeterministicAndPendingTypes(t *testing.T) {
	cases := []struct {
		name            string
		typ             ResponseType
		rubric          string
		answer          string
		marked, correct bool
	}{
		{"multi select", ResponseMultiSelect, `{"criteria":[{"id":"c","marks":1}],"answerKey":{"correctOptionIds":["a","b"]}}`, `{"selectedOptionIds":["b","a"]}`, true, true},
		{"exact normalized", ResponseExactShortAnswer, `{"criteria":[{"id":"c","marks":1}],"answerKey":{"acceptedAnswers":["cell membrane"]}}`, `{"text":"  CELL   membrane "}`, true, true},
		{"numeric tolerance", ResponseNumericAnswer, `{"criteria":[{"id":"c","marks":1}],"answerKey":{"numericValue":2.5,"tolerance":0.1}}`, `{"value":2.55}`, true, true},
		{"written pending", ResponseEssay, `{"criteria":[{"id":"c","marks":1}]}`, `{"text":"runtime answer"}`, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, marked, correct, err := markAnswer(tc.typ, []byte(tc.rubric), tc.answer)
			if err != nil || marked != tc.marked || correct != tc.correct {
				t.Fatalf("markAnswer = marked %v correct %v err %v", marked, correct, err)
			}
		})
	}
}
