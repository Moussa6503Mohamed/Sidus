package curriculummap

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

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

var suffixCounter atomic.Int64

// randomSuffix returns a value unique across runs, not just within one process: these tests
// seed rows into a database that persists for the whole test session (and across repeated
// `go test` invocations against the same disposable instance), and content_sources.source_url
// is unique. A per-process counter alone collides on the second run.
func randomSuffix() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatInt(suffixCounter.Add(1), 36)
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

// --- Source gate re-validated on every node write (T-0006 review fix 2) ---

// setSourceState rewrites a content_sources row's gate-relevant columns, simulating a source
// that has been un-approved, unlinked, or re-linked after a node was already grounded in it.
func setSourceState(t *testing.T, db *sql.DB, sourceID, status string, catalogueSyllabusID *string) {
	t.Helper()
	if _, err := db.Exec(
		`UPDATE content_sources SET status = $1, catalogue_syllabus_id = $2 WHERE id = $3`,
		status, catalogueSyllabusID, sourceID,
	); err != nil {
		t.Fatalf("break source state: %v", err)
	}
}

// nodeRowSnapshot is the persisted state a rejected write must leave byte-identical.
type nodeRowSnapshot struct {
	status        string
	label         string
	sourceID      string
	updatedAt     time.Time
	eventCount    int
	parentNodeID  sql.NullString
	nodeCodeValue string
}

func snapshotNodeRow(t *testing.T, db *sql.DB, nodeID string) nodeRowSnapshot {
	t.Helper()
	var s nodeRowSnapshot
	if err := db.QueryRow(
		`SELECT status, label, content_source_id, updated_at, parent_node_id, node_code
		 FROM curriculum_map_nodes WHERE id = $1`, nodeID,
	).Scan(&s.status, &s.label, &s.sourceID, &s.updatedAt, &s.parentNodeID, &s.nodeCodeValue); err != nil {
		t.Fatalf("snapshot node row: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM curriculum_map_events WHERE node_id = $1`, nodeID).Scan(&s.eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	return s
}

func assertNodeRowUnchanged(t *testing.T, db *sql.DB, nodeID string, before nodeRowSnapshot) {
	t.Helper()
	after := snapshotNodeRow(t, db, nodeID)
	if after.status != before.status {
		t.Fatalf("status changed by a rejected write: %q -> %q", before.status, after.status)
	}
	if !after.updatedAt.Equal(before.updatedAt) {
		t.Fatalf("updated_at changed by a rejected write: %v -> %v", before.updatedAt, after.updatedAt)
	}
	if after.label != before.label || after.sourceID != before.sourceID ||
		after.nodeCodeValue != before.nodeCodeValue || after.parentNodeID != before.parentNodeID {
		t.Fatalf("node row mutated by a rejected write:\n before = %+v\n after  = %+v", before, after)
	}
	if after.eventCount != before.eventCount {
		t.Fatalf("event count = %d, want %d (a rejected write must append no event)", after.eventCount, before.eventCount)
	}
}

// brokenSourceState names each failed source state reachable for a node's *stored* source. An
// unknown stored source is not reachable here — content_source_id is a NOT NULL FK, so the row
// cannot be deleted out from under a node; unknown sources are covered on the PATCH-repoint
// path in TestPostgresStore_Integration_UpdateRepointToUnknownSource.
type brokenSourceState struct {
	name      string
	apply     func(t *testing.T, db *sql.DB, sourceID, nodeSyllabusID, otherSyllabusID string)
	wantError error
}

func brokenStoredSourceStates() []brokenSourceState {
	return []brokenSourceState{
		{
			name: "unapproved",
			apply: func(t *testing.T, db *sql.DB, sourceID, nodeSyllabusID, _ string) {
				setSourceState(t, db, sourceID, "pending", &nodeSyllabusID)
			},
			wantError: ErrUnapprovedSource,
		},
		{
			name: "unlinked",
			apply: func(t *testing.T, db *sql.DB, sourceID, _, _ string) {
				setSourceState(t, db, sourceID, "approved", nil)
			},
			wantError: ErrUnlinkedSource,
		},
		{
			name: "mismatched",
			apply: func(t *testing.T, db *sql.DB, sourceID, _, otherSyllabusID string) {
				setSourceState(t, db, sourceID, "approved", &otherSyllabusID)
			},
			wantError: ErrMismatchedSource,
		},
	}
}

// runStoredSourceGateCases creates a fresh node (each with its own source row, so breaking one
// case's source cannot affect another), breaks the source, then runs action — which must fail
// with the expected stable error and write nothing.
func runStoredSourceGateCases(
	t *testing.T,
	prepare func(t *testing.T, store *PostgresStore, nodeID string),
	action func(ctx context.Context, store *PostgresStore, nodeID string) error,
) {
	t.Helper()
	for _, tc := range brokenStoredSourceStates() {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			store := NewPostgresStore(db)
			ctx := context.Background()

			syllabusID := getActiveSyllabusID(t, db, "0610")
			otherSyllabusID := getActiveSyllabusID(t, db, "5090")
			source := seedApprovedLinkedSource(t, db, syllabusID)

			node, err := store.CreateNode(ctx, CreateInput{
				ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
				NodeCode: "GATE-" + randomSuffix(), Label: "gate", ContentSourceID: source,
			})
			if err != nil {
				t.Fatalf("create node: %v", err)
			}
			if prepare != nil {
				prepare(t, store, node.ID)
			}

			tc.apply(t, db, source, syllabusID, otherSyllabusID)
			before := snapshotNodeRow(t, db, node.ID)

			if err := action(ctx, store, node.ID); !errors.Is(err, tc.wantError) {
				t.Fatalf("err = %v, want %v", err, tc.wantError)
			}
			assertNodeRowUnchanged(t, db, node.ID, before)
		})
	}
}

