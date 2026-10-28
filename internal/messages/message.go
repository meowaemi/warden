// Package messages defines what warden reports to its users.
package messages

import (
	"fmt"
	"time"
)

// Severity is how serious a problem is.
type Severity int

// Severities, from least to most serious.
const (
	Info Severity = iota
	Warning
	Error
)

// String returns the name of s, as it appears in JSON and logs.
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

// MarshalText writes s as its name.
func (s Severity) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Outcome is what warden did about a problem.
type Outcome int

// Outcomes. Reported only describes a problem; Fixed and FixFailed tell how
// a fix ended; WouldFix is a fix that is possible but was not applied.
const (
	Reported Outcome = iota
	Fixed
	WouldFix
	FixFailed
)

// String returns the name of o, as it appears in JSON and logs.
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

// MarshalText writes o as its name.
func (o Outcome) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

// Target is where a problem is: a container of a pod, or a node when Pod is
// empty. Owner fields name the workload that runs the pod.
type Target struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
	Node      string `json:"node,omitempty"`
	OwnerKind string `json:"ownerKind,omitempty"`
	OwnerName string `json:"ownerName,omitempty"`
	OwnerUID  string `json:"ownerUID,omitempty"`
}

// String returns namespace/pod, or node/name for a node.
func (t Target) String() string {
	if t.Pod == "" && t.Node != "" {
		return "node/" + t.Node
	}
	return t.Namespace + "/" + t.Pod
}

// Detail is one extra fact about a problem.
type Detail struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Message is one problem as users see it. Time is when it started.
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

// Key identifies a problem across checks.
func (m Message) Key() string {
	return m.Target.String() + "/" + m.Target.Container + "/" + m.Reason
}

// UnmarshalText reads a severity name; an unknown name is an error.
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

// UnmarshalText reads an outcome name; an unknown name is an error.
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
