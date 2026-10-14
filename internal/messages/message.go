package messages

import (
	"time"
)

type Severity int

const (
	Info Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Info:
		return "INFO"
	case Warning:
		return "WARNING"
	case Error:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Outcome int

const (
	Reported Outcome = iota
	Fixed
	WouldFix
	FixFailed
)

func (o Outcome) String() string {
	switch o {
	case Reported:
		return "REPORTED"
	case Fixed:
		return "FIXED"
	case WouldFix:
		return "WOULD_FIX"
	case FixFailed:
		return "FIX_FAILED"
	default:
		return "UNKNOWN"
	}
}

type Target struct {
	Namespace string
	Pod       string
	Container string
	Node      string
	OwnerKind string
	OwnerName string
}

func (t Target) String() string {
	return t.Namespace + "/" + t.Pod
}

type Detail struct {
	Key   string
	Value string
}

type Message struct {
	Severity Severity
	Outcome  Outcome
	Reason   string
	Rule     string
	Title    string
	Text     string
	Target   Target
	Details  []Detail
	Logs     []string
	Time     time.Time
}

func (m Message) Key() string {
	return m.Target.String() + "/" + m.Target.Container + "/" + m.Reason
}
