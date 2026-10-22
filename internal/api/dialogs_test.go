package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDialogsStartOutsideOneDriveFolders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)        // macOS / Linux
	t.Setenv("USERPROFILE", home) // Windows
	for _, d := range []string{"Downloads", "Documents", ".ssh"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	rememberDialogPath("", false)
	lastDialogDir.dir = ""

	if got := dialogDir("save"); got != filepath.Join(home, "Downloads") {
		t.Errorf("first dialog should open in Downloads, got %s", got)
	}
	if got := dialogDir("key"); got != filepath.Join(home, ".ssh") {
		t.Errorf("SSH keys should open in ~/.ssh, got %s", got)
	}
	picked := filepath.Join(home, "exports", "orders.json")
	_ = os.MkdirAll(filepath.Dir(picked), 0o700)
	rememberDialogPath(picked, false)
	if got := dialogDir("file"); got != filepath.Dir(picked) {
		t.Errorf("next dialog should reopen the last folder, got %s", got)
	}
}
