package rules

import "warden/internal/messages"

// Reasons of node problems.
const (
	NodeNotReady    = "NodeNotReady"    // connected, but cannot run pods
	NodeUnreachable = "NodeUnreachable" // stopped reporting: down or cut off
	MemoryPressure  = "MemoryPressure"
	DiskPressure    = "DiskPressure"
	PIDPressure     = "PIDPressure"
	NodeCordoned    = "NodeCordoned" // marked unschedulable
)

// NodeRules returns the rules for node problems.
func NodeRules() []Rule {
	return []Rule{
		nodeRule("node-not-ready", NodeNotReady, messages.Error,
			"Server down",
			"The node is NotReady: it is connected to the cluster but cannot run pods"),
		nodeRule("node-unreachable", NodeUnreachable, messages.Error,
			"Server unreachable",
			"The node stopped reporting to the cluster: it is down or has lost its network connection"),
		nodeRule("node-memory-pressure", MemoryPressure, messages.Warning,
			"Server low on memory",
			"The node is running out of memory; Kubernetes may evict pods from it"),
		nodeRule("node-disk-pressure", DiskPressure, messages.Warning,
			"Server low on disk space",
			"The node is running out of disk space; Kubernetes may evict pods from it"),
		nodeRule("node-pid-pressure", PIDPressure, messages.Warning,
			"Server low on process IDs",
			"The node is running out of process IDs; new processes may fail to start"),
		nodeRule("node-cordoned", NodeCordoned, messages.Info,
			"Server cordoned",
			"The node is marked unschedulable: running pods stay, new pods are not placed on it"),
	}
}

// nodeRule builds a node rule from its message.
func nodeRule(rule, reason string, severity messages.Severity, title, text string) Base {
	return Base{Message: messages.Message{
		Rule:     rule,
		Reason:   reason,
		Severity: severity,
		Title:    title,
		Text:     text,
	}}
}
