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
// SyncResponse answers an agent sync. ServerTime is the center's clock when
// the response was built: the agent uses it to estimate its own clock skew,
// because command deadlines and observation times are in center time.
// A rejected snapshot or result is still acknowledged, so the agent drops it
// instead of resending it forever; the conflict lists say why.
type SyncResponse struct {
	AckIDs            []string           `json:"ack_ids"`
	ResultAckIDs      []string           `json:"result_ack_ids"`
	Commands          []Command          `json:"commands"`
	ResultConflicts   []ResultConflict   `json:"result_conflicts,omitempty"`
	SnapshotConflicts []SnapshotConflict `json:"snapshot_conflicts,omitempty"`
	ServerTime        time.Time          `json:"server_time"`
}
type ResultConflict struct {
	CommandID string `json:"command_id"`
	Reason    string `json:"reason"`
}
type SnapshotConflict struct {
	SnapshotID string `json:"snapshot_id"`
	Reason     string `json:"reason"`
}
