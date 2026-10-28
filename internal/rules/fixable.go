package rules

import (
	"strconv"
	"warden/internal/messages"
)

// FixableRule is a rule that can also fix what it finds.
type FixableRule interface {
	Rule
	// Fix tries to fix the problem of input.
	Fix(input Input) (Result, error)
	// ReportFix describes to the user how the fix ended.
	ReportFix(result Result) messages.Message
}

// Result is the outcome of a fix.
type Result struct {
	Input
	// Success is true when the fix was applied.
	Success bool
	// Note is extra text for the user, may be empty.
	Note string
}

// OOMLimit reports containers killed for lack of memory and raises their
// limit by StepPercent, up to MaxMi.
type OOMLimit struct {
	Base
	StepPercent int
	MaxMi       int
}

// NewOOMLimit returns an OOMLimit rule.
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

// Report adds the memory limit to the message.
func (r OOMLimit) Report(input Input) messages.Message {
	msg := r.Base.Report(input)
	msg.Details = append(msg.Details, messages.Detail{
		Key:   "Memory limit",
		Value: strconv.Itoa(input.MemoryLimitMi) + "Mi",
	})
	return msg
}

// Fix is not implemented yet.
func (r OOMLimit) Fix(input Input) (Result, error) {
	return Result{}, nil
}

// ReportFix describes the outcome of a fix.
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
