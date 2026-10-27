package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestPost(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("%s %s", r.Method, r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		if r.URL.Query().Get("token") == "bad" {
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()

	if err := Post(context.Background(), srv.URL+"/hook?token=s3cret", map[string]string{"state": "failed"}); err != nil {
		t.Fatal(err)
	}
	if got["state"] != "failed" {
		t.Fatalf("payload: %v", got)
	}
	err := Post(context.Background(), srv.URL+"/hook?token=bad", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "token") {
		t.Fatalf("a refused webhook should fail without showing the URL: %v", err)
	}
	for _, bad := range []string{"", "ftp://example.com", "example.com/hook", "file:///etc/passwd", "https://"} {
		if err := Post(context.Background(), bad, nil); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestDesktopPassesTextAsData(t *testing.T) {
	var name string
	var args, env []string
	orig := Command
	t.Cleanup(func() { Command = orig })
	Command = func(_ context.Context, e []string, n string, a ...string) error {
		name, args, env = n, a, e
		return nil
	}
	title, msg := `Task "nightly" failed`, `it's $(rm -rf ~) and "quotes"`
	if err := Desktop(context.Background(), title, msg); err != nil {
		t.Fatal(err)
	}
	switch runtime.GOOS {
	case "windows":
		if name != "powershell" || !slices.Contains(env, "M3S_TITLE="+title) || !slices.Contains(env, "M3S_MESSAGE="+msg) {
			t.Fatalf("%s %v %v", name, args, env)
		}
	default:
		if n := len(args); n < 2 || args[n-2] != title || args[n-1] != msg {
			t.Fatalf("%s %v: title and message must be the last two arguments", name, args)
		}
	}
}
