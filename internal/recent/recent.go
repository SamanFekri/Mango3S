// Package recent keeps the per-connection "Collection History": the
// collections a user opened most recently, persisted across restarts.
package recent

import (
	"path/filepath"
	"sort"
	"sync"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// MaxPerProfile caps the history of one connection.
const MaxPerProfile = 50

// Entry is one recently opened collection.
type Entry struct {
	DB         string    `json:"db"`
	Coll       string    `json:"coll"`
	LastOpened time.Time `json:"lastOpened"`
	Count      int       `json:"count"`
}

type file struct {
	Version  int                `json:"version"`
	Profiles map[string][]Entry `json:"profiles"`
}

// Store persists history in collection_history.json.
type Store struct {
	mu   sync.Mutex
	path string
	now  func() time.Time
}

// New keeps history in dir/collection_history.json.
func New(dir string) *Store {
	return &Store{path: filepath.Join(dir, "collection_history.json"), now: time.Now}
}

func (s *Store) load() (file, error) {
	f := file{Version: 1, Profiles: map[string][]Entry{}}
	if err := config.ReadJSON(s.path, &f); err != nil {
		return f, err
	}
	if f.Profiles == nil {
		f.Profiles = map[string][]Entry{}
	}
	return f, nil
}

// Record notes that db.coll was opened on a connection.
func (s *Store) Record(profileID, db, coll string) error {
	if profileID == "" || db == "" || coll == "" {
		return apperr.Invalid("Connection, database and collection are required.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return err
	}
	list := f.Profiles[profileID]
	e := Entry{DB: db, Coll: coll, LastOpened: s.now().UTC(), Count: 1}
	out := []Entry{}
	for _, x := range list {
		if x.DB == db && x.Coll == coll {
			e.Count = x.Count + 1
			continue
		}
		out = append(out, x)
	}
	out = append([]Entry{e}, out...)
	if len(out) > MaxPerProfile {
		out = out[:MaxPerProfile]
	}
	f.Profiles[profileID] = out
	return config.WriteJSON(s.path, f)
}

// List returns the history of a connection, most recent first.
func (s *Store) List(profileID string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return nil, err
	}
	out := append([]Entry{}, f.Profiles[profileID]...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastOpened.After(out[j].LastOpened) })
	return out, nil
}

// Clear deletes a connection's history.
func (s *Store) Clear(profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return err
	}
	delete(f.Profiles, profileID)
	return config.WriteJSON(s.path, f)
}

// Copy duplicates a connection's history (used by "Duplicate connection").
func (s *Store) Copy(fromID, toID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return err
	}
	if list, ok := f.Profiles[fromID]; ok {
		f.Profiles[toID] = append([]Entry{}, list...)
		return config.WriteJSON(s.path, f)
	}
	return nil
}

// SetClock replaces time.Now (tests).
func (s *Store) SetClock(now func() time.Time) { s.now = now }
