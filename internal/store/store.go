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
