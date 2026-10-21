package api

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mango3s/internal/apperr"
	"mango3s/internal/backup"
)

// Storage cleanup (Settings › Diagnostics › Storage): shows what Mango3S keeps
// on disk and lets anyone free the parts that are safe to lose — logs, caches
// and leftover temporary files, plus (opt-in) query history. Connections,
// settings, saved queries, tasks and backups are never touched here.

// StorageItem is one category of files.
type StorageItem struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Bytes       int64  `json:"bytes"`
	Files       int    `json:"files"`
	// Clearable is false for data shown only for information (backups).
	Clearable bool `json:"clearable"`
	// Recommended items are pre-selected: losing them costs nothing.
	Recommended bool `json:"recommended"`
}

// StorageUsage is the whole overview.
type StorageUsage struct {
	ConfigDir string        `json:"configDir"`
	Items     []StorageItem `json:"items"`
}

// StorageCleanResult reports what ClearStorage freed.
type StorageCleanResult struct {
	FreedBytes int64        `json:"freedBytes"`
	Cleared    []string     `json:"cleared"`
	Skipped    []string     `json:"skipped"` // human-readable reasons
	Usage      StorageUsage `json:"usage"`
}

// tempPatterns are the temporary files Mango3S creates in the OS temp folder.
var tempPatterns = []string{"mango3s-update-*", "mango3s-ai-*", "mango3s-tool-*.yaml", "mango3s-api-session.json"}

type storageCategory struct {
	item  StorageItem
	paths func(a *App) []string        // files/folders counted for the size
	clear func(a *App) (string, error) // returns a skip reason, if any
}

func (a *App) storageCategories() []storageCategory {
	dir := a.store.Dir()
	return []storageCategory{
		{
			item: StorageItem{ID: "logs", Label: "Logs", Recommended: true, Clearable: true,
				Description: "Diagnostic log files. The current log is emptied, older ones are deleted."},
			paths: func(a *App) []string { return []string{filepath.Join(dir, "logs")} },
			clear: func(a *App) (string, error) { return "", a.clearLogs(filepath.Join(dir, "logs")) },
		},
		{
			item: StorageItem{ID: "cache", Label: "Cached server info", Recommended: true, Clearable: true,
				Description: "About and support links saved from the Mango3S server; fetched again when needed."},
			paths: func(a *App) []string { return []string{filepath.Join(dir, appInfoFile)} },
			clear: func(a *App) (string, error) {
				a.update.mu.Lock()
				a.update.appInfo = nil
				a.update.mu.Unlock()
				return "", removeIfExists(filepath.Join(dir, appInfoFile))
			},
		},
		{
			item: StorageItem{ID: "temp", Label: "Temporary files", Recommended: true, Clearable: true,
				Description: "Downloaded update installers and scratch files left in the system temp folder."},
			paths: func(a *App) []string { return tempFiles() },
			clear: func(a *App) (string, error) { return a.clearTempFiles() },
		},
		{
			item: StorageItem{ID: "history", Label: "Query history", Clearable: true,
				Description: "Every query, pipeline and script you ran (History panel). Saved queries in the Library are kept."},
			paths: func(a *App) []string { return []string{filepath.Join(dir, "history.json")} },
			clear: func(a *App) (string, error) { return "", a.history.Clear() },
		},
		{
			item: StorageItem{ID: "recent", Label: "Recently opened collections", Clearable: true,
				Description: "The recent-collections list for each connection."},
			paths: func(a *App) []string { return []string{filepath.Join(dir, "collection_history.json")} },
			clear: func(a *App) (string, error) { return "", a.recent.ClearAll() },
		},
		{
			item: StorageItem{ID: "backups", Label: "Backups (before-images)",
				Description: "Copies of documents saved before Mango3S changed them, so changes can be undone. Manage them in Library › Backups."},
			paths: func(a *App) []string { return []string{backup.Dir(dir)} },
		},
	}
}

// GetStorageUsage reports how much space each category uses.
func (a *App) GetStorageUsage() StorageUsage {
	u := StorageUsage{ConfigDir: a.store.Dir()}
	for _, c := range a.storageCategories() {
		it := c.item
		for _, p := range c.paths(a) {
			n, files := diskUsage(p)
			it.Bytes += n
			it.Files += files
		}
		u.Items = append(u.Items, it)
	}
	return u
}

// ClearStorage deletes the selected categories (IDs from GetStorageUsage).
func (a *App) ClearStorage(ids []string) (StorageCleanResult, error) {
	before := map[string]int64{}
	for _, it := range a.GetStorageUsage().Items {
		before[it.ID] = it.Bytes
	}
	cats := map[string]storageCategory{}
	for _, c := range a.storageCategories() {
		cats[c.item.ID] = c
	}
	var res StorageCleanResult
	for _, id := range ids {
		c, ok := cats[id]
		if !ok || !c.item.Clearable {
			return StorageCleanResult{}, a.encode("ClearStorage", apperr.Invalid("%s can't be cleared here", id))
		}
		skip, err := c.clear(a)
		if err != nil {
			return StorageCleanResult{}, a.encode("ClearStorage", apperr.Wrap(apperr.CodeInternal, err, "Could not clear "+strings.ToLower(c.item.Label)))
		}
		if skip != "" {
			res.Skipped = append(res.Skipped, skip)
		}
		res.Cleared = append(res.Cleared, id)
	}
	res.Usage = a.GetStorageUsage()
	for _, it := range res.Usage.Items {
		if d := before[it.ID] - it.Bytes; d > 0 {
			res.FreedBytes += d
		}
	}
	a.log.Info("storage cleared", "categories", ids, "freedBytes", res.FreedBytes)
	return res, nil
}

// clearLogs empties the log file in use (it stays open for appending) and
// deletes every other file in the logs folder.
func (a *App) clearLogs(logDir string) error {
	entries, err := os.ReadDir(logDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := filepath.Join(logDir, e.Name())
		if a.logPath != "" && sameFile(p, a.logPath) {
			if err := os.Truncate(p, 0); err != nil {
				return err
			}
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}

// clearTempFiles removes Mango3S's temp files, except a downloaded update that
// is waiting to be installed or still downloading.
func (a *App) clearTempFiles() (string, error) {
	pending, _ := a.update.tracker.PendingArtifact()
	a.update.mu.Lock()
	downloading := a.update.downloading
	a.update.mu.Unlock()
	skip := ""
	for _, p := range tempFiles() {
		isUpdate := strings.HasPrefix(filepath.Base(p), "mango3s-update-")
		if isUpdate && (downloading || (pending != "" && sameFile(p, pending))) {
			skip = "An update that is downloading or ready to install was kept."
			continue
		}
		if err := os.RemoveAll(p); err != nil && !os.IsNotExist(err) {
			return skip, err
		}
	}
	return skip, nil
}

func tempFiles() []string {
	var out []string
	for _, pat := range tempPatterns {
		m, _ := filepath.Glob(filepath.Join(os.TempDir(), pat))
		out = append(out, m...)
	}
	return out
}

// diskUsage returns the size and file count of a file or folder (0 if missing).
func diskUsage(path string) (int64, int) {
	var total int64
	var files int
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
				files++
			}
		}
		return nil
	})
	return total, files
}

func removeIfExists(p string) error {
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func sameFile(a, b string) bool {
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}
