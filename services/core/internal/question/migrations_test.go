package question

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// migrationsDir is the repository's SQL migration set, relative to this package.
const migrationsDir = "../../migrations"

// seedPattern matches any statement that would insert or copy rows into the private-content
// tables introduced by T-0007.
var seedPattern = regexp.MustCompile(`(?is)(insert\s+into|copy)\s+"?(questions|question_rubric_versions|question_events)"?`)

// TestMigrations_SeedNoQuestionOrRubricContent is a guard, not a formality: the public repository
// may hold schema, code, contracts, docs, and tests — never question prompts, rubric structures,
// mark schemes, or any other content. Question and rubric rows exist only in a runtime database,
// written by a future private editorial workflow.
func TestMigrations_SeedNoQuestionOrRubricContent(t *testing.T) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		checked++
		if match := seedPattern.Find(body); match != nil {
			t.Fatalf("migration %s seeds private question/rubric content (%q); no such data may live in this repository",
				entry.Name(), strings.TrimSpace(string(match)))
		}
	}
	if checked == 0 {
		t.Fatal("no migrations were checked; the guard would pass vacuously")
	}
}

// TestMigrations_QuestionTablesExist keeps the guard above honest: it must be scanning a
// migration set that actually creates the tables it is protecting.
func TestMigrations_QuestionTablesExist(t *testing.T) {
	for file, table := range map[string]string{
		"0012_create_questions.sql":                "questions",
		"0013_create_question_rubric_versions.sql": "question_rubric_versions",
		"0014_create_question_events.sql":          "question_events",
	} {
		body, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.Contains(string(body), "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("%s does not create %s", file, table)
		}
	}
}
