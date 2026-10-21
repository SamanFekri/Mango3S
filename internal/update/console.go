package update

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The API console sends one request at a time to any route in APICatalog (or
// any path at all), filling {variables} from its own set and saving values
// from responses — tokens, created IDs, the admin session cookie — so a
// developer can walk through a whole flow by hand. It is a test-mode tool:
// the app only enables it when MANGO3S_TEST_MODE is set (`make dev`).

// ConsoleRequest is what to send. Path, Query and Body may contain {variables}.
type ConsoleRequest struct {
	EndpointID string            `json:"endpointId"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      string            `json:"query"`
	Body       string            `json:"body"`
	Auth       string            `json:"auth"` // AuthNone, AuthInstallation, …
	Headers    map[string]string `json:"headers,omitempty"`
}

// ConsoleResponse is what came back.
type ConsoleResponse struct {
	EndpointID      string            `json:"endpointId"`
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Status          int               `json:"status"` // 0 when no response arrived
	StatusText      string            `json:"statusText"`
	DurationMs      int64             `json:"durationMs"`
	RequestHeaders  map[string]string `json:"requestHeaders"`
	RequestBody     string            `json:"requestBody,omitempty"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
	ResponseBody    string            `json:"responseBody,omitempty"`
	Error           string            `json:"error,omitempty"`
	// Captured lists the variables this response set (values masked).
	Captured map[string]string `json:"captured,omitempty"`
}

// Console keeps the variables between requests.
type Console struct {
	mu   sync.Mutex
	vars map[string]string
}

// ConsoleDefaultVars are the variables every console starts with.
var ConsoleDefaultVars = []string{
	"os", "arch", "appVersion", "deviceFingerprint", "installationId", "installationToken",
	"email", "password", "newPassword", "code", "recoveryKey", "accountToken",
	"adminUser", "adminEmail", "adminPassword", "adminCookie",
	"notificationId", "policyVersion", "artifactId", "clientId", "machineRowId", "releaseId",
	"linkId", "roleId", "adminUserId", "sessionId", "inviteId", "inviteToken", "policyId",
	"templateId", "backupId",
}

// SecretVars are shown masked.
var SecretVars = map[string]bool{
	"installationToken": true, "password": true, "newPassword": true, "recoveryKey": true,
	"accountToken": true, "adminPassword": true, "adminCookie": true, "inviteToken": true, "code": true,
}

// NewConsole starts a console with the default variables plus initial.
func NewConsole(initial map[string]string) *Console {
	c := &Console{vars: map[string]string{"policyVersion": "1"}}
	for _, k := range ConsoleDefaultVars {
		if _, ok := c.vars[k]; !ok {
			c.vars[k] = ""
		}
	}
	for k, v := range initial {
		c.vars[k] = v
	}
	return c
}

// Vars returns a copy of the variables.
func (c *Console) Vars() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.vars))
	for k, v := range c.vars {
		out[k] = v
	}
	return out
}

// SetVar sets (or, with an empty name, ignores) a variable.
func (c *Console) SetVar(name, value string) {
	if name = strings.TrimSpace(name); name == "" {
		return
	}
	c.mu.Lock()
	c.vars[name] = value
	c.mu.Unlock()
}

// RequestFor returns the catalog's example request for endpoint id.
func RequestFor(id string) (ConsoleRequest, bool) {
	e, ok := FindEndpoint(id)
	if !ok {
		return ConsoleRequest{}, false
	}
	return ConsoleRequest{EndpointID: e.ID, Method: e.Method, Path: e.Path, Query: e.Query, Body: e.Body, Auth: e.Auth}, true
}

