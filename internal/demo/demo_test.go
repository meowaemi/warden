package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
	"warden/internal/control"
)

// operator is the operator token of the test center.
const operator = "demo-test-operator"

// call sends one operator request to the demo handler and decodes the JSON
// answer into out, if out is not nil. It returns the HTTP status.
func call(t *testing.T, h http.Handler, method, path string, body, out any) int {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &b)
	r.Header.Set("Authorization", "Bearer "+operator)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if out != nil && w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, w.Body.Bytes())
		}
	}
	return w.Code
}

// wait polls cond until it holds, failing the test after a few seconds.
func wait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// siteView is the part of /api/sites the tests read.
type siteView struct {
	ID          string    `json:"id"`
	Stale       bool      `json:"stale"`
	LastSuccess time.Time `json:"last_success"`
}

// site returns the demo site, false if it is not registered yet.
func site(t *testing.T, h http.Handler) (siteView, bool) {
	var sites []siteView
	call(t, h, "GET", "/api/sites", nil, &sites)
	if len(sites) == 0 {
		return siteView{}, false
	}
	return sites[0], true
}

// statuses returns the status of each demo command by ID.
func statuses(t *testing.T, h http.Handler) map[string]string {
	var cs []control.Command
	call(t, h, "GET", "/api/commands?site_id=demo-site", nil, &cs)
	m := map[string]string{}
	for _, c := range cs {
		m[c.ID] = c.Status
	}
	return m
}

// history returns the stored snapshots of the demo site.
func history(t *testing.T, h http.Handler) []control.Snapshot {
	var hs []control.Snapshot
	call(t, h, "GET", "/api/history?site_id=demo-site", nil, &hs)
	return hs
}

// command returns a restart of the demo app that expires after ttl.
func command(id string, ttl time.Duration) control.Command {
	return control.Command{ID: id, SiteID: "demo-site", Kind: "restart_deployment", Namespace: "demo", Name: "demo-app", TargetUID: "demo-deployment-uid", ExpiresAt: time.Now().Add(ttl).UTC()}
}

// TestMemoryFix walks the operator's fix: OOM reported, memory raised on the
// owner the problem names, OOM gone.
func TestMemoryFix(t *testing.T) {
	dir := t.TempDir()
	st, err := control.Open(filepath.Join(dir, "center.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, cleanup, err := Start(ctx, st, operator, filepath.Join(dir, "agent.json"), 50*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	type problem struct {
		Reason string `json:"reason"`
		Target struct {
			Namespace string `json:"namespace"`
			Container string `json:"container"`
			OwnerKind string `json:"ownerKind"`
			OwnerName string `json:"ownerName"`
			OwnerUID  string `json:"ownerUID"`
		} `json:"target"`
	}
	oom := func() (problem, bool) {
		var sites []struct {
			Messages []problem `json:"messages"`
		}
		call(t, h, "GET", "/api/sites", nil, &sites)
		for _, s := range sites {
			for _, m := range s.Messages {
				if m.Reason == "OOMKilled" {
					return m, true
				}
			}
		}
		return problem{}, false
	}
	var p problem
	wait(t, "simulated OOM reported", func() bool { var ok bool; p, ok = oom(); return ok })

	fix := control.Command{ID: "raise-memory", SiteID: "demo-site", Kind: control.KindSetResources, TargetKind: p.Target.OwnerKind, Namespace: p.Target.Namespace, Name: p.Target.OwnerName, TargetUID: p.Target.OwnerUID, Container: p.Target.Container, Resources: &control.Resources{MemoryLimit: "256Mi"}, ExpiresAt: time.Now().Add(time.Minute).UTC()}
	if code := call(t, h, "POST", "/api/commands", fix, nil); code != http.StatusOK {
		t.Fatalf("queue fix from the problem's owner: HTTP %d", code)
	}
	wait(t, "fix applied", func() bool { return statuses(t, h)[fix.ID] == "applied" })
	wait(t, "OOM gone", func() bool { _, ok := oom(); return !ok })
}

// TestLinkOutage cuts the agent's link: commands wait or expire and the site
// goes stale; after reconnection the buffered snapshots arrive and the
// waiting command runs once.
func TestLinkOutage(t *testing.T) {
	dir := t.TempDir()
	st, err := control.Open(filepath.Join(dir, "center.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, cleanup, err := Start(ctx, st, operator, filepath.Join(dir, "agent.json"), 50*time.Millisecond, 300*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	var link struct{ Connected, Simulated bool }
	if call(t, h, "GET", "/api/demo/link", nil, &link); !link.Simulated || !link.Connected {
		t.Fatalf("demo link: %+v", link)
	}
	wait(t, "site registered and fresh", func() bool { s, ok := site(t, h); return ok && !s.Stale })

	call(t, h, "POST", "/api/demo/link", map[string]bool{"connected": false}, nil)
	before, _ := site(t, h)

	long, short := command("long", time.Minute), command("short", 300*time.Millisecond)
	for _, c := range []control.Command{long, short} {
		var got control.Command
		if code := call(t, h, "POST", "/api/commands", c, &got); code != http.StatusOK || got.Status != "queued" {
			t.Fatalf("queue %s: %d %+v", c.ID, code, got)
		}
	}

	wait(t, "short command expires", func() bool { return statuses(t, h)["short"] == "expired" })
	wait(t, "site goes stale", func() bool { s, _ := site(t, h); return s.Stale })
	if s := statuses(t, h)["long"]; s != "queued" {
		t.Fatalf("long command dispatched without a link: %s", s)
	}

	// Let the agent buffer a few observations while it is offline.
	time.Sleep(300 * time.Millisecond)
	seen := map[string]bool{}
	for _, s := range history(t, h) {
		seen[s.ID] = true
	}

	call(t, h, "POST", "/api/demo/link", map[string]bool{"connected": true}, nil)
	wait(t, "long command applied", func() bool { return statuses(t, h)["long"] == "applied" })
	wait(t, "site fresh again", func() bool { s, _ := site(t, h); return !s.Stale })

	uploaded, offline := 0, false
	for _, s := range history(t, h) {
		if !seen[s.ID] {
			uploaded++
			offline = offline || s.ObservedAt.After(before.LastSuccess)
		}
	}
	if uploaded < 2 || !offline {
		t.Fatalf("buffered offline observations not uploaded: %d new, offline=%v", uploaded, offline)
	}

	var again control.Command
	if code := call(t, h, "POST", "/api/commands", long, &again); code != http.StatusOK || again.Status != "applied" {
		t.Fatalf("same request ID did not return the original command: %d %+v", code, again)
	}
	conflict := long
	conflict.Name = "another-app"
	if code := call(t, h, "POST", "/api/commands", conflict, nil); code != http.StatusConflict {
		t.Fatalf("conflicting request ID: HTTP %d, want 409", code)
	}
	if s := statuses(t, h)["short"]; s != "expired" {
		t.Fatalf("expired command changed to %s", s)
	}
}
