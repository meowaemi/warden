// Package control is the center: site state, the command queue and the
// sync protocol between agents and the center.
package control

import (
	"time"
	"warden/internal/messages"
)

// Snapshot is one observation of a site. Collected is false when the agent
// could not read the cluster; Error then says why.
type Snapshot struct {
	ID         string             `json:"id"`
	SiteID     string             `json:"site_id"`
	ObservedAt time.Time          `json:"observed_at"`
	Collected  bool               `json:"collected"`
	Error      string             `json:"error,omitempty"`
	Messages   []messages.Message `json:"messages"`
}

// Command is a deferred change of one workload. Container and Resources are
// set only for set_resources, Replicas only for scale.
type Command struct {
	ID         string     `json:"id"`
	SiteID     string     `json:"site_id"`
	Kind       string     `json:"kind"`
	TargetKind string     `json:"target_kind,omitempty"`
	Namespace  string     `json:"namespace"`
	Name       string     `json:"name"`
	TargetUID  string     `json:"target_uid"`
	Container  string     `json:"container,omitempty"`
	Resources  *Resources `json:"resources,omitempty"`
	Replicas   *int32     `json:"replicas,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	Status     string     `json:"status"`
	Note       string     `json:"note,omitempty"`
}

// Resources holds Kubernetes quantities such as "500m" or "256Mi";
// an empty field is left unchanged.
type Resources struct {
	CPURequest    string `json:"cpu_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

// Result is the outcome of a command reported by an agent.
type Result struct {
	CommandID  string    `json:"command_id"`
	Status     string    `json:"status"`
	Note       string    `json:"note,omitempty"`
	FinishedAt time.Time `json:"finished_at"`
}

// SyncRequest carries the snapshots and results an agent has queued.
type SyncRequest struct {
	SiteID    string     `json:"site_id"`
	Snapshots []Snapshot `json:"snapshots"`
	Results   []Result   `json:"results"`
}

// SyncResponse acknowledges what the center took, rejected entries included,
// and returns at most one command. ServerTime lets the agent correct its clock.
type SyncResponse struct {
	AckIDs            []string           `json:"ack_ids"`
	ResultAckIDs      []string           `json:"result_ack_ids"`
	Commands          []Command          `json:"commands"`
	ResultConflicts   []ResultConflict   `json:"result_conflicts,omitempty"`
	SnapshotConflicts []SnapshotConflict `json:"snapshot_conflicts,omitempty"`
	ServerTime        time.Time          `json:"server_time"`
}

// ResultConflict explains why a result was rejected.
type ResultConflict struct {
	CommandID string `json:"command_id"`
	Reason    string `json:"reason"`
}

// SnapshotConflict explains why a snapshot was rejected.
type SnapshotConflict struct {
	SnapshotID string `json:"snapshot_id"`
	Reason     string `json:"reason"`
}
