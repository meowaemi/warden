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

// MarshalText makes the severity appear as its name in JSON.
func (s Severity) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

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

// MarshalText makes the outcome appear as its name in JSON.
func (o Outcome) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

type Target struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
	Node      string `json:"node,omitempty"`
	OwnerKind string `json:"ownerKind,omitempty"`
	OwnerName string `json:"ownerName,omitempty"`
}

func (t Target) String() string {
	return t.Namespace + "/" + t.Pod
}

type Detail struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Message struct {
	Severity Severity  `json:"severity"`
	Outcome  Outcome   `json:"outcome"`
	Reason   string    `json:"reason"`
	Rule     string    `json:"rule"`
	Title    string    `json:"title"`
	Text     string    `json:"text"`
	Target   Target    `json:"target"`
	Details  []Detail  `json:"details"`
	Logs     []string  `json:"logs,omitempty"`
	Time     time.Time `json:"time"`
}

func (m Message) Key() string {
	return m.Target.String() + "/" + m.Target.Container + "/" + m.Reason
}
