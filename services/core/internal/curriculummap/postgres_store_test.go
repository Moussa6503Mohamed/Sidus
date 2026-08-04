package curriculummap

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// These integration tests run against a real, disposable PostgreSQL instance that has had the
// migrations applied. They write immutable curriculum_map_events rows and cannot clean up
// after themselves, so TEST_DATABASE_URL MUST point at the disposable postgres-test service in
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

// seedApprovedLinkedSource inserts a content_sources row that is approved and linked to
// syllabusID, and returns its id. Used so integration tests can pass the source gate without
// depending on the two pre-seeded (but pending, unlinked) 0610/5090 rows.
func seedApprovedLinkedSource(t *testing.T, db *sql.DB, syllabusID string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO content_sources (title, source_url, owner, source_hash, licence_reference, permitted_use, allowed_audience, status, catalogue_syllabus_id)
		VALUES ($1, $2, 'Test Owner', 'hash', 'licence', 'test use', 'test audience', 'approved', $3)
		RETURNING id`,
		"curriculum-map test source", "https://example.org/curriculum-map-test-"+randomSuffix(), syllabusID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed approved source: %v", err)
	}
	return id
}

func seedPendingSource(t *testing.T, db *sql.DB) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO content_sources (title, source_url, status)
		VALUES ('pending test source', $1, 'pending')
		RETURNING id`,
		"https://example.org/curriculum-map-pending-"+randomSuffix(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed pending source: %v", err)
	}
	return id
}

var suffixCounter int

func randomSuffix() string {
	suffixCounter++
	return "n" + string(rune('a'+suffixCounter%26)) + string(rune('0'+suffixCounter/26%10))
}

func getActiveSyllabusID(t *testing.T, db *sql.DB, code string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`SELECT id FROM syllabuses WHERE syllabus_code = $1 AND status = 'active' LIMIT 1`, code).Scan(&id); err != nil {
		t.Fatalf("get active syllabus %s: %v", code, err)
	}
	return id
}

