package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"mango3s/internal/apperr"
)

// The shape Claude Code 2.1 prints for -p --output-format json --json-schema (captured from a real run).
const claudeCodeOK = `{"is_error":false,"num_turns":2,"subtype":"success","api_error_status":null,"result":"{\"kind\":\"find\"}",
"structured_output":{"kind":"find","filter":"{ status: \"A\" }","projection":"","sort":"","limit":0,"pipeline":[],"update":"","explanation":"x","warnings":[]},
"usage":{"input_tokens":649,"output_tokens":52},"modelUsage":{"claude-haiku-4-5":{"outputTokens":13},"claude-sonnet-5":{"outputTokens":52}},"type":"result"}`

type call struct {
	dir, path string
	args      []string
	stdin     string
}

func fakeRunner(t *testing.T, respond func(c call) ([]byte, []byte, error)) *[]call {
	t.Helper()
	calls := &[]call{}
	old := CommandRunner
	CommandRunner = func(_ context.Context, dir, path string, args []string, stdin []byte) ([]byte, []byte, error) {
		c := call{dir, path, args, string(stdin)}
		*calls = append(*calls, c)
		return respond(c)
	}
	t.Cleanup(func() { CommandRunner = old })
	return calls
}

func argAfter(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func TestClaudeCodeRunsWithoutToolsOrSettings(t *testing.T) {
	calls := fakeRunner(t, func(c call) ([]byte, []byte, error) {
		entries, _ := os.ReadDir(c.dir)
		if len(entries) != 0 {
			t.Errorf("Claude Code must run in an empty directory, found %d entries", len(entries))
		}
		return []byte(claudeCodeOK), nil, nil
	})
	cc := &ClaudeCode{Path: "/bin/claude", Model: "sonnet"}
	raw, u, err := cc.Generate(context.Background(), "SYSTEM", "USER PROMPT", AnswerSchema())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"filter":"{ status: \"A\" }"`) {
		t.Fatalf("structured_output must be used: %s", raw)
	}
	if u.InputTokens != 649 || u.OutputTokens != 52 || u.Model != "Claude Code · claude-sonnet-5" {
		t.Fatalf("usage: %+v", u)
	}
	c := (*calls)[0]
	if c.stdin != "USER PROMPT" {
		t.Fatalf("the prompt goes on stdin, never in argv: %q", c.stdin)
	}
	for flag, want := range map[string]string{"--tools": "", "--setting-sources": "", "--output-format": "json", "--system-prompt": "SYSTEM", "--model": "sonnet"} {
		if v, ok := argAfter(c.args, flag); !ok || v != want {
			t.Errorf("%s = %q (present %v), want %q", flag, v, ok, want)
		}
	}
	for _, f := range []string{"-p", "--no-session-persistence", "--strict-mcp-config"} {
		if !hasArg(c.args, f) {
			t.Errorf("missing %s in %v", f, c.args)
		}
	}
	schema, _ := argAfter(c.args, "--json-schema")
	if !json.Valid([]byte(schema)) || !strings.Contains(schema, `"kind"`) {
		t.Fatalf("schema: %s", schema)
	}
	if strings.Contains(strings.Join(c.args, " "), "dangerously") {
		t.Fatal("permissions must never be bypassed")
	}
}

func hasArg(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func TestClaudeCodeErrors(t *testing.T) {
	fakeRunner(t, func(call) ([]byte, []byte, error) {
		return []byte(`{"is_error":true,"subtype":"success","result":"API Error: 429 rate limited","api_error_status":429}`), nil, errors.New("exit 1")
	})
	_, _, err := (&ClaudeCode{Path: "c"}).Generate(context.Background(), "s", "u", AnswerSchema())
	if !apperr.Is(err, apperr.CodeTimeout) {
		t.Fatalf("rate limit: %v", err)
	}
	fakeRunner(t, func(call) ([]byte, []byte, error) {
		return nil, []byte("Invalid API key · Please run /login\n"), errors.New("exit status 1")
	})
	_, _, err = (&ClaudeCode{Path: "c"}).Generate(context.Background(), "s", "u", AnswerSchema())
	if !apperr.Is(err, apperr.CodeAuthFailed) {
		t.Fatalf("signed out: %v", err)
	}
}

func TestDetectClaudeCode(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/claude"
	_ = os.WriteFile(path, nil, 0o755)
	fakeRunner(t, func(c call) ([]byte, []byte, error) {
		if c.args[0] == "--version" {
			return []byte("2.1.220 (Claude Code)\n"), nil, nil
		}
		// No email or org must be surfaced.
		return []byte(`{"loggedIn":true,"authMethod":"claude.ai","email":"me@example.com","orgId":"o","subscriptionType":"pro"}`), nil, nil
	})
	info := DetectClaudeCode(context.Background(), path)
	if !info.Found || info.Version != "2.1.220" || info.SignedIn == nil || !*info.SignedIn || info.Account != "claude.ai (pro)" {
		t.Fatalf("%+v", info)
	}
	b, _ := json.Marshal(info)
	if strings.Contains(string(b), "example.com") {
		t.Fatal("the account email must not be exposed")
	}
	fakeRunner(t, func(c call) ([]byte, []byte, error) {
		if c.args[0] == "--version" {
			return []byte("2.1.220"), nil, nil
		}
		return []byte(`{"loggedIn":false}`), nil, errors.New("exit 1")
	})
	if info := DetectClaudeCode(context.Background(), path); info.SignedIn == nil || *info.SignedIn || !strings.Contains(info.Message, "sign in") {
		t.Fatalf("signed out: %+v", info)
	}
	old := LookPath
	LookPath = func(string) (string, error) { return "", errors.New("no") }
	t.Setenv("HOME", t.TempDir())
	defer func() { LookPath = old }()
	if info := DetectClaudeCode(context.Background(), ""); info.Found || !strings.Contains(info.Message, "Not installed") {
		t.Fatalf("missing: %+v", info)
	}
}

func TestCodexUsesReadOnlySandboxAndSchemaFile(t *testing.T) {
	calls := fakeRunner(t, func(c call) ([]byte, []byte, error) {
		schemaPath, _ := argAfter(c.args, "--output-schema")
		if b, err := os.ReadFile(schemaPath); err != nil || !strings.Contains(string(b), `"kind"`) {
			t.Errorf("schema file: %v", err)
		}
		out, _ := argAfter(c.args, "-o")
		_ = os.WriteFile(out, []byte(`{"kind":"find","filter":"{}"}`), 0o600)
		return []byte("progress…"), nil, nil
	})
	raw, u, err := (&Codex{Path: "/bin/codex", Model: "gpt-5"}).Generate(context.Background(), "SYSTEM", "USER", AnswerSchema())
	if err != nil || raw != `{"kind":"find","filter":"{}"}` || u.Model != "Codex · gpt-5" {
		t.Fatalf("%q %+v %v", raw, u, err)
	}
	c := (*calls)[0]
	if c.args[0] != "exec" || c.args[len(c.args)-1] != "-" {
		t.Fatalf("args: %v", c.args)
	}
	if v, _ := argAfter(c.args, "--sandbox"); v != "read-only" {
		t.Fatal("Codex must run read-only")
	}
	for _, f := range []string{"--ephemeral", "--skip-git-repo-check"} {
		if !hasArg(c.args, f) {
			t.Errorf("missing %s", f)
		}
	}
	if !strings.HasPrefix(c.stdin, "SYSTEM") || !strings.Contains(c.stdin, "USER") {
		t.Fatalf("stdin: %q", c.stdin)
	}
	fakeRunner(t, func(call) ([]byte, []byte, error) {
		return nil, []byte("Not logged in. Run codex login"), errors.New("exit 1")
	})
	if _, _, err := (&Codex{Path: "c"}).Generate(context.Background(), "s", "u", AnswerSchema()); !apperr.Is(err, apperr.CodeAuthFailed) {
		t.Fatalf("codex signed out: %v", err)
	}
}

func TestLocalOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_, _ = io.WriteString(w, `{"models":[{"name":"qwen2.5-coder:7b"},{"name":"llama3.2"}]}`)
		case "/api/chat":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["model"] != "qwen2.5-coder:7b" || body["stream"] != false || body["format"] == nil {
				t.Errorf("request: %v", body)
			}
			msgs := body["messages"].([]any)
			if !strings.Contains(msgs[0].(map[string]any)["content"].(string), "JSON schema") {
				t.Error("the schema must also be in the system prompt")
			}
			_, _ = io.WriteString(w, "{\"message\":{\"content\":\"```json\\n{\\\"kind\\\":\\\"find\\\"}\\n```\"},\"prompt_eval_count\":120,\"eval_count\":30}")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	models, err := ListLocalModels(context.Background(), srv.URL, LocalOllama, "")
	if err != nil || strings.Join(models, ",") != "qwen2.5-coder:7b,llama3.2" {
		t.Fatalf("%v %v", models, err)
	}
	raw, u, err := (&Local{URL: srv.URL, API: LocalOllama, Model: "qwen2.5-coder:7b"}).Generate(context.Background(), "S", "U", AnswerSchema())
	if err != nil || ExtractJSON(raw) != `{"kind":"find"}` || u.InputTokens != 120 || u.OutputTokens != 30 {
		t.Fatalf("%q %+v %v", raw, u, err)
	}
	if _, _, err := (&Local{URL: srv.URL, API: LocalOllama}).Generate(context.Background(), "S", "U", AnswerSchema()); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatal("a model is required")
	}
}

