package catalogue

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lib/pq"
)

// These integration tests run against a real, disposable PostgreSQL instance that has had the
// migrations applied (which seed the Biology subject and realign the catalogue scope).
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

// TestPostgres_Integration_SeedAndResolve confirms exact active and historical Biology scope
// and that registry resolution excludes retired syllabuses.
func TestPostgres_Integration_SeedAndResolve(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	var cambridgeCount, activeCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM syllabuses WHERE board = 'Cambridge International'`).Scan(&cambridgeCount); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cambridgeCount != 3 {
		t.Fatalf("Cambridge International syllabus count = %d, want 3 (0610/9700 active plus historical 5090)", cambridgeCount)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM syllabuses WHERE board = 'Cambridge International' AND status = 'active'`).Scan(&activeCount); err != nil {
		t.Fatalf("count active: %v", err)
	}
	if activeCount != 2 {
		t.Fatalf("active Cambridge International syllabus count = %d, want 2", activeCount)
	}

	id0610, found, err := store.ResolveActiveSyllabusByCode(ctx, "0610")
	if err != nil || !found || id0610 == "" {
		t.Fatalf("resolve 0610: id=%q found=%v err=%v", id0610, found, err)
	}
	id9700, found, err := store.ResolveActiveSyllabusByCode(ctx, "9700")
	if err != nil || !found || id9700 == "" {
		t.Fatalf("resolve 9700: id=%q found=%v err=%v", id9700, found, err)
	}
	if _, found, err := store.ResolveActiveSyllabusByCode(ctx, "5090"); err != nil || found {
		t.Fatalf("resolve retired 5090: found=%v err=%v, want false/nil", found, err)
	}
	if _, found, _ := store.ResolveActiveSyllabusByCode(ctx, "9999"); found {
		t.Fatal("resolve 9999: found=true, want false (unknown)")
	}

	// 0610 remains IGCSE / Extended.
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

	// 9700 is one metadata-only combined AS & A Level row with nullable track/year.
	syl9700, err := store.GetSyllabus(ctx, id9700, true)
	if err != nil {
		t.Fatalf("get 9700: %v", err)
	}
	if syl9700.Board != "Cambridge International" || syl9700.SyllabusCode != "9700" ||
		syl9700.Qualification != "International AS & A Level" ||
		syl9700.DisplayName != "Cambridge International AS & A Level Biology" ||
		syl9700.Track != nil || syl9700.CurriculumYear != nil || syl9700.Status != StatusActive {
		t.Fatalf("9700 seed = %+v, want exact combined active metadata", syl9700)
	}

	var id5090 string
	if err := db.QueryRowContext(ctx, `
		SELECT id FROM syllabuses
		WHERE board = 'Cambridge International' AND syllabus_code = '5090' AND track IS NULL AND status = 'retired'`,
	).Scan(&id5090); err != nil {
		t.Fatalf("get retained retired 5090: %v", err)
	}
	if _, err := store.GetSyllabus(ctx, id5090, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("active-only get 5090 err = %v, want ErrNotFound", err)
	}
	historical5090, err := store.GetSyllabus(ctx, id5090, false)
	if err != nil || historical5090.Status != StatusRetired {
		t.Fatalf("historical get 5090 = %+v err=%v, want retained retired row", historical5090, err)
	}
}

// TestPostgres_Integration_BiologyScopeMigrationRerun proves the SQL itself is idempotent and
// catalogue-only. It preserves 5090 identity/timestamps/history/source rows and never seeds
// 9700 source, map, question, or rubric content.
func TestPostgres_Integration_BiologyScopeMigrationRerun(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	type historicalState struct {
		id                      string
		createdAt, updatedAt    time.Time
		eventCount, sourceCount int
	}
	read5090 := func() historicalState {
		t.Helper()
		var state historicalState
		if err := db.QueryRowContext(ctx, `
			SELECT id, created_at, updated_at FROM syllabuses
			WHERE board = 'Cambridge International' AND syllabus_code = '5090' AND track IS NULL AND status = 'retired'`,
		).Scan(&state.id, &state.createdAt, &state.updatedAt); err != nil {
			t.Fatalf("read retired 5090: %v", err)
		}
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM syllabus_events WHERE syllabus_id = $1`, state.id).Scan(&state.eventCount); err != nil {
			t.Fatalf("count 5090 events: %v", err)
		}
		if err := db.QueryRowContext(ctx, `
			SELECT count(*) FROM content_sources
			WHERE syllabus_code = '5090' OR catalogue_syllabus_id = $1`, state.id,
		).Scan(&state.sourceCount); err != nil {
			t.Fatalf("count 5090 sources: %v", err)
		}
		return state
	}

	assert9700CatalogueOnly := func() {
		t.Helper()
		var syllabusID string
		var count int
		if err := db.QueryRowContext(ctx, `
			SELECT id FROM syllabuses
			WHERE board = 'Cambridge International' AND syllabus_code = '9700' AND track IS NULL
			  AND qualification = 'International AS & A Level'
			  AND display_name = 'Cambridge International AS & A Level Biology'
			  AND curriculum_year IS NULL AND status = 'active'`,
		).Scan(&syllabusID); err != nil {
			t.Fatalf("read exact 9700 row: %v", err)
		}
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM syllabuses WHERE syllabus_code = '9700' AND status = 'active'`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("active 9700 row count = %d err=%v, want 1", count, err)
		}
		queries := map[string]string{
			"content sources":  `SELECT count(*) FROM content_sources WHERE syllabus_code = '9700' OR catalogue_syllabus_id = $1`,
			"curriculum nodes": `SELECT count(*) FROM curriculum_map_nodes WHERE syllabus_id = $1`,
			"questions":        `SELECT count(*) FROM questions WHERE syllabus_id = $1`,
			"rubrics":          `SELECT count(*) FROM question_rubric_versions r JOIN questions q ON q.id = r.question_id WHERE q.syllabus_id = $1`,
		}
		for name, query := range queries {
			if err := db.QueryRowContext(ctx, query, syllabusID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("9700 %s count = %d err=%v, want 0", name, count, err)
			}
		}
	}

	before := read5090()
	assert9700CatalogueOnly()
	body, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0016_realign_biology_vertical_slice.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("rerun migration pass %d: %v", i, err)
		}
	}
	after := read5090()
	if after != before {
		t.Fatalf("5090 state changed on rerun: before=%+v after=%+v", before, after)
	}
	assert9700CatalogueOnly()
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

