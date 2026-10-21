package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaskBody(t *testing.T) {
	req := maskBody([]byte(`{"email":"ada@example.com","password":"hunter2-long","code":"123456","nested":[{"recoveryKey":"RK-ABCDEFGH"}]}`), true)
	for _, leaked := range []string{"hunter2-long", "123456", "RK-ABCDEFGH"} {
		if strings.Contains(req, leaked) {
			t.Errorf("request body leaks %q:\n%s", leaked, req)
		}
	}
	if !strings.Contains(req, "ada@example.com") || !strings.Contains(req, "••••EFGH") {
		t.Errorf("request body lost non-secret data or the masked suffix:\n%s", req)
	}

	resp := maskBody([]byte(`{"installationToken":"tok_abcdef1234","error":{"code":"invalid_code"}}`), false)
	if strings.Contains(resp, "tok_abcdef1234") {
		t.Errorf("response body leaks the token:\n%s", resp)
	}
	if !strings.Contains(resp, "invalid_code") {
		t.Errorf("a response's error code must stay readable:\n%s", resp)
	}

	if got := maskBody([]byte("<html>"), false); !strings.Contains(got, "not JSON") {
		t.Errorf("non-JSON bodies are summarized, got %q", got)
	}
}

func TestMaskHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer it_secret_token_9876")
	h.Set("X-Installation-Token", "abcdefgh")
	h.Set("Content-Type", "application/json")
	got := maskHeaders(h)
	if got["Authorization"] != "Bearer ••••9876" || got["X-Installation-Token"] != "••••efgh" || got["Content-Type"] != "application/json" {
		t.Fatalf("unexpected headers: %v", got)
	}
}

func TestTraceRecordsRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"installationId": "inst-1", "installationToken": "it_token_value"})
	}))
	defer srv.Close()

	var seen []TraceEntry
	EnableTrace(10, func(e TraceEntry) { seen = append(seen, e) })
	ClearTrace()
	resp, err := New(srv.URL).Register(context.Background(), RegisterRequest{DeviceFingerprint: "fp"})
	if err != nil || resp.InstallationToken != "it_token_value" {
		t.Fatalf("tracing must not change the response: %+v, %v", resp, err)
	}
	entries := TraceEntries()
	if len(entries) != 1 || len(seen) != 1 {
		t.Fatalf("want one recorded request, got %d (listener %d)", len(entries), len(seen))
	}
	e := entries[0]
	if e.Method != http.MethodPost || !strings.HasSuffix(e.URL, "/v1/installations/register") || e.Status != 200 {
		t.Errorf("unexpected entry: %+v", e)
	}
	if !strings.Contains(e.RequestBody, `"deviceFingerprint": "fp"`) || strings.Contains(e.ResponseBody, "it_token_value") {
		t.Errorf("bodies not recorded/masked as expected:\nreq %s\nresp %s", e.RequestBody, e.ResponseBody)
	}
}
