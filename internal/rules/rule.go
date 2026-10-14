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

type Input struct {
	PodName       string
	Namespace     string
	ContainerName string
	Reason        string
	Message       string // text Kubernetes gives together with Reason, may be empty
	ExitCode      int32
	RestartCount  int32
	MemoryLimitMi int
	Since         time.Time // when Kubernetes says the problem started, current if unknown
}

// Base implements Name, Match and Report using a prepared message.
// Rule, Reason, Severity, Title and Text are taken from the message;
// Report fills in the parts that depend on the container.
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
	}
	msg.Details = []messages.Detail{
		{Key: "Restarts", Value: strconv.Itoa(int(input.RestartCount))},
	}
	if input.Message != "" {
		msg.Details = append(msg.Details, messages.Detail{Key: "Kubernetes", Value: input.Message})
	}
	return msg
}

// Fallback reports problems that no other rule matched, using only what
// Kubernetes itself says about the container. It matches every input, so it
// must be applied only after all other rules failed to match.
type Fallback struct{}

func (Fallback) Name() string { return "fallback" }

func (Fallback) Match(input Input) bool { return true }

func (f Fallback) Report(input Input) messages.Message {
	b := Base{Message: messages.Message{
		Rule:     f.Name(),
		Reason:   input.Reason,
		Severity: messages.Warning,
		Title:    input.Reason,
		Text:     "Kubernetes reports the container as " + input.Reason,
	}}
	return b.Report(input)
}
