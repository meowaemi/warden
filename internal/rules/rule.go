// Package rules turns problems found in the cluster into user messages.
package rules

import (
	"strconv"
	"time"
	"warden/internal/messages"
)

// Rule recognizes a kind of problem and describes it to the user.
type Rule interface {
	// Name identifies the rule in messages and logs.
	Name() string
	// Match reports whether the rule knows this problem.
	Match(input Input) bool
	// Report describes a matched problem to the user.
	Report(input Input) messages.Message
}

// Input is one problem found in the cluster. A node problem has no pod.
// Owner fields name the workload that runs the pod: a fix changes it, since
// a change of the pod itself is lost when the pod is recreated.
type Input struct {
	NodeName      string
	PodName       string
	OwnerKind     string
	OwnerName     string
	OwnerUID      string
	Namespace     string
	ContainerName string
	Reason        string
	Message       string // Kubernetes text for Reason, may be empty
	ExitCode      int32
	RestartCount  int32
	MemoryLimitMi int
	Since         time.Time // when the problem started, zero if unknown
	Recovered     bool      // the container crashed but runs again
}

// Base is a rule built from a prepared message; it matches by Reason.
type Base struct {
	messages.Message
}

// Name returns the rule name of the prepared message.
func (b Base) Name() string { return b.Rule }

// Match reports whether input has the prepared message's Reason.
func (b Base) Match(input Input) bool {
	return input.Reason == b.Reason
}

// Report fills the message for input. A recovered container is only a warning.
func (b Base) Report(input Input) messages.Message {
	msg := b.Message
	msg.Outcome = messages.Reported
	msg.Time = input.Since.UTC()
	msg.Target = messages.Target{
		Namespace: input.Namespace,
		Pod:       input.PodName,
		Container: input.ContainerName,
		Node:      input.NodeName,
		OwnerKind: input.OwnerKind,
		OwnerName: input.OwnerName,
		OwnerUID:  input.OwnerUID,
	}
	msg.Details = nil
	if input.PodName != "" {
		msg.Details = append(msg.Details, messages.Detail{Key: "Restarts", Value: strconv.Itoa(int(input.RestartCount))})
	}
	if input.Message != "" {
		msg.Details = append(msg.Details, messages.Detail{Key: "Kubernetes", Value: input.Message})
	}
	if input.Recovered {
		msg.Severity = messages.Warning
		msg.Details = append(msg.Details, messages.Detail{Key: "State", Value: "running again"})
	}
	return msg
}

// Fallback reports a problem no other rule matched, so none goes unnoticed.
// It matches everything and must be tried last.
type Fallback struct{}

// Name returns "fallback".
func (Fallback) Name() string { return "fallback" }

// Match matches every input.
func (Fallback) Match(input Input) bool { return true }

// Report uses the Kubernetes reason as the title, with Error severity.
func (f Fallback) Report(input Input) messages.Message {
	text := "Kubernetes reports the container as " + input.Reason
	if input.Recovered {
		text = "The container failed with " + input.Reason + " and is running again"
	}
	b := Base{Message: messages.Message{
		Rule:     f.Name(),
		Reason:   input.Reason,
		Severity: messages.Error,
		Title:    input.Reason,
		Text:     text,
	}}
	return b.Report(input)
}
