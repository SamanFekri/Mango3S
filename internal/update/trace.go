package update

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Request tracing is a development aid: when enabled (MANGO3S_PLATFORM_TRACE=1,
// which `make dev` sets), every platform call — API requests and artifact
// downloads — is recorded in memory so Settings › Diagnostics and
// `mango3s platform-check` can show exactly what the app sent and what came
// back. Secrets (passwords, tokens, codes, recovery keys) are masked before
// anything is recorded. Off by default, so release builds never keep traffic.

// TraceEntry is one recorded platform request.
type TraceEntry struct {
	ID              int64             `json:"id"`
	Time            time.Time         `json:"time"`
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Status          int               `json:"status"` // 0 when no response arrived
	DurationMs      int64             `json:"durationMs"`
	Error           string            `json:"error,omitempty"`
	RequestHeaders  map[string]string `json:"requestHeaders,omitempty"`
	RequestBody     string            `json:"requestBody,omitempty"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
	ResponseBody    string            `json:"responseBody,omitempty"`
}

const (
	traceBodyLimit = 16 << 10 // recorded body text, after masking
	traceReadLimit = 1 << 20  // matches the client's own response read limit
)

type tracer struct {
	mu      sync.Mutex
	entries []TraceEntry
	max     int
	nextID  int64
	listen  func(TraceEntry)
}

var activeTracer atomic.Pointer[tracer]

// EnableTrace starts recording the most recent max platform requests and
// calls onEntry (may be nil) for each one as it completes. Safe to call once
// at startup; later calls only replace the listener.
func EnableTrace(max int, onEntry func(TraceEntry)) {
	if t := activeTracer.Load(); t != nil {
		t.mu.Lock()
		t.listen = onEntry
		t.mu.Unlock()
		return
	}
	activeTracer.Store(&tracer{max: max, listen: onEntry})
	HTTPClient.Transport = &tracingTransport{base: transportOf(HTTPClient)}
	DownloadClient.Transport = &tracingTransport{base: transportOf(DownloadClient), skipResponseBody: true}
}

// TraceEnabled reports whether requests are being recorded.
func TraceEnabled() bool { return activeTracer.Load() != nil }

// TraceEntries returns the recorded requests, oldest first.
func TraceEntries() []TraceEntry {
	t := activeTracer.Load()
	if t == nil {
		return []TraceEntry{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]TraceEntry{}, t.entries...)
}

// ClearTrace forgets every recorded request.
func ClearTrace() {
	if t := activeTracer.Load(); t != nil {
		t.mu.Lock()
		t.entries = nil
		t.mu.Unlock()
	}
}

func (t *tracer) record(e TraceEntry) {
	t.mu.Lock()
	t.nextID++
	e.ID = t.nextID
	t.entries = append(t.entries, e)
	if len(t.entries) > t.max {
		t.entries = append([]TraceEntry{}, t.entries[len(t.entries)-t.max:]...)
	}
	listen := t.listen
	t.mu.Unlock()
	if listen != nil {
		listen(e)
	}
}

func transportOf(c *http.Client) http.RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	return http.DefaultTransport
}

type tracingTransport struct {
	base http.RoundTripper
	// skipResponseBody leaves streamed bodies (artifact downloads) untouched.
	skipResponseBody bool
}

func (tt *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t := activeTracer.Load()
	if t == nil {
		return tt.base.RoundTrip(req)
	}
	e := TraceEntry{Time: time.Now(), Method: req.Method, URL: req.URL.String(), RequestHeaders: maskHeaders(req.Header)}
	if req.Body != nil && req.GetBody != nil {
		if body, err := req.GetBody(); err == nil {
			data, _ := io.ReadAll(io.LimitReader(body, traceReadLimit))
			body.Close()
			e.RequestBody = maskBody(data, true)
		}
	}
	resp, err := tt.base.RoundTrip(req)
	e.DurationMs = time.Since(e.Time).Milliseconds()
	if err != nil {
		e.Error = err.Error()
		t.record(e)
		return nil, err
	}
	e.Status = resp.StatusCode
	e.ResponseHeaders = maskHeaders(resp.Header)
	if !tt.skipResponseBody && resp.Body != nil {
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, traceReadLimit))
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(data))
		e.ResponseBody = maskBody(data, false)
		if readErr != nil {
			e.Error = "reading response: " + readErr.Error()
		}
		e.DurationMs = time.Since(e.Time).Milliseconds()
	}
	t.record(e)
	return resp, nil
}

// secretKeys are JSON fields whose values are never recorded (compared
// case-insensitively).
var secretKeys = map[string]bool{
	"password": true, "currentpassword": true, "newpassword": true,
	"token": true, "installationtoken": true, "accounttoken": true, "accesstoken": true, "refreshtoken": true,
	"recoverykey": true, "data": true,
}

// requestSecretKeys are masked in request bodies only: "code" is the emailed
// verification code there, but the error code (e.g. "invalid_code") in a
// response is exactly what a developer needs to see.
var requestSecretKeys = map[string]bool{"code": true}

var secretHeaders = map[string]bool{"authorization": true, "x-installation-token": true, "cookie": true, "set-cookie": true}

func maskSecret(s string) string {
	if len(s) <= 4 {
		return "••••"
	}
	return "••••" + s[len(s)-4:]
}

func maskHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for k, vs := range h {
		v := strings.Join(vs, ", ")
		if secretHeaders[strings.ToLower(k)] {
			if scheme, rest, ok := strings.Cut(v, " "); ok && strings.EqualFold(scheme, "Bearer") {
				v = "Bearer " + maskSecret(rest)
			} else {
				v = maskSecret(v)
			}
		}
		out[k] = v
	}
	return out
}

// MaskRequestBody returns a JSON request body with its secrets masked, for
// printing (see maskBody).
func MaskRequestBody(body string) string { return maskBody([]byte(body), true) }

// maskBody pretty-prints a JSON body with secret fields masked; anything that
// isn't JSON is summarized rather than recorded.
func maskBody(data []byte, request bool) string {
	if len(bytes.TrimSpace(data)) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return "(" + strconv.Itoa(len(data)) + " bytes, not JSON)"
	}
	out, err := json.MarshalIndent(maskValue(v, request), "", "  ")
	if err != nil {
		return "(unprintable body)"
	}
	if len(out) > traceBodyLimit {
		return string(out[:traceBodyLimit]) + "\n… (truncated)"
	}
	return string(out)
}

func maskValue(v any, request bool) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			lk := strings.ToLower(k)
			if s, ok := val.(string); ok && (secretKeys[lk] || request && requestSecretKeys[lk]) {
				x[k] = maskSecret(s)
				continue
			}
			x[k] = maskValue(val, request)
		}
		return x
	case []any:
		for i := range x {
			x[i] = maskValue(x[i], request)
		}
		return x
	default:
		return v
	}
}
