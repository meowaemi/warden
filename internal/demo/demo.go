// Package demo runs a center with a simulated site agent. Nothing touches
// Kubernetes; collection and execution are faked.
package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"
	"warden/internal/agent"
	"warden/internal/control"
	"warden/internal/logger"
	"warden/internal/messages"
)

// Start runs the simulated agent and returns the center handler, extended
// with /api/demo/link to cut and restore the agent's link, and a cleanup func.
func Start(ctx context.Context, st *control.Store, operator, statePath string, interval, stale time.Duration) (http.Handler, func(), error) {
	var connected atomic.Bool
	connected.Store(true)
	token := "warden-demo-site-token"
	center := control.Handler(st, operator, map[string]string{"demo-site": token}, stale)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	// gate stands between agent and center and fails while the link is cut.
	gate := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !connected.Load() {
			http.Error(w, "simulated link outage", 503)
			return
		}
		center.ServeHTTP(w, r)
	}), ReadHeaderTimeout: 5 * time.Second}
	child, cancel := context.WithCancel(ctx)
	go func() { _ = gate.Serve(listener) }()
	var observations atomic.Uint64
	var applied atomic.Uint64
	// The simulated container is OOM killed until its memory limit is raised.
	var memoryRaised atomic.Bool
	target := messages.Target{Namespace: "demo", Pod: "demo-app", Container: "app", OwnerKind: control.Deployment, OwnerName: "demo-app", OwnerUID: "demo-deployment-uid"}
	go func() {
		err := agent.Run(child, agent.Config{SiteID: "demo-site", CenterURL: "http://" + listener.Addr().String(), Token: token, StatePath: statePath, Interval: interval, MaxSnapshots: 1000}, func(context.Context) (control.Snapshot, error) {
			n := observations.Add(1)
			now := time.Now().UTC()
			msgs := []messages.Message{{Severity: messages.Warning, Outcome: messages.Reported, Reason: "DemoObservation", Rule: "demo", Title: "Simulated site observation", Text: fmt.Sprintf("Observation %d; simulated applied commands %d. No real Kubernetes connection.", n, applied.Load()), Target: target, Time: now}}
			if !memoryRaised.Load() {
				msgs = append(msgs, messages.Message{Severity: messages.Error, Outcome: messages.Reported, Reason: "OOMKilled", Rule: "oom-limit", Title: "Container killed: out of memory", Text: "SIMULATED: the container exceeded its memory limit and was killed", Target: target, Details: []messages.Detail{{Key: "Memory limit", Value: "128Mi"}}, Time: now})
			}
			return control.Snapshot{Messages: msgs}, nil
		}, func(_ context.Context, c control.Command) control.Result {
			if c.Namespace != "demo" || c.Name != "demo-app" || c.TargetUID != "demo-deployment-uid" {
				return control.Result{Status: "failed", Note: "demo target must be demo/demo-app with UID demo-deployment-uid"}
			}
			if c.Kind == control.KindSetResources && c.Container != "app" {
				return control.Result{Status: "failed", Note: "SIMULATED: container " + c.Container + " not found in the pod template"}
			}
			if c.Kind == control.KindSetResources && c.Resources.MemoryLimit != "" {
				memoryRaised.Store(true)
			}
			applied.Add(1)
			return control.Result{Status: "applied", Note: "SIMULATED acknowledgement; no Kubernetes resources changed"}
		})
		if err != nil {
			logger.Log.Error("demo agent stopped", "err", err)
		}
	}()
	mux := http.NewServeMux()
	mux.Handle("/", center)
	mux.HandleFunc("/api/demo/link", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+operator {
			http.Error(w, "unauthorized", 401)
			return
		}
		if r.Method == "POST" {
			var body struct {
				Connected bool `json:"connected"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
				http.Error(w, "invalid JSON", 400)
				return
			}
			connected.Store(body.Connected)
		} else if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"connected": connected.Load(), "simulated": true})
	})
	return mux, func() { cancel(); _ = gate.Close() }, nil
}
