package question

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// These integration tests run against a real, disposable PostgreSQL instance that has had the
// migrations applied. They write immutable question_events rows and cannot clean up after
// themselves, so TEST_DATABASE_URL MUST point at the disposable postgres-test service in
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

var suffixCounter atomic.Int64

// randomSuffix is unique across runs, not just within one process: these tests seed rows into a
// database that persists across repeated `go test` invocations, and content_sources.source_url is
// unique.
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

func seedActiveTestSyllabus(t *testing.T, db *sql.DB) string {
	t.Helper()
	var subjectID, id string
	if err := db.QueryRow(`SELECT id FROM subjects WHERE name = 'Biology'`).Scan(&subjectID); err != nil {
		t.Fatalf("get Biology subject: %v", err)
	}
	suffix := randomSuffix()
	if err := db.QueryRow(`
		INSERT INTO syllabuses (board, syllabus_code, subject_id, qualification, track, display_name, status)
		VALUES ('Test Board', $1, $2, 'Test Qualification', $3, 'Question test syllabus', 'active')
		RETURNING id`, "QN-"+suffix, subjectID, "track-"+suffix,
	).Scan(&id); err != nil {
		t.Fatalf("seed active test syllabus: %v", err)
	}
	return id
}

// seedApprovedLinkedSource inserts an approved content source linked to syllabusID. Test
// scaffolding only: it carries no real rights metadata and no source material.
func seedApprovedLinkedSource(t *testing.T, db *sql.DB, syllabusID string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO content_sources (title, source_url, owner, source_hash, licence_reference, permitted_use, allowed_audience, status, catalogue_syllabus_id)
		VALUES ($1, $2, 'Test Owner', 'hash', 'licence', 'test use', 'test audience', 'approved', $3)
		RETURNING id`,
		"question test source", "https://example.org/question-test-"+randomSuffix(), syllabusID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed approved source: %v", err)
	}
	return id
}

// seedNode inserts a curriculum-map node directly (bypassing the curriculummap package, which is
// not this package's unit under test) with the requested lifecycle status.
func seedNode(t *testing.T, db *sql.DB, syllabusID, sourceID, status string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO curriculum_map_nodes (syllabus_id, node_kind, node_code, label, status, content_source_id)
		VALUES ($1, 'objective', $2, 'test node', $3, $4)
		RETURNING id`,
		syllabusID, "QN-"+randomSuffix(), status, sourceID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed curriculum map node: %v", err)
	}
	return id
}

// testRubric is an original, minimal rubric structure written by the test — never a mark scheme.
func testRubric() (json.RawMessage, int) {
	return json.RawMessage(`{"criteria":[{"id":"c1","marks":2},{"id":"c2","marks":1}]}`), 3
}

type fixture struct {
	db         *sql.DB
	store      *PostgresStore
	ctx        context.Context
	syllabusID string
	otherSyl   string
	sourceID   string
	nodeID     string
}

// newFixture builds a fully-grounded starting point: an active syllabus, an approved+linked
// source, and a verified node.
func newFixture(t *testing.T) fixture {
	t.Helper()
	db := openTestDB(t)
	syllabusID := getActiveSyllabusID(t, db, "0610")
	otherSyl := seedActiveTestSyllabus(t, db)
	sourceID := seedApprovedLinkedSource(t, db, syllabusID)
	return fixture{
		db:         db,
		store:      NewPostgresStore(db),
		ctx:        context.Background(),
		syllabusID: syllabusID,
		otherSyl:   otherSyl,
		sourceID:   sourceID,
		nodeID:     seedNode(t, db, syllabusID, sourceID, "verified"),
	}
}

