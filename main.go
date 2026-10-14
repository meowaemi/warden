package main

import (
	"context"
	"os"
	"os/signal"
	"time"
	"warden/internal/core"
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
		rules.NewOOMLimit(25, 512),
	}

	core.Run(ctx, cl, ruleset, 10*time.Second)
}
