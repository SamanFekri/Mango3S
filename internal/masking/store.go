package masking

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// RuleSet is a named, reusable list of rules (e.g. "PII", "Payments").
type RuleSet struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Rules     []Rule    `json:"rules"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store persists rule sets in masking.json.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore keeps rule sets in dir/masking.json.
func NewStore(dir string) *Store { return &Store{path: filepath.Join(dir, "masking.json")} }

type file struct {
	Version  int       `json:"version"`
	RuleSets []RuleSet `json:"ruleSets"`
}

func (s *Store) load() (file, error) {
	var f file
	err := config.ReadJSON(s.path, &f)
	return f, err
}

// List returns all rule sets by name.
func (s *Store) List() ([]RuleSet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return nil, err
	}
	out := append([]RuleSet{}, f.RuleSets...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// Get returns one rule set.
func (s *Store) Get(id string) (RuleSet, error) {
	list, err := s.List()
	if err != nil {
		return RuleSet{}, err
	}
	for _, r := range list {
		if r.ID == id {
			return r, nil
		}
	}
	return RuleSet{}, apperr.New(apperr.CodeNotFound, "Masking rule set not found.")
}

// Save validates (with a throwaway key) and stores a rule set.
func (s *Store) Save(rs RuleSet) (RuleSet, error) {
	rs.Name = strings.TrimSpace(rs.Name)
	if rs.Name == "" {
		return RuleSet{}, apperr.Invalid("A name is required.")
	}
	if _, err := Compile(rs.Rules, []byte("validation-only-key-0123456789")); err != nil {
		return RuleSet{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return RuleSet{}, err
	}
	rs.UpdatedAt = time.Now().UTC()
	if rs.ID == "" {
		rs.ID = config.NewID()
		f.RuleSets = append(f.RuleSets, rs)
	} else {
		found := false
		for i := range f.RuleSets {
			if f.RuleSets[i].ID == rs.ID {
				f.RuleSets[i] = rs
				found = true
			}
		}
		if !found {
			return RuleSet{}, apperr.New(apperr.CodeNotFound, "Masking rule set not found.")
		}
	}
	f.Version = 1
	return rs, config.WriteJSON(s.path, f)
}

// Delete removes a rule set.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return err
	}
	out := f.RuleSets[:0]
	for _, r := range f.RuleSets {
		if r.ID != id {
			out = append(out, r)
		}
	}
	f.RuleSets = out
	f.Version = 1
	return config.WriteJSON(s.path, f)
}