func TestPostgresStore_Integration_CreateAndSourceGate(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	syllabusID := getActiveSyllabusID(t, db, "0610")
	otherSyllabusID := getActiveSyllabusID(t, db, "5090")
	approvedSource := seedApprovedLinkedSource(t, db, syllabusID)
	pendingSource := seedPendingSource(t, db)

	node, err := store.CreateNode(ctx, CreateInput{
		ActorID:         "test-actor",
		SyllabusID:      syllabusID,
		NodeKind:        KindTopic,
		NodeCode:        "IT-" + randomSuffix(),
		Label:           "Integration topic",
		ContentSourceID: approvedSource,
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	if node.Status != StatusDraft {
		t.Fatalf("status = %q, want draft", node.Status)
	}

	if _, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: "IT2-" + randomSuffix(), Label: "x", ContentSourceID: pendingSource,
	}); !errors.Is(err, ErrUnapprovedSource) {
		t.Fatalf("pending source: err = %v, want ErrUnapprovedSource", err)
	}

	mismatchedSource := seedApprovedLinkedSource(t, db, otherSyllabusID)
	if _, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: "IT3-" + randomSuffix(), Label: "x", ContentSourceID: mismatchedSource,
	}); !errors.Is(err, ErrMismatchedSource) {
		t.Fatalf("mismatched source: err = %v, want ErrMismatchedSource", err)
	}

	if _, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: "00000000-0000-0000-0000-000000000000", NodeKind: KindTopic,
		NodeCode: "IT4-" + randomSuffix(), Label: "x", ContentSourceID: approvedSource,
	}); !errors.Is(err, ErrUnknownSyllabus) {
		t.Fatalf("unknown syllabus: err = %v, want ErrUnknownSyllabus", err)
	}

	// One immutable node_created event was written.
	var eventCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM curriculum_map_events WHERE node_id = $1`, node.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
}

func TestPostgresStore_Integration_ParentSyllabusAndCycle(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	syllabusID := getActiveSyllabusID(t, db, "0610")
	otherSyllabusID := getActiveSyllabusID(t, db, "5090")
	source := seedApprovedLinkedSource(t, db, syllabusID)
	otherSource := seedApprovedLinkedSource(t, db, otherSyllabusID)

	parent, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: "P-" + randomSuffix(), Label: "parent", ContentSourceID: source,
	})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	otherSyllabusParent, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: otherSyllabusID, NodeKind: KindTopic,
		NodeCode: "OP-" + randomSuffix(), Label: "other parent", ContentSourceID: otherSource,
	})
	if err != nil {
		t.Fatalf("create other-syllabus parent: %v", err)
	}

	if _, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, ParentNodeID: &otherSyllabusParent.ID,
		NodeKind: KindTopic, NodeCode: "C-" + randomSuffix(), Label: "child", ContentSourceID: source,
	}); !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("cross-syllabus parent: err = %v, want ErrInvalidParent", err)
	}

	child, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, ParentNodeID: &parent.ID,
		NodeKind: KindTopic, NodeCode: "C2-" + randomSuffix(), Label: "child", ContentSourceID: source,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	// Attempt to make parent a child of its own child: cycle.
	_, err = store.UpdateNode(ctx, parent.ID, UpdateInput{
		ActorID: "test-actor", ParentNodeIDSet: true, ParentNodeID: &child.ID,
	})
	if !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("cycle attempt: err = %v, want ErrInvalidParent", err)
	}
}

func TestPostgresStore_Integration_DuplicateCodeAndLifecycle(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	syllabusID := getActiveSyllabusID(t, db, "0610")
	source := seedApprovedLinkedSource(t, db, syllabusID)
	code := "DUP-" + randomSuffix()

	first, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: code, Label: "first", ContentSourceID: source,
	})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	if _, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: code, Label: "dup", ContentSourceID: source,
	}); !errors.Is(err, ErrDuplicateNodeCode) {
		t.Fatalf("duplicate code: err = %v, want ErrDuplicateNodeCode", err)
	}

	verified, err := store.VerifyNode(ctx, first.ID, "test-reviewer")
	if err != nil || verified.Status != StatusVerified {
		t.Fatalf("verify: node=%+v err=%v", verified, err)
	}

	if _, err := store.UpdateNode(ctx, first.ID, UpdateInput{ActorID: "test-actor", Label: strPtr("changed")}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("update verified node: err = %v, want ErrInvalidTransition", err)
	}

	retired, err := store.RetireNode(ctx, first.ID, "test-reviewer")
	if err != nil || retired.Status != StatusRetired {
		t.Fatalf("retire: node=%+v err=%v", retired, err)
	}

	if _, err := store.RetireNode(ctx, first.ID, "test-reviewer"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("re-retire: err = %v, want ErrInvalidTransition", err)
	}

	var eventTypes []string
	rows, err := db.QueryContext(ctx, `SELECT event_type FROM curriculum_map_events WHERE node_id = $1 ORDER BY created_at ASC`, first.ID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var et string
		if err := rows.Scan(&et); err != nil {
			t.Fatalf("scan event: %v", err)
		}
		eventTypes = append(eventTypes, et)
	}
	if len(eventTypes) != 3 || eventTypes[0] != "node_created" || eventTypes[1] != "node_verified" || eventTypes[2] != "node_retired" {
		t.Fatalf("event types = %v, want [node_created node_verified node_retired]", eventTypes)
	}
}

// TestPostgresStore_Integration_EventImmutability confirms curriculum_map_events rows cannot
// be updated or deleted (the trigger rejects both).
func TestPostgresStore_Integration_EventImmutability(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	syllabusID := getActiveSyllabusID(t, db, "0610")
	source := seedApprovedLinkedSource(t, db, syllabusID)
	node, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: "IMM-" + randomSuffix(), Label: "x", ContentSourceID: source,
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}

	var eventID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM curriculum_map_events WHERE node_id = $1 LIMIT 1`, node.ID).Scan(&eventID); err != nil {
		t.Fatalf("find event: %v", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE curriculum_map_events SET actor_id = 'tampered' WHERE id = $1`, eventID); err == nil {
		t.Fatal("UPDATE on curriculum_map_events succeeded, want rejection by trigger")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM curriculum_map_events WHERE id = $1`, eventID); err == nil {
		t.Fatal("DELETE on curriculum_map_events succeeded, want rejection by trigger")
	}
}
