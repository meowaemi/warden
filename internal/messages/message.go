package messages

import (
	"fmt"
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
func (o Outcome) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

type Target struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
	Node      string `json:"node,omitempty"`
	OwnerKind string `json:"ownerKind,omitempty"`
	OwnerName string `json:"ownerName,omitempty"`
}

// String returns "namespace/pod", or "node/<name>" for a problem of the
// node itself, which has no pod.
func (t Target) String() string {
	if t.Pod == "" && t.Node != "" {
		return "node/" + t.Node
	}
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
func (s *Severity) UnmarshalText(text []byte) error {
	switch string(text) {
	case "INFO":
		*s = Info
	case "WARNING":
		*s = Warning
	case "ERROR":
		*s = Error
	default:
		return fmt.Errorf("unknown severity %q", text)
	}
	return nil
}
func (o *Outcome) UnmarshalText(text []byte) error {
	switch string(text) {
	case "REPORTED":
		*o = Reported
	case "FIXED":
		*o = Fixed
	case "WOULD_FIX":
		*o = WouldFix
	case "FIX_FAILED":
		*o = FixFailed
	default:
		return fmt.Errorf("unknown outcome %q", text)
	}
	return nil
}