// TestPostgresStore_Integration_UpdateRevalidatesSourceGate: PATCH re-runs the gate even when
// the request never mentions contentSourceId.
func TestPostgresStore_Integration_UpdateRevalidatesSourceGate(t *testing.T) {
	runStoredSourceGateCases(t, nil, func(ctx context.Context, store *PostgresStore, nodeID string) error {
		_, err := store.UpdateNode(ctx, nodeID, UpdateInput{ActorID: "test-actor", Label: strPtr("changed label")})
		return err
	})
}

// TestPostgresStore_Integration_VerifyRevalidatesSourceGate: verify re-runs the gate before the
// status transition and its event.
func TestPostgresStore_Integration_VerifyRevalidatesSourceGate(t *testing.T) {
	runStoredSourceGateCases(t, nil, func(ctx context.Context, store *PostgresStore, nodeID string) error {
		_, err := store.VerifyNode(ctx, nodeID, "test-reviewer")
		return err
	})
}

// TestPostgresStore_Integration_RetireRevalidatesSourceGate: retire re-runs the gate from draft.
func TestPostgresStore_Integration_RetireRevalidatesSourceGate(t *testing.T) {
	runStoredSourceGateCases(t, nil, func(ctx context.Context, store *PostgresStore, nodeID string) error {
		_, err := store.RetireNode(ctx, nodeID, "test-reviewer")
		return err
	})
}

// TestPostgresStore_Integration_RetireVerifiedRevalidatesSourceGate: the gate also applies on
// the verified → retired transition.
func TestPostgresStore_Integration_RetireVerifiedRevalidatesSourceGate(t *testing.T) {
	runStoredSourceGateCases(t,
		func(t *testing.T, store *PostgresStore, nodeID string) {
			if _, err := store.VerifyNode(context.Background(), nodeID, "test-reviewer"); err != nil {
				t.Fatalf("prepare verify: %v", err)
			}
		},
		func(ctx context.Context, store *PostgresStore, nodeID string) error {
			_, err := store.RetireNode(ctx, nodeID, "test-reviewer")
			return err
		},
	)
}

// TestPostgresStore_Integration_UpdateRepointToUnknownSource covers the unknown-source state,
// which is only reachable by supplying a source id that resolves to no row.
func TestPostgresStore_Integration_UpdateRepointToUnknownSource(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	syllabusID := getActiveSyllabusID(t, db, "0610")
	source := seedApprovedLinkedSource(t, db, syllabusID)
	node, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: "UNK-" + randomSuffix(), Label: "x", ContentSourceID: source,
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	before := snapshotNodeRow(t, db, node.ID)

	if _, err := store.UpdateNode(ctx, node.ID, UpdateInput{
		ActorID: "test-actor", ContentSourceID: strPtr("00000000-0000-0000-0000-000000000000"),
	}); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("err = %v, want ErrUnknownSource", err)
	}
	assertNodeRowUnchanged(t, db, node.ID, before)
}

// --- Syllabus validation on list (T-0006 review fix 3) ---

// seedActiveSyllabus creates an isolated active catalogue syllabus so the "no verified nodes"
// assertion cannot be disturbed by nodes other tests create under 0610/5090.
func seedActiveSyllabus(t *testing.T, db *sql.DB) string {
	t.Helper()
	var subjectID string
	if err := db.QueryRow(`SELECT id FROM subjects ORDER BY created_at ASC LIMIT 1`).Scan(&subjectID); err != nil {
		t.Fatalf("get seeded subject: %v", err)
	}
	var id string
	suffix := randomSuffix()
	if err := db.QueryRow(`
		INSERT INTO syllabuses (board, syllabus_code, subject_id, qualification, track, display_name, status)
		VALUES ('Test Board', $1, $2, 'Test Qualification', $3, 'Curriculum-map list test syllabus', 'active')
		RETURNING id`,
		"CM-"+suffix, subjectID, "track-"+suffix,
	).Scan(&id); err != nil {
		t.Fatalf("seed active syllabus: %v", err)
	}
	return id
}

