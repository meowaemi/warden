package rules

import (
	"strconv"
	"warden/internal/messages"
)

type FixableRule interface {
	Rule

	Fix(input Input) (Result, error)

	ReportFix(result Result) messages.Message
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
	Base
	StepPercent int
	MaxMi       int
}

// NewOOMLimit returns an OOMLimit rule that raises the limit by stepPercent
// at a time, up to maxMi mebibytes.
func NewOOMLimit(stepPercent, maxMi int) OOMLimit {
	return OOMLimit{
		Base: Base{Message: messages.Message{
			Rule:     "oom-limit",
			Reason:   "OOMKilled",
			Severity: messages.Error,
			Title:    "Container killed: out of memory",
			Text:     "The container exceeded its memory limit and was killed",
		}},
		StepPercent: stepPercent,
		MaxMi:       maxMi,
	}
}

func (r OOMLimit) Report(input Input) messages.Message {
	msg := r.Base.Report(input)
	msg.Details = append(msg.Details, messages.Detail{
		Key:   "Memory limit",
		Value: strconv.Itoa(input.MemoryLimitMi) + "Mi",
	})
	return msg
}

func (r OOMLimit) Fix(input Input) (Result, error) {

	return Result{}, nil
}

func (r OOMLimit) ReportFix(result Result) messages.Message {
	msg := r.Report(result.Input)

	if result.Success {
		msg.Severity = messages.Info
		msg.Outcome = messages.Fixed
		msg.Title = "Pod memory limit increased"
		msg.Text = "The memory limit was successfully increased to prevent OOMKilled errors"
	} else {
		msg.Severity = messages.Error
		msg.Outcome = messages.FixFailed
		msg.Title = "Pod terminated: OOMKilled"
		msg.Text = "Failed to increase the pod memory limit"
	}

	if result.Note != "" {
		msg.Details = append(msg.Details, messages.Detail{Key: "Result", Value: result.Note})
	}

	return msg
}
