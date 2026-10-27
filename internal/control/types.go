package control

import (
	"time"
	"warden/internal/messages"
)

type Snapshot struct {
	ID         string             `json:"id"`
	SiteID     string             `json:"site_id"`
	ObservedAt time.Time          `json:"observed_at"`
	Collected  bool               `json:"collected"`
	Error      string             `json:"error,omitempty"`
	Messages   []messages.Message `json:"messages"`
}
type Command struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	TargetUID string    `json:"target_uid"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`
	Note      string    `json:"note,omitempty"`
}
type Result struct {
	CommandID  string    `json:"command_id"`
	Status     string    `json:"status"`
	Note       string    `json:"note,omitempty"`
	FinishedAt time.Time `json:"finished_at"`
}
type SyncRequest struct {
	SiteID    string     `json:"site_id"`
	Snapshots []Snapshot `json:"snapshots"`
	Results   []Result   `json:"results"`
}
type SyncResponse struct {
	AckIDs          []string         `json:"ack_ids"`
	ResultAckIDs    []string         `json:"result_ack_ids"`
	Commands        []Command        `json:"commands"`
	ResultConflicts []ResultConflict `json:"result_conflicts,omitempty"`
}
type ResultConflict struct {
	CommandID string `json:"command_id"`
	Reason    string `json:"reason"`
}
