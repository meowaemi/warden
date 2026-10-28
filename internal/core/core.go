// Package core runs the rules over the cluster: once per snapshot for an
// agent, or in a loop with an in-memory store for the local mode.
package core

import (
	"context"
	"fmt"
	"strings"
	"time"
	"warden/internal/kuber"
	"warden/internal/logger"
	"warden/internal/messages"
	"warden/internal/rules"
	"warden/internal/store"
)

// Run calls Check every interval until ctx is canceled.
func Run(ctx context.Context, cl *kuber.Client, namespace string, ruleset []rules.Rule, st *store.Store, interval time.Duration) {
	names := make([]string, 0, len(ruleset))
	for _, r := range ruleset {
		names = append(names, r.Name())
	}
	logger.Log.Info("warden started",
		"namespace", kuber.NamespaceLabel(namespace),
		"interval", interval,
		"rules", strings.Join(names, ","),
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		Check(ctx, cl, namespace, ruleset, st)

		select {
		case <-ctx.Done():
			logger.Log.Info("shutting down")
			return
		case <-ticker.C:
		}
	}
}

// Check runs the rules once, updates st and prints the problems that are
// new since the previous check. Errors are logged and leave st unchanged.
func Check(ctx context.Context, cl *kuber.Client, namespace string, ruleset []rules.Rule, st *store.Store) {
	inputs, err := cl.Inputs(ctx, namespace)
	if err != nil {
		logger.Log.Error("check failed", "err", err)
		return
	}

	var found []messages.Message
	for _, in := range inputs {
		matched := false
		for _, r := range ruleset {
			if !r.Match(in) {
				continue
			}
			matched = true
			found = append(found, report(r, in))
		}
		if !matched {
			found = append(found, report(rules.Fallback{}, in))
		}
	}

	added, removed := st.Update(found)
	for _, msg := range added {
		logger.Appeared(msg)
		printMessage(msg)
	}
	for _, msg := range removed {
		logger.Resolved(msg)
	}
}

// report builds the rule's message; with no time from Kubernetes it uses now.
func report(r rules.Rule, in rules.Input) messages.Message {
	msg := r.Report(in)
	if msg.Time.IsZero() {
		msg.Time = time.Now().UTC()
	}
	return msg
}

// printMessage prints a message for the user.
func printMessage(m messages.Message) {
	target := m.Target.String()
	if m.Target.Container != "" {
		target += " (" + m.Target.Container + ")"
	}
	fmt.Printf("[%s] %s | %s\n", m.Severity, m.Title, target)
	fmt.Printf("    %s\n", m.Text)
	for _, d := range m.Details {
		fmt.Printf("    %s: %s\n", d.Key, d.Value)
	}
}
