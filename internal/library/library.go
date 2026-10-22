// Package library persists saved queries, pipelines, shell scripts and SQL
// statements (library.json) and a bounded execution history (history.json)
// in the configuration directory.
package library

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"mango3s/internal/aggregate"
	"mango3s/internal/apperr"
	"mango3s/internal/charts"
	"mango3s/internal/config"
)

// Kinds of saved items and history entries.
const (
	KindQuery       = "query"
	KindAggregation = "aggregation"
	KindScript      = "script"
	KindSQL         = "sql"
	KindChart       = "chart"
)

func validKind(k string) bool {
	return k == KindQuery || k == KindAggregation || k == KindScript || k == KindSQL || k == KindChart
}

// QueryPayload is a saved find query.
type QueryPayload struct {
	Filter     string `json:"filter"`
	Projection string `json:"projection"`
	Sort       string `json:"sort"`
	Skip       int64  `json:"skip"`
	Limit      int64  `json:"limit"`
}

// Item is a saved, named entry.
type Item struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	DB          string            `json:"db"`
	Coll        string            `json:"coll"`
	Query       *QueryPayload     `json:"query,omitempty"`
	Stages      []aggregate.Stage `json:"stages,omitempty"`
	Text        string            `json:"text,omitempty"`  // shell script or SQL
	Chart       *charts.Spec      `json:"chart,omitempty"` // a chart; Query holds its filter
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// Validate checks that the payload matches the kind.
func (it Item) Validate() error {
	if !validKind(it.Kind) {
		return apperr.Invalid("Unknown item type %q.", it.Kind)
	}
	if strings.TrimSpace(it.Name) == "" {
		return apperr.Invalid("A name is required.")
	}
	switch it.Kind {
	case KindQuery:
		if it.Query == nil {
			return apperr.Invalid("A saved query needs a query.")
		}
	case KindAggregation:
		if len(it.Stages) == 0 {
			return apperr.Invalid("A saved pipeline needs at least one stage.")
		}
	case KindChart:
		if it.Chart == nil || strings.TrimSpace(it.Chart.X) == "" {
			return apperr.Invalid("A saved chart needs a field for its x axis.")
		}
		if it.Coll == "" {
			return apperr.Invalid("A saved chart belongs to a collection.")
		}
	default:
		if strings.TrimSpace(it.Text) == "" {
			return apperr.Invalid("The %s is empty.", it.Kind)
		}
	}
	return nil
}

type libraryFile struct {
	Version int    `json:"version"`
	Items   []Item `json:"items"`
}

// Store holds saved items.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore keeps items in dir/library.json.
func NewStore(dir string) *Store { return &Store{path: filepath.Join(dir, "library.json")} }

func (s *Store) load() ([]Item, error) {
	var f libraryFile
	if err := config.ReadJSON(s.path, &f); err != nil {
		return nil, err
	}
	return f.Items, nil
}

// List returns items, most recently updated first; kind "" means all.
func (s *Store) List(kind string) ([]Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.load()
	if err != nil {
		return nil, err
	}
	out := []Item{}
	for _, it := range items {
		if kind == "" || it.Kind == kind {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Save inserts (empty ID) or updates an item.
func (s *Store) Save(it Item) (Item, error) {
	it.Name = strings.TrimSpace(it.Name)
	if err := it.Validate(); err != nil {
		return Item{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.load()
	if err != nil {
		return Item{}, err
	}
	now := time.Now().UTC()
	it.UpdatedAt = now
	if it.ID == "" {
		it.ID = config.NewID()
		it.CreatedAt = now
		items = append(items, it)
	} else {
		found := false
		for i := range items {
			if items[i].ID == it.ID {
				it.CreatedAt = items[i].CreatedAt
				items[i] = it
				found = true
			}
		}
		if !found {
			return Item{}, apperr.New(apperr.CodeNotFound, "Saved item not found.")
		}
	}
	return it, config.WriteJSON(s.path, libraryFile{Version: 1, Items: items})
}

// Delete removes an item; deleting a missing item is not an error.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.load()
	if err != nil {
		return err
	}
	out := items[:0]
	for _, it := range items {
		if it.ID != id {
			out = append(out, it)
		}
	}
	return config.WriteJSON(s.path, libraryFile{Version: 1, Items: out})
}

// Entry is one executed query, pipeline, script or SQL statement.
type Entry struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	ConnName   string            `json:"connName"`
	DB         string            `json:"db"`
	Coll       string            `json:"coll"`
	Summary    string            `json:"summary"` // one-line display text
	Query      *QueryPayload     `json:"query,omitempty"`
	Stages     []aggregate.Stage `json:"stages,omitempty"`
	Text       string            `json:"text,omitempty"`
	At         time.Time         `json:"at"`
	DurationMs int64             `json:"durationMs"`
	OK         bool              `json:"ok"`
	Error      string            `json:"error,omitempty"`
}

// History keeps the most recent entries, newest first.
type History struct {
	mu      sync.Mutex
	path    string
	max     int
	entries []Entry
	loaded  bool
}

// NewHistory keeps up to max entries in dir/history.json.
func NewHistory(dir string, max int) *History {
	if max <= 0 {
		max = 500
	}
	return &History{path: filepath.Join(dir, "history.json"), max: max}
}

type historyFile struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

func (h *History) ensureLoaded() error {
	if h.loaded {
		return nil
	}
	var f historyFile
	if err := config.ReadJSON(h.path, &f); err != nil {
		return err
	}
	h.entries = f.Entries
	h.loaded = true
	return nil
}

func sameRun(a, b Entry) bool {
	return a.Kind == b.Kind && a.DB == b.DB && a.Coll == b.Coll && a.ConnName == b.ConnName && a.Summary == b.Summary
}

// Add records an entry. Re-running the newest entry updates it instead of
// adding a duplicate.
func (h *History) Add(e Entry) error {
	if !validKind(e.Kind) {
		return apperr.Invalid("Unknown history type %q.", e.Kind)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ensureLoaded(); err != nil {
		return err
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	if len(h.entries) > 0 && sameRun(h.entries[0], e) {
		e.ID = h.entries[0].ID
		h.entries[0] = e
	} else {
		e.ID = config.NewID()
		h.entries = append([]Entry{e}, h.entries...)
		if len(h.entries) > h.max {
			h.entries = h.entries[:h.max]
		}
	}
	return config.WriteJSON(h.path, historyFile{Version: 1, Entries: h.entries})
}

// List returns entries newest first, filtered by kind and a case-insensitive
// search over summary, database and collection.
func (h *History) List(kind, search string, limit int) ([]Entry, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ensureLoaded(); err != nil {
		return nil, err
	}
	search = strings.ToLower(strings.TrimSpace(search))
	out := []Entry{}
	for _, e := range h.entries {
		if kind != "" && e.Kind != kind {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(e.Summary+" "+e.DB+"."+e.Coll+" "+e.ConnName), search) {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Clear deletes all history.
func (h *History) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = nil
	h.loaded = true
	return config.WriteJSON(h.path, historyFile{Version: 1, Entries: []Entry{}})
}
