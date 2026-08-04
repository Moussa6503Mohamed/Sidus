// Package curriculummap implements metadata-only curriculum-map infrastructure: nodes that
// will later carry topic maps, learning objectives, practical skills, and assessment rules for
// a syllabus, plus an immutable audit trail of node mutations. It never stores syllabus text,
// objective wording, topic labels, assessment text, questions, mark schemes, or any other
// derivative content — only editorial structure/identity metadata, and a placeholder label
// field for a future private, approved authoring workflow.
//
// Every node must reference an approved content_sources row whose catalogue_syllabus_id
// matches the node's syllabus. This package is the sole authority for that source gate: no
// other package (and no AI service) may create or verify a curriculum-map node.
package curriculummap

import (
	"strings"
	"time"
)

// NodeKind is the kind of curriculum-map node.
type NodeKind string

const (
	KindTopic          NodeKind = "topic"
	KindObjective      NodeKind = "objective"
	KindPracticalSkill NodeKind = "practical_skill"
	KindAssessmentRule NodeKind = "assessment_rule"
)

// IsValidKind reports whether k is a known node kind.
func IsValidKind(k NodeKind) bool {
	switch k {
	case KindTopic, KindObjective, KindPracticalSkill, KindAssessmentRule:
		return true
	default:
		return false
	}
}

// Status is the lifecycle state of a curriculum-map node.
type Status string

const (
	// StatusDraft is a node being authored; only its editor (or another editor/reviewer/admin)
	// may still change it, and it is not returned by reader endpoints.
	StatusDraft Status = "draft"
	// StatusVerified is a reviewer/admin-confirmed node; visible to readers.
	StatusVerified Status = "verified"
	// StatusRetired is a withdrawn node; kept for history, not resolvable as a new parent.
	StatusRetired Status = "retired"
)

// IsValidStatus reports whether s is a known lifecycle status.
func IsValidStatus(s Status) bool {
	switch s {
	case StatusDraft, StatusVerified, StatusRetired:
		return true
	default:
		return false
	}
}

// Node is a curriculum-map record: hierarchy position, editorial identity, lifecycle status,
// and the approved content-source it is grounded in. It never carries syllabus/objective/
// assessment text.
type Node struct {
	ID              string    `json:"id"`
	SyllabusID      string    `json:"syllabusId"`
	ParentNodeID    *string   `json:"parentNodeId"`
	NodeKind        NodeKind  `json:"nodeKind"`
	NodeCode        string    `json:"nodeCode"`
	Label           string    `json:"label"`
	Status          Status    `json:"status"`
	ContentSourceID string    `json:"contentSourceId"`
	SourceLocator   *string   `json:"sourceLocator"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// EventType is the kind of change recorded in a curriculum-map audit event.
type EventType string

const (
	EventNodeCreated  EventType = "node_created"
	EventNodeUpdated  EventType = "node_updated"
	EventNodeVerified EventType = "node_verified"
	EventNodeRetired  EventType = "node_retired"
)

// Event is an immutable audit record of a curriculum-map node mutation. It records which
// fields changed (names only) and who changed them (the verified Clerk subject) — never field
// values, and never any source material.
type Event struct {
	ID            string    `json:"id"`
	NodeID        string    `json:"nodeId"`
	EventType     EventType `json:"eventType"`
	ActorID       string    `json:"actorId"`
	EventTime     time.Time `json:"eventTime"`
	ChangedFields []string  `json:"changedFields"`
	CreatedAt     time.Time `json:"createdAt"`
}

// CreateInput is the payload for creating a draft Node.
type CreateInput struct {
	ActorID         string
	SyllabusID      string
	ParentNodeID    *string
	NodeKind        NodeKind
	NodeCode        string
	Label           string
	ContentSourceID string
	SourceLocator   *string
}

// UpdateInput is the payload for PATCHing a draft Node. Every field is an optional pointer:
// nil means "leave unchanged". ActorID is required. The syllabus a node belongs to is not
// patchable (changing it would require re-validating the whole subtree); create a new node
// under the correct syllabus instead.
type UpdateInput struct {
	ActorID          string
	ParentNodeID     *string
	ParentNodeIDSet  bool
	NodeKind         *NodeKind
	NodeCode         *string
	Label            *string
	ContentSourceID  *string
	SourceLocator    *string
	SourceLocatorSet bool
}

// blank reports whether a string is empty or whitespace-only.
func blank(s string) bool { return strings.TrimSpace(s) == "" }
