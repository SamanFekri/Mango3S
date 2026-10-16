package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mango3s/internal/config"
)

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	touch := func(name string) string {
		p := filepath.Join(dir, exe(name))
		_ = os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755)
		return p
	}
	touch("mongodump")
	touch("mongoexport")
	legacy := filepath.Join(t.TempDir(), "mongoimport")
	_ = os.WriteFile(legacy, nil, 0o755)
	oldRun, oldLook := Runner, LookPath
	defer func() { Runner, LookPath = oldRun, oldLook }()
	LookPath = func(string) (string, error) { return "", errors.New("not found") }
	Runner = func(_ context.Context, path string) (string, error) {
		switch filepath.Base(path) {
		case exe("mongodump"):
			return "mongodump version: 100.10.0\ngit version: abc", nil
		case exe("mongoimport"):
			return "mongoimport version: r4.2.1", nil
		}
		return "", errors.New("exec format error")
	}
	cfg := config.ToolsConfig{Dir: dir, Paths: map[string]string{"mongoimport": legacy, "mongosh": "/nope/mongosh"}}
	got := map[string]Info{}
	for _, i := range Detect(context.Background(), cfg, "") {
		got[i.Name] = i
	}
	if d := got["mongodump"]; d.Status != StatusOK || d.Version != "100.10.0" || d.Source != "folder" {
		t.Fatalf("mongodump: %+v", d)
	}
	if d := got["mongoimport"]; d.Status != StatusIncompatible || d.Source != "override" {
		t.Fatalf("legacy tools must be flagged: %+v", d)
	}
	if d := got["mongosh"]; d.Status != StatusError {
		t.Fatalf("a configured path that doesn't exist is an error: %+v", d)
	}
	if d := got["mongoexport"]; d.Status != StatusError {
		t.Fatalf("a tool that won't run is an error: %+v", d)
	}
	if d := got["mongotop"]; d.Status != StatusMissing || d.Message == "" {
		t.Fatalf("missing tools explain where to get them: %+v", d)
	}
}

func TestMongoshServerCompatibility(t *testing.T) {
	oldRun, oldLook := Runner, LookPath
	defer func() { Runner, LookPath = oldRun, oldLook }()
	LookPath = func(name string) (string, error) { return "/bin/" + name, nil }
	Runner = func(context.Context, string) (string, error) { return "2.3.1", nil }
	info := detectOne(context.Background(), config.ToolsConfig{}, "mongosh", "shell", "", "4.0.28")
	if info.Status != StatusIncompatible {
		t.Fatalf("mongosh 2.x vs MongoDB 4.0: %+v", info)
	}
	info = detectOne(context.Background(), config.ToolsConfig{}, "mongosh", "shell", "", "8.0.1")
	if info.Status != StatusOK {
		t.Fatalf("mongosh 2.x vs MongoDB 8: %+v", info)
	}
}
