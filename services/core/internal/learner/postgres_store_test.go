package learner

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func seedPracticeQuestionWith(t *testing.T, f fixture, options []Option, raw json.RawMessage) (string, string) {
	t.Helper()
	questionID := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "verified", 7, "multiple_choice", options)
	var rubricID string
	if err := f.db.QueryRow(`
		INSERT INTO question_rubric_versions (question_id, version, question_revision, rubric, max_marks, status, created_by, reviewed_by)
		VALUES ($1, 1, 7, $2, 2, 'verified', 'opaque-editor', 'opaque-reviewer') RETURNING id
	`, questionID, raw).Scan(&rubricID); err != nil {
		t.Fatalf("seed practice rubric: %v", err)
	}
	setCanonicalRubric(t, f.db, questionID, rubricID)
	return questionID, rubricID
}

func seedPracticeQuestion(t *testing.T, f fixture) (string, string) {
	t.Helper()
	options := []Option{{ID: "opt-a", Label: "opaque-a"}, {ID: "opt-b", Label: "opaque-b"}, {ID: "opt-c", Label: "opaque-c"}}
	raw := json.RawMessage(`{"criteria":[{"id":"c1","marks":2}],"answerKey":{"correctOptionId":"opt-b"},"feedback":{"correctExplanation":"opaque-correct","incorrectExplanations":[{"optionId":"opt-a","explanation":"opaque-wrong-a"},{"optionId":"opt-c","explanation":"opaque-wrong-c"}]}}`)
	return seedPracticeQuestionWith(t, f, options, raw)
}

func seedLicensedPracticeQuestion(t *testing.T, f fixture, licensedSourceID string) string {
	t.Helper()
	options := []Option{{ID: "opt-a", Label: "opaque-a"}, {ID: "opt-b", Label: "opaque-b"}}
	encoded, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	var questionID string
	if err := f.db.QueryRow(`
		INSERT INTO questions
			(syllabus_id, curriculum_map_node_id, response_type, language, prompt, options,
			 status, content_revision, origin_type)
		VALUES ($1, $2, 'multiple_choice', 'en', $3, $4, 'verified', 1, 'licensed_adaptation')
		RETURNING id
	`, f.syllabusID, f.nodeID, "learner runtime prompt "+randomSuffix(), encoded).Scan(&questionID); err != nil {
		t.Fatalf("seed licensed question: %v", err)
	}
	if _, err := f.db.Exec(`
		INSERT INTO question_provenance
			(question_id, content_source_id, source_locator, origin_type, verified_actor_id)
		VALUES ($1, $2, $3, 'licensed_adaptation', 'opaque-editor')
	`, questionID, licensedSourceID, "metadata-locator-"+randomSuffix()); err != nil {
		t.Fatalf("seed licensed provenance: %v", err)
	}
	raw := json.RawMessage(`{"criteria":[{"id":"c1","marks":1}],"answerKey":{"correctOptionId":"opt-b"},"feedback":{"correctExplanation":"opaque-correct","incorrectExplanations":[{"optionId":"opt-a","explanation":"opaque-wrong"}]}}`)
	var rubricID string
	if err := f.db.QueryRow(`
		INSERT INTO question_rubric_versions
			(question_id, version, question_revision, rubric, max_marks, status, created_by, reviewed_by)
		VALUES ($1, 1, 1, $2, 1, 'verified', 'opaque-editor', 'opaque-reviewer') RETURNING id
	`, questionID, raw).Scan(&rubricID); err != nil {
		t.Fatalf("seed licensed rubric: %v", err)
	}
	setCanonicalRubric(t, f.db, questionID, rubricID)
	return questionID
}

// These integration tests run against a real, disposable PostgreSQL instance that has had the
// migrations applied. They write questions/rubric-version/curriculum-map/content-source rows
// (some immutable at the DB level) and cannot clean up after themselves, so TEST_DATABASE_URL
// MUST point at the disposable postgres-test service in docker-compose.test.yml — never the dev
// or prod database. Skipped unless it is set. This mirrors
// services/core/internal/question/postgres_store_test.go's fixture pattern exactly (D-0010
// precedent: small per-package test-scaffolding duplication over a shared cross-package helper).

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

