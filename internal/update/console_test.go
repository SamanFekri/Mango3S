package update

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range APICatalog {
		if e.ID == "" || e.Group == "" || e.Name == "" || !strings.HasPrefix(e.Path, "/") {
			t.Errorf("incomplete endpoint: %+v", e)
		}
		if seen[e.ID] {
			t.Errorf("duplicate endpoint ID %q", e.ID)
		}
		seen[e.ID] = true
		if e.Body != "" {
			// Every example body is JSON once its placeholders are filled.
			var missing []string
			vars := map[string]string{}
			for _, m := range placeholder.FindAllStringSubmatch(e.Body, -1) {
				vars[m[1]] = "1"
			}
			if filled := fillJSON(e.Body, vars, &missing); !json.Valid([]byte(filled)) {
				t.Errorf("%s: example body is not JSON: %s", e.ID, filled)
			}
		}
		switch e.Auth {
		case AuthNone, AuthInstallation, AuthAccount, AuthDevice, AuthAdmin:
		default:
			t.Errorf("%s: unknown auth %q", e.ID, e.Auth)
		}
	}
}

func TestConsoleSendsFillsAndCaptures(t *testing.T) {
	var got struct {
		path, query, auth, cookie string
		body                      map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.query, got.auth, got.cookie = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), r.Header.Get("Cookie")
		got.body = nil
		if data, _ := io.ReadAll(r.Body); len(data) > 0 {
			_ = json.Unmarshal(data, &got.body)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/installations/register":
			_, _ = w.Write([]byte(`{"installationId":"inst-9","installationToken":"it_secret_1234"}`))
		case "/api/auth/login":
			http.SetCookie(w, &http.Cookie{Name: "mango3s_admin", Value: "sess-abc"})
			_, _ = w.Write([]byte(`{"admin":{"username":"ada"}}`))
		case "/api/clients":
			_, _ = w.Write([]byte(`{"items":[{"id":"client-7"}],"total":1}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := NewConsole(map[string]string{"os": "macos", "arch": "arm64", "appVersion": "0.9.0", "deviceFingerprint": "fp-12345678"})
	send := func(id string) ConsoleResponse {
		t.Helper()
		req, ok := RequestFor(id)
		if !ok {
			t.Fatalf("no endpoint %s", id)
		}
		return c.Send(context.Background(), srv.URL, req)
	}

	// Authenticated call before registering: refused, nothing sent.
	if r := send("installations.heartbeat"); !strings.Contains(r.Error, "installationId") || r.Status != 0 {
		t.Fatalf("want a missing-variable error, got %+v", r)
	}

	r := send("installations.register")
	if r.Status != 200 || got.body["os"] != "macos" || got.body["deviceFingerprint"] != "fp-12345678" {
		t.Fatalf("register: %+v body %v", r, got.body)
	}
	if r.Captured["installationId"] != "inst-9" || r.Captured["installationToken"] != "••••1234" {
		t.Fatalf("captured: %v", r.Captured)
	}

	send("installations.heartbeat")
	if got.path != "/v1/installations/inst-9/heartbeat" || got.auth != "Bearer it_secret_1234" {
		t.Fatalf("heartbeat used %s with %q", got.path, got.auth)
	}
	send("updates.check")
	if !strings.Contains(got.query, "os=macos") || !strings.Contains(got.query, "currentVersion=0.9.0") {
		t.Fatalf("query: %s", got.query)
	}

	c.SetVar("adminUser", "ada")
	c.SetVar("adminPassword", "correct horse")
	if r := send("admin.login"); r.Captured["adminCookie"] == "" {
		t.Fatalf("admin cookie not captured: %+v", r)
	}
	if r := send("admin.clients"); r.Captured["clientId"] != "client-7" || got.cookie != "mango3s_admin=sess-abc" {
		t.Fatalf("admin call: cookie %q, captured %v", got.cookie, r.Captured)
	}
	send("admin.clients.get")
	if got.path != "/api/clients/client-7" {
		t.Fatalf("path variable not filled: %s", got.path)
	}
}

func TestConsoleRejectsBadBodyAndUploads(t *testing.T) {
	c := NewConsole(nil)
	if r := c.Send(context.Background(), "http://127.0.0.1:1", ConsoleRequest{Method: "POST", Path: "/x", Body: "{nope"}); r.Error != "the body isn't valid JSON" {
		t.Fatalf("got %+v", r)
	}
	req, _ := RequestFor("admin.backups.restore-upload")
	if r := c.Send(context.Background(), "http://127.0.0.1:1", req); !strings.Contains(r.Error, "multipart") {
		t.Fatalf("got %+v", r)
	}
}
