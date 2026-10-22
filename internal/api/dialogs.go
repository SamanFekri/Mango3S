package api

import (
	"os"
	"path/filepath"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Native file dialogs. Without a starting folder Windows opens them in
// Documents, which OneDrive's folder backup moves into OneDrive: the dialog
// then lists and previews the files there from inside Mango3S, and security
// software reports Mango3S reading OneDrive. Every dialog therefore starts in
// the folder the user last chose, or else in Downloads (which OneDrive doesn't
// move); SSH key pickers start in ~/.ssh.

var lastDialogDir struct {
	sync.Mutex
	dir string
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// dialogDir is the folder a dialog opens in.
func dialogDir(kind string) string {
	home, _ := os.UserHomeDir()
	if kind == "key" {
		if d := filepath.Join(home, ".ssh"); isDir(d) {
			return d
		}
	}
	lastDialogDir.Lock()
	last := lastDialogDir.dir
	lastDialogDir.Unlock()
	if isDir(last) {
		return last
	}
	if d := filepath.Join(home, "Downloads"); isDir(d) {
		return d
	}
	if isDir(home) {
		return home
	}
	return ""
}

// rememberDialogPath makes the folder of a chosen path the next starting folder.
func rememberDialogPath(p string, isFolder bool) {
	if p == "" {
		return
	}
	dir := p
	if !isFolder {
		dir = filepath.Dir(p)
	}
	lastDialogDir.Lock()
	lastDialogDir.dir = dir
	lastDialogDir.Unlock()
}

func (a *App) openFileDialog(kind string, o wruntime.OpenDialogOptions) (string, error) {
	if o.DefaultDirectory == "" {
		o.DefaultDirectory = dialogDir(kind)
	}
	p, err := wruntime.OpenFileDialog(a.ctx, o)
	if kind != "key" {
		rememberDialogPath(p, false)
	}
	return p, err
}

func (a *App) openDirectoryDialog(o wruntime.OpenDialogOptions) (string, error) {
	if o.DefaultDirectory == "" {
		o.DefaultDirectory = dialogDir("folder")
	}
	p, err := wruntime.OpenDirectoryDialog(a.ctx, o)
	rememberDialogPath(p, true)
	return p, err
}

func (a *App) saveFileDialog(o wruntime.SaveDialogOptions) (string, error) {
	if o.DefaultDirectory == "" {
		o.DefaultDirectory = dialogDir("save")
	}
	p, err := wruntime.SaveFileDialog(a.ctx, o)
	rememberDialogPath(p, false)
	return p, err
}