func (f fixture) createQuestion(t *testing.T) Question {
	t.Helper()
	q, err := f.store.CreateQuestion(f.ctx, CreateInput{
		ActorID:             "test-editor",
		SyllabusID:          f.syllabusID,
		CurriculumMapNodeID: f.nodeID,
		ResponseType:        ResponseShortAnswer,
		Language:            "en",
		Prompt:              "Original prompt written by the test.",
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	return q
}

func TestPostgresStore_Integration_CreateAndNodeGate(t *testing.T) {
	f := newFixture(t)

	q := f.createQuestion(t)
	if q.Status != StatusDraft {
		t.Fatalf("status = %q, want draft", q.Status)
	}
	if q.SyllabusID != f.syllabusID || q.CurriculumMapNodeID != f.nodeID {
		t.Fatalf("grounding not persisted: %+v", q)
	}

	draftNode := seedNode(t, f.db, f.syllabusID, f.sourceID, "draft")
	retiredNode := seedNode(t, f.db, f.syllabusID, f.sourceID, "retired")
	otherSource := seedApprovedLinkedSource(t, f.db, f.otherSyl)
	otherSyllabusNode := seedNode(t, f.db, f.otherSyl, otherSource, "verified")

	for name, tc := range map[string]struct {
		nodeID  string
		wantErr error
	}{
		"draft node":          {draftNode, ErrUnverifiedNode},
		"retired node":        {retiredNode, ErrUnverifiedNode},
		"node other syllabus": {otherSyllabusNode, ErrMismatchedNode},
		"unknown node":        {"00000000-0000-0000-0000-000000000000", ErrUnknownNode},
		"malformed node id":   {"not-a-uuid", ErrUnknownNode},
		"empty node id":       {"", ErrUnknownNode},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.store.CreateQuestion(f.ctx, CreateInput{
				ActorID: "test-editor", SyllabusID: f.syllabusID, CurriculumMapNodeID: tc.nodeID,
				ResponseType: ResponseShortAnswer, Language: "en", Prompt: "p",
			}); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}

	if _, err := f.store.CreateQuestion(f.ctx, CreateInput{
		ActorID: "test-editor", SyllabusID: "00000000-0000-0000-0000-000000000000",
		CurriculumMapNodeID: f.nodeID, ResponseType: ResponseShortAnswer, Language: "en", Prompt: "p",
	}); !errors.Is(err, ErrUnknownSyllabus) {
		t.Fatalf("unknown syllabus: err = %v, want ErrUnknownSyllabus", err)
	}

	var eventCount int
	if err := f.db.QueryRow(`SELECT count(*) FROM question_events WHERE question_id = $1`, q.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
}

// --- Grounding gate re-validated on every write ---

// questionRowSnapshot is the persisted state a rejected write must leave byte-identical.
type questionRowSnapshot struct {
	status          string
	prompt          string
	language        string
	responseType    string
	nodeID          string
	contentRevision int
	updatedAt       time.Time
	eventCount      int
	rubricCount     int
}

func snapshotQuestionRow(t *testing.T, db *sql.DB, id string) questionRowSnapshot {
	t.Helper()
	var s questionRowSnapshot
	if err := db.QueryRow(
		`SELECT status, prompt, language, response_type, curriculum_map_node_id, content_revision, updated_at
		 FROM questions WHERE id = $1`, id,
	).Scan(&s.status, &s.prompt, &s.language, &s.responseType, &s.nodeID, &s.contentRevision, &s.updatedAt); err != nil {
		t.Fatalf("snapshot question row: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM question_events WHERE question_id = $1`, id).Scan(&s.eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM question_rubric_versions WHERE question_id = $1`, id).Scan(&s.rubricCount); err != nil {
		t.Fatalf("count rubric versions: %v", err)
	}
	return s
}

func assertQuestionRowUnchanged(t *testing.T, db *sql.DB, id string, before questionRowSnapshot) {
	t.Helper()
	after := snapshotQuestionRow(t, db, id)
	if after != before {
		t.Fatalf("question state changed by a rejected write:\n before = %+v\n after  = %+v", before, after)
	}
}

type groundingBreak struct {
	name    string
	apply   func(t *testing.T, f fixture)
	wantErr error
}

func groundingBreaks() []groundingBreak {
	return []groundingBreak{
		{
			name: "node retired after creation",
			apply: func(t *testing.T, f fixture) {
				setNodeStatus(t, f.db, f.nodeID, "retired")
			},
			wantErr: ErrUnverifiedNode,
		},
		{
			name: "node reverted to draft",
			apply: func(t *testing.T, f fixture) {
				setNodeStatus(t, f.db, f.nodeID, "draft")
			},
			wantErr: ErrUnverifiedNode,
		},
		{
			name: "source un-approved",
			apply: func(t *testing.T, f fixture) {
				setSourceState(t, f.db, f.sourceID, "pending", &f.syllabusID)
			},
			wantErr: ErrUnapprovedSource,
		},
		{
			name: "source unlinked",
			apply: func(t *testing.T, f fixture) {
				setSourceState(t, f.db, f.sourceID, "approved", nil)
			},
			wantErr: ErrUnlinkedSource,
		},
		{
			name: "source re-linked to another syllabus",
			apply: func(t *testing.T, f fixture) {
				setSourceState(t, f.db, f.sourceID, "approved", &f.otherSyl)
			},
			wantErr: ErrMismatchedSource,
		},
	}
}

func setNodeStatus(t *testing.T, db *sql.DB, nodeID, status string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE curriculum_map_nodes SET status = $1 WHERE id = $2`, status, nodeID); err != nil {
		t.Fatalf("set node status: %v", err)
	}
}

func setSourceState(t *testing.T, db *sql.DB, sourceID, status string, catalogueSyllabusID *string) {
	t.Helper()
	if _, err := db.Exec(
		`UPDATE content_sources SET status = $1, catalogue_syllabus_id = $2 WHERE id = $3`,
		status, catalogueSyllabusID, sourceID,
	); err != nil {
		t.Fatalf("break source state: %v", err)
	}
}

// runGroundingCases builds a fresh fixture per case (its own source and node, so breaking one
// case cannot affect another), optionally prepares the question, breaks the grounding, then runs
// action — which must fail with the expected stable error and write nothing.
func runGroundingCases(
	t *testing.T,
	prepare func(t *testing.T, f fixture, q Question),
	action func(f fixture, q Question) error,
) {
	t.Helper()
	for _, tc := range groundingBreaks() {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			q := f.createQuestion(t)
			if prepare != nil {
				prepare(t, f, q)
			}
			tc.apply(t, f)
			before := snapshotQuestionRow(t, f.db, q.ID)

			if err := action(f, q); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			assertQuestionRowUnchanged(t, f.db, q.ID, before)
		})
	}
}

func TestPostgresStore_Integration_UpdateRevalidatesGrounding(t *testing.T) {
	runGroundingCases(t, nil, func(f fixture, q Question) error {
		prompt := "A different original prompt."
		_, err := f.store.UpdateQuestion(f.ctx, q.ID, UpdateInput{ActorID: "test-editor", Prompt: &prompt})
		return err
	})
}

func TestPostgresStore_Integration_CreateRubricVersionRevalidatesGrounding(t *testing.T) {
	runGroundingCases(t, nil, func(f fixture, q Question) error {
		rubric, maxMarks := testRubric()
		_, err := f.store.CreateRubricVersion(f.ctx, q.ID, CreateRubricVersionInput{
			ActorID: "test-editor", Rubric: rubric, MaxMarks: maxMarks,
		})
		return err
	})
}

func TestPostgresStore_Integration_VerifyRubricVersionRevalidatesGrounding(t *testing.T) {
	runGroundingCases(t,
		func(t *testing.T, f fixture, q Question) {
			rubric, maxMarks := testRubric()
			if _, err := f.store.CreateRubricVersion(f.ctx, q.ID, CreateRubricVersionInput{
				ActorID: "test-editor", Rubric: rubric, MaxMarks: maxMarks,
			}); err != nil {
				t.Fatalf("prepare rubric version: %v", err)
			}
		},
		func(f fixture, q Question) error {
			_, err := f.store.VerifyRubricVersion(f.ctx, q.ID, 1, "test-reviewer")
			return err
		},
	)
}

func TestPostgresStore_Integration_VerifyQuestionRevalidatesGrounding(t *testing.T) {
	runGroundingCases(t,
		func(t *testing.T, f fixture, q Question) { verifyRubric(t, f, q) },
		func(f fixture, q Question) error {
			_, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer")
			return err
		},
	)
}

func TestPostgresStore_Integration_RetireQuestionRevalidatesGrounding(t *testing.T) {
	runGroundingCases(t, nil, func(f fixture, q Question) error {
		_, err := f.store.RetireQuestion(f.ctx, q.ID, "test-reviewer")
		return err
	})
}

func verifyRubric(t *testing.T, f fixture, q Question) RubricVersion {
	t.Helper()
	rubric, maxMarks := testRubric()
	v, err := f.store.CreateRubricVersion(f.ctx, q.ID, CreateRubricVersionInput{
		ActorID: "test-editor", Rubric: rubric, MaxMarks: maxMarks,
	})
	if err != nil {
		t.Fatalf("create rubric version: %v", err)
	}
	verified, err := f.store.VerifyRubricVersion(f.ctx, q.ID, v.Version, "test-reviewer")
	if err != nil {
		t.Fatalf("verify rubric version: %v", err)
	}
	return verified
}

// --- Rubric versions ---

func TestPostgresStore_Integration_RubricVersionsMonotonicAndUnique(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)
	rubric, maxMarks := testRubric()

	for want := 1; want <= 3; want++ {
		v, err := f.store.CreateRubricVersion(f.ctx, q.ID, CreateRubricVersionInput{
			ActorID: "test-editor", Rubric: rubric, MaxMarks: maxMarks,
		})
		if err != nil {
			t.Fatalf("create rubric version %d: %v", want, err)
		}
		if v.Version != want {
			t.Fatalf("version = %d, want %d", v.Version, want)
		}
		if v.Status != RubricDraft || v.CreatedBy != "test-editor" || v.ReviewedBy != nil {
			t.Fatalf("unexpected new version state: %+v", v)
		}
	}

	// The unique index is real: a duplicate (question, version) pair cannot be inserted even by
	// going around the allocator.
	if _, err := f.db.Exec(`
		INSERT INTO question_rubric_versions (question_id, version, rubric, max_marks, created_by)
		VALUES ($1, 1, $2, $3, 'test-editor')`, q.ID, []byte(rubric), maxMarks,
	); err == nil {
		t.Fatal("duplicate (question_id, version) insert succeeded, want unique-index rejection")
	}

	versions, err := f.store.ListRubricVersions(f.ctx, q.ID)
	if err != nil {
		t.Fatalf("list rubric versions: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("versions = %d, want 3", len(versions))
	}
	for i, v := range versions {
		if v.Version != i+1 {
			t.Fatalf("versions[%d].version = %d, want %d", i, v.Version, i+1)
		}
	}
}

// TestPostgresStore_Integration_RubricVersionContentImmutable proves the DB trigger — not just
// application code — refuses to rewrite a stored rubric version's content or delete it.
func TestPostgresStore_Integration_RubricVersionContentImmutable(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)
	v := verifyRubric(t, f, q)

	var versionID string
	if err := f.db.QueryRow(
		`SELECT id FROM question_rubric_versions WHERE question_id = $1 AND version = $2`, q.ID, v.Version,
	).Scan(&versionID); err != nil {
		t.Fatalf("find rubric version: %v", err)
	}

	for name, stmt := range map[string]string{
		"rewrite version number": `UPDATE question_rubric_versions SET version = 99 WHERE id = $1`,
		// Question verification reads question_revision, so a direct SQL rewrite would be a way
		// to re-point a stale rubric at the current question content and bypass the check.
		"rewrite question revision": `UPDATE question_rubric_versions SET question_revision = 99 WHERE id = $1`,
		"rewrite rubric":            `UPDATE question_rubric_versions SET rubric = '{"criteria":[{"id":"x","marks":1}]}' WHERE id = $1`,
		"rewrite max marks":         `UPDATE question_rubric_versions SET max_marks = 99 WHERE id = $1`,
		"rewrite question":          `UPDATE question_rubric_versions SET question_id = question_id WHERE id = $1`,
		"rewrite creator":           `UPDATE question_rubric_versions SET created_by = 'tampered' WHERE id = $1`,
		"delete":                    `DELETE FROM question_rubric_versions WHERE id = $1`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.db.Exec(stmt, versionID)
			if name == "rewrite question" {
				// Setting a column to its own value is not a content change and is allowed; the
				// trigger fires on a real change only.
				if err != nil {
					t.Fatalf("no-op update rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%q succeeded, want rejection by the immutability trigger", name)
			}
		})
	}
}

func TestPostgresStore_Integration_RubricValidationRejectedBeforeWrite(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)
	before := snapshotQuestionRow(t, f.db, q.ID)

	for name, tc := range map[string]struct {
		rubric   string
		maxMarks int
		wantErr  error
	}{
		"marks mismatch":  {`{"criteria":[{"id":"c1","marks":2}]}`, 5, ErrInvalidMaxMarks},
		"zero max marks":  {`{"criteria":[{"id":"c1","marks":2}]}`, 0, ErrInvalidMaxMarks},
		"empty criteria":  {`{"criteria":[]}`, 3, ErrInvalidRubric},
		"unknown field":   {`{"criteria":[{"id":"c1","marks":3}],"answers":1}`, 3, ErrInvalidRubric},
		"malformed json":  {`{"criteria":`, 3, ErrInvalidRubric},
		"duplicate ident": {`{"criteria":[{"id":"c1","marks":2},{"id":"c1","marks":1}]}`, 3, ErrInvalidRubric},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.store.CreateRubricVersion(f.ctx, q.ID, CreateRubricVersionInput{
				ActorID: "test-editor", Rubric: json.RawMessage(tc.rubric), MaxMarks: tc.maxMarks,
			}); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
	assertQuestionRowUnchanged(t, f.db, q.ID, before)
}

// --- Lifecycle ---

func TestPostgresStore_Integration_VerifyRequiresVerifiedRubric(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)

	before := snapshotQuestionRow(t, f.db, q.ID)
	if _, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer"); !errors.Is(err, ErrMissingVerifiedRubric) {
		t.Fatalf("no rubric: err = %v, want ErrMissingVerifiedRubric", err)
	}
	assertQuestionRowUnchanged(t, f.db, q.ID, before)

	rubric, maxMarks := testRubric()
	if _, err := f.store.CreateRubricVersion(f.ctx, q.ID, CreateRubricVersionInput{
		ActorID: "test-editor", Rubric: rubric, MaxMarks: maxMarks,
	}); err != nil {
		t.Fatalf("create rubric version: %v", err)
	}
	beforeDraftRubric := snapshotQuestionRow(t, f.db, q.ID)
	if _, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer"); !errors.Is(err, ErrMissingVerifiedRubric) {
		t.Fatalf("draft rubric only: err = %v, want ErrMissingVerifiedRubric", err)
	}
	assertQuestionRowUnchanged(t, f.db, q.ID, beforeDraftRubric)

	if _, err := f.store.VerifyRubricVersion(f.ctx, q.ID, 1, "test-reviewer"); err != nil {
		t.Fatalf("verify rubric version: %v", err)
	}
	verified, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer")
	if err != nil || verified.Status != StatusVerified {
		t.Fatalf("verify question: q=%+v err=%v", verified, err)
	}
}

// --- Question content revision ---

// TestPostgresStore_Integration_ContentRevisionIncrementsOnce pins the increment rule against a
// real database: one successful content update, one revision — and nothing else moves it.
func TestPostgresStore_Integration_ContentRevisionIncrementsOnce(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)
	if q.ContentRevision != 1 {
		t.Fatalf("new question contentRevision = %d, want 1", q.ContentRevision)
	}

	prompt := "A second original prompt."
	language := "fr"
	updated, err := f.store.UpdateQuestion(f.ctx, q.ID, UpdateInput{
		ActorID: "test-editor", Prompt: &prompt, Language: &language,
	})
	if err != nil {
		t.Fatalf("update question: %v", err)
	}
	if updated.ContentRevision != 2 {
		t.Fatalf("contentRevision after a two-field update = %d, want 2", updated.ContentRevision)
	}

	// A no-op update must not consume a revision.
	before := snapshotQuestionRow(t, f.db, q.ID)
	if _, err := f.store.UpdateQuestion(f.ctx, q.ID, UpdateInput{ActorID: "test-editor", Prompt: &prompt}); !errors.Is(err, ErrNoChanges) {
		t.Fatalf("no-op update: err = %v, want ErrNoChanges", err)
	}
	assertQuestionRowUnchanged(t, f.db, q.ID, before)

	// Neither must a rejected one.
	badNode := "00000000-0000-0000-0000-000000000000"
	if _, err := f.store.UpdateQuestion(f.ctx, q.ID, UpdateInput{ActorID: "test-editor", CurriculumMapNodeID: &badNode}); !errors.Is(err, ErrUnknownNode) {
		t.Fatalf("rejected update: err = %v, want ErrUnknownNode", err)
	}
	assertQuestionRowUnchanged(t, f.db, q.ID, before)

	// Verify and retire are not content changes.
	verifyRubric(t, f, q)
	verified, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer")
	if err != nil {
		t.Fatalf("verify question: %v", err)
	}
	if verified.ContentRevision != 2 {
		t.Fatalf("contentRevision after verify = %d, want 2", verified.ContentRevision)
	}
	retired, err := f.store.RetireQuestion(f.ctx, q.ID, "test-reviewer")
	if err != nil {
		t.Fatalf("retire question: %v", err)
	}
	if retired.ContentRevision != 2 {
		t.Fatalf("contentRevision after retire = %d, want 2", retired.ContentRevision)
	}
}

// TestPostgresStore_Integration_VerifyRequiresRubricForCurrentRevision is the review finding
// against a real database: a rubric verified before an edit must not authorise verification, and
// a new rubric at the current revision must.
func TestPostgresStore_Integration_VerifyRequiresRubricForCurrentRevision(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)

	stale := verifyRubric(t, f, q)
	if stale.QuestionRevision != 1 {
		t.Fatalf("first rubric questionRevision = %d, want 1", stale.QuestionRevision)
	}

	prompt := "A materially different original prompt."
	if _, err := f.store.UpdateQuestion(f.ctx, q.ID, UpdateInput{ActorID: "test-editor", Prompt: &prompt}); err != nil {
		t.Fatalf("update question: %v", err)
	}

	before := snapshotQuestionRow(t, f.db, q.ID)
	if _, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer"); !errors.Is(err, ErrStaleVerifiedRubric) {
		t.Fatalf("verify with a stale rubric: err = %v, want ErrStaleVerifiedRubric", err)
	}
	assertQuestionRowUnchanged(t, f.db, q.ID, before)

	// The stale version is untouched: still verified, still at revision 1, still readable.
	versions, err := f.store.ListRubricVersions(f.ctx, q.ID)
	if err != nil {
		t.Fatalf("list rubric versions: %v", err)
	}
	if len(versions) != 1 || versions[0].Status != RubricVerified || versions[0].QuestionRevision != 1 {
		t.Fatalf("stale version was altered: %+v", versions)
	}

	// Appending and verifying a rubric at the current revision unblocks verification.
	current := verifyRubric(t, f, q)
	if current.Version != 2 || current.QuestionRevision != 2 {
		t.Fatalf("new version = %+v, want version 2 at questionRevision 2", current)
	}
	verified, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer")
	if err != nil || verified.Status != StatusVerified {
		t.Fatalf("verify after a current rubric: q=%+v err=%v", verified, err)
	}
}

// TestPostgresStore_Integration_ConcurrentRubricAllocationAndRevision checks that adding a
// revision stamp did not weaken the row-locked allocator: concurrent writers still get distinct,
// monotonic version numbers, and every version carries a revision the question actually had.
func TestPostgresStore_Integration_ConcurrentRubricAllocationAndRevision(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)
	rubric, maxMarks := testRubric()

	const writers = 6
	var wg sync.WaitGroup
	results := make([]RubricVersion, writers)
	errs := make([]error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = f.store.CreateRubricVersion(f.ctx, q.ID, CreateRubricVersionInput{
				ActorID: "test-editor", Rubric: rubric, MaxMarks: maxMarks,
			})
		}(i)
	}
	wg.Wait()

	seen := map[int]bool{}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent create %d: %v", i, err)
		}
		v := results[i]
		if seen[v.Version] {
			t.Fatalf("version %d allocated twice", v.Version)
		}
		seen[v.Version] = true
		if v.QuestionRevision != 1 {
			t.Fatalf("version %d questionRevision = %d, want 1", v.Version, v.QuestionRevision)
		}
	}
	for want := 1; want <= writers; want++ {
		if !seen[want] {
			t.Fatalf("version %d was never allocated: %v", want, seen)
		}
	}

	// Now race edits against rubric creation. Whatever interleaving wins, every stored version
	// must carry a revision the question genuinely passed through, and the question's final
	// revision must equal 1 + the number of successful edits.
	const editors = 4
	edits := make([]error, editors)
	creates := make([]error, editors)
	for i := 0; i < editors; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			prompt := "Concurrent original prompt " + strconv.Itoa(i)
			_, edits[i] = f.store.UpdateQuestion(f.ctx, q.ID, UpdateInput{ActorID: "test-editor", Prompt: &prompt})
		}(i)
		go func(i int) {
			defer wg.Done()
			_, creates[i] = f.store.CreateRubricVersion(f.ctx, q.ID, CreateRubricVersionInput{
				ActorID: "test-editor", Rubric: rubric, MaxMarks: maxMarks,
			})
		}(i)
	}
	wg.Wait()

	successfulEdits := 0
	for _, err := range edits {
		if err == nil {
			successfulEdits++
			continue
		}
		// A same-value prompt is the only legitimate failure here.
		if !errors.Is(err, ErrNoChanges) {
			t.Fatalf("concurrent edit: %v", err)
		}
	}
	for _, err := range creates {
		if err != nil {
			t.Fatalf("concurrent rubric create during edits: %v", err)
		}
	}

	final, err := f.store.GetQuestion(f.ctx, q.ID, false)
	if err != nil {
		t.Fatalf("get question: %v", err)
	}
	if final.ContentRevision != 1+successfulEdits {
		t.Fatalf("contentRevision = %d, want %d (one per successful edit)", final.ContentRevision, 1+successfulEdits)
	}

	versions, err := f.store.ListRubricVersions(f.ctx, q.ID)
	if err != nil {
		t.Fatalf("list rubric versions: %v", err)
	}
	if len(versions) != writers+editors {
		t.Fatalf("versions = %d, want %d", len(versions), writers+editors)
	}
	for i, v := range versions {
		if v.Version != i+1 {
			t.Fatalf("versions[%d].version = %d, want %d", i, v.Version, i+1)
		}
		if v.QuestionRevision < 1 || v.QuestionRevision > final.ContentRevision {
			t.Fatalf("version %d carries questionRevision %d, outside 1..%d", v.Version, v.QuestionRevision, final.ContentRevision)
		}
	}
}

func TestPostgresStore_Integration_LifecycleAndAuditTrail(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)
	verifyRubric(t, f, q)

	if _, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer"); err != nil {
		t.Fatalf("verify question: %v", err)
	}
	prompt := "Changed prompt"
	if _, err := f.store.UpdateQuestion(f.ctx, q.ID, UpdateInput{ActorID: "test-editor", Prompt: &prompt}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("update verified question: err = %v, want ErrInvalidTransition", err)
	}
	if _, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("re-verify: err = %v, want ErrInvalidTransition", err)
	}

	retired, err := f.store.RetireQuestion(f.ctx, q.ID, "test-reviewer")
	if err != nil || retired.Status != StatusRetired {
		t.Fatalf("retire: q=%+v err=%v", retired, err)
	}
	if _, err := f.store.RetireQuestion(f.ctx, q.ID, "test-reviewer"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("re-retire: err = %v, want ErrInvalidTransition", err)
	}

	// A retired question is invisible to reader endpoints.
	if _, err := f.store.GetQuestion(f.ctx, q.ID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get retired verified-only: err = %v, want ErrNotFound", err)
	}
	listed, err := f.store.ListQuestions(f.ctx, f.syllabusID, &f.nodeID, true)
	if err != nil {
		t.Fatalf("list questions: %v", err)
	}
	for _, item := range listed {
		if item.ID == q.ID {
			t.Fatal("a retired question is still returned by the verified-only list")
		}
	}

	var eventTypes []string
	rows, err := f.db.Query(`SELECT event_type FROM question_events WHERE question_id = $1 ORDER BY created_at ASC, event_type ASC`, q.ID)
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
	want := []string{"question_created", "rubric_version_created", "rubric_version_verified", "question_verified", "question_retired"}
	if len(eventTypes) != len(want) {
		t.Fatalf("event types = %v, want %v", eventTypes, want)
	}
	for i, w := range want {
		if eventTypes[i] != w {
			t.Fatalf("event types = %v, want %v", eventTypes, want)
		}
	}
}

// TestPostgresStore_Integration_EventImmutability confirms question_events rows cannot be updated
// or deleted (the trigger rejects both).
func TestPostgresStore_Integration_EventImmutability(t *testing.T) {
	f := newFixture(t)
	q := f.createQuestion(t)

	var eventID string
	if err := f.db.QueryRow(`SELECT id FROM question_events WHERE question_id = $1 LIMIT 1`, q.ID).Scan(&eventID); err != nil {
		t.Fatalf("find event: %v", err)
	}
	if _, err := f.db.Exec(`UPDATE question_events SET actor_id = 'tampered' WHERE id = $1`, eventID); err == nil {
		t.Fatal("UPDATE on question_events succeeded, want rejection by trigger")
	}
	if _, err := f.db.Exec(`DELETE FROM question_events WHERE id = $1`, eventID); err == nil {
		t.Fatal("DELETE on question_events succeeded, want rejection by trigger")
	}
}

// TestPostgresStore_Integration_AuditRecordsNamesOnly asserts the audit trail stores field names
// and the verified actor — never a prompt, a rubric, or any other value.
func TestPostgresStore_Integration_AuditRecordsNamesOnly(t *testing.T) {
	f := newFixture(t)
	prompt := "A uniquely identifiable original prompt " + randomSuffix()
	q, err := f.store.CreateQuestion(f.ctx, CreateInput{
		ActorID: "test-editor", SyllabusID: f.syllabusID, CurriculumMapNodeID: f.nodeID,
		ResponseType: ResponseStructuredResponse, Language: "en", Prompt: prompt,
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	verifyRubric(t, f, q)

	rows, err := f.db.Query(`SELECT actor_id, changed_fields FROM question_events WHERE question_id = $1`, q.ID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	allowed := map[string]bool{
		"syllabusId": true, "curriculumMapNodeId": true, "responseType": true, "language": true,
		"prompt": true, "status": true, "contentRevision": true, "rubricVersion": true,
		"questionRevision": true, "rubricVersionStatus": true,
	}
	seenActors := 0
	for rows.Next() {
		var actor string
		var fields []byte
		if err := rows.Scan(&actor, &fields); err != nil {
			t.Fatalf("scan event: %v", err)
		}
		if actor != "test-editor" && actor != "test-reviewer" {
			t.Fatalf("actor = %q, want a verified subject", actor)
		}
		seenActors++
		for _, f := range parsePgTextArray(string(fields)) {
			if !allowed[f] {
				t.Fatalf("changed_fields contains %q — the audit trail must carry field NAMES only", f)
			}
		}
	}
	if seenActors == 0 {
		t.Fatal("no events found")
	}
}

// parsePgTextArray splits a Postgres text[] literal such as {a,b} into its elements.
func parsePgTextArray(raw string) []string {
	trimmed := strings.Trim(raw, "{}")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.Trim(p, `"`))
	}
	return out
}

// --- Listing ---

func TestPostgresStore_Integration_ListValidatesSyllabusAndFiltersNode(t *testing.T) {
	f := newFixture(t)

	if _, err := f.store.ListQuestions(f.ctx, "00000000-0000-0000-0000-000000000000", nil, true); !errors.Is(err, ErrUnknownSyllabus) {
		t.Fatalf("unknown syllabus: err = %v, want ErrUnknownSyllabus", err)
	}
	if _, err := f.store.ListQuestions(f.ctx, "not-a-uuid", nil, true); !errors.Is(err, ErrUnknownSyllabus) {
		t.Fatalf("malformed syllabus: err = %v, want ErrUnknownSyllabus", err)
	}

	q := f.createQuestion(t)
	verifyRubric(t, f, q)
	if _, err := f.store.VerifyQuestion(f.ctx, q.ID, "test-reviewer"); err != nil {
		t.Fatalf("verify question: %v", err)
	}

	matching, err := f.store.ListQuestions(f.ctx, f.syllabusID, &f.nodeID, true)
	if err != nil {
		t.Fatalf("list by node: %v", err)
	}
	if len(matching) != 1 || matching[0].ID != q.ID {
		t.Fatalf("node-filtered list = %+v, want exactly the verified question", matching)
	}

	otherNode := seedNode(t, f.db, f.syllabusID, f.sourceID, "verified")
	empty, err := f.store.ListQuestions(f.ctx, f.syllabusID, &otherNode, true)
	if err != nil {
		t.Fatalf("list by empty node: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty node filter returned %d items, want 0", len(empty))
	}
}

// TestPostgresStore_Integration_ListValidatesNodeFilter covers the review fix: an optional node
// filter is resolved against the curriculum map instead of being passed straight into the WHERE
// clause, where an unknown or foreign node produced an indistinguishable empty 200.
func TestPostgresStore_Integration_ListValidatesNodeFilter(t *testing.T) {
	f := newFixture(t)

	otherSource := seedApprovedLinkedSource(t, f.db, f.otherSyl)
	foreignNode := seedNode(t, f.db, f.otherSyl, otherSource, "verified")

	for name, tc := range map[string]struct {
		nodeID  string
		wantErr error
	}{
		"unknown node":      {"00000000-0000-0000-0000-000000000000", ErrUnknownNode},
		"malformed node id": {"not-a-uuid", ErrUnknownNode},
		"empty node id":     {"", ErrUnknownNode},
		"foreign node":      {foreignNode, ErrMismatchedNode},
	} {
		t.Run(name, func(t *testing.T) {
			node := tc.nodeID
			if _, err := f.store.ListQuestions(f.ctx, f.syllabusID, &node, true); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}

	// A real node of this syllabus with no verified questions is still 200-with-empty, not an
	// error — including a draft node, which a reader may legitimately filter by.
	draftNode := seedNode(t, f.db, f.syllabusID, f.sourceID, "draft")
	for name, node := range map[string]string{"verified node": f.nodeID, "draft node": draftNode} {
		t.Run(name+" with no questions", func(t *testing.T) {
			n := node
			items, err := f.store.ListQuestions(f.ctx, f.syllabusID, &n, true)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(items) != 0 {
				t.Fatalf("items = %d, want 0", len(items))
			}
		})
	}

	// The syllabus is still resolved first: a bad syllabus wins over the node filter.
	node := f.nodeID
	if _, err := f.store.ListQuestions(f.ctx, "not-a-uuid", &node, true); !errors.Is(err, ErrUnknownSyllabus) {
		t.Fatalf("unknown syllabus with a node filter: err = %v, want ErrUnknownSyllabus", err)
	}
}

func TestPostgresStore_Integration_UnknownQuestionIDs(t *testing.T) {
	f := newFixture(t)

	for name, id := range map[string]string{
		"unknown id":   "00000000-0000-0000-0000-000000000000",
		"malformed id": "not-a-uuid",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.store.GetQuestion(f.ctx, id, true); !errors.Is(err, ErrNotFound) {
				t.Fatalf("get: err = %v, want ErrNotFound", err)
			}
			if _, err := f.store.ListRubricVersions(f.ctx, id); !errors.Is(err, ErrNotFound) {
				t.Fatalf("list rubric versions: err = %v, want ErrNotFound", err)
			}
			prompt := "x"
			if _, err := f.store.UpdateQuestion(f.ctx, id, UpdateInput{ActorID: "a", Prompt: &prompt}); !errors.Is(err, ErrNotFound) {
				t.Fatalf("update: err = %v, want ErrNotFound", err)
			}
			if _, err := f.store.VerifyQuestion(f.ctx, id, "a"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("verify: err = %v, want ErrNotFound", err)
			}
		})
	}
}
