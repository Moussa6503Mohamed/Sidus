// Package uploadintake owns private admin PDF intake metadata. It never parses, returns, or
// exposes uploaded bytes; only an opaque local-object reference crosses the database boundary.
package uploadintake

import "time"

type Status string

const (
	StatusQuarantined       Status = "quarantined"
	StatusScanClean         Status = "scan_clean"
	StatusReviewPending     Status = "review_pending"
	StatusDeletionRequested Status = "deletion_requested"
)

type Upload struct {
	ID               string    `json:"id"`
	ObjectRef        string    `json:"objectRef"`
	OriginalFilename string    `json:"originalFilename"`
	MediaType        string    `json:"mediaType"`
	ByteSize         int64     `json:"byteSize"`
	SHA256           string    `json:"sha256"`
	ContentSourceID  *string   `json:"contentSourceId"`
	Status           Status    `json:"status"`
	RetentionState   string    `json:"retentionState"`
	UploadedBy       string    `json:"uploadedBy"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type ReviewJob struct {
	ID              string    `json:"id"`
	UploadID        string    `json:"uploadId"`
	ContentSourceID string    `json:"contentSourceId"`
	Status          string    `json:"status"`
	AdapterVersion  string    `json:"adapterVersion"`
	CreatedAt       time.Time `json:"createdAt"`
}

type CreateInput struct {
	ObjectRef, OriginalFilename, MediaType, SHA256, ActorID string
	ByteSize                                                int64
}