var suffixCounter atomic.Int64

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

func seedSyllabus(t *testing.T, db *sql.DB, status string) string {
	t.Helper()
	var subjectID, id string
	if err := db.QueryRow(`SELECT id FROM subjects WHERE name = 'Biology'`).Scan(&subjectID); err != nil {
		t.Fatalf("get Biology subject: %v", err)
	}
	suffix := randomSuffix()
	if err := db.QueryRow(`
		INSERT INTO syllabuses (board, syllabus_code, subject_id, qualification, track, display_name, status)
		VALUES ('Test Board', $1, $2, 'Test Qualification', $3, 'Learner test syllabus', $4)
		RETURNING id`, "LN-"+suffix, subjectID, "track-"+suffix, status,
	).Scan(&id); err != nil {
		t.Fatalf("seed test syllabus: %v", err)
	}
	return id
}

func seedActiveTestSyllabus(t *testing.T, db *sql.DB) string {
	t.Helper()
	return seedSyllabus(t, db, "active")
}

// seedSource inserts a content source with the requested lifecycle status, optionally linked to
// a catalogue syllabus. Test scaffolding only: it carries no real rights metadata and no source
// material.
func seedSource(t *testing.T, db *sql.DB, status string, catalogueSyllabusID *string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO content_sources (title, source_url, owner, source_hash, licence_reference, permitted_use, allowed_audience, status, catalogue_syllabus_id)
		VALUES ($1, $2, 'Test Owner', 'hash', 'licence', 'test use', 'test audience', $3, $4)
		RETURNING id`,
		"learner test source", "https://example.org/learner-test-"+randomSuffix(), status, catalogueSyllabusID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed source: %v", err)
	}
	return id
}

func seedApprovedLinkedSource(t *testing.T, db *sql.DB, syllabusID string) string {
	t.Helper()
	return seedSource(t, db, "approved", &syllabusID)
}

// seedNode inserts a curriculum-map node directly (bypassing the curriculummap package, which is
// not this package's unit under test) with the requested lifecycle status.
func seedNode(t *testing.T, db *sql.DB, syllabusID, sourceID, status string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO curriculum_map_nodes (syllabus_id, node_kind, node_code, label, status, content_source_id)
		VALUES ($1, 'objective', $2, 'learner test node', $3, $4)
		RETURNING id`,
		syllabusID, "LN-"+randomSuffix(), status, sourceID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed curriculum map node: %v", err)
	}
	return id
}

// seedQuestion inserts a question row directly (bypassing the question package, which is not
// this package's unit under test) with the requested lifecycle status and content revision.
func seedQuestion(t *testing.T, db *sql.DB, syllabusID, nodeID, status string, contentRevision int, responseType string, options []Option) string {
	t.Helper()
	var optionsArg any
	if options != nil {
		encoded, err := json.Marshal(options)
		if err != nil {
			t.Fatalf("marshal options: %v", err)
		}
		optionsArg = encoded
	}
	var id string
	err := db.QueryRow(`
		INSERT INTO questions (syllabus_id, curriculum_map_node_id, response_type, language, prompt, options, status, content_revision)
		VALUES ($1, $2, $3, 'en', $4, $5, $6, $7)
		RETURNING id`,
		syllabusID, nodeID, responseType, "learner test prompt "+randomSuffix(), optionsArg, status, contentRevision,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed question: %v", err)
	}
	return id
}

// seedRubricVersion inserts a rubric version directly, stamped with the given questionRevision.
func seedRubricVersion(t *testing.T, db *sql.DB, questionID string, version, questionRevision int, status string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO question_rubric_versions (question_id, version, question_revision, rubric, max_marks, status, created_by)
		VALUES ($1, $2, $3, $4, 1, $5, 'test-reviewer')
		RETURNING id`,
		questionID, version, questionRevision, json.RawMessage(`{"criteria":[{"id":"c1","marks":1}]}`), status,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed rubric version: %v", err)
	}
	return id
}

func setCanonicalRubric(t *testing.T, db *sql.DB, questionID, rubricID string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE questions SET canonical_rubric_version_id = $1 WHERE id = $2`, rubricID, questionID); err != nil {
		t.Fatalf("set canonical rubric: %v", err)
	}
}

