package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func init() {
	// Keep retry tests fast; the shape (count, transient-only) is what matters.
	RetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
}

func TestRegisterAndHeartbeat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/installations/register":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, sent := body["clientKey"]; sent {
				t.Errorf("the free app must not send a client key: %v", body)
			}
			if body["deviceFingerprint"] != "fp" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(RegisterResponse{InstallationID: "inst-1", InstallationToken: "it_token"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/installations/inst-1/heartbeat":
			if r.Header.Get("Authorization") != "Bearer it_token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.Register(context.Background(), RegisterRequest{DeviceFingerprint: "fp"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.InstallationID != "inst-1" || resp.InstallationToken != "it_token" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if err := c.Heartbeat(context.Background(), "inst-1", "it_token", HeartbeatRequest{AppVersion: "1.0.0", OS: "linux", Arch: "amd64"}); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}

	if err := c.Heartbeat(context.Background(), "inst-1", "wrong", HeartbeatRequest{}); !IsUnauthorized(err) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestTransientFailuresAreRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := New(srv.URL).Heartbeat(context.Background(), "i", "t", HeartbeatRequest{}); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3 (two 503s, then success)", calls)
	}
}

func TestClientErrorsAreNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	err := New(srv.URL).Heartbeat(context.Background(), "i", "t", HeartbeatRequest{})
	if !IsForbidden(err) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("a 403 is final and must not be retried; calls = %d", calls)
	}
}

func TestGivesUpAfterLastRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	if err := New(srv.URL).Heartbeat(context.Background(), "i", "t", HeartbeatRequest{}); !IsTransient(err) {
		t.Fatalf("expected the last transient error, got %v", err)
	}
	if want := len(RetryDelays) + 1; calls != want {
		t.Fatalf("calls = %d, want %d", calls, want)
	}
}

func TestUnreachableServerIsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing listening any more

	err := New(url).Heartbeat(context.Background(), "i", "t", HeartbeatRequest{})
	if err == nil || !IsTransient(err) {
		t.Fatalf("expected a transient connection error, got %v", err)
	}
}

func TestCheckForUpdateAndForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer revoked" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UpdateCheckResult{
			UpdateAvailable: true,
			Latest: &LatestRelease{
				Version: "2.0.0", Channel: "stable",
				Artifact: Artifact{ID: "a1", FileName: "mango3s.zip", DownloadURL: "/downloads/a1", SHA256: "deadbeef"},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.CheckForUpdate(context.Background(), "it_token", "linux", "amd64", "stable", "1.0.0")
	if err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if !result.UpdateAvailable || result.Latest == nil || result.Latest.Version != "2.0.0" {
		t.Fatalf("unexpected result: %+v", result)
	}

	_, err = c.CheckForUpdate(context.Background(), "revoked", "linux", "amd64", "stable", "1.0.0")
	if !IsForbidden(err) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestNotificationsAndAck(t *testing.T) {
	acked := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/notifications":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]Notification{{ID: "n1", Title: "Hi", Body: "There"}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/notifications/n1/ack":
			acked = "n1"
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL)
	list, err := c.ListNotifications(context.Background(), "it_token")
	if err != nil || len(list) != 1 || list[0].ID != "n1" {
		t.Fatalf("ListNotifications: %+v, err=%v", list, err)
	}
	if err := c.Ack(context.Background(), "it_token", "n1"); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if acked != "n1" {
		t.Fatalf("server did not see the ack")
	}
}

func TestDownloadArtifactVerifiesChecksum(t *testing.T) {
	content := []byte("fake installer bytes")
	sum := sha256.Sum256(content)
	expected := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	c := New(srv.URL)
	dir := t.TempDir()
	dest := filepath.Join(dir, "artifact.bin")

	var gotDone, gotTotal int64
	if err := c.DownloadArtifact(context.Background(), "", "/downloads/a1", dest, expected, func(done, total int64) {
		gotDone, gotTotal = done, total
	}); err != nil {
		t.Fatalf("DownloadArtifact: %v", err)
	}
	if gotDone == 0 {
		t.Fatalf("progress callback never reported bytes")
	}
	_ = gotTotal
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != string(content) {
		t.Fatalf("downloaded file mismatch: %v", err)
	}

	// A mismatched checksum must fail and remove the partial file.
	badDest := filepath.Join(dir, "bad.bin")
	err = c.DownloadArtifact(context.Background(), "", "/downloads/a1", badDest, "0000", nil)
	if err == nil {
		t.Fatalf("expected a checksum mismatch error")
	}
	if _, statErr := os.Stat(badDest); !os.IsNotExist(statErr) {
		t.Fatalf("partial file was not removed after checksum failure")
	}
}
