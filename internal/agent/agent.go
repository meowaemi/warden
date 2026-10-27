package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"warden/internal/control"
)

type Config struct {
	SiteID, CenterURL, Token, StatePath string
	Interval                            time.Duration
	MaxSnapshots                        int
}
type Collector func(context.Context) (control.Snapshot, error)
type Executor func(context.Context, control.Command) control.Result

type diskState struct {
	SiteID    string                    `json:"site_id"`
	Snapshots []control.Snapshot        `json:"snapshots"`
	Results   map[string]control.Result `json:"results"`
	Ledger    map[string]control.Result `json:"ledger"`
	Dropped   uint64                    `json:"dropped_snapshots"`
}
type worker struct {
	mu     sync.Mutex
	cfg    Config
	state  diskState
	client *http.Client
}

func open(cfg Config) (*worker, error) {
	w := &worker{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
	b, err := os.ReadFile(cfg.StatePath)
	if err == nil {
		if len(b) > 128<<20 {
			return nil, errors.New("agent state exceeds 128 MiB")
		}
		if err = json.Unmarshal(b, &w.state); err != nil {
			return nil, fmt.Errorf("decode agent state: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if w.state.SiteID != "" && w.state.SiteID != cfg.SiteID {
		return nil, errors.New("agent state belongs to another site")
	}
	w.state.SiteID = cfg.SiteID
	if w.state.Results == nil {
		w.state.Results = map[string]control.Result{}
	}
	if w.state.Ledger == nil {
		w.state.Ledger = map[string]control.Result{}
	}
	for id, r := range w.state.Ledger {
		if r.Status == "executing" {
			r.Status = "unknown"
			r.Note = "agent restarted during execution; manual reconciliation required"
			r.FinishedAt = time.Now().UTC()
			w.state.Ledger[id] = r
			w.state.Results[id] = r
		}
	}
	if err := w.save(); err != nil {
		return nil, err
	}
	return w, nil
}
func (w *worker) save() error {
	b, err := json.Marshal(w.state)
	if err != nil {
		return err
	}
	dir := filepath.Dir(w.cfg.StatePath)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".warden-agent-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, w.cfg.StatePath); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func (w *worker) collect(ctx context.Context, collect Collector) error {
	s, err := collect(ctx)
	if ctx.Err() != nil {
		return nil
	}
	s.SiteID = w.cfg.SiteID
	s.ObservedAt = time.Now().UTC()
	id := make([]byte, 16)
	if _, e := rand.Read(id); e != nil {
		return e
	}
	s.ID = hex.EncodeToString(id)
	if err != nil {
		s.Collected = false
		s.Error = err.Error()
		s.Messages = nil
	} else {
		s.Collected = true
	}
	if len(s.Error) > 3000 {
		s.Error = s.Error[:3000]
	}
	if len(s.Messages) > 2000 {
		s.Messages = nil
		s.Collected = false
		s.Error = "observation exceeded 2000 messages; payload discarded"
	}
	encoded, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(encoded) > 256<<10 {
		s.Messages = nil
		s.Collected = false
		s.Error = "observation exceeded 256 KiB; payload discarded"
		encoded, e = json.Marshal(s)
		if e != nil {
			return e
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.state.Snapshots) >= w.cfg.MaxSnapshots {
		n := len(w.state.Snapshots) - w.cfg.MaxSnapshots + 1
		w.state.Snapshots = w.state.Snapshots[n:]
		w.state.Dropped += uint64(n)
		log.Printf("agent snapshot backlog full: discarded %d oldest observations; total=%d", n, w.state.Dropped)
	}
	queueBytes := len(encoded)
	for _, old := range w.state.Snapshots {
		v, e := json.Marshal(old)
		if e != nil {
			return e
		}
		queueBytes += len(v)
	}
	for queueBytes > 32<<20 && len(w.state.Snapshots) > 0 {
		v, _ := json.Marshal(w.state.Snapshots[0])
		queueBytes -= len(v)
		w.state.Snapshots = w.state.Snapshots[1:]
		w.state.Dropped++
		log.Printf("agent backlog exceeded 32 MiB: oldest observation discarded")
	}
	if w.state.Dropped > 0 {
		if s.Error != "" {
			s.Error += "; "
		}
		s.Error += fmt.Sprintf("backlog overflow: %d oldest observations discarded cumulatively", w.state.Dropped)
	}
	w.state.Snapshots = append(w.state.Snapshots, s)
	return w.save()
}
func (w *worker) syncOnce(ctx context.Context, execute Executor) error {
	w.mu.Lock()
	n := len(w.state.Snapshots)
	if n > 100 {
		n = 100
	}
	q := control.SyncRequest{SiteID: w.cfg.SiteID, Snapshots: append([]control.Snapshot(nil), w.state.Snapshots[:n]...)}
	for _, r := range w.state.Results {
		q.Results = append(q.Results, r)
	}
	sort.Slice(q.Results, func(i, j int) bool {
		if q.Results[i].FinishedAt.Equal(q.Results[j].FinishedAt) {
			return q.Results[i].CommandID < q.Results[j].CommandID
		}
		return q.Results[i].FinishedAt.Before(q.Results[j].FinishedAt)
	})
	if len(q.Results) > 100 {
		q.Results = q.Results[:100]
	}
	w.mu.Unlock()
	b, err := json.Marshal(q)
	if err != nil {
		return err
	}
	for len(b) > 3<<20 && len(q.Snapshots) > 0 {
		q.Snapshots = q.Snapshots[:len(q.Snapshots)-1]
		b, err = json.Marshal(q)
		if err != nil {
			return err
		}
	}
	if len(b) > 3<<20 {
		return errors.New("result batch exceeds request limit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(w.cfg.CenterURL, "/")+"/api/agent/sync", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+w.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		log.Printf("agent sync unavailable: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("agent sync rejected: HTTP %d", resp.StatusCode)
		return nil
	}
	var reply control.SyncResponse
	if err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&reply); err != nil {
		log.Printf("agent sync invalid response: %v", err)
		return nil
	}
	for _, conflict := range reply.ResultConflicts {
		log.Printf("agent result %s rejected by center: %s", conflict.CommandID, conflict.Reason)
	}
	sent := map[string]bool{}
	for _, s := range q.Snapshots {
		sent[s.ID] = true
	}
	ack := map[string]bool{}
	for _, id := range reply.AckIDs {
		if sent[id] {
			ack[id] = true
		}
	}
	w.mu.Lock()
	kept := w.state.Snapshots[:0]
	for _, s := range w.state.Snapshots {
		if !ack[s.ID] {
			kept = append(kept, s)
		}
	}
	w.state.Snapshots = kept
	sentResults := map[string]bool{}
	for _, r := range q.Results {
		sentResults[r.CommandID] = true
	}
	for _, id := range reply.ResultAckIDs {
		if sentResults[id] {
			delete(w.state.Results, id)
		}
	}
	err = w.save()
	w.mu.Unlock()
	if err != nil {
		return err
	}
	if len(reply.Commands) > 1 {
		return errors.New("center returned more than one command")
	}
	for _, cmd := range reply.Commands {
		if err = w.execute(ctx, cmd, execute); err != nil {
			return err
		}
	}
	return nil
}
func (w *worker) execute(ctx context.Context, c control.Command, execute Executor) error {
	if ctx.Err() != nil {
		return nil
	}
	if c.ID == "" || c.SiteID != w.cfg.SiteID {
		return errors.New("invalid command identity")
	}
	w.mu.Lock()
	if old, ok := w.state.Ledger[c.ID]; ok {
		w.state.Results[c.ID] = old
		err := w.save()
		w.mu.Unlock()
		return err
	}
	if len(w.state.Ledger) >= 10000 {
		w.mu.Unlock()
		return errors.New("execution ledger full; archive state before continuing")
	}
	r := control.Result{CommandID: c.ID, Status: "executing"}
	w.state.Ledger[c.ID] = r
	err := w.save()
	w.mu.Unlock()
	if err != nil {
		return err
	}
	switch {
	case !c.ExpiresAt.After(time.Now()):
		r.Status = "expired"
		r.Note = "expired before execution"
	case c.Kind != "restart_deployment" || c.TargetUID == "" || c.Namespace == "" || c.Name == "":
		r.Status = "failed"
		r.Note = "unsupported or incomplete command"
	default:
		runCtx, cancel := context.WithDeadline(ctx, c.ExpiresAt)
		r = safeExecute(runCtx, c, execute)
		cancel()
		r.CommandID = c.ID
		if r.Status != "applied" && r.Status != "failed" && r.Status != "expired" && r.Status != "unknown" {
			r.Status = "unknown"
			r.Note = "executor returned an invalid outcome"
		}
	}
	r.FinishedAt = time.Now().UTC()
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state.Ledger[c.ID] = r
	w.state.Results[c.ID] = r
	return w.save()
}

func safeExecute(ctx context.Context, c control.Command, execute Executor) (r control.Result) {
	defer func() {
		if recover() != nil {
			r = control.Result{CommandID: c.ID, Status: "unknown", Note: "executor panicked; manual reconciliation required"}
		}
	}()
	r = execute(ctx, c)
	if len(r.Note) > 4096 {
		r.Note = r.Note[:4096]
	}
	return
}

func Run(ctx context.Context, cfg Config, collect Collector, execute Executor) error {
	if cfg.SiteID == "" || cfg.CenterURL == "" || cfg.Token == "" || cfg.StatePath == "" || collect == nil || execute == nil {
		return errors.New("agent requires site, center URL, token, state path, collector and executor")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.MaxSnapshots <= 0 {
		cfg.MaxSnapshots = 1000
	}
	if cfg.MaxSnapshots > 10000 {
		return errors.New("MaxSnapshots exceeds 10000")
	}
	w, err := open(cfg)
	if err != nil {
		return err
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	errs := make(chan error, 1)
	go func() {
		defer close(done)
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		for {
			if e := w.collect(child, collect); e != nil {
				errs <- e
				cancel()
				return
			}
			select {
			case <-child.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	defer func() { cancel(); <-done }()
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		if err = w.syncOnce(child, execute); err != nil {
			return err
		}
		select {
		case e := <-errs:
			return e
		case <-child.Done():
			select {
			case e := <-errs:
				return e
			default:
				return nil
			}
		case <-ticker.C:
		}
	}
}