func setContentRevision(t *testing.T, db *sql.DB, questionID string, revision int) {
	t.Helper()
	if _, err := db.Exec(`UPDATE questions SET content_revision = $1 WHERE id = $2`, revision, questionID); err != nil {
		t.Fatalf("set content revision: %v", err)
	}
}

// seedEligibleQuestion builds a question that satisfies every learner eligibility gate: verified
// question, a verified canonical rubric stamped at the question's current content revision,
// running against the caller-supplied (already verified/approved-linked) node.
func seedEligibleQuestion(t *testing.T, db *sql.DB, syllabusID, nodeID string) string {
	t.Helper()
	qID := seedQuestion(t, db, syllabusID, nodeID, "verified", 1, "short_answer", nil)
	rubricID := seedRubricVersion(t, db, qID, 1, 1, "verified")
	setCanonicalRubric(t, db, qID, rubricID)
	return qID
}

type fixture struct {
	db         *sql.DB
	store      *PostgresStore
	ctx        context.Context
	syllabusID string
	sourceID   string
	nodeID     string
}

// newFixture builds a fully-grounded starting point: an active syllabus, an approved+linked
// source, and a verified node.
func newFixture(t *testing.T) fixture {
	t.Helper()
	db := openTestDB(t)
	syllabusID := getActiveSyllabusID(t, db, "0610")
	sourceID := seedApprovedLinkedSource(t, db, syllabusID)
	return fixture{
		db:         db,
		store:      NewPostgresStore(db),
		ctx:        context.Background(),
		syllabusID: syllabusID,
		sourceID:   sourceID,
		nodeID:     seedNode(t, db, syllabusID, sourceID, "verified"),
	}
}

// --- GetQuestion eligibility gate ---

func TestPostgresStore_Integration_GetQuestion_Eligible_ReturnsProjection(t *testing.T) {
	f := newFixture(t)
	options := []Option{{ID: "opt-a", Label: "opaque-a"}, {ID: "opt-b", Label: "opaque-b"}}
	qID := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "verified", 1, "multiple_choice", options)
	rubricID := seedRubricVersion(t, f.db, qID, 1, 1, "verified")
	setCanonicalRubric(t, f.db, qID, rubricID)

	got, err := f.store.GetQuestion(f.ctx, qID)
	if err != nil {
		t.Fatalf("GetQuestion: %v", err)
	}
	if got.ID != qID || got.SyllabusID != f.syllabusID || got.CurriculumMapNodeID != f.nodeID {
		t.Fatalf("projection identity mismatch: %+v", got)
	}
	if got.ResponseType != ResponseMultipleChoice || got.ContentRevision != 1 {
		t.Fatalf("projection content mismatch: %+v", got)
	}
	if len(got.Options) != 2 || got.Options[0].ID != "opt-a" || got.Options[1].Label != "opaque-b" {
		t.Fatalf("options round-trip mismatch: %+v", got.Options)
	}
}

