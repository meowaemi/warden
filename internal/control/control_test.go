package control

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
	"warden/internal/messages"
)

func request(t *testing.T, h http.Handler, method, path, token string, body any) (int, []byte) {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &b)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes()
}
func command(id string) Command {
	return Command{ID: id, SiteID: "a", Kind: "restart_deployment", Namespace: "default", Name: "app", TargetUID: "uid-1", ExpiresAt: time.Now().Add(time.Hour).UTC()}
}
func TestAuthPersistenceAndFreshness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(s, "op", map[string]string{"a": "aa", "b": "bb"}, time.Minute)
	if code, _ := request(t, h, "GET", "/api/sites", "", nil); code != 401 {
		t.Fatal(code)
	}
	if code, _ := request(t, h, "POST", "/api/agent/sync", "bb", SyncRequest{SiteID: "a"}); code != 403 {
		t.Fatal(code)
	}
	now := time.Now().UTC()
	success := Snapshot{ID: "new", SiteID: "a", ObservedAt: now, Collected: true, Messages: []messages.Message{{Reason: "OOM", Severity: messages.Error, Outcome: messages.Reported}}}
	if code, b := request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a", Snapshots: []Snapshot{success}}); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	older := success
	older.ID = "older"
	older.ObservedAt = now.Add(-time.Hour)
	older.Messages = nil
	failure := Snapshot{ID: "failure", SiteID: "a", ObservedAt: now.Add(time.Millisecond), Error: "no API"}
	if code, b := request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a", Snapshots: []Snapshot{failure, older}}); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.data.Sites["a"].Latest.Messages) != 1 {
		t.Fatal("lost latest successful state")
	}
	if reopened.data.Sites["a"].LastError != "no API" {
		t.Fatal("backfill erased failure")
	}
	code, b := request(t, Handler(reopened, "op", map[string]string{"a": "aa"}, time.Minute), "GET", "/api/sites", "op", nil)
	var views []struct {
		Stale bool `json:"stale"`
	}
	if code != 200 || json.Unmarshal(b, &views) != nil || !views[0].Stale {
		t.Fatalf("%d %s", code, b)
	}
}
func TestCommandsRedeliveryIsolationAndResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, _ := Open(path)
	h := Handler(s, "op", map[string]string{"a": "aa", "b": "bb"}, time.Minute)
	c := command("one")
	for i := 0; i < 2; i++ {
		if code, b := request(t, h, "POST", "/api/commands", "op", c); code != 200 {
			t.Fatalf("%d %s", code, b)
		}
	}
	conflict := c
	conflict.Name = "different"
	if code, _ := request(t, h, "POST", "/api/commands", "op", conflict); code != 409 {
		t.Fatal(code)
	}
	if code, b := request(t, h, "POST", "/api/commands", "op", command("two")); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	for i := 0; i < 2; i++ {
		_, b := request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a"})
		var out SyncResponse
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Commands) != 1 || out.Commands[0].ID != "one" {
			t.Fatalf("redelivery: %s", b)
		}
	}
	_, b := request(t, h, "POST", "/api/agent/sync", "bb", SyncRequest{SiteID: "b"})
	var out SyncResponse
	_ = json.Unmarshal(b, &out)
	if len(out.Commands) != 0 {
		t.Fatal("site leak")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h = Handler(reopened, "op", map[string]string{"a": "aa"}, time.Minute)
	result := Result{CommandID: "one", Status: "applied", FinishedAt: time.Now().UTC()}
	req := SyncRequest{SiteID: "a", Results: []Result{result}}
	for i := 0; i < 2; i++ {
		code, b := request(t, h, "POST", "/api/agent/sync", "aa", req)
		if code != 200 {
			t.Fatalf("%d %s", code, b)
		}
		_ = json.Unmarshal(b, &out)
		if len(out.Commands) != 1 || out.Commands[0].ID != "two" {
			t.Fatalf("next: %s", b)
		}
	}
	req.Results[0].Status = "failed"
	if code, b := request(t, h, "POST", "/api/agent/sync", "aa", req); code != 200 {
		t.Fatal(code)
	} else {
		var response SyncResponse
		_ = json.Unmarshal(b, &response)
		if len(response.ResultConflicts) != 1 {
			t.Fatalf("expected explicit conflict receipt: %s", b)
		}
	}
}
func TestExpiryAndWriteRollback(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.json"))
	h := Handler(s, "op", map[string]string{"a": "aa"}, time.Minute)
	c := command("expire")
	if code, _ := request(t, h, "POST", "/api/commands", "op", c); code != 200 {
		t.Fatal(code)
	}
	s.data.Commands[0].ExpiresAt = time.Now().Add(-time.Second)
	code, b := request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a"})
	var out SyncResponse
	_ = json.Unmarshal(b, &out)
	if code != 200 || len(out.Commands) != 0 || s.data.Commands[0].Status != "expired" {
		t.Fatalf("%d %s", code, b)
	}
	broken, _ := Open(filepath.Join(t.TempDir(), "missing"))
	broken.path = t.TempDir()
	hb := Handler(broken, "op", map[string]string{"a": "aa"}, time.Minute)
	if code, _ := request(t, hb, "POST", "/api/commands", "op", command("fail")); code != 500 {
		t.Fatal(code)
	}
	if len(broken.data.Commands) != 0 {
		t.Fatal("failed persistence changed memory")
	}
}

