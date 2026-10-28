package rules

import (
	"strconv"
	"time"
	"warden/internal/messages"
)

type Rule interface {
	Name() string
	Match(input Input) bool
	Report(input Input) messages.Message
}

// Input describes one problem. For a node (server) problem PodName is empty
// and NodeName is set; for a pod problem NodeName is the node it runs on, or
// empty if it is not scheduled yet.
type Input struct {
	NodeName      string
	PodName       string
	Namespace     string
	ContainerName string
	Reason        string
	Message       string // text Kubernetes gives together with Reason, may be empty
	ExitCode      int32
	RestartCount  int32
	MemoryLimitMi int
	Since         time.Time // when Kubernetes says the problem started, zero if unknown
	Recovered     bool      // the container failed but is running again
}

// Base implements Name, Match and Report using a prepared message.
// Rule, Reason, Severity, Title and Text are taken from the message;
// Report fills in the parts that depend on the container. A container that
// has recovered is reported as a Warning whatever the prepared severity is.
type Base struct {
	messages.Message
}

func (b Base) Name() string { return b.Rule }

func (b Base) Match(input Input) bool {
	return input.Reason == b.Reason
}

func (b Base) Report(input Input) messages.Message {
	msg := b.Message
	msg.Outcome = messages.Reported
	msg.Time = input.Since.UTC()
	msg.Target = messages.Target{
		Namespace: input.Namespace,
		Pod:       input.PodName,
		Container: input.ContainerName,
		Node:      input.NodeName,
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

// Fallback reports problems that no other rule matched, using only what
// Kubernetes itself says about the container. It matches every input, so it
// must be applied only after all other rules failed to match.
//
// The severity is Error: the container or pod does not run. A container that
// has recovered becomes a Warning in Base.Report.
type Fallback struct{}

func (Fallback) Name() string { return "fallback" }

func (Fallback) Match(input Input) bool { return true }

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
