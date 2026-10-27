package control

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"
	"warden/internal/messages"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
var dnsName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

func validName(s string) bool { return len(s) > 0 && len(s) <= 253 && dnsName.MatchString(s) }

type httpError struct {
	code int
	text string
}

func (e httpError) Error() string   { return e.text }
func fail(code int, s string) error { return httpError{code, s} }
func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, err error) {
	var he httpError
	if errors.As(err, &he) {
		write(w, he.code, map[string]string{"error": he.text})
	} else {
		write(w, 500, map[string]string{"error": "persistent storage failure"})
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBody)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fail(400, "invalid or oversized JSON payload")
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fail(400, "expected one JSON value")
	}
	return nil
}
func equalToken(a, b string) bool {
	return a != "" && b != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func Handler(st *Store, operatorToken string, siteTokens map[string]string, staleAfter time.Duration) http.Handler {
	if staleAfter <= 0 {
		staleAfter = 30 * time.Second
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			token = ""
		}
		site := ""
		if r.URL.Path == "/api/agent/sync" {
			for id, t := range siteTokens {
				if equalToken(token, t) {
					if site != "" {
						write(w, 401, map[string]string{"error": "ambiguous site credential"})
						return
					}
					site = id
				}
			}
			if site == "" {
				write(w, 401, map[string]string{"error": "unauthorized"})
				return
			}
		} else if !equalToken(token, operatorToken) {
			write(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		var inputCommand Command
		var inputSync SyncRequest
		if r.Method == http.MethodPost {
			var body any
			switch r.URL.Path {
			case "/api/commands":
				body = &inputCommand
			case "/api/agent/sync":
				body = &inputSync
			}
			if body != nil {
				if err := decode(w, r, body); err != nil {
					problem(w, err)
					return
				}
			}
		}
		st.mu.Lock()
		defer st.mu.Unlock()
		now := time.Now().UTC()
		switch r.URL.Path {
		case "/api/sites":
			if r.Method != "GET" {
				w.WriteHeader(405)
				return
			}
			views := []any{}
			for _, s := range st.data.Sites {
				ms := []any{}
				if s.Latest != nil {
					for _, m := range s.Latest.Messages {
						ms = append(ms, struct {
							messages.Message
							Key string `json:"key"`
						}{m, m.Key()})
					}
				}
				views = append(views, map[string]any{"id": s.ID, "last_contact": s.LastContact, "last_success": s.LastSuccess, "stale": s.LastSuccess.IsZero() || now.Sub(s.LastSuccess) > staleAfter || s.LastCollectionFailed, "messages": ms, "last_error": s.LastError})
			}
			write(w, 200, views)
		case "/api/history":
			if r.Method != "GET" {
				w.WriteHeader(405)
				return
			}
			h := []Snapshot{}
			if s := st.data.Sites[r.URL.Query().Get("site_id")]; s != nil {
				h = s.History
			}
			write(w, 200, h)
		case "/api/commands":
			if r.Method == "GET" {
				if err := st.update(func(d *state) error { expire(d, now); return nil }); err != nil {
					problem(w, err)
					return
				}
				cs := []Command{}
				for _, c := range st.data.Commands {
					if r.URL.Query().Get("site_id") == "" || c.SiteID == r.URL.Query().Get("site_id") {
						cs = append(cs, c)
					}
				}
				write(w, 200, cs)
				return
			}
			if r.Method != "POST" {
				w.WriteHeader(405)
				return
			}
			c := inputCommand
			err := st.update(func(d *state) error {
				for _, old := range d.Commands {
					if old.ID == c.ID {
						a, b := old, c
						a.CreatedAt = time.Time{}
						a.Status = ""
						a.Note = ""
						b.CreatedAt = time.Time{}
						b.Status = ""
						b.Note = ""
						if !reflect.DeepEqual(a, b) {
							return fail(409, "command ID already has different content")
						}
						c = old
						return nil
					}
				}
				if !identifier.MatchString(c.ID) || siteTokens[c.SiteID] == "" || c.Kind != "restart_deployment" || !validName(c.Namespace) || !validName(c.Name) || !identifier.MatchString(c.TargetUID) || !c.ExpiresAt.After(now) || c.ExpiresAt.After(now.Add(24*time.Hour)) {
					return fail(400, "invalid command, unknown site, or deadline outside next 24 hours")
				}
				if c.Status != "" || !c.CreatedAt.IsZero() || c.Note != "" {
					return fail(400, "server-managed command fields must be omitted")
				}
				if len(d.Commands) >= MaxCommands {
					return fail(409, "command capacity reached; archive state before accepting more")
				}
				c.CreatedAt = now
				c.Status = "queued"
				d.Commands = append(d.Commands, c)
				return nil
			})
			if err != nil {
				problem(w, err)
				return
			}
			write(w, 200, c)
		case "/api/agent/sync":
			if r.Method != "POST" {
				w.WriteHeader(405)
				return
			}
			req := inputSync
			if req.SiteID != site {
				problem(w, fail(403, "site identity mismatch"))
				return
			}
			if len(req.Snapshots) > MaxBatch || len(req.Results) > MaxBatch {
				problem(w, fail(400, "batch exceeds 100 entries"))
				return
			}
			resp := SyncResponse{AckIDs: []string{}, ResultAckIDs: []string{}, Commands: []Command{}}
			err := st.update(func(d *state) error {
				s := d.Sites[site]
				if s == nil {
					if len(d.Sites) >= MaxSites {
						return fail(409, "site capacity reached")
					}
					s = &siteState{ID: site, History: []Snapshot{}}
					d.Sites[site] = s
				}
				s.LastContact = now
				for _, snap := range req.Snapshots {
					if snap.SiteID != site {
						return fail(403, "snapshot site identity mismatch")
					}
					if !identifier.MatchString(snap.ID) || snap.ObservedAt.IsZero() || snap.ObservedAt.After(now.Add(time.Minute)) || len(snap.Messages) > 2000 || len(snap.Error) > 4096 {
						return fail(400, "invalid snapshot")
					}
					exists := false
					for _, old := range s.History {
						if old.ID == snap.ID {
							if !reflect.DeepEqual(old, snap) {
								return fail(409, "snapshot ID conflict")
							}
							exists = true
							break
						}
					}
					if !exists {
						s.History = append(s.History, snap)
						if len(s.History) > MaxHistory {
							s.History = s.History[len(s.History)-MaxHistory:]
						}
						if snap.ObservedAt.After(s.LastObservation) {
							s.LastObservation = snap.ObservedAt
							s.LastCollectionFailed = !snap.Collected
							s.LastError = snap.Error
							if !snap.Collected && s.LastError == "" {
								s.LastError = "collection failed"
							}
						}
						if snap.Collected && snap.ObservedAt.After(s.LastSuccess) {
							copy := snap
							s.Latest = &copy
							s.LastSuccess = snap.ObservedAt
						}
					}
					resp.AckIDs = append(resp.AckIDs, snap.ID)
				}
				for _, res := range req.Results {
					reject := func(reason string) {
						resp.ResultAckIDs = append(resp.ResultAckIDs, res.CommandID)
						resp.ResultConflicts = append(resp.ResultConflicts, ResultConflict{CommandID: res.CommandID, Reason: reason})
					}
					idx := -1
					for i, c := range d.Commands {
						if c.ID == res.CommandID && c.SiteID == site {
							idx = i
							break
						}
					}
					if idx < 0 {
						reject("unknown command for authenticated site")
						continue
					}
					c := &d.Commands[idx]
					if res.Status != "applied" && res.Status != "failed" && res.Status != "unknown" && res.Status != "expired" {
						reject("invalid result status")
						continue
					}
					if res.FinishedAt.IsZero() || res.FinishedAt.After(now.Add(time.Minute)) || len(res.Note) > 4096 {
						reject("invalid result timestamp or note")
						continue
					}
					old, hasResult := d.Results[c.ID]
					if hasResult && reflect.DeepEqual(old, res) {
						resp.ResultAckIDs = append(resp.ResultAckIDs, c.ID)
						continue
					}
					if c.Status == "queued" {
						reject("command was not dispatched")
						continue
					}
					if terminal(c.Status) && c.Status != "unknown" {
						reject("definitive command outcome already stored")
						continue
					}
					if hasResult && old.Status == "unknown" && res.Status == "unknown" {
						reject("different unknown outcome already stored")
						continue
					}
					d.Results[c.ID] = res
					c.Status = res.Status
					c.Note = res.Note
					resp.ResultAckIDs = append(resp.ResultAckIDs, c.ID)
				}
				expire(d, now)
				for i := range d.Commands {
					c := &d.Commands[i]
					if c.SiteID == site && !terminal(c.Status) {
						c.Status = "dispatched"
						resp.Commands = append(resp.Commands, *c)
						break
					}
				}
				return nil
			})
			if err != nil {
				problem(w, err)
				return
			}
			write(w, 200, resp)
		default:
			write(w, 404, map[string]string{"error": fmt.Sprintf("unknown route %s", r.URL.Path)})
		}
	})
}
