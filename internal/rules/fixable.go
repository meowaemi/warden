package rules

import "warden/internal/messages"

type FixableRule interface {
	Rule

	Fix(input Input) (Result, error)

	Report(result Result) messages.Message
}

type Result struct {
	Success bool
	Reason  string
}

// rules
