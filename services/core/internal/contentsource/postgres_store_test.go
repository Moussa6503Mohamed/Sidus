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

// TestPostgresStore_Integration exercises PostgresStore against a real database. It is
// skipped unless TEST_DATABASE_URL is set (e.g. to the docker-compose postgres service),
// so `go test ./...` does not require a live database.
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
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM content_source_reviews WHERE content_source_id = $1`, source.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM content_sources WHERE id = $1`, source.ID)
	})

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
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM content_source_events WHERE content_source_id = $1`, source.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM content_sources WHERE id = $1`, source.ID)
	})

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
	// reviews cleanup (reject inserted one)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM content_source_reviews WHERE content_source_id = $1`, source.ID)
	})
}
