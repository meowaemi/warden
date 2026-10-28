package control

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// MaxHistory is the number of snapshots kept per site.
const MaxHistory = 500

// MaxCommands caps the number of stored commands.
const MaxCommands = 2000

// MaxSites caps the number of sites.
const MaxSites = 100

// MaxBatch caps the snapshots or results in one sync.
const MaxBatch = 100

// MaxBody caps a request body.
const MaxBody = 4 << 20

// siteState is what the center knows about one site. Latest is the last
// successful snapshot; a later failed one only sets LastError.
type siteState struct {
	ID                   string     `json:"id"`
	LastContact          time.Time  `json:"last_contact"`
	LastSuccess          time.Time  `json:"last_success"`
	LastObservation      time.Time  `json:"last_observation"`
	LastCollectionFailed bool       `json:"last_collection_failed"`
	LastError            string     `json:"last_error"`
	Latest               *Snapshot  `json:"latest,omitempty"`
	History              []Snapshot `json:"history"`
}

// state is everything the center persists.
type state struct {
	Sites    map[string]*siteState `json:"sites"`
	Commands []Command             `json:"commands"`
	Results  map[string]Result     `json:"results"`
}

// Store is the center state kept in one JSON file.
type Store struct {
	mu   sync.Mutex
	path string
	data state
}

// Open loads the state file; a missing file means an empty state.
func Open(path string) (*Store, error) {
	s := &Store{path: path, data: state{Sites: map[string]*siteState{}, Commands: []Command{}, Results: map[string]Result{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 64<<20 {
		return nil, errors.New("state file too large")
	}
	if err = json.Unmarshal(b, &s.data); err != nil {
		return nil, err
	}
	if s.data.Sites == nil {
		s.data.Sites = map[string]*siteState{}
	}
	if s.data.Results == nil {
		s.data.Results = map[string]Result{}
	}
	if len(s.data.Sites) > MaxSites || len(s.data.Commands) > MaxCommands {
		return nil, errors.New("state exceeds limits")
	}
	return s, nil
}

// update applies fn to a copy of the state, writes it atomically and only
// then swaps it in. On any error the state stays as it was.
func (s *Store) update(fn func(*state) error) error {
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	var next state
	if err = json.Unmarshal(b, &next); err != nil {
		return err
	}
	if err = fn(&next); err != nil {
		return err
	}
	b, err = json.Marshal(next)
	if err != nil {
		return err
	}
	if len(b) > 64<<20 {
		return fail(409, "persistent state capacity reached")
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".warden-*")
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
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, s.path); err != nil {
		return err
	}
	if err = SyncDir(filepath.Dir(s.path)); err != nil {
		return err
	}
	s.data = next
	return nil
}

// SyncDir fsyncs a directory so a rename in it survives power loss.
// On Windows, which cannot fsync a directory, it does nothing.
func SyncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// due reports whether an open command has passed its deadline.
func due(d *state, now time.Time) bool {
	for _, c := range d.Commands {
		if !terminal(c.Status) && !now.Before(c.ExpiresAt) {
			return true
		}
	}
	return false
}

// terminal reports whether a command status is final. Only unknown can
// still be refined by a late result.
func terminal(status string) bool {
	return status == "applied" || status == "failed" || status == "expired" || status == "unknown"
}

// expire closes overdue commands: a queued one never ran and expires,
// a dispatched one may have run and becomes unknown.
func expire(d *state, now time.Time) {
	for i := range d.Commands {
		c := &d.Commands[i]
		if !terminal(c.Status) && !now.Before(c.ExpiresAt) {
			if c.Status == "dispatched" {
				c.Status = "unknown"
				c.Note = "Delivery deadline passed after dispatch; execution outcome unconfirmed"
			} else {
				c.Status = "expired"
				c.Note = "Command deadline passed before dispatch"
			}
		}
	}
}
