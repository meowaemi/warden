// Package logger is warden's operational log for the developer. Times are
// written in UTC.
package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
	"warden/internal/messages"
)

var Log = slog.New(
	slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				a.Value = slog.StringValue(a.Value.Time().UTC().Format(time.DateTime))
			}
			return a
		},
	}),
)

// Appeared logs a problem that has just appeared, at the level matching its
// severity.
func Appeared(m messages.Message) {
	Log.Log(context.Background(), levelOf(m.Severity), "problem appeared", problemAttrs(m)...)
}

// Resolved logs a problem that is gone.
func Resolved(m messages.Message) {
	Log.Info("problem resolved", problemAttrs(m)...)
}

// problemAttrs returns the data of a problem as log key-value pairs.
func problemAttrs(m messages.Message) []any {
	return []any{
		"key", m.Key(),
		"rule", m.Rule,
		"reason", m.Reason,
		"namespace", m.Target.Namespace,
		"pod", m.Target.Pod,
		"container", m.Target.Container,
		"node", m.Target.Node,
		"since", m.Time.Format(time.DateTime),
		"title", m.Title,
	}
}

// levelOf returns the log level matching a message severity.
func levelOf(s messages.Severity) slog.Level {
	switch s {
	case messages.Error:
		return slog.LevelError
	case messages.Warning:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
