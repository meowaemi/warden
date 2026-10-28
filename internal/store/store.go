// Package store keeps the current problems of the local mode in memory.
package store

import (
	"sort"
	"sync"
	"warden/internal/messages"
)

// Store is a concurrency-safe set of problems keyed by Message.Key.
type Store struct {
	mu    sync.Mutex
	items map[string]messages.Message
}

// New returns an empty store.
func New() *Store {
	return &Store{items: make(map[string]messages.Message)}
}

// Update replaces the problems with found and returns those that appeared
// and disappeared. A known problem keeps its first-seen time.
func (s *Store) Update(found []messages.Message) (added, removed []messages.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]bool, len(found))
	for _, msg := range found {
		key := msg.Key()
		seen[key] = true

		if old, ok := s.items[key]; ok {
			msg.Time = old.Time
		} else {
			added = append(added, msg)
		}
		s.items[key] = msg
	}

	for key, msg := range s.items {
		if !seen[key] {
			removed = append(removed, msg)
			delete(s.items, key)
		}
	}

	return added, removed
}

// List returns the problems, oldest first.
func (s *Store) List() []messages.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := make([]messages.Message, 0, len(s.items))
	for _, msg := range s.items {
		list = append(list, msg)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Time.Before(list[j].Time)
	})
	return list
}
