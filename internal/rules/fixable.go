package rules

import (
	"strconv"
	"warden/internal/messages"
)

type FixableRule interface {
	Rule

	Fix(input Input) (Result, error)

	Report(result Result) messages.Message
}

type Result struct {
	Input
	Success bool
	Note    string
}

// OOMLimit manages the automatic scaling of memory limits for Kubernetes pods
// that have failed with an OOMKilled error.
//
// It monitors pod lifecycle events and increments the memory allocations
// by StepPercent until the limit reaches MaxMi.
type OOMLimit struct {
	StepPercent int
	MaxMi       int
}

func (OOMLimit) Name() string { return "oom-limit" }

func (OOMLimit) Match(input Input) bool {
	return input.Reason == "OOMKilled"
}

func (r OOMLimit) Fix(input Input) (Result, error) {

	return Result{}, nil
}

func (r OOMLimit) Report(result Result) messages.Message {
	msg := messages.Message{
		Reason: result.Reason,
		Rule:   r.Name(),
		Target: messages.Target{
			Namespace: result.Namespace,
			Pod:       result.PodName,
			Container: result.ContainerName,
		},
		Details: []messages.Detail{
			{Key: "Restarts", Value: strconv.Itoa(int(result.RestartCount))},
		},
	}

	if result.Success {
		msg.Severity = messages.Warning
		msg.Outcome = messages.Fixed
		msg.Title = "Pod memory limit increased"
		msg.Text = "The memory limit was successfully increased to prevent OOMKilled errors"
	} else {
		msg.Severity = messages.Critical
		msg.Outcome = messages.FixFailed
		msg.Title = "Pod terminated: OOMKilled"
		msg.Text = "Failed to increase the pod memory limit"
		msg.Hint = "Check cluster capacity, resource quotas, or your service permissions"
	}

	if result.Note != "" {
		msg.Details = append(msg.Details, messages.Detail{Key: "Result", Value: result.Note})
	}

	return msg
}