func TestPostgresStore_Integration_ListValidatesSyllabus(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	// Unknown syllabus id → stable domain error, never an empty 200.
	if _, err := store.ListNodesBySyllabus(ctx, "00000000-0000-0000-0000-000000000000", nil); !errors.Is(err, ErrUnknownSyllabus) {
		t.Fatalf("unknown syllabus: err = %v, want ErrUnknownSyllabus", err)
	}

	// A malformed (non-UUID) id is unknown too, not an infrastructure failure.
	if _, err := store.ListNodesBySyllabus(ctx, "not-a-uuid", nil); !errors.Is(err, ErrUnknownSyllabus) {
		t.Fatalf("malformed syllabus: err = %v, want ErrUnknownSyllabus", err)
	}

	// An inactive (draft) syllabus is rejected the same way.
	inactiveID := seedActiveSyllabus(t, db)
	if _, err := db.Exec(`UPDATE syllabuses SET status = 'draft' WHERE id = $1`, inactiveID); err != nil {
		t.Fatalf("set syllabus draft: %v", err)
	}
	if _, err := store.ListNodesBySyllabus(ctx, inactiveID, nil); !errors.Is(err, ErrUnknownSyllabus) {
		t.Fatalf("inactive syllabus: err = %v, want ErrUnknownSyllabus", err)
	}

	// A known active syllabus with no nodes is an empty list, not an error.
	emptyID := seedActiveSyllabus(t, db)
	nodes, err := store.ListNodesBySyllabus(ctx, emptyID, nil)
	if err != nil {
		t.Fatalf("active syllabus with no nodes: err = %v, want nil", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("items = %d, want 0", len(nodes))
	}

	// An explicit status filter narrows results to exactly that status.
	verifiedOnly := StatusVerified
	filtered, err := store.ListNodesBySyllabus(ctx, emptyID, &verifiedOnly)
	if err != nil {
		t.Fatalf("filtered list: err = %v, want nil", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("filtered items = %d, want 0", len(filtered))
	}
}

// --- Ancestor chain locking and cycle detection (T-0006 review fix 4) ---

// TestPostgresStore_Integration_AncestorChainLocking exercises the row-locked ancestor walk:
// self-parent, an indirect (multi-hop) cycle, and a cross-syllabus parent are all rejected as
// invalid_parent with no write. Every ancestor traversed is locked FOR UPDATE inside the write
// transaction, so a concurrent re-parent cannot slip a cycle past the check; the assertion
// available to a single-connection test is that the walk still terminates and rejects.
func TestPostgresStore_Integration_AncestorChainLocking(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	syllabusID := getActiveSyllabusID(t, db, "0610")
	otherSyllabusID := getActiveSyllabusID(t, db, "5090")
	source := seedApprovedLinkedSource(t, db, syllabusID)
	otherSource := seedApprovedLinkedSource(t, db, otherSyllabusID)

	newNode := func(code string, parent *string) Node {
		t.Helper()
		n, err := store.CreateNode(ctx, CreateInput{
			ActorID: "test-actor", SyllabusID: syllabusID, ParentNodeID: parent,
			NodeKind: KindTopic, NodeCode: code + "-" + randomSuffix(), Label: "x", ContentSourceID: source,
		})
		if err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
		return n
	}

	// Chain a -> b -> c (c is the deepest descendant).
	a := newNode("LA", nil)
	b := newNode("LB", &a.ID)
	c := newNode("LC", &b.ID)

	// Self-parent.
	before := snapshotNodeRow(t, db, a.ID)
	if _, err := store.UpdateNode(ctx, a.ID, UpdateInput{
		ActorID: "test-actor", ParentNodeIDSet: true, ParentNodeID: &a.ID,
	}); !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("self-parent: err = %v, want ErrInvalidParent", err)
	}
	assertNodeRowUnchanged(t, db, a.ID, before)

	// Indirect cycle: a -> c would close a -> b -> c -> a, found only by walking (and locking)
	// more than one ancestor.
	if _, err := store.UpdateNode(ctx, a.ID, UpdateInput{
		ActorID: "test-actor", ParentNodeIDSet: true, ParentNodeID: &c.ID,
	}); !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("indirect cycle: err = %v, want ErrInvalidParent", err)
	}
	assertNodeRowUnchanged(t, db, a.ID, before)

	// Cross-syllabus parent.
	crossParent, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: otherSyllabusID, NodeKind: KindTopic,
		NodeCode: "LX-" + randomSuffix(), Label: "x", ContentSourceID: otherSource,
	})
	if err != nil {
		t.Fatalf("create cross-syllabus parent: %v", err)
	}
	beforeC := snapshotNodeRow(t, db, c.ID)
	if _, err := store.UpdateNode(ctx, c.ID, UpdateInput{
		ActorID: "test-actor", ParentNodeIDSet: true, ParentNodeID: &crossParent.ID,
	}); !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("cross-syllabus parent: err = %v, want ErrInvalidParent", err)
	}
	assertNodeRowUnchanged(t, db, c.ID, beforeC)

	// A valid deeper re-parent still succeeds (the walk terminates at the chain root).
	if _, err := store.UpdateNode(ctx, c.ID, UpdateInput{
		ActorID: "test-actor", ParentNodeIDSet: true, ParentNodeID: &a.ID,
	}); err != nil {
		t.Fatalf("valid re-parent: err = %v, want nil", err)
	}
}