func TestLocalOpenAICompatibleFallsBackToJSONObject(t *testing.T) {
	var formats []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Error("the stored key is sent as a bearer token")
		}
		switch r.URL.Path {
		case "/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"local-model"}]}`)
		case "/v1/chat/completions":
			var body struct {
				ResponseFormat struct {
					Type string `json:"type"`
				} `json:"response_format"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			formats = append(formats, body.ResponseFormat.Type)
			if body.ResponseFormat.Type == "json_schema" {
				w.WriteHeader(400)
				_, _ = io.WriteString(w, `{"error":{"message":"json_schema not supported"}}`)
				return
			}
			_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Here you go: {\"kind\":\"aggregate\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":6}}`)
		}
	}))
	defer srv.Close()
	if m, err := ListLocalModels(context.Background(), srv.URL+"/v1", LocalOpenAI, "k"); err != nil || m[0] != "local-model" {
		t.Fatalf("%v %v", m, err)
	}
	raw, u, err := (&Local{URL: srv.URL, API: LocalOpenAI, Model: "local-model", APIKey: "k"}).Generate(context.Background(), "S", "U", AnswerSchema())
	if err != nil || ExtractJSON(raw) != `{"kind":"aggregate"}` || u.OutputTokens != 6 {
		t.Fatalf("%q %+v %v", raw, u, err)
	}
	if strings.Join(formats, ",") != "json_schema,json_object" {
		t.Fatalf("formats tried: %v", formats)
	}
}

