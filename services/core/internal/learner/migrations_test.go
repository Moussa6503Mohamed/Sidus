package learner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttemptMigrationIsAdditiveIdempotentAndImmutable(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "0019_create_learner_mcq_attempts.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(body)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS learner_attempts",
		"CREATE TABLE IF NOT EXISTS learner_attempt_events",
		"question_content_revision INTEGER NOT NULL",
		"canonical_rubric_version_id UUID NOT NULL",
		"CHECK (status IN ('open', 'submitted'))",
		"learner_attempt_events_immutable",
		"learner_attempts_immutable_pins",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"INSERT INTO questions", "INSERT INTO question_rubric_versions", "COPY ", "DROP TABLE learner_attempts"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration contains forbidden content %q", forbidden)
		}
	}
}