var placeholder = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_]*)\}`)

// Send sends req to baseURL. Unset variables are reported instead of sent.
func (c *Console) Send(ctx context.Context, baseURL string, req ConsoleRequest) ConsoleResponse {
	vars := c.Vars()
	res := ConsoleResponse{EndpointID: req.EndpointID, Method: strings.ToUpper(strings.TrimSpace(req.Method)), RequestHeaders: map[string]string{}}
	if res.Method == "" {
		res.Method = http.MethodGet
	}
	if e, ok := FindEndpoint(req.EndpointID); ok && e.Multipart {
		res.Error = "this endpoint takes a file upload (multipart form); use the dashboard for it"
		return res
	}
	var missing []string
	fill := func(s string, enc func(string) string) string {
		return placeholder.ReplaceAllStringFunc(s, func(m string) string {
			name := m[1 : len(m)-1]
			v, ok := vars[name]
			if !ok || v == "" {
				missing = append(missing, name)
				return m
			}
			return enc(v)
		})
	}
	path := fill(strings.TrimSpace(req.Path), func(v string) string { return strings.ReplaceAll(v, "/", "%2F") })
	query := fill(strings.TrimPrefix(strings.TrimSpace(req.Query), "?"), func(v string) string { return queryEscape(v) })
	body := fillJSON(req.Body, vars, &missing)

	headers := map[string]string{}
	switch req.Auth {
	case AuthInstallation:
		headers["Authorization"] = "Bearer " + need(vars, "installationToken", &missing)
	case AuthAccount:
		headers["Authorization"] = "Bearer " + need(vars, "accountToken", &missing)
	case AuthDevice:
		if t := vars["installationToken"]; t != "" {
			headers["X-Installation-Token"] = t
		}
	case AuthAdmin:
		headers["Cookie"] = need(vars, "adminCookie", &missing)
	}
	for k, v := range req.Headers {
		headers[k] = fill(v, func(v string) string { return v })
	}
	if len(missing) > 0 {
		res.Error = "set these variables first: " + strings.Join(dedupe(missing), ", ")
		return res
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	res.URL = strings.TrimRight(baseURL, "/") + path
	if query != "" {
		res.URL += "?" + query
	}
	var rd io.Reader
	if strings.TrimSpace(body) != "" {
		if !json.Valid([]byte(body)) {
			res.Error = "the body isn't valid JSON"
			return res
		}
		rd = strings.NewReader(body)
		headers["Content-Type"] = "application/json"
		res.RequestBody = body
	}
	hreq, err := http.NewRequestWithContext(ctx, res.Method, res.URL, rd)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	for k, v := range headers {
		hreq.Header.Set(k, v)
	}
	res.RequestHeaders = maskHeaders(hreq.Header)

	client := &http.Client{Transport: transportOf(HTTPClient), Timeout: 60 * time.Second}
	start := time.Now()
	resp, err := client.Do(hreq)
	res.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = explainProbeError(err)
		return res
	}
	defer resp.Body.Close()
	res.Status, res.StatusText = resp.StatusCode, http.StatusText(resp.StatusCode)
	res.ResponseHeaders = maskHeaders(resp.Header)
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	res.DurationMs = time.Since(start).Milliseconds()
	res.ResponseBody = describeBody(data, resp.Header.Get("Content-Type"))

	if resp.StatusCode < 300 {
		res.Captured = c.capture(req.EndpointID, resp, data)
	}
	return res
}

// capture stores the values the endpoint's Captures name.
func (c *Console) capture(endpointID string, resp *http.Response, data []byte) map[string]string {
	e, ok := FindEndpoint(endpointID)
	if !ok || len(e.Captures) == 0 {
		return nil
	}
	var doc any
	_ = json.Unmarshal(data, &doc)
	out := map[string]string{}
	for field, name := range e.Captures {
		var v string
		if field == "$cookie" {
			var parts []string
			for _, ck := range resp.Cookies() {
				parts = append(parts, ck.Name+"="+ck.Value)
			}
			v = strings.Join(parts, "; ")
		} else {
			v = lookupJSON(doc, field)
		}
		if v == "" {
			continue
		}
		c.SetVar(name, v)
		if SecretVars[name] {
			out[name] = maskSecret(v)
		} else {
			out[name] = v
		}
	}
	return out
}

// lookupJSON follows a dotted path ("items.0.id") and returns a scalar as text.
func lookupJSON(doc any, path string) string {
	cur := doc
	for _, part := range strings.Split(path, ".") {
		switch x := cur.(type) {
		case map[string]any:
			cur = x[part]
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(x) {
				return ""
			}
			cur = x[i]
		default:
			return ""
		}
	}
	switch x := cur.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

// fillJSON replaces "{name}" (quoted) with the JSON string of the value and a
// bare {name} with the value as-is (for numbers and booleans).
func fillJSON(body string, vars map[string]string, missing *[]string) string {
	quoted := regexp.MustCompile(`"\{([A-Za-z][A-Za-z0-9_]*)\}"`)
	body = quoted.ReplaceAllStringFunc(body, func(m string) string {
		name := m[2 : len(m)-2]
		v, ok := vars[name]
		if !ok || v == "" {
			*missing = append(*missing, name)
			return m
		}
		b, _ := json.Marshal(v)
		return string(b)
	})
	return placeholder.ReplaceAllStringFunc(body, func(m string) string {
		name := m[1 : len(m)-1]
		v, ok := vars[name]
		if !ok || v == "" {
			*missing = append(*missing, name)
			return m
		}
		return v
	})
}

func need(vars map[string]string, name string, missing *[]string) string {
	v := vars[name]
	if v == "" {
		*missing = append(*missing, name)
	}
	return v
}

func queryEscape(v string) string {
	r := strings.NewReplacer("%", "%25", "&", "%26", "=", "%3D", "+", "%2B", " ", "%20", "#", "%23")
	return r.Replace(v)
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

// describeBody pretty-prints JSON, shows text as-is and summarizes binaries.
func describeBody(data []byte, contentType string) string {
	if len(data) == 0 {
		return ""
	}
	more := ""
	if len(data) > 1<<20 {
		data, more = data[:1<<20], " (first 1 MB)"
	}
	mt, _, _ := mime.ParseMediaType(contentType)
	if json.Valid(data) {
		var buf bytes.Buffer
		if json.Indent(&buf, bytes.TrimSpace(data), "", "  ") == nil {
			return buf.String()
		}
	}
	if strings.HasPrefix(mt, "text/") || mt == "" && isText(data) {
		s := string(data)
		if len(s) > 64<<10 {
			s = s[:64<<10] + "\n… (truncated)"
		}
		return s
	}
	return fmt.Sprintf("(%s, %d bytes%s — not shown)", orNone(mt), len(data), more)
}

func isText(data []byte) bool {
	for _, b := range data[:min(len(data), 512)] {
		if b == 0 {
			return false
		}
	}
	return true
}
