// Package core contains the main work loop of warden: it periodically asks
// the cluster for problematic containers, runs them through the rules and
// keeps the resulting messages in a store.
//
// Messages are meant for warden users: a message is printed once, when its
// problem first appears. The operational log written through the logger
// package is for the developer: it records startup, failed checks and
// problems appearing or being resolved, not every check.
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

// Run calls Check immediately and then once per interval until ctx is
// canceled. It blocks for the whole time warden is running.
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

// Check performs a single pass: it lists problems in namespace (all
// namespaces if empty), matches each of them against ruleset and updates st
// with the resulting messages. A problem no rule matched is reported by
// rules.Fallback, so no problem goes unnoticed.
//
// Only problems that are new since the previous pass are printed. Errors are
// logged and leave st untouched; the next pass will try again.
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

// report builds the rule's message for the input. If Kubernetes gave no
// time for the problem, the current time is used.
func report(r rules.Rule, in rules.Input) messages.Message {
	msg := r.Report(in)
	if msg.Time.IsZero() {
		msg.Time = time.Now().UTC()
	}
	return msg
}

// printMessage writes a user-facing message to stdout in a readable form.
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
