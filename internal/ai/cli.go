package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"mango3s/internal/apperr"
)

// CLIInfo describes an installed AI command-line tool (Claude Code, Codex).
type CLIInfo struct {
	Name     string `json:"name"`
	Found    bool   `json:"found"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	SignedIn *bool  `json:"signedIn"` // nil = unknown
	Account  string `json:"account"`  // e.g. "claude.ai (pro)" — never an email or token
	Message  string `json:"message"`
}

// CommandRunner runs a program (replaceable in tests). stdin may be nil.
var CommandRunner = func(ctx context.Context, dir, path string, args []string, stdin []byte) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.WaitDelay = 2 * time.Second
	err = cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

// LookPath resolves a program on PATH (replaceable in tests).
var LookPath = exec.LookPath

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// findCLI locates a CLI: the configured path, PATH, then usual install folders.
func findCLI(name, override string) (string, error) {
	if override != "" {
		if st, err := os.Stat(override); err != nil || st.IsDir() {
			return "", apperr.Invalid("The configured path %s does not exist.", override)
		}
		return override, nil
	}
	if p, err := LookPath(exeName(name)); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	dirs := []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".claude", "local"), filepath.Join(home, ".npm-global", "bin"),
		"/opt/homebrew/bin", "/usr/local/bin", filepath.Join(home, "AppData", "Roaming", "npm")}
	for _, d := range dirs {
		p := filepath.Join(d, exeName(name))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", nil
}

var semver = regexp.MustCompile(`\d+\.\d+\.\d+`)

// DetectClaudeCode reports whether Claude Code is installed and signed in.
func DetectClaudeCode(ctx context.Context, override string) CLIInfo {
	info := CLIInfo{Name: "Claude Code"}
	path, err := findCLI("claude", override)
	if err != nil {
		info.Message = apperr.From(err).Message
		return info
	}
	if path == "" {
		info.Message = "Not installed. Install Claude Code from https://claude.com/claude-code and sign in with `claude`."
		return info
	}
	info.Found, info.Path = true, path
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if out, _, err := CommandRunner(cctx, "", path, []string{"--version"}, nil); err == nil {
		info.Version = semver.FindString(string(out))
	}
	// `claude auth status` prints JSON and makes no model request.
	out, _, err := CommandRunner(cctx, "", path, []string{"auth", "status", "--json"}, nil)
	var st struct {
		LoggedIn         *bool  `json:"loggedIn"`
		AuthMethod       string `json:"authMethod"`
		SubscriptionType string `json:"subscriptionType"`
	}
	if jerr := json.Unmarshal(out, &st); jerr == nil && st.LoggedIn != nil {
		info.SignedIn = st.LoggedIn
		if *st.LoggedIn {
			info.Account = st.AuthMethod
			if st.SubscriptionType != "" {
				info.Account += " (" + st.SubscriptionType + ")"
			}
		} else {
			info.Message = "Installed but not signed in. Run `claude` once and sign in."
		}
	} else if err != nil {
		info.Message = "Installed, but the sign-in status could not be read; Test will tell."
	}
	return info
}

// DetectCodex reports whether the Codex CLI is installed and signed in.
func DetectCodex(ctx context.Context, override string) CLIInfo {
	info := CLIInfo{Name: "Codex"}
	path, err := findCLI("codex", override)
	if err != nil {
		info.Message = apperr.From(err).Message
		return info
	}
	if path == "" {
		info.Message = "Not installed. Install the Codex CLI (npm i -g @openai/codex) and sign in with `codex login`."
		return info
	}
	info.Found, info.Path = true, path
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if out, _, err := CommandRunner(cctx, "", path, []string{"--version"}, nil); err == nil {
		info.Version = semver.FindString(string(out))
	}
	out, errOut, err := CommandRunner(cctx, "", path, []string{"login", "status"}, nil)
	text := strings.ToLower(string(out) + string(errOut))
	switch {
	case err == nil && strings.Contains(text, "logged in"):
		yes := true
		info.SignedIn = &yes
		if strings.Contains(text, "chatgpt") {
			info.Account = "ChatGPT"
		} else if strings.Contains(text, "api key") {
			info.Account = "API key"
		}
	case strings.Contains(text, "not logged in"):
		no := false
		info.SignedIn = &no
		info.Message = "Installed but not signed in. Run `codex login`."
	default:
		info.Message = "Installed, but the sign-in status could not be read; Test will tell."
	}
	return info
}

// emptyDir is a fresh working directory with nothing in it, so a CLI agent
// has no project files around it.
func emptyDir() (string, func(), error) {
	dir, err := os.MkdirTemp("", "mango3s-ai-*")
	if err != nil {
		return "", func() {}, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

func cliError(name string, ctx context.Context, err error, stderr []byte) error {
	if ctx.Err() != nil {
		return apperr.From(ctx.Err())
	}
	msg := strings.TrimSpace(string(stderr))
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "log in") || strings.Contains(lower, "login") || strings.Contains(lower, "not logged") || strings.Contains(lower, "authenticat"):
		return apperr.New(apperr.CodeAuthFailed, "%s is not signed in. Sign in from a terminal, then try again.", name)
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "usage limit") || strings.Contains(lower, "429"):
		return apperr.New(apperr.CodeTimeout, "%s reports a usage or rate limit; try again later.", name)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if msg == "" {
			msg = exitErr.Error()
		}
		return apperr.New(apperr.CodeUnreachable, "%s failed: %s", name, msg)
	}
	return apperr.Wrap(apperr.CodeUnreachable, err, "Could not run "+name+".")
}

// ClaudeCode generates answers with the user's installed Claude Code CLI,
// using its own sign-in. It runs non-interactively with every tool disabled,
// no session persistence, no MCP servers and no user/project settings, in an
// empty directory.
type ClaudeCode struct {
	Path  string
	Model string // empty = Claude Code's default
}

// Generate implements Provider.
func (c *ClaudeCode) Generate(ctx context.Context, system, user string, schema map[string]any) (string, Usage, error) {
	schemaJSON, _ := json.Marshal(schema)
	args := []string{"-p", "--output-format", "json", "--json-schema", string(schemaJSON), "--system-prompt", system,
		"--tools", "", "--no-session-persistence", "--setting-sources", "", "--strict-mcp-config"}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	dir, cleanup, err := emptyDir()
	if err != nil {
		return "", Usage{}, apperr.Wrap(apperr.CodeInternal, err, "Could not prepare a working folder for Claude Code.")
	}
	defer cleanup()
	out, stderr, err := CommandRunner(ctx, dir, c.Path, args, []byte(user))
	var res struct {
		IsError          bool            `json:"is_error"`
		Subtype          string          `json:"subtype"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
		APIErrorStatus   *int            `json:"api_error_status"`
		Usage            struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
		ModelUsage map[string]struct {
			OutputTokens int64 `json:"outputTokens"`
		} `json:"modelUsage"`
	}
	if jerr := json.Unmarshal(out, &res); jerr != nil {
		if err != nil {
			return "", Usage{}, cliError("Claude Code", ctx, err, stderr)
		}
		return "", Usage{}, apperr.New(apperr.CodeMongo, "Claude Code returned something other than JSON.")
	}
	u := Usage{InputTokens: res.Usage.InputTokens, OutputTokens: res.Usage.OutputTokens, Model: "Claude Code"}
	// The main model is the one that wrote the most output.
	var best int64 = -1
	for m, mu := range res.ModelUsage {
		if mu.OutputTokens > best {
			best, u.Model = mu.OutputTokens, "Claude Code · "+m
		}
	}
	if res.IsError || (res.Subtype != "" && res.Subtype != "success") {
		msg := res.Result
		if msg == "" {
			msg = res.Subtype
		}
		if res.APIErrorStatus != nil && *res.APIErrorStatus == 429 {
			return "", u, apperr.New(apperr.CodeTimeout, "Claude Code reports a usage or rate limit; try again later.")
		}
		return "", u, cliError("Claude Code", ctx, errors.New(msg), []byte(msg))
	}
	if len(res.StructuredOutput) > 0 && string(res.StructuredOutput) != "null" {
		return string(res.StructuredOutput), u, nil
	}
	return res.Result, u, nil
}

