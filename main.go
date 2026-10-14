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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	core.Run(ctx, cl, metav1.NamespaceAll, ruleset, 10*time.Second)
}
