package teacher

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"strings"
	"time"
)

type PostgresStore struct{ db *sql.DB }

func NewPostgresStore(db *sql.DB) *PostgresStore { return &PostgresStore{db} }
func invalidID(err error) bool                   { var e *pq.Error; return errors.As(err, &e) && e.Code == "22P02" }
func hashToken(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func (p *PostgresStore) CreateClass(ctx context.Context, owner, name string) (Class, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 1 || len([]rune(name)) > 120 {
		return Class{}, ErrInvalidAssignment
	}
	var c Class
	e := p.db.QueryRowContext(ctx, `INSERT INTO teacher_classes(owner_subject_id,name) VALUES($1,$2) RETURNING id,name,status,created_at`, owner, name).Scan(&c.ID, &c.Name, &c.Status, &c.CreatedAt)
	if e != nil {
		return c, e
	}
	_, e = p.db.ExecContext(ctx, `INSERT INTO teacher_class_events(class_id,event_type,actor_id,changed_fields) VALUES($1,'class_created',$2,ARRAY['name'])`, c.ID, owner)
	return c, e
}
func (p *PostgresStore) ListClasses(ctx context.Context, owner string) ([]Class, error) {
	rows, e := p.db.QueryContext(ctx, `SELECT id,name,status,created_at FROM teacher_classes WHERE owner_subject_id=$1 ORDER BY created_at DESC`, owner)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Class
	for rows.Next() {
		var c Class
		if e = rows.Scan(&c.ID, &c.Name, &c.Status, &c.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (p *PostgresStore) ownClass(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, owner, id string) (bool, error) {
	var x int
	e := q.QueryRowContext(ctx, `SELECT 1 FROM teacher_classes WHERE id=$1 AND owner_subject_id=$2 AND status='active'`, id, owner).Scan(&x)
	if errors.Is(e, sql.ErrNoRows) || invalidID(e) {
		return false, nil
	}
	return e == nil, e
}
func (p *PostgresStore) CreateInvite(ctx context.Context, owner, classID string, ttl time.Duration, max int) (Invite, error) {
	if ttl < time.Minute || ttl > 30*24*time.Hour || max < 1 || max > 100 {
		return Invite{}, ErrInvalidAssignment
	}
	ok, e := p.ownClass(ctx, p.db, owner, classID)
	if e != nil {
		return Invite{}, e
	}
	if !ok {
		return Invite{}, ErrNotFound
	}
	token, e := newToken()
	if e != nil {
		return Invite{}, e
	}
	out := Invite{Token: token, ExpiresAt: time.Now().UTC().Add(ttl), MaxUses: max}
	var id string
	e = p.db.QueryRowContext(ctx, `INSERT INTO teacher_class_invites(class_id,token_hash,expires_at,max_uses) VALUES($1,$2,$3,$4) RETURNING id`, classID, hashToken(token), out.ExpiresAt, max).Scan(&id)
	if e != nil {
		return Invite{}, e
	}
	_, e = p.db.ExecContext(ctx, `INSERT INTO teacher_class_events(class_id,event_type,actor_id,changed_fields) VALUES($1,'invite_created',$2,ARRAY['expiresAt','maxUses'])`, classID, owner)
	return out, e
}
func (p *PostgresStore) Roster(ctx context.Context, owner, classID string) ([]RosterMember, error) {
	ok, e := p.ownClass(ctx, p.db, owner, classID)
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, ErrNotFound
	}
	rows, e := p.db.QueryContext(ctx, `SELECT id,status,consented_at FROM teacher_class_memberships WHERE class_id=$1 ORDER BY consented_at DESC`, classID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []RosterMember
	for rows.Next() {
		var m RosterMember
		if e = rows.Scan(&m.MembershipID, &m.Status, &m.ConsentedAt); e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (p *PostgresStore) AcceptInvite(ctx context.Context, learner, token string) (Class, error) {
	token = strings.TrimSpace(token)
	if len(token) < 20 || len(token) > 200 {
		return Class{}, ErrInviteUnavailable
	}
	tx, e := p.db.BeginTx(ctx, nil)
	if e != nil {
		return Class{}, e
	}
	defer tx.Rollback()
	var invite, classID string
	var expires time.Time
	var uses, max int
	var c Class
	e = tx.QueryRowContext(ctx, `SELECT i.id,i.class_id,i.expires_at,i.accepted_uses,i.max_uses,c.name,c.status,c.created_at FROM teacher_class_invites i JOIN teacher_classes c ON c.id=i.class_id WHERE i.token_hash=$1 FOR UPDATE OF i`, hashToken(token)).Scan(&invite, &classID, &expires, &uses, &max, &c.Name, &c.Status, &c.CreatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return Class{}, ErrInviteUnavailable
	}
	if e != nil {
		return Class{}, e
	}
	if !expires.After(time.Now()) || uses >= max || c.Status != "active" {
		return Class{}, ErrInviteUnavailable
	}
	c.ID = classID
	var member string
	var status string
	e = tx.QueryRowContext(ctx, `SELECT id,status FROM teacher_class_memberships WHERE class_id=$1 AND learner_subject_id=$2 FOR UPDATE`, classID, learner).Scan(&member, &status)
	if e == nil && status == "active" {
		if e = tx.Commit(); e != nil {
			return Class{}, e
		}
		return c, nil
	}
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return Class{}, e
	}
	if member == "" {
		e = tx.QueryRowContext(ctx, `INSERT INTO teacher_class_memberships(class_id,learner_subject_id,status,consented_at) VALUES($1,$2,'active',now()) RETURNING id`, classID, learner).Scan(&member)
	} else {
		_, e = tx.ExecContext(ctx, `UPDATE teacher_class_memberships SET status='active',consented_at=now(),revoked_at=NULL,updated_at=now() WHERE id=$1`, member)
	}
	if e != nil {
		return Class{}, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE teacher_class_invites SET accepted_uses=accepted_uses+1 WHERE id=$1`, invite); e != nil {
		return Class{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO teacher_class_events(class_id,membership_id,event_type,actor_id,changed_fields) VALUES($1,$2,'membership_accepted',$3,ARRAY['status','consentedAt'])`, classID, member, learner); e != nil {
		return Class{}, e
	}
	return c, tx.Commit()
}
func (p *PostgresStore) RevokeMembership(ctx context.Context, learner, classID string) error {
	r, e := p.db.ExecContext(ctx, `UPDATE teacher_class_memberships SET status='revoked',revoked_at=now(),updated_at=now() WHERE class_id=$1 AND learner_subject_id=$2 AND status='active'`, classID, learner)
	if invalidID(e) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrNotFound
	}
	_, e = p.db.ExecContext(ctx, `INSERT INTO teacher_class_events(class_id,event_type,actor_id,changed_fields) VALUES($1,'membership_revoked',$2,ARRAY['status','revokedAt'])`, classID, learner)
	return e
}

// eligible assignment snapshot query mirrors the live learner gate, but snapshots only safe delivery fields.
const assignmentQuestion = `SELECT q.id,q.response_type,q.language,q.prompt,q.options,q.content_revision FROM questions q JOIN curriculum_map_nodes n ON n.id=q.curriculum_map_node_id JOIN content_sources s ON s.id=n.content_source_id JOIN question_rubric_versions rv ON rv.id=q.canonical_rubric_version_id AND rv.question_id=q.id WHERE q.id=$1 AND q.syllabus_id=$2 AND q.curriculum_map_node_id=$3 AND q.status='verified' AND rv.status='verified' AND rv.question_revision=q.content_revision AND n.status='verified' AND s.status='approved' AND s.catalogue_syllabus_id=q.syllabus_id`

func (p *PostgresStore) CreateAssignment(ctx context.Context, owner, classID string, in CreateAssignmentInput) (Assignment, error) {
	in.Title = strings.TrimSpace(in.Title)
	if len([]rune(in.Title)) < 1 || len([]rune(in.Title)) > 160 || len(in.QuestionIDs) < 1 || len(in.QuestionIDs) > 50 || (in.MarkingMode != "" && in.MarkingMode != MarkingAutomated && in.MarkingMode != MarkingManualTeacher) {
		return Assignment{}, ErrInvalidAssignment
	}
	if in.MarkingMode == "" {
		in.MarkingMode = MarkingAutomated
	}
	seen := map[string]bool{}
	for _, id := range in.QuestionIDs {
		if id == "" || seen[id] {
			return Assignment{}, ErrInvalidAssignment
		}
		seen[id] = true
	}
	tx, e := p.db.BeginTx(ctx, nil)
	if e != nil {
		return Assignment{}, e
	}
	defer tx.Rollback()
	ok, e := p.ownClass(ctx, tx, owner, classID)
	if e != nil {
		return Assignment{}, e
	}
	if !ok {
		return Assignment{}, ErrNotFound
	}
	items := make([]AssignmentItem, 0, len(in.QuestionIDs))
	for _, qid := range in.QuestionIDs {
		var it AssignmentItem
		var options []byte
		e = tx.QueryRowContext(ctx, assignmentQuestion, qid, in.SyllabusID, in.ModuleID).Scan(&it.ID, &it.ResponseType, &it.Language, &it.Prompt, &options, &it.ContentRevision)
		if errors.Is(e, sql.ErrNoRows) || invalidID(e) {
			return Assignment{}, ErrInvalidAssignment
		}
		if e != nil {
			return Assignment{}, e
		}
		if json.Unmarshal(options, &it.Options) != nil {
			return Assignment{}, ErrInvalidAssignment
		}
		items = append(items, it)
	}
	settings, _ := json.Marshal(map[string]any{"questionCount": len(items), "markingMode": in.MarkingMode})
	var out Assignment
	e = tx.QueryRowContext(ctx, `INSERT INTO teacher_assignments(class_id,owner_subject_id,title,syllabus_id,module_id,marking_mode,settings_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,class_id,title,marking_mode,created_at`, classID, owner, in.Title, in.SyllabusID, in.ModuleID, in.MarkingMode, settings).Scan(&out.ID, &out.ClassID, &out.Title, &out.MarkingMode, &out.CreatedAt)
	if e != nil {
		return Assignment{}, e
	}
	out.ItemCount = len(items)
	for i, it := range items {
		raw, _ := json.Marshal(it)
		if _, e = tx.ExecContext(ctx, `INSERT INTO teacher_assignment_items(assignment_id,ordinal,question_id,snapshot) VALUES($1,$2,$3,$4)`, out.ID, i+1, it.ID, raw); e != nil {
			return Assignment{}, e
		}
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO teacher_assignment_events(assignment_id,actor_id,event_type,changed_fields) VALUES($1,$2,'assignment_created',ARRAY['title','moduleId','questionSelection','markingMode'])`, out.ID, owner); e != nil {
		return Assignment{}, e
	}
	return out, tx.Commit()
}
func (p *PostgresStore) ListAssignments(ctx context.Context, owner, classID string) ([]Assignment, error) {
	ok, e := p.ownClass(ctx, p.db, owner, classID)
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, ErrNotFound
	}
	rows, e := p.db.QueryContext(ctx, `SELECT a.id,a.class_id,a.title,a.marking_mode,a.created_at,(SELECT count(*) FROM teacher_assignment_items i WHERE i.assignment_id=a.id),(SELECT count(*) FROM teacher_assignment_runs r WHERE r.assignment_id=a.id),(SELECT count(*) FROM teacher_class_memberships m WHERE m.class_id=a.class_id AND m.status='active') FROM teacher_assignments a WHERE a.class_id=$1 ORDER BY a.created_at DESC`, classID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Assignment
	for rows.Next() {
		var a Assignment
		if e = rows.Scan(&a.ID, &a.ClassID, &a.Title, &a.MarkingMode, &a.CreatedAt, &a.ItemCount, &a.StartedCount, &a.ActiveMemberCount); e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (p *PostgresStore) ListLearnerAssignments(ctx context.Context, learner string) ([]LearnerAssignment, error) {
	rows, e := p.db.QueryContext(ctx, `SELECT a.id,c.name,a.title,a.marking_mode,(SELECT count(*) FROM teacher_assignment_items i WHERE i.assignment_id=a.id),CASE WHEN r.id IS NULL THEN 'not_started' ELSE r.status END FROM teacher_assignments a JOIN teacher_classes c ON c.id=a.class_id JOIN teacher_class_memberships m ON m.class_id=c.id AND m.learner_subject_id=$1 AND m.status='active' LEFT JOIN teacher_assignment_runs r ON r.assignment_id=a.id AND r.learner_subject_id=$1 WHERE c.status='active' ORDER BY a.created_at DESC`, learner)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []LearnerAssignment
	for rows.Next() {
		var a LearnerAssignment
		if e = rows.Scan(&a.ID, &a.ClassName, &a.Title, &a.MarkingMode, &a.ItemCount, &a.State); e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (p *PostgresStore) StartAssignment(ctx context.Context, learner, id string) (StartedAssignment, error) {
	tx, e := p.db.BeginTx(ctx, nil)
	if e != nil {
		return StartedAssignment{}, e
	}
	defer tx.Rollback()
	var out StartedAssignment
	e = tx.QueryRowContext(ctx, `SELECT a.id,a.marking_mode FROM teacher_assignments a JOIN teacher_classes c ON c.id=a.class_id JOIN teacher_class_memberships m ON m.class_id=c.id AND m.learner_subject_id=$2 AND m.status='active' WHERE a.id=$1 AND c.status='active' FOR UPDATE OF a`, id, learner).Scan(&out.AssignmentID, &out.MarkingMode)
	if errors.Is(e, sql.ErrNoRows) || invalidID(e) {
		return out, ErrNotFound
	}
	if e != nil {
		return out, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO teacher_assignment_runs(assignment_id,learner_subject_id) VALUES($1,$2) ON CONFLICT(assignment_id,learner_subject_id) DO NOTHING`, id, learner)
	if e != nil {
		return out, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO teacher_assignment_events(assignment_id,actor_id,event_type,changed_fields) VALUES($1,$2,'assignment_started',ARRAY['status']) ON CONFLICT DO NOTHING`, id, learner)
	if e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT snapshot FROM teacher_assignment_items WHERE assignment_id=$1 ORDER BY ordinal`, id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var it AssignmentItem
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if json.Unmarshal(raw, &it) != nil {
			return out, fmt.Errorf("decode assignment snapshot")
		}
		out.Items = append(out.Items, it)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	return out, tx.Commit()
}
