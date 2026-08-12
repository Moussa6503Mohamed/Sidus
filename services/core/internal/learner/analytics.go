package learner

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// LearningAnalytics is a deliberately reduced, owner-only progress projection. It has no answer,
// rubric, source, provenance, model, cost, or question-level data.
type LearningAnalytics struct {
	ScoredItems     int                 `json:"scoredItems"`
	AwardedMarks    int                 `json:"awardedMarks"`
	PossibleMarks   int                 `json:"possibleMarks"`
	PendingMarking  int                 `json:"pendingMarking"`
	WithheldMarking int                 `json:"withheldMarking"`
	Syllabuses      []AnalyticsSyllabus `json:"syllabuses"`
	Modules         []AnalyticsModule   `json:"modules"`
	RecentActivity  []AnalyticsActivity `json:"recentActivity"`
}
type AnalyticsSyllabus struct {
	SyllabusID    string `json:"syllabusId"`
	ScoredItems   int    `json:"scoredItems"`
	AwardedMarks  int    `json:"awardedMarks"`
	PossibleMarks int    `json:"possibleMarks"`
}
type AnalyticsModule struct {
	SyllabusID    string `json:"syllabusId"`
	ModuleID      string `json:"moduleId"`
	ModuleCode    string `json:"moduleCode"`
	ModuleLabel   string `json:"moduleLabel"`
	ScoredItems   int    `json:"scoredItems"`
	AwardedMarks  int    `json:"awardedMarks"`
	PossibleMarks int    `json:"possibleMarks"`
}
type AnalyticsActivity struct {
	EventType    string    `json:"eventType"`
	ModuleLabel  string    `json:"moduleLabel"`
	AwardedMarks *int      `json:"awardedMarks,omitempty"`
	MaxMarks     *int      `json:"maxMarks,omitempty"`
	OccurredAt   time.Time `json:"occurredAt"`
}

type AnalyticsStore interface {
	GetLearningAnalytics(context.Context, string) (LearningAnalytics, error)
}

func (p *PostgresStore) GetLearningAnalytics(ctx context.Context, subject string) (LearningAnalytics, error) {
	var out LearningAnalytics
	err := p.db.QueryRowContext(ctx, `SELECT
	 COUNT(*) FILTER (WHERE event_type IN ('deterministic_scored','automated_marking_accepted')),
	 COALESCE(SUM(awarded_marks) FILTER (WHERE event_type IN ('deterministic_scored','automated_marking_accepted')),0),
	 COALESCE(SUM(max_marks) FILTER (WHERE event_type IN ('deterministic_scored','automated_marking_accepted')),0),
	 COUNT(*) FILTER (WHERE event_type='pending_marking' AND NOT EXISTS (SELECT 1 FROM learning_analytics_events x WHERE x.attempt_id=e.attempt_id AND x.event_type IN ('automated_marking_accepted','automated_marking_withheld'))),
	 COUNT(*) FILTER (WHERE event_type='automated_marking_withheld')
	 FROM learning_analytics_events e WHERE learner_subject_id=$1`, subject).Scan(&out.ScoredItems, &out.AwardedMarks, &out.PossibleMarks, &out.PendingMarking, &out.WithheldMarking)
	if err != nil {
		return LearningAnalytics{}, fmt.Errorf("analytics totals: %w", err)
	}
	rows, err := p.db.QueryContext(ctx, `SELECT syllabus_id::text,COUNT(*),COALESCE(SUM(awarded_marks),0),COALESCE(SUM(max_marks),0) FROM learning_analytics_events WHERE learner_subject_id=$1 AND event_type IN ('deterministic_scored','automated_marking_accepted') GROUP BY syllabus_id ORDER BY syllabus_id`, subject)
	if err != nil {
		return LearningAnalytics{}, fmt.Errorf("analytics syllabuses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var x AnalyticsSyllabus
		if err := rows.Scan(&x.SyllabusID, &x.ScoredItems, &x.AwardedMarks, &x.PossibleMarks); err != nil {
			return LearningAnalytics{}, err
		}
		out.Syllabuses = append(out.Syllabuses, x)
	}
	if err := rows.Err(); err != nil {
		return LearningAnalytics{}, err
	}
	rows, err = p.db.QueryContext(ctx, `SELECT syllabus_id::text,module_id::text,module_code,module_label,COUNT(*),COALESCE(SUM(awarded_marks),0),COALESCE(SUM(max_marks),0) FROM learning_analytics_events WHERE learner_subject_id=$1 AND event_type IN ('deterministic_scored','automated_marking_accepted') GROUP BY syllabus_id,module_id,module_code,module_label ORDER BY module_label`, subject)
	if err != nil {
		return LearningAnalytics{}, fmt.Errorf("analytics modules: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var x AnalyticsModule
		if err := rows.Scan(&x.SyllabusID, &x.ModuleID, &x.ModuleCode, &x.ModuleLabel, &x.ScoredItems, &x.AwardedMarks, &x.PossibleMarks); err != nil {
			return LearningAnalytics{}, err
		}
		out.Modules = append(out.Modules, x)
	}
	if err := rows.Err(); err != nil {
		return LearningAnalytics{}, err
	}
	rows, err = p.db.QueryContext(ctx, `SELECT event_type,module_label,awarded_marks,max_marks,created_at FROM learning_analytics_events WHERE learner_subject_id=$1 ORDER BY created_at DESC,id DESC LIMIT 10`, subject)
	if err != nil {
		return LearningAnalytics{}, fmt.Errorf("analytics activity: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var x AnalyticsActivity
		if err := rows.Scan(&x.EventType, &x.ModuleLabel, &x.AwardedMarks, &x.MaxMarks, &x.OccurredAt); err != nil {
			return LearningAnalytics{}, err
		}
		out.RecentActivity = append(out.RecentActivity, x)
	}
	if err := rows.Err(); err != nil {
		return LearningAnalytics{}, err
	}
	return out, nil
}

func recordAnalyticsSubmissionTx(ctx context.Context, tx *sql.Tx, subject, attemptID, syllabusID, moduleID, moduleCode, moduleLabel string, responseType ResponseType, scored bool, awarded, max int) error {
	event := "pending_marking"
	var got, possible any
	if scored {
		event = "deterministic_scored"
		got = awarded
		possible = max
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO learning_analytics_events(learner_subject_id,attempt_id,event_type,syllabus_id,module_id,module_code,module_label,response_type,awarded_marks,max_marks) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(attempt_id,event_type) DO NOTHING`, subject, attemptID, event, syllabusID, moduleID, moduleCode, moduleLabel, responseType, got, possible)
	return err
}

func recordAnalyticsMarkingTerminalTx(ctx context.Context, tx *sql.Tx, attemptID, event string, awarded, max *int) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO learning_analytics_events(learner_subject_id,attempt_id,event_type,syllabus_id,module_id,module_code,module_label,response_type,awarded_marks,max_marks)
	 SELECT learner_subject_id,attempt_id,$2,syllabus_id,module_id,module_code,module_label,response_type,$3,$4 FROM learning_analytics_events WHERE attempt_id=$1 AND event_type='pending_marking'
	 ON CONFLICT(attempt_id,event_type) DO NOTHING`, attemptID, event, awarded, max)
	return err
}
