// Package secrets stores connection passwords outside the profile file.
//
// The preferred backend is the operating system credential store (macOS
// Keychain, Windows Credential Manager, Secret Service on Linux). When that is
// unavailable (headless Linux, containers) an AES-256-GCM encrypted file is
// used instead; its key lives next to it with 0600 permissions, which protects
// against casual disclosure (backups, screen sharing, accidental commits) but
// not against an attacker with access to the user's account.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

// ErrNotFound is returned when no secret exists for an ID.
var ErrNotFound = errors.New("secret not found")

// Store persists secrets keyed by an opaque ID (the profile ID).
type Store interface {
	Get(id string) (string, error)
	Set(id, secret string) error
	Delete(id string) error
	Backend() string
}

// EnvBackend forces a backend: "keyring", "file" or "memory".
const EnvBackend = "MANGO3S_SECRET_BACKEND"

const service = "Mango3S"

// Open selects a backend: the env override if set, otherwise the OS keyring
// when a probe succeeds, otherwise the encrypted file in dir.
func Open(dir string) (Store, error) {
	switch os.Getenv(EnvBackend) {
	case "keyring":
		return KeyringStore{}, nil
	case "file":
		return NewFileStore(dir)
	case "memory":
		return NewMemoryStore(), nil
	}
	if probeKeyring() {
		return KeyringStore{}, nil
	}
	return NewFileStore(dir)
}

func probeKeyring() bool {
	const probe = "__mango3s_probe__"
	if err := keyring.Set(service, probe, "ok"); err != nil {
		return false
	}
	v, err := keyring.Get(service, probe)
	_ = keyring.Delete(service, probe)
	return err == nil && v == "ok"
}

// KeyringStore uses the OS credential store.
type KeyringStore struct{}

func (KeyringStore) Backend() string { return "keyring" }

func (KeyringStore) Get(id string) (string, error) {
	v, err := keyring.Get(service, id)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

func (KeyringStore) Set(id, secret string) error { return keyring.Set(service, id, secret) }

func (KeyringStore) Delete(id string) error {
	err := keyring.Delete(service, id)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// FileStore keeps secrets AES-GCM encrypted in secrets.enc.
type FileStore struct {
	mu   sync.Mutex
	path string
	aead cipher.AEAD
}

// NewFileStore opens (or initializes) the encrypted store in dir.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	key, err := loadOrCreateKey(filepath.Join(dir, "secret.key"))
	if err != nil {
		return nil, fmt.Errorf("secret key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &FileStore{path: filepath.Join(dir, "secrets.enc"), aead: aead}, nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		key, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil || len(key) != 32 {
			return nil, errors.New("key file is corrupt")
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (f *FileStore) Backend() string { return "file" }

func (f *FileStore) load() (map[string]string, error) {
	m := map[string]string{}
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	return m, json.Unmarshal(data, &m)
}

func (f *FileStore) save(m map[string]string) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func (f *FileStore) Get(id string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.load()
	if err != nil {
		return "", err
	}
	enc, ok := m[id]
	if !ok {
		return "", ErrNotFound
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	ns := f.aead.NonceSize()
	if err != nil || len(raw) < ns {
		return "", errors.New("stored secret is corrupt")
	}
	plain, err := f.aead.Open(nil, raw[:ns], raw[ns:], []byte(id))
	if err != nil {
		return "", errors.New("stored secret cannot be decrypted (was secret.key replaced?)")
	}
	return string(plain), nil
}

func (f *FileStore) Set(id, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.load()
	if err != nil {
		return err
	}
	nonce := make([]byte, f.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	// The ID is bound as associated data so ciphertexts cannot be swapped between entries.
	sealed := f.aead.Seal(nonce, nonce, []byte(secret), []byte(id))
	m[id] = base64.StdEncoding.EncodeToString(sealed)
	return f.save(m)
}

func (f *FileStore) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.load()
	if err != nil {
		return err
	}
	if _, ok := m[id]; !ok {
		return nil
	}
	delete(m, id)
	return f.save(m)
}

// MemoryStore is a non-persistent store for tests.
type MemoryStore struct {
	mu sync.Mutex
	m  map[string]string
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{m: map[string]string{}} }

func (s *MemoryStore) Backend() string { return "memory" }

func (s *MemoryStore) Get(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[id]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *MemoryStore) Set(id, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = secret
	return nil
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}
