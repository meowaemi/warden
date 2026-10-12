package rules

import "warden/internal/messages"

type UnfixableRule interface {
	Rule

	Report(input Input) messages.Message
}

// rules