// TestPostgresStore_Integration_AncestorLockBlocksConcurrentReparent asserts the ancestor walk
// actually takes row locks: while one transaction holds a lock on an ancestor, a concurrent
// update of that same ancestor row must block until the first transaction ends.
func TestPostgresStore_Integration_AncestorLockBlocksConcurrentReparent(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	syllabusID := getActiveSyllabusID(t, db, "0610")
	source := seedApprovedLinkedSource(t, db, syllabusID)

	root, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: "CR-" + randomSuffix(), Label: "root", ContentSourceID: source,
	})
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	mid, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, ParentNodeID: &root.ID, NodeKind: KindTopic,
		NodeCode: "CM-" + randomSuffix(), Label: "mid", ContentSourceID: source,
	})
	if err != nil {
		t.Fatalf("create mid: %v", err)
	}
	leaf, err := store.CreateNode(ctx, CreateInput{
		ActorID: "test-actor", SyllabusID: syllabusID, NodeKind: KindTopic,
		NodeCode: "CL-" + randomSuffix(), Label: "leaf", ContentSourceID: source,
	})
	if err != nil {
		t.Fatalf("create leaf: %v", err)
	}

	// Hold a lock on the *root* — an ancestor the walk from `mid` must traverse, not the
	// candidate parent itself. Before the fix the walk read it without a lock.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin blocking tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT id FROM curriculum_map_nodes WHERE id = $1 FOR UPDATE`, root.ID); err != nil {
		t.Fatalf("hold root lock: %v", err)
	}

	// A re-parent of leaf under mid must walk mid -> root and block on the held root lock.
	blockedCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := store.UpdateNode(blockedCtx, leaf.ID, UpdateInput{
			ActorID: "test-actor", ParentNodeIDSet: true, ParentNodeID: &mid.ID,
		})
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("re-parent completed while an ancestor row was locked (err = %v); the ancestor walk is not taking row locks", err)
	case <-time.After(750 * time.Millisecond):
		// Still blocked, as required.
	}

	// Releasing the lock lets the blocked write finish.
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("release lock: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("re-parent after lock release: err = %v, want nil", err)
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

// TestPostgresStore_Integration_MalformedID_NotFoundNotInternalError exercises T-0008: a
// malformed (non-UUID) id given to GetNode, UpdateNode, VerifyNode, or RetireNode must produce
// the same stable not_found behavior as a well-formed but missing id — never a 500 leaking
// Postgres 22P02 driver text.
func TestPostgresStore_Integration_MalformedID_NotFoundNotInternalError(t *testing.T) {
	db := openTestDB(t)
	store := NewPostgresStore(db)
	ctx := context.Background()

	ids := map[string]string{
		"malformed":           "not-a-uuid",
		"well-formed missing": "00000000-0000-0000-0000-000000000000",
	}
	for name, id := range ids {
		t.Run("GetNode/"+name, func(t *testing.T) {
			if _, err := store.GetNode(ctx, id); !errors.Is(err, ErrNotFound) {
				t.Fatalf("GetNode(%q) err = %v, want ErrNotFound", id, err)
			}
		})
		t.Run("UpdateNode/"+name, func(t *testing.T) {
			label := "New label"
			if _, err := store.UpdateNode(ctx, id, UpdateInput{ActorID: "tester", Label: &label}); !errors.Is(err, ErrNotFound) {
				t.Fatalf("UpdateNode(%q) err = %v, want ErrNotFound", id, err)
			}
		})
		t.Run("VerifyNode/"+name, func(t *testing.T) {
			if _, err := store.VerifyNode(ctx, id, "tester"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("VerifyNode(%q) err = %v, want ErrNotFound", id, err)
			}
		})
		t.Run("RetireNode/"+name, func(t *testing.T) {
			if _, err := store.RetireNode(ctx, id, "tester"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("RetireNode(%q) err = %v, want ErrNotFound", id, err)
			}
		})
	}
}
