package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mango3s/internal/api"
	"mango3s/internal/update"
)

const platformAPIUsage = `Usage: mango3s platform-api <command> [flags]

Sends any request the desktop app can make to the Mango3S web platform API,
one at a time — the terminal version of Settings › API console. (The admin
dashboard's routes are not included.)

Commands:
  list [filter]          every endpoint (ID, method, path, auth)
  show <id>              an endpoint's example request
  call <id> [flags]      send it; flags override the example:
        --body JSON | --body @file.json   --query a=1&b=2
        --path /v1/...   --method POST    --auth none|installation|account|device
        -v               also print headers
  vars                   the variables ({name} in paths, queries and bodies)
  set name=value ...     set variables (e.g. set email=ada@example.com password=...)
  reset                  start the variables over from this computer's registration

Responses fill variables too (installation and account tokens, IDs), so a
flow can be walked step by step:

  mango3s platform-api call installations.register
  mango3s platform-api call notifications.list
  mango3s platform-api set email=ada@example.com password=...
  mango3s platform-api call account.login
  mango3s platform-api call account.get

Variables are kept in the system temp folder (Settings › Storage clears them).
Global flags (before the command):
`

// runPlatformAPI is `mango3s platform-api` (see `make platform-api`).
func runPlatformAPI(args []string) error {
	fs := flag.NewFlagSet("platform-api", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), platformAPIUsage)
		fs.PrintDefaults()
	}
	url := fs.String("url", api.PlatformURL(), "platform base URL (default: MANGO3S_PLATFORM_URL or the built-in URL)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fs.Usage()
		return nil
	}
	cmd, rest := rest[0], rest[1:]

	switch cmd {
	case "list":
		filter := strings.ToLower(strings.Join(rest, " "))
		group := ""
		for _, e := range update.APICatalog {
			if filter != "" && !strings.Contains(strings.ToLower(e.Group+" "+e.ID+" "+e.Name+" "+e.Path), filter) {
				continue
			}
			if e.Group != group {
				group = e.Group
				fmt.Printf("\n%s\n", group)
			}
			fmt.Printf("  %-36s %-6s %-48s %s\n", e.ID, e.Method, e.Path, e.Auth)
		}
		return nil

	case "show":
		if len(rest) != 1 {
			return errors.New("usage: platform-api show <id>")
		}
		e, ok := update.FindEndpoint(rest[0])
		if !ok {
			return fmt.Errorf("unknown endpoint %q (see `platform-api list`)", rest[0])
		}
		fmt.Printf("%s — %s\n%s\n\n%s %s", e.ID, e.Name, e.Description, e.Method, e.Path)
		if e.Query != "" {
			fmt.Printf("?%s", e.Query)
		}
		fmt.Printf("\nAuth: %s\n", e.Auth)
		if e.Body != "" {
			fmt.Printf("Body:\n%s\n", e.Body)
		}
		if len(e.Captures) > 0 {
			fmt.Printf("Saves: %v\n", e.Captures)
		}
		return nil
	}

	sess, err := loadAPISession()
	if err != nil {
		return err
	}
	console := update.NewConsole(sess)

	switch cmd {
	case "vars":
		printVars(console.Vars())
		return nil
	case "set":
		for _, kv := range rest {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				return fmt.Errorf("expected name=value, got %q", kv)
			}
			console.SetVar(k, v)
		}
		return saveAPISession(console.Vars())
	case "reset":
		_ = os.Remove(apiSessionPath())
		sess, err := loadAPISession()
		if err != nil {
			return err
		}
		printVars(sess)
		return nil
	case "call":
		return callEndpoint(*url, console, rest)
	}
	return fmt.Errorf("unknown command %q (try list, show, call, vars, set, reset)", cmd)
}

func callEndpoint(baseURL string, console *update.Console, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: platform-api call <id> [--body JSON] [--query q] [--path p] [--method M] [--auth a] [-v]")
	}
	id := args[0]
	req, ok := update.RequestFor(id)
	if !ok {
		return fmt.Errorf("unknown endpoint %q (see `platform-api list`)", id)
	}
	fs := flag.NewFlagSet("call", flag.ContinueOnError)
	body := fs.String("body", req.Body, "JSON body, or @file")
	query := fs.String("query", req.Query, "query string")
	path := fs.String("path", req.Path, "path")
	method := fs.String("method", req.Method, "HTTP method")
	auth := fs.String("auth", req.Auth, "none, installation, account or device")
	verbose := fs.Bool("v", false, "print headers")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if strings.HasPrefix(*body, "@") {
		data, err := os.ReadFile(strings.TrimPrefix(*body, "@"))
		if err != nil {
			return err
		}
		*body = string(data)
	}
	req.Body, req.Query, req.Path, req.Method, req.Auth = *body, *query, *path, *method, *auth

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res := console.Send(ctx, baseURL, req)
	if err := saveAPISession(console.Vars()); err != nil {
		fmt.Fprintln(os.Stderr, "(couldn't save variables:", err, ")")
	}

	if res.URL != "" {
		fmt.Printf("%s %s\n", res.Method, res.URL)
	}
	if *verbose {
		printHeaders(res.RequestHeaders, "> ")
	}
	if res.RequestBody != "" {
		fmt.Println(indent(update.MaskRequestBody(res.RequestBody), "> "))
	}
	if res.Status != 0 {
		fmt.Printf("\n%d %s  (%d ms)\n", res.Status, res.StatusText, res.DurationMs)
	}
	if *verbose {
		printHeaders(res.ResponseHeaders, "< ")
	}
	if res.ResponseBody != "" {
		fmt.Println(res.ResponseBody)
	}
	if len(res.Captured) > 0 {
		keys := make([]string, 0, len(res.Captured))
		for k := range res.Captured {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Println()
		for _, k := range keys {
			fmt.Printf("saved %s = %s\n", k, res.Captured[k])
		}
	}
	if res.Error != "" {
		fmt.Fprintln(os.Stderr, "\n"+res.Error)
	}
	if res.Error != "" || res.Status >= 400 {
		os.Exit(1)
	}
	return nil
}

func printVars(vars map[string]string) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := vars[k]
		if update.SecretVars[k] && v != "" {
			v = "••••" + v[max(0, len(v)-4):]
		}
		fmt.Printf("  %-20s %s\n", k, v)
	}
}

func apiSessionPath() string { return filepath.Join(os.TempDir(), "mango3s-api-session.json") }

// loadAPISession returns the saved variables, or — the first time — this
// computer's registration and platform names.
func loadAPISession() (map[string]string, error) {
	vars := map[string]string{}
	data, err := os.ReadFile(apiSessionPath())
	if err == nil {
		if err := json.Unmarshal(data, &vars); err != nil {
			return nil, fmt.Errorf("%s is damaged; run `platform-api reset`", apiSessionPath())
		}
		return vars, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if app, err := newApp(); err == nil {
		vars = api.ConsoleDefaults(app)
	} else {
		vars = map[string]string{"os": update.HostOS(), "arch": update.HostArch(), "appVersion": api.Version, "deviceFingerprint": "api-console-" + update.NewDeviceFingerprint()}
	}
	return update.NewConsole(vars).Vars(), saveAPISession(update.NewConsole(vars).Vars())
}

func saveAPISession(vars map[string]string) error {
	data, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(apiSessionPath(), data, 0o600)
}
