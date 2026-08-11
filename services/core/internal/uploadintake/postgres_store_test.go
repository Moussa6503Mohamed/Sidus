package uploadintake

import (
	"context"
	"database/sql"
	"errors"
	_ "github.com/lib/pq"
	"os"
	"strconv"
	"testing"
	"time"
)

// Uses disposable sidus-test only. Events are intentionally immutable and are never cleaned up.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	d := os.Getenv("TEST_DATABASE_URL")
	if d == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, e := sql.Open("postgres", d)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
func TestPostgresStore_Integration_UploadLifecycleAndAudit(t *testing.T) {
	db := testDB(t)
	s := NewPostgresStore(db)
	ctx := context.Background()
	ref := strconv.FormatInt(time.Now().UnixNano(), 16)
	for len(ref) < 32 {
		ref = "0" + ref
	}
	ref = ref[:32]
	v, e := s.Create(ctx, CreateInput{ObjectRef: ref, OriginalFilename: "private.pdf", MediaType: "application/pdf", ByteSize: 8, SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ActorID: "admin"})
	if e != nil {
		t.Fatal(e)
	}
	if v.Status != StatusQuarantined {
		t.Fatalf("status=%s", v.Status)
	}
	v, e = s.MarkScanClean(ctx, v.ID, "admin")
	if e != nil || v.Status != StatusScanClean {
		t.Fatalf("scan clean %+v %v", v, e)
	}
	v, e = s.RequestDeletion(ctx, v.ID, "admin")
	if e != nil || v.Status != StatusDeletionRequested || v.RetentionState != "deletion_requested" {
		t.Fatalf("delete request %+v %v", v, e)
	}
	var eventID string
	if e = db.QueryRow(`SELECT id FROM private_upload_events WHERE upload_id=$1 LIMIT 1`, v.ID).Scan(&eventID); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`UPDATE private_upload_events SET actor_id='x' WHERE id=$1`, eventID); e == nil {
		t.Fatal("immutable event updated")
	}
	if _, e = s.MarkScanClean(ctx, "not-a-uuid", "admin"); !errors.Is(e, ErrNotFound) {
		t.Fatalf("bad id=%v", e)
	}
}