func TestPostgresStore_Integration_GetQuestion_DraftQuestion_NotFound(t *testing.T) {
	f := newFixture(t)
	qID := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "draft", 1, "short_answer", nil)
	rubricID := seedRubricVersion(t, f.db, qID, 1, 1, "verified")
	setCanonicalRubric(t, f.db, qID, rubricID)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_RetiredQuestion_NotFound(t *testing.T) {
	f := newFixture(t)
	qID := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "retired", 1, "short_answer", nil)
	rubricID := seedRubricVersion(t, f.db, qID, 1, 1, "verified")
	setCanonicalRubric(t, f.db, qID, rubricID)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_NoCanonicalRubric_NotFound(t *testing.T) {
	f := newFixture(t)
	qID := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "verified", 1, "short_answer", nil)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_CanonicalRubricDraft_NotFound(t *testing.T) {
	f := newFixture(t)
	qID := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "verified", 1, "short_answer", nil)
	rubricID := seedRubricVersion(t, f.db, qID, 1, 1, "draft")
	setCanonicalRubric(t, f.db, qID, rubricID)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_CanonicalRubricStale_NotFound(t *testing.T) {
	f := newFixture(t)
	qID := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "verified", 1, "short_answer", nil)
	rubricID := seedRubricVersion(t, f.db, qID, 1, 1, "verified")
	setCanonicalRubric(t, f.db, qID, rubricID)
	// The question was edited after the rubric was verified: content_revision moved to 2, but
	// the canonical rubric is still stamped at revision 1 — stale, must not be delivered.
	setContentRevision(t, f.db, qID, 2)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_NodeNotVerified_NotFound(t *testing.T) {
	f := newFixture(t)
	draftNode := seedNode(t, f.db, f.syllabusID, f.sourceID, "draft")
	qID := seedEligibleQuestion(t, f.db, f.syllabusID, draftNode)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_NodeRetired_NotFound(t *testing.T) {
	f := newFixture(t)
	retiredNode := seedNode(t, f.db, f.syllabusID, f.sourceID, "retired")
	qID := seedEligibleQuestion(t, f.db, f.syllabusID, retiredNode)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_SourceNotApproved_NotFound(t *testing.T) {
	f := newFixture(t)
	pendingSource := seedSource(t, f.db, "pending", &f.syllabusID)
	node := seedNode(t, f.db, f.syllabusID, pendingSource, "verified")
	qID := seedEligibleQuestion(t, f.db, f.syllabusID, node)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_SourceUnlinked_NotFound(t *testing.T) {
	f := newFixture(t)
	unlinkedSource := seedSource(t, f.db, "approved", nil)
	node := seedNode(t, f.db, f.syllabusID, unlinkedSource, "verified")
	qID := seedEligibleQuestion(t, f.db, f.syllabusID, node)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_SourceMismatchedSyllabus_NotFound(t *testing.T) {
	f := newFixture(t)
	otherSyl := seedActiveTestSyllabus(t, f.db)
	mismatchedSource := seedSource(t, f.db, "approved", &otherSyl)
	node := seedNode(t, f.db, f.syllabusID, mismatchedSource, "verified")
	qID := seedEligibleQuestion(t, f.db, f.syllabusID, node)

	if _, err := f.store.GetQuestion(f.ctx, qID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_MalformedID_NotFound(t *testing.T) {
	f := newFixture(t)
	if _, err := f.store.GetQuestion(f.ctx, "not-a-uuid"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_GetQuestion_UnknownID_NotFound(t *testing.T) {
	f := newFixture(t)
	if _, err := f.store.GetQuestion(f.ctx, "00000000-0000-0000-0000-000000000000"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStore_Integration_LicensedSourceRegressionExcludesDeliveryAndAttempt(t *testing.T) {
	tests := map[string]func(*testing.T, fixture, string){
		"expired": func(t *testing.T, f fixture, id string) {
			if _, err := f.db.Exec(`UPDATE content_sources SET status='expired' WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
		},
		"rejected": func(t *testing.T, f fixture, id string) {
			if _, err := f.db.Exec(`UPDATE content_sources SET status='rejected' WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
		},
		"unlinked": func(t *testing.T, f fixture, id string) {
			if _, err := f.db.Exec(`UPDATE content_sources SET catalogue_syllabus_id=NULL WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
		},
		"rights incomplete": func(t *testing.T, f fixture, id string) {
			if _, err := f.db.Exec(`UPDATE content_sources SET licence_reference=NULL WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
		},
		"foreign link": func(t *testing.T, f fixture, id string) {
			other := seedActiveTestSyllabus(t, f.db)
			if _, err := f.db.Exec(`UPDATE content_sources SET catalogue_syllabus_id=$1 WHERE id=$2`, other, id); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			licensedSourceID := seedApprovedLinkedSource(t, f.db, f.syllabusID)
			questionID := seedLicensedPracticeQuestion(t, f, licensedSourceID)
			if _, err := f.store.GetQuestion(f.ctx, questionID); err != nil {
				t.Fatalf("licensed question should start eligible: %v", err)
			}
			attempt, err := f.store.CreateAttempt(f.ctx, "owner-a", questionID)
			if err != nil {
				t.Fatalf("licensed attempt should start eligible: %v", err)
			}
			mutate(t, f, licensedSourceID)
			if _, err := f.store.GetQuestion(f.ctx, questionID); err != ErrNotFound {
				t.Fatalf("GetQuestion err=%v want ErrNotFound", err)
			}
			items, err := f.store.ListQuestions(f.ctx, f.syllabusID, nil)
			if err != nil {
				t.Fatalf("ListQuestions: %v", err)
			}
			for _, item := range items {
				if item.ID == questionID {
					t.Fatal("regressed licensed question leaked through list")
				}
			}
			if _, err := f.store.CreateAttempt(f.ctx, "owner-a", questionID); err != ErrNotFound {
				t.Fatalf("CreateAttempt err=%v want ErrNotFound", err)
			}
			if _, err := f.store.SubmitAttempt(f.ctx, "owner-a", attempt.AttemptID, "opt-b"); err != ErrAttemptNotFound {
				t.Fatalf("SubmitAttempt err=%v want ErrAttemptNotFound", err)
			}
		})
	}
}

// --- Canonical rubric ownership gate (T-0015 review fix) ---

// TestPostgresStore_Integration_CanonicalRubric_OwnershipGate proves the eligibility query
// requires rv.question_id = q.id, not merely rv.id = q.canonical_rubric_version_id. Before this
// fix, pointing question A's canonical_rubric_version_id at a rubric row that actually belongs to
// a different, unrelated question B (same content revision, verified) made A eligible purely
// because a row existed at that id — a learner could read a question whose canonical rubric was
// never verified/selected for it. The gate must reject that: A stays not-found, and B (the real
// owner) is unaffected.
func TestPostgresStore_Integration_CanonicalRubric_OwnershipGate(t *testing.T) {
	f := newFixture(t)
	qA := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "verified", 1, "short_answer", nil)
	qB := seedEligibleQuestion(t, f.db, f.syllabusID, f.nodeID)

	var rubricBID string
	if err := f.db.QueryRow(
		`SELECT canonical_rubric_version_id FROM questions WHERE id = $1`, qB,
	).Scan(&rubricBID); err != nil {
		t.Fatalf("read qB canonical rubric id: %v", err)
	}
	// Disposable-test direct setup: point A's canonical rubric at B's verified, current-revision
	// rubric row. A real editorial workflow can never produce this state (canonical selection is
	// scoped to the question's own rubric versions) — this simulates it directly to prove the
	// read-time gate, not the write-time workflow, is what protects a learner here.
	setCanonicalRubric(t, f.db, qA, rubricBID)

	if _, err := f.store.GetQuestion(f.ctx, qA); err != ErrNotFound {
		t.Fatalf("GetQuestion(qA) err = %v, want ErrNotFound (canonical rubric belongs to a different question)", err)
	}

	items, err := f.store.ListQuestions(f.ctx, f.syllabusID, nil)
	if err != nil {
		t.Fatalf("ListQuestions: %v", err)
	}
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if ids[qA] {
		t.Fatalf("qA leaked into learner list despite rubric-ownership mismatch: %v", ids)
	}
	if !ids[qB] {
		t.Fatalf("qB (legitimate rubric owner) missing from learner list: %v", ids)
	}

	if got, err := f.store.GetQuestion(f.ctx, qB); err != nil || got.ID != qB {
		t.Fatalf("GetQuestion(qB) = %+v, %v; want qB eligible and unaffected", got, err)
	}
}

// --- ListActiveSyllabuses ---

func TestPostgresStore_Integration_ListActiveSyllabuses_OnlyActiveReturnedWithSafeProjection(t *testing.T) {
	f := newFixture(t)
	active := seedSyllabus(t, f.db, "active")
	draft := seedSyllabus(t, f.db, "draft")
	retired := seedSyllabus(t, f.db, "retired")

	items, err := f.store.ListActiveSyllabuses(f.ctx)
	if err != nil {
		t.Fatalf("ListActiveSyllabuses: %v", err)
	}
	byID := map[string]Syllabus{}
	for _, s := range items {
		byID[s.ID] = s
	}
	if _, ok := byID[active]; !ok {
		t.Fatalf("active syllabus %s missing from result: %v", active, byID)
	}
	if _, ok := byID[draft]; ok {
		t.Fatalf("draft syllabus %s leaked into active-only result", draft)
	}
	if _, ok := byID[retired]; ok {
		t.Fatalf("retired syllabus %s leaked into active-only result", retired)
	}

	got := byID[active]
	if got.Board == "" || got.SyllabusCode == "" || got.Qualification == "" || got.DisplayName == "" {
		t.Fatalf("active syllabus projection missing expected fields: %+v", got)
	}
}

// --- ListQuestions ---

func TestPostgresStore_Integration_ListQuestions_UnknownSyllabus_Error(t *testing.T) {
	f := newFixture(t)
	if _, err := f.store.ListQuestions(f.ctx, "00000000-0000-0000-0000-000000000000", nil); err != ErrUnknownSyllabus {
		t.Fatalf("err = %v, want ErrUnknownSyllabus", err)
	}
}

func TestPostgresStore_Integration_ListQuestions_MalformedSyllabus_Error(t *testing.T) {
	f := newFixture(t)
	if _, err := f.store.ListQuestions(f.ctx, "not-a-uuid", nil); err != ErrUnknownSyllabus {
		t.Fatalf("err = %v, want ErrUnknownSyllabus", err)
	}
}

func TestPostgresStore_Integration_ListQuestions_InactiveSyllabus_Error(t *testing.T) {
	f := newFixture(t)
	draftSyllabus := seedSyllabus(t, f.db, "draft")
	if _, err := f.store.ListQuestions(f.ctx, draftSyllabus, nil); err != ErrUnknownSyllabus {
		t.Fatalf("err = %v, want ErrUnknownSyllabus", err)
	}
}

func TestPostgresStore_Integration_ListQuestions_UnknownNodeFilter_Error(t *testing.T) {
	f := newFixture(t)
	unknown := "00000000-0000-0000-0000-000000000000"
	if _, err := f.store.ListQuestions(f.ctx, f.syllabusID, &unknown); err != ErrUnknownNode {
		t.Fatalf("err = %v, want ErrUnknownNode", err)
	}
}

func TestPostgresStore_Integration_ListQuestions_MalformedNodeFilter_Error(t *testing.T) {
	f := newFixture(t)
	malformed := "not-a-uuid"
	if _, err := f.store.ListQuestions(f.ctx, f.syllabusID, &malformed); err != ErrUnknownNode {
		t.Fatalf("err = %v, want ErrUnknownNode", err)
	}
}

func TestPostgresStore_Integration_ListQuestions_MismatchedNodeFilter_Error(t *testing.T) {
	f := newFixture(t)
	otherSyl := seedActiveTestSyllabus(t, f.db)
	otherSource := seedApprovedLinkedSource(t, f.db, otherSyl)
	otherNode := seedNode(t, f.db, otherSyl, otherSource, "verified")

	if _, err := f.store.ListQuestions(f.ctx, f.syllabusID, &otherNode); err != ErrMismatchedNode {
		t.Fatalf("err = %v, want ErrMismatchedNode", err)
	}
}

func TestPostgresStore_Integration_ListQuestions_EmptySyllabus_ReturnsEmptyList(t *testing.T) {
	f := newFixture(t)
	empty := seedActiveTestSyllabus(t, f.db)

	items, err := f.store.ListQuestions(f.ctx, empty, nil)
	if err != nil {
		t.Fatalf("ListQuestions: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %d, want 0", len(items))
	}
}

func TestPostgresStore_Integration_ListQuestions_OnlyEligibleQuestionsReturned(t *testing.T) {
	f := newFixture(t)
	eligible := seedEligibleQuestion(t, f.db, f.syllabusID, f.nodeID)
	draft := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "draft", 1, "short_answer", nil)
	retired := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "retired", 1, "short_answer", nil)
	noRubric := seedQuestion(t, f.db, f.syllabusID, f.nodeID, "verified", 1, "short_answer", nil)
	_ = draft
	_ = retired
	_ = noRubric

	items, err := f.store.ListQuestions(f.ctx, f.syllabusID, nil)
	if err != nil {
		t.Fatalf("ListQuestions: %v", err)
	}
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[eligible] {
		t.Fatalf("expected eligible question %s in results: %v", eligible, ids)
	}
	if ids[draft] || ids[retired] || ids[noRubric] {
		t.Fatalf("ineligible questions leaked into results: %v", ids)
	}
}

func TestPostgresStore_Integration_AttemptPinsOwnershipAndSubmit(t *testing.T) {
	f := newFixture(t)
	questionID, rubricID := seedPracticeQuestion(t, f)
	attempt, err := f.store.CreateAttempt(f.ctx, "owner-a", questionID)
	if err != nil {
		t.Fatalf("CreateAttempt: %v", err)
	}
	if attempt.QuestionID != questionID || attempt.Status != AttemptOpen || attempt.MaxMarks != 2 {
		t.Fatalf("attempt = %+v", attempt)
	}
	var owner, pinnedRubric, selected string
	var revision int
	var isCorrect *bool
	if err := f.db.QueryRow(`SELECT learner_subject_id, question_content_revision, canonical_rubric_version_id,
		COALESCE(selected_option_id,''), is_correct FROM learner_attempts WHERE id=$1`, attempt.AttemptID).
		Scan(&owner, &revision, &pinnedRubric, &selected, &isCorrect); err != nil {
		t.Fatal(err)
	}
	if owner != "owner-a" || revision != 7 || pinnedRubric != rubricID || selected != "" || isCorrect != nil {
		t.Fatalf("pins/pre-submit = %s %d %s %q %v", owner, revision, pinnedRubric, selected, isCorrect)
	}
	if _, err := f.store.SubmitAttempt(f.ctx, "owner-b", attempt.AttemptID, "opt-b"); err != ErrAttemptNotFound {
		t.Fatalf("foreign = %v", err)
	}
	if _, err := f.store.SubmitAttempt(f.ctx, "owner-a", attempt.AttemptID, "stale"); err != ErrInvalidOption {
		t.Fatalf("stale = %v", err)
	}
	result, err := f.store.SubmitAttempt(f.ctx, "owner-a", attempt.AttemptID, "opt-b")
	if err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}
	if !result.IsCorrect || result.AwardedMarks != 2 || result.CorrectOptionID != "opt-b" || len(result.Feedback.IncorrectExplanations) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if _, err := f.store.SubmitAttempt(f.ctx, "owner-a", attempt.AttemptID, "opt-a"); err != ErrAttemptSubmitted {
		t.Fatalf("replay = %v", err)
	}
	var eventCount int
	if err := f.db.QueryRow(`SELECT count(*) FROM learner_attempt_events WHERE attempt_id=$1`, attempt.AttemptID).Scan(&eventCount); err != nil || eventCount != 2 {
		t.Fatalf("events=%d err=%v", eventCount, err)
	}
}

func assertNoAttemptOrEventForQuestion(t *testing.T, db *sql.DB, questionID string) {
	t.Helper()
	var attempts, events int
	if err := db.QueryRow(`SELECT count(*) FROM learner_attempts WHERE question_id=$1`, questionID).Scan(&attempts); err != nil {
		t.Fatalf("count learner attempts: %v", err)
	}
	if err := db.QueryRow(`
		SELECT count(*) FROM learner_attempt_events e
		JOIN learner_attempts a ON a.id=e.attempt_id
		WHERE a.question_id=$1`, questionID).Scan(&events); err != nil {
		t.Fatalf("count learner attempt events: %v", err)
	}
	if attempts != 0 || events != 0 {
		t.Fatalf("attempts=%d events=%d, want no mutation", attempts, events)
	}
}

func TestPostgresStore_Integration_CreateAttemptRejectsDuplicateWrongExplanationAndMissingOption(t *testing.T) {
	f := newFixture(t)
	options := []Option{{ID: "opt-a", Label: "opaque-a"}, {ID: "opt-b", Label: "opaque-b"}, {ID: "opt-c", Label: "opaque-c"}}
	corrupt := json.RawMessage(`{"criteria":[{"id":"c1","marks":2}],"answerKey":{"correctOptionId":"opt-b"},"feedback":{"correctExplanation":"opaque-correct","incorrectExplanations":[{"optionId":"opt-a","explanation":"opaque-wrong-a"},{"optionId":"opt-a","explanation":"opaque-wrong-a-duplicate"}]}}`)
	questionID, _ := seedPracticeQuestionWith(t, f, options, corrupt)

	if _, err := f.store.CreateAttempt(f.ctx, "owner-a", questionID); err != ErrNotFound {
		t.Fatalf("CreateAttempt err=%v, want ErrNotFound", err)
	}
	assertNoAttemptOrEventForQuestion(t, f.db, questionID)
}

func TestPostgresStore_Integration_CreateAttemptRejectsDuplicateCurrentOptionID(t *testing.T) {
	f := newFixture(t)
	options := []Option{{ID: "opt-a", Label: "opaque-a"}, {ID: "opt-b", Label: "opaque-b"}, {ID: "opt-a", Label: "opaque-duplicate"}}
	raw := json.RawMessage(`{"criteria":[{"id":"c1","marks":2}],"answerKey":{"correctOptionId":"opt-b"},"feedback":{"correctExplanation":"opaque-correct","incorrectExplanations":[{"optionId":"opt-a","explanation":"opaque-wrong-a"},{"optionId":"opt-a","explanation":"opaque-wrong-a-duplicate"}]}}`)
	questionID, _ := seedPracticeQuestionWith(t, f, options, raw)

	if _, err := f.store.CreateAttempt(f.ctx, "owner-a", questionID); err != ErrNotFound {
		t.Fatalf("CreateAttempt err=%v, want ErrNotFound", err)
	}
	assertNoAttemptOrEventForQuestion(t, f.db, questionID)
}

func TestPostgresStore_Integration_AttemptEligibilityAndPinnedLifecycle(t *testing.T) {
	f := newFixture(t)
	questionID, _ := seedPracticeQuestion(t, f)
	attempt, err := f.store.CreateAttempt(f.ctx, "owner-a", questionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE questions SET status='retired', content_revision=8 WHERE id=$1`, questionID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE content_sources SET status='pending' WHERE id=$1`, f.sourceID); err != nil {
		t.Fatal(err)
	}
	result, err := f.store.SubmitAttempt(f.ctx, "owner-a", attempt.AttemptID, "opt-a")
	if err != nil || result.IsCorrect || result.AwardedMarks != 0 {
		t.Fatalf("pinned submit = %+v, %v", result, err)
	}
	if _, err := f.store.CreateAttempt(f.ctx, "owner-a", questionID); err != ErrNotFound {
		t.Fatalf("ineligible create = %v", err)
	}
}

func TestPostgresStore_Integration_ParallelSubmitExactlyOnce(t *testing.T) {
	f := newFixture(t)
	questionID, _ := seedPracticeQuestion(t, f)
	attempt, err := f.store.CreateAttempt(f.ctx, "owner-a", questionID)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, errs[index] = f.store.SubmitAttempt(context.Background(), "owner-a", attempt.AttemptID, "opt-b")
		}(i)
	}
	wg.Wait()
	success, conflict := 0, 0
	for _, submitErr := range errs {
		if submitErr == nil {
			success++
		}
		if submitErr == ErrAttemptSubmitted {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("parallel errors = %#v", errs)
	}
}

func TestPostgresStore_Integration_ListQuestions_NodeFilterNarrowsResults(t *testing.T) {
	f := newFixture(t)
	otherNode := seedNode(t, f.db, f.syllabusID, f.sourceID, "verified")
	inNode := seedEligibleQuestion(t, f.db, f.syllabusID, f.nodeID)
	inOtherNode := seedEligibleQuestion(t, f.db, f.syllabusID, otherNode)

	items, err := f.store.ListQuestions(f.ctx, f.syllabusID, &f.nodeID)
	if err != nil {
		t.Fatalf("ListQuestions: %v", err)
	}
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[inNode] {
		t.Fatalf("expected %s (in filtered node) present: %v", inNode, ids)
	}
	if ids[inOtherNode] {
		t.Fatalf("did not expect %s (different node) present: %v", inOtherNode, ids)
	}
}