func TestLateResultRefinesUnknownAndDoesNotWedgeSnapshots(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.json"))
	h := Handler(s, "op", map[string]string{"a": "aa"}, time.Minute)
	c := command("late")
	request(t, h, "POST", "/api/commands", "op", c)
	request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a"})
	s.data.Commands[0].ExpiresAt = time.Now().Add(-time.Second)
	request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a"})
	if s.data.Commands[0].Status != "unknown" {
		t.Fatal("dispatched expiry falsely definitive")
	}
	res := Result{CommandID: "late", Status: "applied", FinishedAt: time.Now().UTC()}
	code, b := request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a", Results: []Result{res}})
	if code != 200 || s.data.Commands[0].Status != "applied" {
		t.Fatalf("late refinement %d %s", code, b)
	}
	res.Status = "failed"
	snap := Snapshot{ID: "telemetry", SiteID: "a", ObservedAt: time.Now().UTC(), Collected: true}
	code, b = request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a", Results: []Result{res}, Snapshots: []Snapshot{snap}})
	var response SyncResponse
	_ = json.Unmarshal(b, &response)
	if code != 200 || len(response.AckIDs) != 1 || len(response.ResultAckIDs) != 1 || len(response.ResultConflicts) != 1 || s.data.Commands[0].Status != "applied" {
		t.Fatalf("conflict wedges data or mutates terminal outcome %d %s", code, b)
	}
}

func TestSuccessfulObservationWithBacklogWarningIsFresh(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.json"))
	h := Handler(s, "op", map[string]string{"a": "aa"}, time.Minute)
	snap := Snapshot{ID: "warning", SiteID: "a", ObservedAt: time.Now().UTC(), Collected: true, Error: "backlog overflow: 1 oldest observations discarded cumulatively"}
	code, b := request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a", Snapshots: []Snapshot{snap}})
	if code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	_, b = request(t, h, "GET", "/api/sites", "op", nil)
	var sites []struct {
		Stale     bool   `json:"stale"`
		LastError string `json:"last_error"`
	}
	if err := json.Unmarshal(b, &sites); err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Stale || sites[0].LastError == "" {
		t.Fatalf("freshness conflated with backlog warning: %s", b)
	}
}

func TestSlowSyncBodyDoesNotLockOtherRequests(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.json"))
	h := Handler(s, "op", map[string]string{"a": "aa"}, time.Minute)
	reader, writer := io.Pipe()
	defer writer.Close()
	defer reader.Close()
	r := httptest.NewRequest("POST", "/api/agent/sync", reader)
	r.Header.Set("Authorization", "Bearer aa")
	done := make(chan struct{})
	go func() { h.ServeHTTP(httptest.NewRecorder(), r); close(done) }()
	if _, err := writer.Write([]byte(`{"site_id":`)); err != nil {
		t.Fatal(err)
	}
	readDone := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		get := httptest.NewRequest("GET", "/api/sites", nil)
		get.Header.Set("Authorization", "Bearer op")
		h.ServeHTTP(w, get)
		readDone <- w.Code
	}()
	select {
	case code := <-readDone:
		if code != 200 {
			t.Fatal(code)
		}
	case <-time.After(time.Second):
		t.Fatal("slow POST holds global state lock")
	}
	writer.Close()
	<-done
}
func TestMalformedResultsDoNotBlockTelemetryOrChangeOtherSite(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.json"))
	h := Handler(s, "op", map[string]string{"a": "aa", "b": "bb"}, time.Minute)
	request(t, h, "POST", "/api/commands", "op", command("own-a"))
	request(t, h, "POST", "/api/agent/sync", "aa", SyncRequest{SiteID: "a"})
	snap := Snapshot{ID: "b-observation", SiteID: "b", ObservedAt: time.Now().UTC(), Collected: true}
	req := SyncRequest{SiteID: "b", Snapshots: []Snapshot{snap}, Results: []Result{{CommandID: "own-a", Status: "applied", FinishedAt: time.Now().UTC()}, {CommandID: "missing", Status: "nonsense"}}}
	code, b := request(t, h, "POST", "/api/agent/sync", "bb", req)
	var response SyncResponse
	_ = json.Unmarshal(b, &response)
	if code != 200 || len(response.AckIDs) != 1 || len(response.ResultAckIDs) != 2 || len(response.ResultConflicts) != 2 || s.data.Commands[0].Status != "dispatched" {
		t.Fatalf("invalid receipt blocked telemetry or spoofed another site: %d %s", code, b)
	}
}
