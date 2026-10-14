package store

import (
	"sync"
	"warden/internal/messages"
)

type Store struct {
	mu    sync.Mutex
	items map[string]messages.Message
}

func New() *Store {
	return &Store{items: make(map[string]messages.Message)}
}

// Update replaces the stored problems with found, the messages of one check.
// A known problem keeps its first-seen Time, everything else is refreshed.
// Problems absent from found are deleted.
// It returns the problems that appeared and the ones that disappeared.
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
