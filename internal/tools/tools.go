// Package tools finds the MongoDB command-line utilities (mongosh and the
// MongoDB Database Tools), reports their versions and flags missing or
// incompatible installs. Mango3S's own import, export and shell are built
// in; these paths are for the user's reference and external use.
package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mango3s/internal/config"
)

// Known lists the utilities Mango3S looks for.
var Known = []struct{ Name, Kind, Purpose string }{
	{"mongosh", "shell", "MongoDB Shell"},
	{"mongodump", "dbtools", "Binary backup"},
	{"mongorestore", "dbtools", "Restore a backup"},
	{"mongoexport", "dbtools", "Export JSON/CSV"},
	{"mongoimport", "dbtools", "Import JSON/CSV/TSV"},
	{"bsondump", "dbtools", "Inspect BSON files"},
	{"mongostat", "dbtools", "Server statistics"},
	{"mongotop", "dbtools", "Per-collection activity"},
	{"mongofiles", "dbtools", "GridFS files"},
}

// Status values.
const (
	StatusOK           = "ok"
	StatusMissing      = "missing"
	StatusIncompatible = "incompatible"
	StatusError        = "error"
)

// Info is the detection result for one tool.
type Info struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Path    string `json:"path"`
	Source  string `json:"source"` // override | folder | PATH | common location
	Version string `json:"version"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Runner runs "<path> --version" (replaceable in tests).
var Runner = func(ctx context.Context, path string) (string, error) {
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	return string(out), err
}

// LookPath resolves a name on PATH (replaceable in tests).
var LookPath = exec.LookPath

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func commonDirs() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin"}
	case "windows":
		var dirs []string
		for _, pat := range []string{`C:\Program Files\MongoDB\Tools\*\bin`, `C:\Program Files\mongosh`, `C:\Program Files\MongoDB\Server\*\bin`} {
			m, _ := filepath.Glob(pat)
			sort.Sort(sort.Reverse(sort.StringSlice(m)))
			dirs = append(dirs, m...)
		}
		return dirs
	default:
		return []string{"/usr/bin", "/usr/local/bin", "/snap/bin"}
	}
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Detect finds every known tool. serverVersion (optional, e.g. "8.0.4")
// enables compatibility notes for the connected server.
func Detect(ctx context.Context, cfg config.ToolsConfig, serverVersion string) []Info {
	out := make([]Info, len(Known))
	var wg sync.WaitGroup
	for i, k := range Known {
		wg.Add(1)
		go func(i int, name, kind, purpose string) {
			defer wg.Done()
			out[i] = detectOne(ctx, cfg, name, kind, purpose, serverVersion)
		}(i, k.Name, k.Kind, k.Purpose)
	}
	wg.Wait()
	return out
}

func locate(cfg config.ToolsConfig, name string) (path, source, problem string) {
	if p := strings.TrimSpace(cfg.Paths[name]); p != "" {
		if !isFile(p) {
			return "", "override", "The configured path " + p + " does not exist."
		}
		return p, "override", ""
	}
	if cfg.Dir != "" {
		p := filepath.Join(cfg.Dir, exe(name))
		if isFile(p) {
			return p, "folder", ""
		}
	}
	if p, err := LookPath(exe(name)); err == nil {
		return p, "PATH", ""
	}
	for _, d := range commonDirs() {
		p := filepath.Join(d, exe(name))
		if isFile(p) {
			return p, "common location", ""
		}
	}
	return "", "", ""
}

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

func detectOne(ctx context.Context, cfg config.ToolsConfig, name, kind, purpose, serverVersion string) Info {
	info := Info{Name: name, Purpose: purpose}
	path, source, problem := locate(cfg, name)
	info.Path, info.Source = path, source
	if problem != "" {
		info.Status, info.Message = StatusError, problem
		return info
	}
	if path == "" {
		info.Status = StatusMissing
		if kind == "shell" {
			info.Message = "Not found. Install mongosh from https://www.mongodb.com/try/download/shell."
		} else {
			info.Message = "Not found. Install the MongoDB Database Tools from https://www.mongodb.com/try/download/database-tools."
		}
		return info
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	text, err := Runner(cctx, path)
	m := versionRe.FindStringSubmatch(text)
	if m == nil {
		info.Status = StatusError
		if err != nil {
			info.Message = "Could not run it: " + firstLine(err.Error())
		} else {
			info.Message = "Could not read its version."
		}
		return info
	}
	info.Version = m[0]
	major, _ := strconv.Atoi(m[1])
	info.Status, info.Message = StatusOK, ""
	switch kind {
	case "dbtools":
		if major < 100 {
			info.Status = StatusIncompatible
			info.Message = "Legacy tools (bundled with MongoDB 4.2 and earlier). Install MongoDB Database Tools 100.x for current servers."
		}
	case "shell":
		if major < 1 {
			info.Status = StatusIncompatible
			info.Message = "Pre-release mongosh; upgrade to a current version."
		} else if sv := majorMinor(serverVersion); sv > 0 && sv < 402 && major >= 2 {
			info.Status = StatusIncompatible
			info.Message = "mongosh 2.x does not support MongoDB servers older than 4.2; use mongosh 1.x."
		}
	}
	return info
}

func majorMinor(v string) int {
	m := versionRe.FindStringSubmatch(v)
	if m == nil {
		return 0
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	return a*100 + b
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
