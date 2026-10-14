// Package core contains the main work loop of warden: it periodically asks
// the cluster for problematic containers, runs them through the rules and
// outputs the resulting messages.
//
// Messages are meant for warden users and are printed separately from the
// operational log written through the logger package.
package core

import (
	"context"
	"fmt"
	"time"
	"warden/internal/kuber"
	"warden/internal/logger"
	"warden/internal/messages"
	"warden/internal/rules"
)

// Run calls Check immediately and then once per interval until ctx is
// canceled. It blocks for the whole time warden is running.
func Run(ctx context.Context, cl *kuber.Client, ruleset []rules.Rule, interval time.Duration) {
	logger.Log.Info("warden started", "interval", interval, "rules", len(ruleset))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		Check(ctx, cl, ruleset)

		select {
		case <-ctx.Done():
			logger.Log.Info("shutting down")
			return
		case <-ticker.C:
		}
	}
}

// Check performs a single pass: it lists problematic containers in the
// "default" namespace, matches each of them against ruleset and prints a
// message for every match. A container no rule matched is reported by
// rules.Fallback, so no problem goes unnoticed. Errors are logged and do not
// stop warden; the next pass will try again.
func Check(ctx context.Context, cl *kuber.Client, ruleset []rules.Rule) {
	inputs, err := cl.Inputs(ctx, "default")
	if err != nil {
		logger.Log.Error("list problems", "err", err)
		return
	}

	if len(inputs) == 0 {
		logger.Log.Info("no problems found")
		return
	}

	for _, in := range inputs {
		matched := false
		for _, r := range ruleset {
			if !r.Match(in) {
				continue
			}
			matched = true
			report(r, in)
		}
		if !matched {
			report(rules.Fallback{}, in)
		}
	}
}

// report builds the rule's message for the input and outputs it.
func report(r rules.Rule, in rules.Input) {
	msg := r.Report(in)
	msg.Time = time.Now().UTC()
	printMessage(msg)
}

// printMessage writes a user-facing message to stdout in a readable form.
func printMessage(m messages.Message) {
	fmt.Printf("[%s] %s | %s/%s (%s)\n",
		m.Severity, m.Title, m.Target.Namespace, m.Target.Pod, m.Target.Container)
	fmt.Printf("    %s\n", m.Text)
	for _, d := range m.Details {
		fmt.Printf("    %s: %s\n", d.Key, d.Value)
	}
	if m.Hint != "" {
		fmt.Printf("    hint: %s\n", m.Hint)
	}
}
