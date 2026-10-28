package core

import (
	"context"
	"time"
	"warden/internal/control"
	"warden/internal/kuber"
	"warden/internal/messages"
	"warden/internal/rules"
)

// Collect runs the rules over the cluster once and returns a snapshot.
// A problem no rule matches is reported by rules.Fallback.
func Collect(ctx context.Context, cl *kuber.Client, namespace string, ruleset []rules.Rule) (control.Snapshot, error) {
	inputs, err := cl.Inputs(ctx, namespace)
	snap := control.Snapshot{ObservedAt: time.Now().UTC(), Messages: []messages.Message{}}
	if err != nil {
		return snap, err
	}
	for _, in := range inputs {
		matched := false
		for _, r := range ruleset {
			if r.Match(in) {
				snap.Messages = append(snap.Messages, report(r, in))
				matched = true
			}
		}
		if !matched {
			snap.Messages = append(snap.Messages, report(rules.Fallback{}, in))
		}
	}
	snap.Collected = true
	return snap, nil
}
