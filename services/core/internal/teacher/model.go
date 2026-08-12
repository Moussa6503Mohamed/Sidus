package teacher

import "time"

type MarkingMode string

const (
	MarkingAutomated     MarkingMode = "automated"
	MarkingManualTeacher MarkingMode = "manual_teacher"
)

type Class struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
type Invite struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	MaxUses   int       `json:"maxUses"`
}

// RosterMember deliberately has no Clerk subject, profile, answers, or analytics.
type RosterMember struct {
	MembershipID string    `json:"membershipId"`
	Status       string    `json:"status"`
	ConsentedAt  time.Time `json:"consentedAt"`
}
type Assignment struct {
	ID                string      `json:"id"`
	ClassID           string      `json:"classId"`
	Title             string      `json:"title"`
	MarkingMode       MarkingMode `json:"markingMode"`
	ItemCount         int         `json:"itemCount"`
	StartedCount      int         `json:"startedCount"`
	ActiveMemberCount int         `json:"activeMemberCount"`
	CreatedAt         time.Time   `json:"createdAt"`
}
type LearnerAssignment struct {
	ID          string      `json:"id"`
	ClassName   string      `json:"className"`
	Title       string      `json:"title"`
	MarkingMode MarkingMode `json:"markingMode"`
	ItemCount   int         `json:"itemCount"`
	State       string      `json:"state"`
}

// AssignmentItem is the explicit learner-safe snapshot. It contains delivery content only.
type AssignmentItem struct {
	ID              string `json:"id"`
	ResponseType    string `json:"responseType"`
	Language        string `json:"language"`
	Prompt          string `json:"prompt"`
	Options         any    `json:"options"`
	ContentRevision int    `json:"contentRevision"`
}
type StartedAssignment struct {
	AssignmentID string           `json:"assignmentId"`
	MarkingMode  MarkingMode      `json:"markingMode"`
	Items        []AssignmentItem `json:"items"`
}
type CreateAssignmentInput struct {
	Title       string      `json:"title"`
	SyllabusID  string      `json:"syllabusId"`
	ModuleID    string      `json:"moduleId"`
	QuestionIDs []string    `json:"questionIds"`
	MarkingMode MarkingMode `json:"markingMode"`
}
