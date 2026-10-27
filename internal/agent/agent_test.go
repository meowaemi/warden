package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"warden/internal/control"
	"warden/internal/messages"
)

func config(t *testing.T, url string) Config {
	t.Helper()
	return Config{SiteID: "remote", CenterURL: url, Token: "site-secret", StatePath: filepath.Join(t.TempDir(), "agent.json"), Interval: time.Millisecond, MaxSnapshots: 3}
}
func observation(context.Context) (control.Snapshot, error) {
	return control.Snapshot{Messages: []messages.Message{{Title: "problem", Severity: messages.Warning}}}, nil
}
func noExecute(context.Context, control.Command) control.Result { panic("unexpected execution") }
func TestOfflineReconnectToRealCenter(t *testing.T) {
	st, err := control.Open(filepath.Join(t.TempDir(), "center.json"))
	if err != nil {
		t.Fatal(err)
	}
	center := httptest.NewServer(control.Handler(st, "operator", map[string]string{"remote": "site-secret"}, time.Minute))
	defer center.Close()
	w, err := open(config(t, center.URL))
	if err != nil {
		t.Fatal(err)
	}
	original := w.cfg.CenterURL
	w.cfg.CenterURL = "http://127.0.0.1:1"
	for i := 0; i < 4; i++ {
		if err = w.collect(context.Background(), observation); err != nil {
			t.Fatal(err)
		}
	}
	if err = w.syncOnce(context.Background(), noExecute); err != nil {
		t.Fatal(err)
	}
	if len(w.state.Snapshots) != 3 || w.state.Dropped != 1 {
		t.Fatalf("unexpected backlog: %+v", w.state)
	}
	w, err = open(w.cfg)
	if err != nil {
		t.Fatal(err)
	}
	w.cfg.CenterURL = original
	if err = w.syncOnce(context.Background(), noExecute); err != nil {
		t.Fatal(err)
	}
	if len(w.state.Snapshots) != 0 {
		t.Fatal("acknowledged snapshots remained")
	}
	req, _ := http.NewRequest(http.MethodGet, center.URL+"/api/history?site_id=remote", nil)
	req.Header.Set("Authorization", "Bearer operator")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var history []control.Snapshot
	if err = json.NewDecoder(resp.Body).Decode(&history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("history count=%d", len(history))
	}
}
func TestSelectiveAcknowledgement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var q control.SyncRequest
		json.NewDecoder(r.Body).Decode(&q)
		json.NewEncoder(rw).Encode(control.SyncResponse{AckIDs: []string{q.Snapshots[1].ID, "unsent"}})
	}))
	defer server.Close()
	w, err := open(config(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = w.collect(context.Background(), observation); err != nil {
			t.Fatal(err)
		}
	}
	first, last := w.state.Snapshots[0].ID, w.state.Snapshots[2].ID
	if err = w.syncOnce(context.Background(), noExecute); err != nil {
		t.Fatal(err)
	}
	w, err = open(w.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.state.Snapshots) != 2 || w.state.Snapshots[0].ID != first || w.state.Snapshots[1].ID != last {
		t.Fatal("selective ack removed wrong observations")
	}
}
func command() control.Command {
	return control.Command{ID: "cmd1", SiteID: "remote", Kind: "restart_deployment", Namespace: "test", Name: "app", TargetUID: "uid1", ExpiresAt: time.Now().Add(time.Minute)}
}
func TestExecutionLedgerRestartExpiryAndPanic(t *testing.T) {
	w, err := open(config(t, "http://unused"))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	exec := func(context.Context, control.Command) control.Result {
		calls++
		return control.Result{Status: "applied"}
	}
	c := command()
	if err = w.execute(context.Background(), c, exec); err != nil {
		t.Fatal(err)
	}
	w, err = open(w.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.execute(context.Background(), c, exec); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || w.state.Results[c.ID].Status != "applied" {
		t.Fatal("duplicate executed or result lost")
	}
	expired := c
	expired.ID = "expired"
	expired.ExpiresAt = time.Now().Add(-time.Second)
	if err = w.execute(context.Background(), expired, exec); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || w.state.Results[expired.ID].Status != "expired" {
		t.Fatal("expired command executed")
	}
	interrupted := c
	interrupted.ID = "interrupted"
	w.state.Ledger[interrupted.ID] = control.Result{CommandID: interrupted.ID, Status: "executing"}
	if err = w.save(); err != nil {
		t.Fatal(err)
	}
	w, err = open(w.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.execute(context.Background(), interrupted, exec); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || w.state.Results[interrupted.ID].Status != "unknown" {
		t.Fatal("interrupted command retried")
	}
	panicked := c
	panicked.ID = "panic"
	if err = w.execute(context.Background(), panicked, noExecute); err != nil {
		t.Fatal(err)
	}
	if w.state.Results[panicked.ID].Status != "unknown" {
		t.Fatal("panic not recorded")
	}
}
func TestRealCenterResultSurvivesRestart(t *testing.T) {
	st, err := control.Open(filepath.Join(t.TempDir(), "center.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(control.Handler(st, "operator", map[string]string{"remote": "site-secret"}, time.Minute))
	defer server.Close()
	c := command()
	body := fmt.Sprintf(`{"id":%q,"site_id":%q,"kind":%q,"namespace":%q,"name":%q,"target_uid":%q,"expires_at":%q}`, c.ID, c.SiteID, c.Kind, c.Namespace, c.Name, c.TargetUID, c.ExpiresAt.Format(time.RFC3339Nano))
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/commands", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer operator")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("create status=%d", resp.StatusCode)
	}
	w, err := open(config(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	exec := func(context.Context, control.Command) control.Result {
		calls++
		return control.Result{Status: "applied"}
	}
	if err = w.syncOnce(context.Background(), exec); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(w.state.Results) != 1 {
		t.Fatal("command not executed durably")
	}
	w, err = open(w.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.syncOnce(context.Background(), exec); err != nil {
		t.Fatal(err)
	}
	if len(w.state.Results) != 0 || calls != 1 {
		t.Fatal("result not acknowledged or command repeated")
	}
	if w.state.Ledger[c.ID].Status != "applied" {
		t.Fatal("ack erased deduplication ledger")
	}
}

func TestRunCollectsWhileSyncBlocked(t *testing.T) {
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
		json.NewEncoder(w).Encode(control.SyncResponse{})
	}))
	defer server.Close()
	cfg := config(t, server.URL)
	cfg.Interval = 5 * time.Millisecond
	cfg.MaxSnapshots = 100
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	observed := make(chan struct{}, 100)
	collector := func(ctx context.Context) (control.Snapshot, error) { observed <- struct{}{}; return observation(ctx) }
	go func() { done <- Run(ctx, cfg, collector, noExecute) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("sync never started")
	}
	for i := 0; i < 3; i++ {
		select {
		case <-observed:
		case <-time.After(time.Second):
			t.Fatal("collection blocked behind sync")
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("agent did not stop")
	}
	w, err := open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.state.Snapshots) < 2 {
		t.Fatal("offline observations not durable")
	}
}

func TestOversizedObservationDoesNotEvictValidBacklog(t *testing.T) {
	w, err := open(config(t, "http://unused"))
	if err != nil {
		t.Fatal(err)
	}
	if err = w.collect(context.Background(), observation); err != nil {
		t.Fatal(err)
	}
	oversized := func(context.Context) (control.Snapshot, error) {
		return control.Snapshot{Messages: []messages.Message{{Text: strings.Repeat("a", 33<<20)}}}, nil
	}
	if err = w.collect(context.Background(), oversized); err != nil {
		t.Fatal(err)
	}
	if len(w.state.Snapshots) != 2 || w.state.Dropped != 0 {
		t.Fatal("oversized rejected observation evicted backlog")
	}
	if w.state.Snapshots[1].Collected || w.state.Snapshots[1].Error == "" {
		t.Fatal("oversized observation not marked failed")
	}
}
