package catalogue

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// These integration tests run against a real, disposable PostgreSQL instance that has had the
// migrations applied (which seed the Biology subject and the two active biology syllabuses).
// They write immutable syllabus_events rows and cannot clean up after themselves, so
// TEST_DATABASE_URL MUST point at the disposable postgres-test service in
// docker-compose.test.yml — never the dev or prod database. Skipped unless it is set.

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	return db
}

// TestPostgres_Integration_SeedAndResolve confirms exactly the two seeded biology syllabuses
// are present and active, and that registry resolution works for known and unknown codes.
func TestPostgres_Integration_SeedAndResolve(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	// Only the two Cambridge International biology syllabuses are seeded.
	var cambridgeCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM syllabuses WHERE board = 'Cambridge International'`).Scan(&cambridgeCount); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cambridgeCount != 2 {
		t.Fatalf("Cambridge International syllabus count = %d, want 2 (only 0610/5090 seeded)", cambridgeCount)
	}

	id0610, found, err := store.ResolveActiveSyllabusByCode(ctx, "0610")
	if err != nil || !found || id0610 == "" {
		t.Fatalf("resolve 0610: id=%q found=%v err=%v", id0610, found, err)
	}
	if _, found, _ := store.ResolveActiveSyllabusByCode(ctx, "9999"); found {
		t.Fatal("resolve 9999: found=true, want false (unknown)")
	}

	// The 0610 seed is IGCSE / Extended; the 5090 seed is O Level / no track.
	syl, err := store.GetSyllabus(ctx, id0610, true)
	if err != nil {
		t.Fatalf("get 0610: %v", err)
	}
	if syl.Qualification != "Cambridge IGCSE" || syl.Track == nil || *syl.Track != "Extended" {
		t.Fatalf("0610 seed = %+v, want IGCSE/Extended", syl)
	}
	if syl.CurriculumYear != nil {
		t.Fatalf("0610 curriculumYear = %v, want nil (never inferred)", *syl.CurriculumYear)
	}
}

// TestPostgres_Integration_UniquenessAndFK exercises the (board, code, track) uniqueness rule,
// the subject FK, and that different boards may reuse a code.
func TestPostgres_Integration_UniquenessAndFK(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	var subjectID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM subjects WHERE name = 'Biology'`).Scan(&subjectID); err != nil {
		t.Fatalf("find Biology subject: %v", err)
	}

	// Duplicate (board, code, track) is rejected.
	if _, err := store.CreateSyllabus(ctx, CreateSyllabusInput{
		ActorID: "admin", Board: "Cambridge International", SyllabusCode: "0610", SubjectID: subjectID,
		Qualification: "Cambridge IGCSE", Track: strPtr("Extended"), DisplayName: "dup", Status: StatusActive,
	}); !errors.Is(err, ErrDuplicateSyllabus) {
		t.Fatalf("duplicate create: err = %v, want ErrDuplicateSyllabus", err)
	}

	// A bad subject FK is rejected.
	if _, err := store.CreateSyllabus(ctx, CreateSyllabusInput{
		ActorID: "admin", Board: "Cambridge International", SyllabusCode: "1111",
		SubjectID: "00000000-0000-0000-0000-000000000000", Qualification: "Cambridge IGCSE",
		DisplayName: "bad fk",
	}); !errors.Is(err, ErrSubjectNotFound) {
		t.Fatalf("bad subject FK: err = %v, want ErrSubjectNotFound", err)
	}

	// A different board may reuse code 0610 (code is not globally unique).
	board := "Test Board " + time.Now().Format("150405.000000000")
	if _, err := store.CreateSyllabus(ctx, CreateSyllabusInput{
		ActorID: "admin", Board: board, SyllabusCode: "0610", SubjectID: subjectID,
		Qualification: "Some Level", DisplayName: "other board 0610", Status: StatusDraft,
	}); err != nil {
		t.Fatalf("different-board same-code create: %v", err)
	}
}

// TestPostgres_Integration_EventImmutability confirms catalogue mutations record an event and
// that syllabus_events rows are immutable (UPDATE and DELETE both rejected by the trigger).
func TestPostgres_Integration_EventImmutability(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	var subjectID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM subjects WHERE name = 'Biology'`).Scan(&subjectID); err != nil {
		t.Fatalf("find Biology subject: %v", err)
	}

	board := "Event Board " + time.Now().Format("150405.000000000")
	syl, err := store.CreateSyllabus(ctx, CreateSyllabusInput{
		ActorID: "admin-subject", Board: board, SyllabusCode: "2222", SubjectID: subjectID,
		Qualification: "Some Level", DisplayName: "event test", Status: StatusDraft,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// No cleanup: this created an immutable syllabus_events row (disposable DB, destroyed after).

	if _, err := store.UpdateSyllabus(ctx, syl.ID, UpdateSyllabusInput{ActorID: "admin-subject", Status: statusPtr(StatusActive)}); err != nil {
		t.Fatalf("update: %v", err)
	}

	var eventCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM syllabus_events WHERE syllabus_id = $1`, syl.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 2 { // created + updated
		t.Fatalf("eventCount = %d, want 2", eventCount)
	}

	if _, err := db.ExecContext(ctx, `UPDATE syllabus_events SET actor_id = 'tampered' WHERE syllabus_id = $1`, syl.ID); err == nil {
		t.Fatal("expected UPDATE on syllabus_events to be rejected by the immutability trigger")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM syllabus_events WHERE syllabus_id = $1`, syl.ID); err == nil {
		t.Fatal("expected DELETE on syllabus_events to be rejected by the immutability trigger")
	}

	// A no-op update is rejected without a new event.
	if _, err := store.UpdateSyllabus(ctx, syl.ID, UpdateSyllabusInput{ActorID: "admin-subject", Status: statusPtr(StatusActive)}); !errors.Is(err, ErrNoChanges) {
		t.Fatalf("no-op update: err = %v, want ErrNoChanges", err)
	}
}

// TestPostgres_Integration_ContentSourceFKColumn confirms the nullable catalogue FK column was
// added to content_sources non-destructively (present, nullable, referencing syllabuses).
func TestPostgres_Integration_ContentSourceFKColumn(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	var isNullable string
	err := db.QueryRowContext(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_name = 'content_sources' AND column_name = 'catalogue_syllabus_id'`).Scan(&isNullable)
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatal("content_sources.catalogue_syllabus_id column is missing")
	}
	if err != nil {
		t.Fatalf("query column: %v", err)
	}
	if isNullable != "YES" {
		t.Fatalf("catalogue_syllabus_id is_nullable = %q, want YES", isNullable)
	}
}

func statusPtr(s SyllabusStatus) *SyllabusStatus { return &s }