func TestLocalServerErrorsAndDetection(t *testing.T) {
	if _, err := ListLocalModels(context.Background(), "ftp://x", LocalOllama, ""); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatal(err)
	}
	if _, err := ListLocalModels(context.Background(), "http://127.0.0.1:1", LocalOllama, ""); !apperr.Is(err, apperr.CodeUnreachable) {
		t.Fatalf("not running: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"models":[{"name":"phi4"}]}`)
	}))
	defer srv.Close()
	old := knownServers
	knownServers = []LocalServer{{Name: "Ollama", URL: srv.URL, API: LocalOllama}, {Name: "Nothing", URL: "http://127.0.0.1:1", API: LocalOpenAI}}
	defer func() { knownServers = old }()
	found := DetectLocalServers(context.Background())
	if len(found) != 1 || found[0].Name != "Ollama" || found[0].Models[0] != "phi4" {
		t.Fatalf("%+v", found)
	}
	if !IsLoopback("http://localhost:11434") || !IsLoopback("http://127.0.0.1:1234") || IsLoopback("http://gpu-box.lan:8000") {
		t.Fatal("loopback detection")
	}
}

func TestExtractJSON(t *testing.T) {
	for in, want := range map[string]string{
		`{"a":1}`:                        `{"a":1}`,
		"```json\n{\"a\":1}\n```":        `{"a":1}`,
		"Sure!\n```\n{\"a\":1}\n```\nok": `{"a":1}`,
		`Answer: {"a":{"b":2}} done`:     `{"a":{"b":2}}`,
		`nothing`:                        `nothing`,
	} {
		if got := ExtractJSON(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}