// Codex generates answers with the user's installed Codex CLI (`codex exec`),
// using its own sign-in, in its default read-only sandbox, without saving the
// session, in an empty directory. Codex has no system-prompt flag, so the
// instructions are sent first in the prompt.
type Codex struct {
	Path  string
	Model string
}

// Generate implements Provider.
func (c *Codex) Generate(ctx context.Context, system, user string, schema map[string]any) (string, Usage, error) {
	dir, cleanup, err := emptyDir()
	if err != nil {
		return "", Usage{}, apperr.Wrap(apperr.CodeInternal, err, "Could not prepare a working folder for Codex.")
	}
	defer cleanup()
	// The schema and answer files live outside the working directory.
	files, cleanFiles, err := emptyDir()
	if err != nil {
		return "", Usage{}, apperr.Wrap(apperr.CodeInternal, err, "Could not prepare files for Codex.")
	}
	defer cleanFiles()
	schemaPath, answerPath := filepath.Join(files, "schema.json"), filepath.Join(files, "answer.json")
	schemaJSON, _ := json.Marshal(schema)
	if err := os.WriteFile(schemaPath, schemaJSON, 0o600); err != nil {
		return "", Usage{}, apperr.Wrap(apperr.CodeInternal, err, "Could not write the answer schema.")
	}
	args := []string{"exec", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--output-schema", schemaPath, "-o", answerPath}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	args = append(args, "-")
	prompt := system + "\n\nDo not run commands or read files; answer only from the information below.\n\n" + user
	_, stderr, err := CommandRunner(ctx, dir, c.Path, args, []byte(prompt))
	answer, rerr := os.ReadFile(answerPath)
	if err != nil && (rerr != nil || len(bytes.TrimSpace(answer)) == 0) {
		return "", Usage{}, cliError("Codex", ctx, err, stderr)
	}
	if rerr != nil || len(bytes.TrimSpace(answer)) == 0 {
		return "", Usage{}, apperr.New(apperr.CodeMongo, "Codex finished without an answer.")
	}
	model := "Codex"
	if c.Model != "" {
		model += " · " + c.Model
	}
	return string(answer), Usage{Model: model}, nil
}
