package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileStore(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.Set("a", "hunter2"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("b", "other"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(dir, "secrets.enc"))
	if strings.Contains(string(raw), "hunter2") {
		t.Fatal("plaintext secret written to disk")
	}
	for _, name := range []string{"secrets.enc", "secret.key"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s perms = %v", name, info.Mode().Perm())
		}
	}

	// A fresh instance reuses the persisted key.
	s2, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := s2.Get("a"); err != nil || v != "hunter2" {
		t.Fatalf("reload: %q %v", v, err)
	}

	if err := s2.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Get("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
	if err := s2.Delete("missing"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
	if v, _ := s2.Get("b"); v != "other" {
		t.Fatal("unrelated entry lost")
	}
}

func TestFileStoreRejectsSwappedCiphertext(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileStore(dir)
	_ = s.Set("a", "A")
	_ = s.Set("b", "B")
	m, _ := s.load()
	m["a"], m["b"] = m["b"], m["a"]
	_ = s.save(m)
	if _, err := s.Get("a"); err == nil {
		t.Fatal("swapped ciphertext decrypted")
	}
}

func TestMemoryStore(t *testing.T) {
	s := NewMemoryStore()
	_ = s.Set("x", "1")
	if v, _ := s.Get("x"); v != "1" {
		t.Fatal("get")
	}
	_ = s.Delete("x")
	if _, err := s.Get("x"); !errors.Is(err, ErrNotFound) {
		t.Fatal("delete")
	}
}

func TestOpenRespectsEnv(t *testing.T) {
	t.Setenv(EnvBackend, "memory")
	s, err := Open(t.TempDir())
	if err != nil || s.Backend() != "memory" {
		t.Fatalf("%v %v", s, err)
	}
	t.Setenv(EnvBackend, "file")
	s, err = Open(t.TempDir())
	if err != nil || s.Backend() != "file" {
		t.Fatalf("%v %v", s, err)
	}
}
