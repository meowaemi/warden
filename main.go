package main

import (
	"context"
	"os"
	"os/signal"
	"time"
	"warden/internal/api"
	"warden/internal/core"
	"warden/internal/kuber"
	"warden/internal/logger"
	"warden/internal/rules"
	"warden/internal/server"
	"warden/internal/store"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	cl, err := kuber.New()
	if err != nil {
		logger.Log.Error("create kube client", "err", err)
		os.Exit(1)
	}

	version, err := cl.Ping(context.Background())
	if err != nil {
		logger.Log.Error("cluster unreachable", "err", err)
		os.Exit(1)
	}
	logger.Log.Info("connected to cluster", "version", version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ruleset := []rules.Rule{
		rules.NewOOMLimit(25, 512),
	}

	st := store.New()

	srv := server.New(":8080", api.Handler(st))
	srv.Start(stop)

	core.Run(ctx, cl, metav1.NamespaceAll, ruleset, st, 10*time.Second)

	if err := srv.Shutdown(5 * time.Second); err != nil {
		logger.Log.Error("http server shutdown", "err", err)
	}
}
