package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func probeServer(t *testing.T, notificationsStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		authed := r.Header.Get("Authorization") == "Bearer tok"
		switch {
		case r.URL.Path == "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.URL.Path == "/v1/app-info":
			_ = json.NewEncoder(w).Encode(AppInfo{Name: "Mango3S"})
		case r.URL.Path == "/v1/installations/register":
			_ = json.NewEncoder(w).Encode(RegisterResponse{InstallationID: "inst-1", InstallationToken: "tok"})
		case !authed:
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/v1/installations/inst-1/heartbeat":
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/v1/updates/check":
			_ = json.NewEncoder(w).Encode(UpdateCheckResult{})
		case r.URL.Path == "/v1/notifications":
			if notificationsStatus != http.StatusOK {
				w.WriteHeader(notificationsStatus)
				return
			}
			_, _ = w.Write([]byte(`[]`))
		case r.URL.Path == "/v1/policies/privacy":
			_ = json.NewEncoder(w).Encode(PrivacyPolicy{Version: 1})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func stepNames(rep ProbeReport) string {
	var parts []string
	for _, s := range rep.Steps {
		mark := "ok"
		if s.Skipped {
			mark = "skip"
		} else if !s.OK {
			mark = "fail"
		}
		parts = append(parts, s.Name+":"+mark)
	}
	return strings.Join(parts, ", ")
}

func TestProbeWithRegistration(t *testing.T) {
	srv := probeServer(t, http.StatusOK)
	defer srv.Close()
	rep := New(srv.URL).Probe(context.Background(), ProbeOptions{
		Heartbeat: true, OS: "darwin", Arch: "arm64", Version: "0.9.0",
		Register: func(ctx context.Context, c *Client) (string, string, error) {
			r, err := c.Register(ctx, RegisterRequest{DeviceFingerprint: "fp"})
			return r.InstallationToken, r.InstallationID, err
		},
	})
	want := "Server reachable:ok, App info:ok, Register installation:ok, Heartbeat:ok, Update check:ok, Notifications:ok, Privacy policy:ok"
	if got := stepNames(rep); got != want || !rep.OK {
		t.Fatalf("got  %s (ok=%v)\nwant %s", got, rep.OK, want)
	}
}

func TestProbeWithoutTokenSkipsAuthenticatedSteps(t *testing.T) {
	srv := probeServer(t, http.StatusOK)
	defer srv.Close()
	rep := New(srv.URL).Probe(context.Background(), ProbeOptions{})
	want := "Server reachable:ok, App info:ok, Update check:skip, Notifications:skip, Privacy policy:skip"
	if got := stepNames(rep); got != want || !rep.OK {
		t.Fatalf("got  %s (ok=%v)\nwant %s", got, rep.OK, want)
	}
}

func TestProbeReportsFailuresWithoutRetrying(t *testing.T) {
	srv := probeServer(t, http.StatusServiceUnavailable)
	defer srv.Close()
	rep := New(srv.URL).Probe(context.Background(), ProbeOptions{Token: "tok", InstallationID: "inst-1"})
	if rep.OK {
		t.Fatal("a failed step must fail the report")
	}
	var n ProbeStep
	for _, s := range rep.Steps {
		if s.Name == "Notifications" {
			n = s
		}
	}
	if n.OK || n.Status != http.StatusServiceUnavailable {
		t.Fatalf("notifications step: %+v", n)
	}
}

func TestProbeUnreachable(t *testing.T) {
	srv := probeServer(t, http.StatusOK)
	url := srv.URL
	srv.Close()
	rep := New(url).Probe(context.Background(), ProbeOptions{Token: "tok"})
	if rep.OK || rep.Steps[0].OK || !strings.Contains(rep.Steps[0].Error, "nothing is listening") {
		t.Fatalf("unexpected first step: %+v", rep.Steps[0])
	}
	for _, s := range rep.Steps[1:] {
		if !s.Skipped {
			t.Errorf("%s should be skipped when the server is unreachable", s.Name)
		}
	}
}

func TestRunCheckOneAtATime(t *testing.T) {
	srv := probeServer(t, http.StatusOK)
	defer srv.Close()
	c := New(srv.URL)
	o := &ProbeOptions{OS: "darwin", Arch: "arm64", Version: "0.9.0"}

	if s := c.RunCheck(context.Background(), CheckNotifications, o); !s.Skipped {
		t.Fatalf("without a token the check is skipped, got %+v", s)
	}
	o.Register = func(ctx context.Context, c *Client) (string, string, error) {
		r, err := c.Register(ctx, RegisterRequest{DeviceFingerprint: "fp"})
		return r.InstallationToken, r.InstallationID, err
	}
	if s := c.RunCheck(context.Background(), CheckRegister, o); !s.OK || o.Token != "tok" || o.InstallationID != "inst-1" {
		t.Fatalf("register: %+v, opts %+v", s, o)
	}
	if s := c.RunCheck(context.Background(), CheckHeartbeat, o); !s.OK || s.Path != "/v1/installations/inst-1/heartbeat" {
		t.Fatalf("heartbeat: %+v", s)
	}
	if s := c.RunCheck(context.Background(), CheckNotifications, o); !s.OK || s.ID != CheckNotifications {
		t.Fatalf("notifications: %+v", s)
	}
	if s := c.RunCheck(context.Background(), "nope", o); s.OK || s.Error == "" {
		t.Fatalf("unknown check: %+v", s)
	}
}

func TestPlatformOSUsesThePlatformNames(t *testing.T) {
	for goos, want := range map[string]string{"darwin": "macos", "windows": "windows", "linux": "linux"} {
		if got := PlatformOS(goos); got != want {
			t.Errorf("PlatformOS(%q) = %q, want %q", goos, got, want)
		}
	}
}
