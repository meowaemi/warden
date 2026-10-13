package main

import (
	"context"
	"os"
	"os/signal"
	"time"
	"warden/internal/kuber"
	"warden/internal/logger"
	"warden/internal/rules"
)

func main() {
	cl, err := kuber.New()
	if err != nil {
		logger.Log.Error("create kube client", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ruleset := []rules.Rule{
		rules.OOMLimit{StepPercent: 25, MaxMi: 512},
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		check(ctx, cl, ruleset)

		select {
		case <-ctx.Done():
			logger.Log.Info("shutting down")
			return
		case <-ticker.C:
		}
	}
}

func check(ctx context.Context, cl *kuber.Client, ruleset []rules.Rule) {
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
		for _, r := range ruleset {
			if !r.Match(in) {
				continue
			}
			logger.Log.Error("rule matched",
				"rule", r.Name(),
				"pod", in.PodName,
				"container", in.ContainerName,
				"reason", in.Reason,
				"limit_mi", in.MemoryLimitMi,
			)
		}
	}
}
