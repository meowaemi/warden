package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"warden/internal/agent"
	"warden/internal/api"
	"warden/internal/control"
	"warden/internal/core"
	"warden/internal/demo"
	"warden/internal/kuber"
	"warden/internal/logger"
	"warden/internal/rules"
	"warden/internal/server"
	"warden/internal/store"
)

func main() {
	if err := run(); err != nil {
		logger.Log.Error("warden stopped", "err", err)
		os.Exit(1)
	}
}
func run() error {
	mode := flag.String("mode", "local", "local, center, agent, or demo")
	addr := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	state := flag.String("state", "data/warden.json", "persistent state file (one process per file)")
	site := flag.String("site", "", "agent site identifier")
	center := flag.String("center", "http://127.0.0.1:8080", "center URL")
	namespace := flag.String("namespace", "", "Kubernetes namespace; empty means all")
	interval := flag.Duration("interval", 10*time.Second, "collection and synchronization interval")
	stale := flag.Duration("stale-after", 30*time.Second, "maximum observation age")
	backlog := flag.Int("max-snapshots", 1000, "agent observation backlog limit")
	web := flag.String("web-dir", "web-next/out", "built frontend directory")
	flag.Parse()
	if *interval <= 0 || *stale <= 0 || *backlog <= 0 {
		return fmt.Errorf("interval, stale-after and max-snapshots must be positive")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ruleset := []rules.Rule{rules.NewOOMLimit(25, 512)}
	switch *mode {
	case "center":
		operator := os.Getenv("WARDEN_OPERATOR_TOKEN")
		if len(operator) < 16 {
			return fmt.Errorf("set WARDEN_OPERATOR_TOKEN with at least 16 characters")
		}
		var tokens map[string]string
		if err := json.Unmarshal([]byte(os.Getenv("WARDEN_SITE_TOKENS")), &tokens); err != nil || len(tokens) == 0 {
			return fmt.Errorf("set WARDEN_SITE_TOKENS to a JSON object mapping site IDs to unique tokens")
		}
		seen := map[string]bool{operator: true}
		for id, t := range tokens {
			if id == "" || len(t) < 16 || seen[t] {
				return fmt.Errorf("site IDs must be nonempty and tokens unique with at least 16 characters")
			}
			seen[t] = true
		}
		st, err := control.Open(*state)
		if err != nil {
			return err
		}
		return serve(ctx, stop, *addr, website(control.Handler(st, operator, tokens, *stale), *web))
	case "demo":
		operator := os.Getenv("WARDEN_OPERATOR_TOKEN")
		if operator == "" {
			operator = "warden-demo-operator"
		}
		st, err := control.Open(*state)
		if err != nil {
			return err
		}
		handler, cleanup, err := demo.Start(ctx, st, operator, filepath.Join(filepath.Dir(*state), "demo-agent.json"), *interval, *stale)
		if err != nil {
			return err
		}
		defer cleanup()
		logger.Log.Info("SIMULATED demo: no real Kubernetes changes", "site", "demo-site")
		return serve(ctx, stop, *addr, website(handler, *web))
	case "agent":
		token := os.Getenv("WARDEN_AGENT_TOKEN")
		if len(token) < 16 {
			return fmt.Errorf("set WARDEN_AGENT_TOKEN with at least 16 characters")
		}
		cl, err := kuber.New(10 * time.Minute)
		if err != nil {
			return err
		}
		return agent.Run(ctx, agent.Config{SiteID: *site, CenterURL: *center, Token: token, StatePath: *state, Interval: *interval, MaxSnapshots: *backlog}, func(c context.Context) (control.Snapshot, error) { return core.Collect(c, cl, *namespace, ruleset) }, cl.ExecuteCommand)
	case "local":
		cl, err := kuber.New(10 * time.Minute)
		if err != nil {
			return err
		}
		st := store.New()
		srv := server.New(*addr, api.Handler(st))
		if err := srv.Start(stop); err != nil {
			return err
		}
		core.Run(ctx, cl, *namespace, ruleset, st, *interval)
		return srv.Shutdown(5 * time.Second)
	default:
		return fmt.Errorf("unknown mode %q", *mode)
	}
}
func website(apiHandler http.Handler, dir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler)
	mux.Handle("/", http.FileServer(http.Dir(dir)))
	return mux
}
func serve(ctx context.Context, stop context.CancelFunc, addr string, h http.Handler) error {
	srv := server.New(addr, h)
	if err := srv.Start(stop); err != nil {
		return err
	}
	<-ctx.Done()
	return srv.Shutdown(5 * time.Second)
}
