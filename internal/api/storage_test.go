package api

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func usage(t *testing.T, f fixture) map[string]StorageItem {
	t.Helper()
	out := map[string]StorageItem{}
	for _, it := range f.app.GetStorageUsage().Items {
		out[it.ID] = it
	}
	return out
}

func TestClearStorageRemovesOnlyTheChosenCategories(t *testing.T) {
	f := newFixture(t)
	t.Setenv("TMPDIR", t.TempDir()) // os.TempDir on macOS/Linux

	logDir := filepath.Join(f.dir, "logs")
	current := filepath.Join(logDir, "mango3s.log")
	writeFile(t, current, "current log line\n")
	writeFile(t, filepath.Join(logDir, "mango3s.log.1"), "rotated\n")
	f.app.logPath = current
	writeFile(t, filepath.Join(f.dir, appInfoFile), `{"info":{}}`)
	writeFile(t, filepath.Join(os.TempDir(), "mango3s-ai-123", "answer.json"), "{}")
	writeFile(t, filepath.Join(os.TempDir(), "unrelated.txt"), "keep me")
	writeFile(t, filepath.Join(f.dir, "history.json"), `{"entries":[]}`)
	writeFile(t, filepath.Join(f.dir, "connections.json"), `{"profiles":[]}`)
	writeFile(t, filepath.Join(f.dir, "backups", "b1", "docs.json"), "[]")

	u := usage(t, f)
	if u["logs"].Files != 2 || u["temp"].Files != 1 || u["backups"].Bytes == 0 {
		t.Fatalf("unexpected usage: %+v", u)
	}
	if !u["logs"].Recommended || u["history"].Recommended || u["backups"].Clearable {
		t.Fatalf("history must be opt-in and backups not clearable: %+v", u)
	}

	res, err := f.app.ClearStorage([]string{"logs", "cache", "temp"})
	if err != nil {
		t.Fatal(err)
	}
	if res.FreedBytes == 0 || len(res.Cleared) != 3 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if info, err := os.Stat(current); err != nil || info.Size() != 0 {
		t.Errorf("the open log must be emptied, not deleted: %v %v", info, err)
	}
	for _, gone := range []string{filepath.Join(logDir, "mango3s.log.1"), filepath.Join(f.dir, appInfoFile), filepath.Join(os.TempDir(), "mango3s-ai-123")} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s should be gone", gone)
		}
	}
	for _, kept := range []string{filepath.Join(os.TempDir(), "unrelated.txt"), filepath.Join(f.dir, "history.json"), filepath.Join(f.dir, "connections.json"), filepath.Join(f.dir, "backups", "b1", "docs.json")} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s must be kept: %v", kept, err)
		}
	}
}

func TestClearStorageKeepsAPendingUpdate(t *testing.T) {
	f := newFixture(t)
	t.Setenv("TMPDIR", t.TempDir())
	pending := filepath.Join(os.TempDir(), "mango3s-update-1.0.0-Mango3S.zip")
	stale := filepath.Join(os.TempDir(), "mango3s-update-0.8.0-Mango3S.zip")
	writeFile(t, pending, "new")
	writeFile(t, stale, "old")
	f.app.update.tracker.SetPendingArtifact(pending, "zip")

	res, err := f.app.ClearStorage([]string{"temp"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pending); err != nil {
		t.Error("the update waiting to be installed must be kept")
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("an old update download should be removed")
	}
	if len(res.Skipped) != 1 {
		t.Errorf("the kept update should be reported: %+v", res.Skipped)
	}
}

func TestClearStorageRejectsBackupsAndUnknownIDs(t *testing.T) {
	f := newFixture(t)
	for _, id := range []string{"backups", "connections"} {
		if _, err := f.app.ClearStorage([]string{id}); decodeErr(t, err).Code != "INVALID_INPUT" {
			t.Errorf("%s: want INVALID_INPUT", id)
		}
	}
}
