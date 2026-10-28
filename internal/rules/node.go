package rules

import "warden/internal/messages"

// Reasons of node (server) problems. kuber reports a node problem as an Input
// with NodeName set and PodName empty.
const (
	NodeNotReady    = "NodeNotReady"    // Ready=False: the kubelet runs but the node cannot run pods
	NodeUnreachable = "NodeUnreachable" // Ready=Unknown: the node stopped reporting, it is down or cut off
	MemoryPressure  = "MemoryPressure"
	DiskPressure    = "DiskPressure"
	PIDPressure     = "PIDPressure"
	NodeCordoned    = "NodeCordoned" // spec.unschedulable: new pods are not placed on the node
)

// NodeRules returns the rules for server problems.
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

func nodeRule(rule, reason string, severity messages.Severity, title, text string) Base {
	return Base{Message: messages.Message{
		Rule:     rule,
		Reason:   reason,
		Severity: severity,
		Title:    title,
		Text:     text,
	}}
}