// TestPostgres_Integration_SubjectAuditAndImmutability confirms admin subject creation records
// a subject_created event with the verified actor and names-only changed_fields, that a
// duplicate (rejected) creation records no event, and that subject_events rows are immutable
// (UPDATE and DELETE both rejected by the trigger).
func TestPostgres_Integration_SubjectAuditAndImmutability(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	name := "Physics " + time.Now().Format("150405.000000000")
	subject, err := store.CreateSubject(ctx, CreateSubjectInput{ActorID: "admin-subject", Name: name})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	// No cleanup: this created an immutable subject_events row (disposable DB, destroyed after).

	var actorID string
	var changedFields []string
	var eventType string
	if err := db.QueryRowContext(ctx,
		`SELECT actor_id, event_type, changed_fields FROM subject_events WHERE subject_id = $1`, subject.ID,
	).Scan(&actorID, &eventType, pq.Array(&changedFields)); err != nil {
		t.Fatalf("query subject event: %v", err)
	}
	if actorID != "admin-subject" {
		t.Fatalf("actor_id = %q, want %q (verified subject)", actorID, "admin-subject")
	}
	if eventType != string(EventSubjectCreated) {
		t.Fatalf("event_type = %q, want %q", eventType, EventSubjectCreated)
	}
	if len(changedFields) != 1 || changedFields[0] != "name" {
		t.Fatalf("changed_fields = %v, want [\"name\"] (names only, never the value)", changedFields)
	}
	for _, f := range changedFields {
		if f == name {
			t.Fatal("changed_fields must never contain the submitted subject name")
		}
	}

	// A duplicate (rejected) subject creation records no new event.
	var beforeCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM subject_events`).Scan(&beforeCount); err != nil {
		t.Fatalf("count events before: %v", err)
	}
	if _, err := store.CreateSubject(ctx, CreateSubjectInput{ActorID: "admin-subject", Name: name}); !errors.Is(err, ErrDuplicateSubject) {
		t.Fatalf("duplicate create: err = %v, want ErrDuplicateSubject", err)
	}
	var afterCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM subject_events`).Scan(&afterCount); err != nil {
		t.Fatalf("count events after: %v", err)
	}
	if afterCount != beforeCount {
		t.Fatalf("subject_events count = %d, want %d (duplicate create must record no event)", afterCount, beforeCount)
	}

	// subject_events rows are immutable even against direct SQL.
	if _, err := db.ExecContext(ctx, `UPDATE subject_events SET actor_id = 'tampered' WHERE subject_id = $1`, subject.ID); err == nil {
		t.Fatal("expected UPDATE on subject_events to be rejected by the immutability trigger")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM subject_events WHERE subject_id = $1`, subject.ID); err == nil {
		t.Fatal("expected DELETE on subject_events to be rejected by the immutability trigger")
	}
}

// TestPostgres_Integration_SeededBiologySubject_NoSubjectEvent confirms the seed exception: the
// Biology subject seeded by migration 0007 was inserted via plain SQL (not the CreateSubject
// application path), so it carries no subject_events row and no invented actor id.
func TestPostgres_Integration_SeededBiologySubject_NoSubjectEvent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	var subjectID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM subjects WHERE name = 'Biology'`).Scan(&subjectID); err != nil {
		t.Fatalf("find Biology subject: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM subject_events WHERE subject_id = $1`, subjectID).Scan(&count); err != nil {
		t.Fatalf("count subject events: %v", err)
	}
	if count != 0 {
		t.Fatalf("seeded Biology subject_events count = %d, want 0 (bootstrap seed, not a human-actor event)", count)
	}
}

// TestPostgres_Integration_MalformedID_NotFoundNotInternalError exercises T-0008: a malformed
// (non-UUID) id given to GetSyllabus or UpdateSyllabus must produce the same stable not_found
// behavior as a well-formed but missing id — never a 500 leaking Postgres 22P02 driver text.
func TestPostgres_Integration_MalformedID_NotFoundNotInternalError(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	malformed := "not-a-uuid"
	wellFormedMissing := "00000000-0000-0000-0000-000000000000"

	for name, id := range map[string]string{
		"malformed":           malformed,
		"well-formed missing": wellFormedMissing,
	} {
		t.Run("GetSyllabus/"+name, func(t *testing.T) {
			_, err := store.GetSyllabus(ctx, id, true)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("GetSyllabus(%q) err = %v, want ErrNotFound", id, err)
			}
		})
		t.Run("UpdateSyllabus/"+name, func(t *testing.T) {
			_, err := store.UpdateSyllabus(ctx, id, UpdateSyllabusInput{ActorID: "tester", Board: strPtr("New Board")})
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("UpdateSyllabus(%q) err = %v, want ErrNotFound", id, err)
			}
		})
	}
}
