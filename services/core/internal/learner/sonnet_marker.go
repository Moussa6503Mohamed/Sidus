package learner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPSonnetMarker is the narrow Core-to-AI adapter. It transmits pins and opaque attempt id,
// never a learner answer, rubric text, key, source, or provenance.
type HTTPSonnetMarker struct {
	baseURL, token string
	client         *http.Client
}

func NewHTTPSonnetMarker(baseURL, token string) (*HTTPSonnetMarker, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if !strings.HasPrefix(baseURL, "https://") || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("Sonnet marker is not configured")
	}
	return &HTTPSonnetMarker{baseURL: baseURL, token: token, client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (m *HTTPSonnetMarker) MarkWrittenAttempt(ctx context.Context, j MarkingJob) (MarkingOutcome, error) {
	criteria := make([]map[string]any, 0, len(j.Criteria))
	for _, c := range j.Criteria {
		criteria = append(criteria, map[string]any{"criterionId": c.ID, "maxMarks": c.MaxMarks, "descriptor": c.Descriptor})
	}
	// This is a private, TLS-protected service request; no browser route sees privateContext.
	// It is intentionally not logged or added to Core audit events.
	body, _ := json.Marshal(map[string]any{"jobId": j.RequestID, "attemptId": j.AttemptID, "questionId": j.QuestionID, "syllabusId": j.SyllabusID, "rubricVersionId": j.RubricVersionID, "rubricCriteria": criteria, "promptContentRef": j.AttemptID, "privateContext": map[string]any{"answer": j.Answer}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/sonnet/marking-jobs", bytes.NewReader(body))
	if err != nil {
		return MarkingOutcome{}, err
	}
	req.Header.Set("Authorization", "Bearer "+m.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return MarkingOutcome{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MarkingOutcome{}, fmt.Errorf("Sonnet marking unavailable")
	}
	var out struct {
		Status MarkingStatus  `json:"status"`
		Reason string         `json:"reason"`
		Result *MarkingResult `json:"result"`
	}
	if json.Unmarshal(data, &out) != nil {
		return MarkingOutcome{}, fmt.Errorf("invalid Sonnet marking response")
	}
	return MarkingOutcome{Status: out.Status, Reason: out.Reason, Result: out.Result}, nil
}
