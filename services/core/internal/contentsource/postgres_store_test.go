package contentsource

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// These integration tests write rows to content_source_reviews and content_source_events,
// both of which are immutable at the DB level (a BEFORE UPDATE OR DELETE trigger rejects
// any mutation — see migrations 0002 and 0004). That means these tests CANNOT clean up
// after themselves: no DELETE against those tables can succeed, and content_sources rows
// referenced by them can't be deleted either (FK, no cascade). An earlier version of this
// file attempted such deletes and swallowed the error, which silently left rows behind in
// whatever database TEST_DATABASE_URL pointed at.
//
// Consequently: TEST_DATABASE_URL MUST point at a disposable PostgreSQL instance (e.g. the
// `postgres-test` service in docker-compose.test.yml), never the dev or prod database.
// Provision it, run the migrations, run these tests, then destroy the container and its
// volume — see docker-compose.test.yml for the disposable service definition. Do not add
// cleanup code here; do not weaken or bypass the immutability triggers to make cleanup
// possible.

// TestPostgresStore_Integration exercises PostgresStore against a real, disposable
// database. It is skipped unless TEST_DATABASE_URL is set, so `go test ./...` does not
// require a live database.
func TestPostgresStore_Integration(t *testing.T) {
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

	store := NewPostgresStore(db)
	ctx := context.Background()

	url := "https://example.org/integration-test-" + time.Now().Format("20060102150405.000000000")

	source, err := store.Create(ctx, CreateInput{Title: "Integration syllabus", SourceURL: url})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// No cleanup: this test creates an immutable content_source_reviews row (see file-level
	// comment above). Requires a disposable database, destroyed after the test run.

	if source.Status != StatusPending {
		t.Fatalf("status = %q, want pending", source.Status)
	}

	_, missing, err := store.Approve(ctx, source.ID, ApproveInput{ReviewerID: "reviewer-1", DecisionDate: time.Now()})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if len(missing) == 0 {
		t.Fatal("expected missing required fields for a source with no rights metadata")
	}

	fetched, err := store.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Status != StatusPending {
		t.Fatalf("status = %q, want still pending after failed approval", fetched.Status)
	}

	rejected, err := store.Reject(ctx, source.ID, RejectInput{ReviewerID: "reviewer-1", Reason: "missing rights fields", DecisionDate: time.Now()})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != StatusRejected {
		t.Fatalf("status = %q, want rejected", rejected.Status)
	}

	var reviewCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM content_source_reviews WHERE content_source_id = $1`, source.ID).Scan(&reviewCount); err != nil {
		t.Fatalf("count reviews: %v", err)
	}
	if reviewCount != 1 {
		t.Fatalf("reviewCount = %d, want 1", reviewCount)
	}

	if _, err := db.ExecContext(ctx, `UPDATE content_source_reviews SET reason = 'tampered' WHERE content_source_id = $1`, source.ID); err == nil {
		t.Fatal("expected UPDATE on content_source_reviews to be rejected by the immutability trigger")
	}
}

// TestPostgresStore_UpdateEventImmutability exercises PATCH-driven updates against a real
// database: it confirms a pending source can be updated, a metadata_updated event is
// recorded with the changed field names, and that event rows are immutable (UPDATE and
// DELETE both rejected by the trigger). Skipped unless TEST_DATABASE_URL is set.
func TestPostgresStore_Integration_UpdateEventImmutability(t *testing.T) {
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

	store := NewPostgresStore(db)
	ctx := context.Background()

	url := "https://example.org/update-immutability-" + time.Now().Format("20060102150405.000000000")
	source, err := store.Create(ctx, CreateInput{Title: "Update syllabus", SourceURL: url})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// No cleanup: this test creates immutable content_source_events rows (see file-level
	// comment above). Requires a disposable database, destroyed after the test run.

	owner := "Cambridge Assessment International Education"
	licence := "CAIE-PUBLIC-SYLLABUS-2026"
	updated, changed, err := store.Update(ctx, source.ID, UpdateInput{
		ActorID:          "curator-1",
		Owner:            &owner,
		LicenceReference: &licence,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Owner == nil || *updated.Owner != owner {
		t.Fatalf("owner = %v, want %q", updated.Owner, owner)
	}
	if updated.Status != StatusPending {
		t.Fatalf("status = %q, want still pending after update", updated.Status)
	}
	wantChanged := []string{"owner", "licenceReference"}
	if len(changed) != len(wantChanged) {
		t.Fatalf("changed = %v, want %v", changed, wantChanged)
	}

	var eventCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM content_source_events WHERE content_source_id = $1`, source.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("eventCount = %d, want 1", eventCount)
	}

	if _, err := db.ExecContext(ctx, `UPDATE content_source_events SET actor_id = 'tampered' WHERE content_source_id = $1`, source.ID); err == nil {
		t.Fatal("expected UPDATE on content_source_events to be rejected by the immutability trigger")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM content_source_events WHERE content_source_id = $1`, source.ID); err == nil {
		t.Fatal("expected DELETE on content_source_events to be rejected by the immutability trigger")
	}

	// A non-pending source cannot be updated.
	if _, err := store.Reject(ctx, source.ID, RejectInput{ReviewerID: "r1", Reason: "test", DecisionDate: time.Now()}); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, _, err := store.Update(ctx, source.ID, UpdateInput{ActorID: "curator-1", Owner: &owner}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("update after reject: err = %v, want ErrInvalidTransition", err)
	}
}

// TestPostgresStore_Integration_UpdateOnlyChangedFields confirms Update diffs supplied
// values against the currently stored row: a field re-supplied with its existing value is
// left out of changed_fields, and a request whose values all match returns ErrNoChanges
// with no write and no audit event. Skipped unless TEST_DATABASE_URL is set.
func TestPostgresStore_Integration_UpdateOnlyChangedFields(t *testing.T) {
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

	store := NewPostgresStore(db)
	ctx := context.Background()

	owner := "Cambridge Assessment International Education"
	url := "https://example.org/update-diff-" + time.Now().Format("20060102150405.000000000")
	source, err := store.Create(ctx, CreateInput{Title: "Diff syllabus", SourceURL: url, Owner: &owner})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// No cleanup: this test creates immutable content_source_events rows (see file-level
	// comment above). Requires a disposable database, destroyed after the test run.

	licence := "CAIE-PUBLIC-SYLLABUS-2026"
	updated, changed, err := store.Update(ctx, source.ID, UpdateInput{
		ActorID:          "curator-1",
		Owner:            &owner, // same as stored: must not appear in changed
		LicenceReference: &licence,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(changed) != 1 || changed[0] != "licenceReference" {
		t.Fatalf("changed = %v, want [licenceReference]", changed)
	}

	if _, _, err := store.Update(ctx, source.ID, UpdateInput{ActorID: "curator-1", Owner: &owner, LicenceReference: &licence}); !errors.Is(err, ErrNoChanges) {
		t.Fatalf("update with all-same values: err = %v, want ErrNoChanges", err)
	}

	var eventCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM content_source_events WHERE content_source_id = $1`, source.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("eventCount = %d, want 1 (no-change request must not create a second event)", eventCount)
	}

	if updated.UpdatedAt.IsZero() {
		t.Fatal("expected updatedAt to be set on the real update")
	}
	unchanged, err := store.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !unchanged.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("updatedAt changed after no-change request: before=%v after=%v", updated.UpdatedAt, unchanged.UpdatedAt)
	}
}
