// Package plans holds short-lived previews of destructive operations. A
// preview (dry run) registers a plan; applying requires the plan's ID, so
// nothing destructive can run without a preview the user has seen. Plans are
// single-use and expire.
package plans

import (
	"sync"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// DefaultTTL is how long a preview stays valid.
const DefaultTTL = 30 * time.Minute

// Store keeps plans of any type in memory.
type Store[T any] struct {
	mu    sync.Mutex
	ttl   time.Duration
	now   func() time.Time
	items map[string]entry[T]
}

type entry[T any] struct {
	plan    T
	expires time.Time
}

// New returns a store whose plans expire after ttl.
func New[T any](ttl time.Duration) *Store[T] {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Store[T]{ttl: ttl, now: time.Now, items: map[string]entry[T]{}}
}

// SetClock replaces the time source (tests).
func (s *Store[T]) SetClock(now func() time.Time) { s.now = now }

// Put registers a plan and returns its ID.
func (s *Store[T]) Put(p T) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	id := config.NewID()
	s.items[id] = entry[T]{plan: p, expires: s.now().Add(s.ttl)}
	return id
}

// Get returns a plan without consuming it.
func (s *Store[T]) Get(id string) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok || s.now().After(e.expires) {
		var zero T
		delete(s.items, id)
		return zero, apperr.New(apperr.CodeNotFound, "The preview has expired or was already applied. Run the preview again.")
	}
	return e.plan, nil
}

// Take returns and removes a plan, so a plan can be applied at most once.
func (s *Store[T]) Take(id string) (T, error) {
	p, err := s.Get(id)
	if err == nil {
		s.mu.Lock()
		delete(s.items, id)
		s.mu.Unlock()
	}
	return p, err
}

func (s *Store[T]) gcLocked() {
	now := s.now()
	for id, e := range s.items {
		if now.After(e.expires) {
			delete(s.items, id)
		}
	}
}
