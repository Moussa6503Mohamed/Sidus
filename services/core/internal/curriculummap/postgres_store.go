package curriculummap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

// maxParentHops bounds the ancestor walk used for cycle detection so a corrupted/very deep
// hierarchy fails safely instead of looping forever.
const maxParentHops = 1000

// PostgresStore is a Store backed by PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore wraps an open *sql.DB as a Store.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

const nodeColumns = `id, syllabus_id, parent_node_id, node_kind, node_code, label, status, content_source_id, source_locator, created_at, updated_at`

func scanNode(row interface{ Scan(...any) error }) (Node, error) {
	var n Node
	err := row.Scan(
		&n.ID, &n.SyllabusID, &n.ParentNodeID, &n.NodeKind, &n.NodeCode, &n.Label, &n.Status,
		&n.ContentSourceID, &n.SourceLocator, &n.CreatedAt, &n.UpdatedAt,
	)
	return n, err
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// syllabusIsActive reports whether id resolves to a known, active catalogue syllabus. An id
// that is not even a well-formed UUID is "not known" rather than an infrastructure failure, so
// a malformed id maps to the same stable unknown_syllabus response as a missing one.
func syllabusIsActive(ctx context.Context, q queryRower, id string) (bool, error) {
	var status string
	err := q.QueryRowContext(ctx, `SELECT status FROM syllabuses WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check syllabus: %w", err)
	}
	return status == "active", nil
}

// isInvalidTextRepresentation reports whether err is Postgres 22P02 (e.g. a non-UUID string
// supplied where a UUID column is compared).
func isInvalidTextRepresentation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "22P02"
}

// validateSourceGate is the sole enforcement point for the curriculum-map source gate: the
// referenced content source must exist, be approved, and be linked to exactly the target
// syllabus.
func validateSourceGate(ctx context.Context, q queryRower, sourceID, syllabusID string) error {
	var status string
	var catalogueSyllabusID *string
	err := q.QueryRowContext(ctx,
		`SELECT status, catalogue_syllabus_id FROM content_sources WHERE id = $1`, sourceID,
	).Scan(&status, &catalogueSyllabusID)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return ErrUnknownSource
	}
	if err != nil {
		return fmt.Errorf("check content source: %w", err)
	}
	if status != "approved" {
		return ErrUnapprovedSource
	}
	if catalogueSyllabusID == nil {
		return ErrUnlinkedSource
	}
	if *catalogueSyllabusID != syllabusID {
		return ErrMismatchedSource
	}
	return nil
}

// validateParent locks and checks a candidate parent: it must exist, belong to syllabusID, and
// its ancestor chain must not reach selfID (cycle). selfID is "" for a brand-new node, for
// which a cycle is impossible.
//
// Every ancestor traversed is row-locked (`SELECT ... FOR UPDATE`) in the caller's transaction,
// not only the candidate parent: an unlocked walk could observe a chain that a concurrent
// transaction is in the middle of re-parenting and admit a cycle. Locks are held until the
// caller's transaction ends. Two transactions re-parenting overlapping chains in opposite
// orders can therefore deadlock; Postgres aborts one of them, which surfaces as a generic
// `500 internal_error` (never a partial write), and the caller may retry.
func validateParent(ctx context.Context, tx *sql.Tx, parentID, syllabusID, selfID string) error {
	var parentSyllabusID string
	err := tx.QueryRowContext(ctx,
		`SELECT syllabus_id FROM curriculum_map_nodes WHERE id = $1 FOR UPDATE`, parentID,
	).Scan(&parentSyllabusID)
	if errors.Is(err, sql.ErrNoRows) || isInvalidTextRepresentation(err) {
		return ErrInvalidParent
	}
	if err != nil {
		return fmt.Errorf("lock parent node: %w", err)
	}
	if parentSyllabusID != syllabusID {
		return ErrInvalidParent
	}
	if selfID == "" {
		return nil
	}

	current := parentID
	for hops := 0; hops < maxParentHops; hops++ {
		if current == selfID {
			return ErrInvalidParent
		}
		var next sql.NullString
		err := tx.QueryRowContext(ctx,
			`SELECT parent_node_id FROM curriculum_map_nodes WHERE id = $1 FOR UPDATE`, current,
		).Scan(&next)
		if errors.Is(err, sql.ErrNoRows) {
			// A dangling ancestor cannot exist behind the FK, but treat it as an invalid parent
			// rather than leaking an infrastructure error.
			return ErrInvalidParent
		}
		if err != nil {
			return fmt.Errorf("walk parent chain: %w", err)
		}
		if !next.Valid {
			return nil
		}
		current = next.String
	}
	return ErrInvalidParent
}

func (p *PostgresStore) CreateNode(ctx context.Context, in CreateInput) (Node, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	active, err := syllabusIsActive(ctx, tx, in.SyllabusID)
	if err != nil {
		return Node{}, err
	}
	if !active {
		return Node{}, ErrUnknownSyllabus
	}

	if in.ParentNodeID != nil {
		if err := validateParent(ctx, tx, *in.ParentNodeID, in.SyllabusID, ""); err != nil {
			return Node{}, err
		}
	}

	if err := validateSourceGate(ctx, tx, in.ContentSourceID, in.SyllabusID); err != nil {
		return Node{}, err
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO curriculum_map_nodes (syllabus_id, parent_node_id, node_kind, node_code, label, content_source_id, source_locator)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+nodeColumns,
		in.SyllabusID, in.ParentNodeID, in.NodeKind, in.NodeCode, in.Label, in.ContentSourceID, in.SourceLocator,
	)
	node, err := scanNode(row)
	if err != nil {
		if mapped := mapNodeWriteErr(err); mapped != nil {
			return Node{}, mapped
		}
		return Node{}, fmt.Errorf("insert node: %w", err)
	}

	changed := []string{"syllabusId", "nodeKind", "nodeCode", "label", "contentSourceId"}
	if in.ParentNodeID != nil {
		changed = append(changed, "parentNodeId")
	}
	if in.SourceLocator != nil {
		changed = append(changed, "sourceLocator")
	}
	if err := insertEvent(ctx, tx, node.ID, EventNodeCreated, in.ActorID, changed); err != nil {
		return Node{}, err
	}

	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit: %w", err)
	}
	return node, nil
}

func (p *PostgresStore) UpdateNode(ctx context.Context, id string, in UpdateInput) (Node, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT `+nodeColumns+` FROM curriculum_map_nodes WHERE id = $1 FOR UPDATE`, id)
	current, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Node{}, ErrNotFound
	}
	if err != nil {
		return Node{}, fmt.Errorf("lock node: %w", err)
	}
	if current.Status != StatusDraft {
		return Node{}, ErrInvalidTransition
	}

	// Every node write must reference an approved source linked to the node's syllabus — not
	// only writes that change contentSourceId. A source approved at creation can later be
	// un-approved, unlinked, or re-linked to another syllabus, so the gate is re-run here
	// against the effective source (the supplied one if any, otherwise the stored one) before
	// any node or event write.
	effectiveSourceID := current.ContentSourceID
	if in.ContentSourceID != nil {
		effectiveSourceID = *in.ContentSourceID
	}
	if err := validateSourceGate(ctx, tx, effectiveSourceID, current.SyllabusID); err != nil {
		return Node{}, err
	}

	var setClauses []string
	var args []any
	var changed []string

	addString := func(field, column string, value *string, cur string) error {
		if value == nil {
			return nil
		}
		if *value == cur {
			return nil
		}
		args = append(args, *value)
		setClauses = append(setClauses, column+" = $"+strconv.Itoa(len(args)))
		changed = append(changed, field)
		return nil
	}

	if err := addString("nodeCode", "node_code", in.NodeCode, current.NodeCode); err != nil {
		return Node{}, err
	}
	if err := addString("label", "label", in.Label, current.Label); err != nil {
		return Node{}, err
	}
	if in.NodeKind != nil && *in.NodeKind != current.NodeKind {
		args = append(args, string(*in.NodeKind))
		setClauses = append(setClauses, "node_kind = $"+strconv.Itoa(len(args)))
		changed = append(changed, "nodeKind")
	}

	if in.ContentSourceID != nil && *in.ContentSourceID != current.ContentSourceID {
		// Already gated above as the effective source.
		args = append(args, *in.ContentSourceID)
		setClauses = append(setClauses, "content_source_id = $"+strconv.Itoa(len(args)))
		changed = append(changed, "contentSourceId")
	}

	if in.ParentNodeIDSet {
		samePtr := (in.ParentNodeID == nil && current.ParentNodeID == nil) ||
			(in.ParentNodeID != nil && current.ParentNodeID != nil && *in.ParentNodeID == *current.ParentNodeID)
		if !samePtr {
			if in.ParentNodeID != nil {
				if *in.ParentNodeID == id {
					return Node{}, ErrInvalidParent
				}
				if err := validateParent(ctx, tx, *in.ParentNodeID, current.SyllabusID, id); err != nil {
					return Node{}, err
				}
			}
			args = append(args, in.ParentNodeID)
			setClauses = append(setClauses, "parent_node_id = $"+strconv.Itoa(len(args)))
			changed = append(changed, "parentNodeId")
		}
	}

	if in.SourceLocatorSet {
		samePtr := (in.SourceLocator == nil && current.SourceLocator == nil) ||
			(in.SourceLocator != nil && current.SourceLocator != nil && *in.SourceLocator == *current.SourceLocator)
		if !samePtr {
			args = append(args, in.SourceLocator)
			setClauses = append(setClauses, "source_locator = $"+strconv.Itoa(len(args)))
			changed = append(changed, "sourceLocator")
		}
	}

	if len(changed) == 0 {
		return Node{}, ErrNoChanges
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, id)
	query := `UPDATE curriculum_map_nodes SET ` + strings.Join(setClauses, ", ") +
		` WHERE id = $` + strconv.Itoa(len(args)) + ` RETURNING ` + nodeColumns

	row = tx.QueryRowContext(ctx, query, args...)
	updated, err := scanNode(row)
	if err != nil {
		if mapped := mapNodeWriteErr(err); mapped != nil {
			return Node{}, mapped
		}
		return Node{}, fmt.Errorf("update node: %w", err)
	}

	if err := insertEvent(ctx, tx, id, EventNodeUpdated, in.ActorID, changed); err != nil {
		return Node{}, err
	}

	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

func (p *PostgresStore) VerifyNode(ctx context.Context, id string, actorID string) (Node, error) {
	return p.transitionStatus(ctx, id, actorID, EventNodeVerified, StatusVerified, []Status{StatusDraft})
}

func (p *PostgresStore) RetireNode(ctx context.Context, id string, actorID string) (Node, error) {
	return p.transitionStatus(ctx, id, actorID, EventNodeRetired, StatusRetired, []Status{StatusDraft, StatusVerified})
}

func (p *PostgresStore) transitionStatus(ctx context.Context, id, actorID string, eventType EventType, next Status, allowedFrom []Status) (Node, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT `+nodeColumns+` FROM curriculum_map_nodes WHERE id = $1 FOR UPDATE`, id)
	current, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Node{}, ErrNotFound
	}
	if err != nil {
		return Node{}, fmt.Errorf("lock node: %w", err)
	}

	allowed := false
	for _, s := range allowedFrom {
		if current.Status == s {
			allowed = true
			break
		}
	}
	if !allowed {
		return Node{}, ErrInvalidTransition
	}

	// Verify/retire are node writes too: re-run the source gate against the node's stored
	// source before the status update and its event, so a node whose source has since become
	// unknown/unapproved/unlinked/mismatched cannot change lifecycle state.
	if err := validateSourceGate(ctx, tx, current.ContentSourceID, current.SyllabusID); err != nil {
		return Node{}, err
	}

	row = tx.QueryRowContext(ctx,
		`UPDATE curriculum_map_nodes SET status = $1, updated_at = now() WHERE id = $2 RETURNING `+nodeColumns,
		string(next), id,
	)
	updated, err := scanNode(row)
	if err != nil {
		return Node{}, fmt.Errorf("update node status: %w", err)
	}

	if err := insertEvent(ctx, tx, id, eventType, actorID, []string{"status"}); err != nil {
		return Node{}, err
	}

	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

func (p *PostgresStore) GetNode(ctx context.Context, id string, verifiedOnly bool) (Node, error) {
	query := `SELECT ` + nodeColumns + ` FROM curriculum_map_nodes WHERE id = $1`
	if verifiedOnly {
		query += ` AND status = 'verified'`
	}
	row := p.db.QueryRowContext(ctx, query, id)
	node, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Node{}, ErrNotFound
	}
	if err != nil {
		return Node{}, fmt.Errorf("get node: %w", err)
	}
	return node, nil
}

func (p *PostgresStore) ListNodesBySyllabus(ctx context.Context, syllabusID string, verifiedOnly bool) ([]Node, error) {
	// An unknown or inactive syllabus is a bad request, not an empty map: returning 200 with an
	// empty list would make a typo'd/retired syllabus id indistinguishable from a real syllabus
	// that has no verified nodes yet.
	active, err := syllabusIsActive(ctx, p.db, syllabusID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrUnknownSyllabus
	}

	query := `SELECT ` + nodeColumns + ` FROM curriculum_map_nodes WHERE syllabus_id = $1`
	if verifiedOnly {
		query += ` AND status = 'verified'`
	}
	query += ` ORDER BY node_code ASC`

	rows, err := p.db.QueryContext(ctx, query, syllabusID)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	nodes := []Node{}
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func insertEvent(ctx context.Context, tx *sql.Tx, nodeID string, eventType EventType, actor string, changed []string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO curriculum_map_events (node_id, event_type, actor_id, changed_fields)
		VALUES ($1, $2, $3, $4)`,
		nodeID, eventType, actor, pq.Array(changed),
	); err != nil {
		return fmt.Errorf("insert curriculum map event: %w", err)
	}
	return nil
}

// mapNodeWriteErr translates a Postgres constraint violation on a node write into a domain
// error, or returns nil if err is not a recognized constraint violation.
func mapNodeWriteErr(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return nil
	}
	switch pqErr.Code {
	case "23505": // unique_violation → duplicate (syllabus_id, node_code)
		return ErrDuplicateNodeCode
	case "23503": // foreign_key_violation → syllabus/parent/source no longer resolves
		return ErrInvalidParent
	default:
		return nil
	}
}
