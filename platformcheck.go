package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"mango3s/internal/api"
	"mango3s/internal/update"
)

const platformCheckUsage = `Usage: mango3s platform-check [flags]

Tests the connection to the Mango3S web platform step by step and prints every
request the app sends and what comes back (secrets masked).

Run every check, or test APIs one at a time with --only (see --list):

  mango3s platform-check --only health
  mango3s platform-check --only notifications,privacy
  mango3s platform-check --register --only heartbeat

By default it uses this computer's own app registration, if it has one, and
only reads. --register instead registers a throwaway installation (labelled
"platform-check") so the authenticated calls can be tried against a fresh
server.

Flags:
`

// runPlatformCheck is `mango3s platform-check` (see `make platform-check`).
func runPlatformCheck(args []string) error {
	fs := flag.NewFlagSet("platform-check", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), platformCheckUsage)
		fs.PrintDefaults()
	}
	url := fs.String("url", api.PlatformURL(), "platform base URL (default: MANGO3S_PLATFORM_URL or the built-in URL)")
	register := fs.Bool("register", false, "register a throwaway installation and test the authenticated calls with it")
	anonymous := fs.Bool("anonymous", false, "don't use this computer's app registration (public calls only, unless --register)")
	verbose := fs.Bool("v", false, "print request and response headers and bodies")
	asJSON := fs.Bool("json", false, "print the report and the recorded requests as JSON")
	only := fs.String("only", "", "comma-separated check IDs to run instead of all (see --list)")
	list := fs.Bool("list", false, "list the checks and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *list {
		for _, c := range update.ProbeChecks {
			note := ""
			if c.NeedsToken {
				note += " [needs registration]"
			}
			if c.Writes {
				note += " [writes]"
			}
			fmt.Printf("%-14s %-5s %-34s %s%s\n", c.ID, c.Method, c.Path, c.Description, note)
		}
		return nil
	}
	var ids []string
	for _, id := range strings.Split(*only, ",") {
		if id = strings.TrimSpace(id); id == "" {
			continue
		}
		if _, ok := update.FindCheck(id); !ok {
			return fmt.Errorf("unknown check %q (see --list)", id)
		}
		ids = append(ids, id)
	}

	var printed int
	update.EnableTrace(500, func(e update.TraceEntry) {
		if *asJSON {
			return
		}
		printed++
		printTraceEntry(e, *verbose)
	})

	opts := update.ProbeOptions{OS: update.HostOS(), Arch: update.HostArch(), Version: api.Version, Heartbeat: true}
	switch {
	case *register:
		opts.Register = func(ctx context.Context, c *update.Client) (string, string, error) {
			resp, err := c.Register(ctx, update.RegisterRequest{
				DeviceFingerprint: "platform-check-" + update.NewDeviceFingerprint(),
				AppVersion:        api.Version, OS: update.HostOS(), Arch: update.HostArch(),
				Label: "platform-check",
			})
			return resp.InstallationToken, resp.InstallationID, err
		}
	case !*anonymous:
		if app, err := newApp(); err == nil {
			opts.Token, opts.InstallationID = api.InstallationCredentials(app)
		} else if !*asJSON {
			fmt.Fprintln(os.Stderr, "(couldn't read the app's registration:", err, "— running public checks only)")
		}
	}

	if !*asJSON {
		fmt.Printf("Mango3S %s → %s\n", api.Version, *url)
		switch {
		case *register:
			fmt.Println("Mode: registering a throwaway installation")
		case opts.Token != "":
			fmt.Println("Mode: this computer's registration (installation " + opts.InstallationID + ")")
		default:
			fmt.Println("Mode: public calls only (this computer isn't registered yet; try --register)")
		}
		fmt.Println()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := update.New(*url)
	var rep update.ProbeReport
	if len(ids) == 0 {
		rep = client.Probe(ctx, opts)
	} else {
		// With --register, registration runs first so the named checks can use it.
		if *register && ids[0] != update.CheckRegister {
			ids = append([]string{update.CheckRegister}, ids...)
		}
		rep = update.ProbeReport{BaseURL: *url, OK: true}
		for _, id := range ids {
			s := client.RunCheck(ctx, id, &opts)
			if !s.OK && !s.Skipped {
				rep.OK = false
			}
			rep.Steps = append(rep.Steps, s)
		}
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{"report": rep, "requests": update.TraceEntries()}); err != nil {
			return err
		}
	} else {
		if printed > 0 {
			fmt.Println()
		}
		printReport(rep)
	}
	if !rep.OK {
		os.Exit(1)
	}
	return nil
}

func printTraceEntry(e update.TraceEntry, verbose bool) {
	status := fmt.Sprint(e.Status)
	if e.Status == 0 {
		status = "---"
	}
	fmt.Printf("  → %-6s %s  %s  %dms", e.Method, e.URL, status, e.DurationMs)
	if e.Error != "" {
		fmt.Printf("  %s", e.Error)
	}
	fmt.Println()
	if !verbose {
		return
	}
	printHeaders(e.RequestHeaders, "      > ")
	if e.RequestBody != "" {
		fmt.Println(indent(e.RequestBody, "      > "))
	}
	printHeaders(e.ResponseHeaders, "      < ")
	if e.ResponseBody != "" {
		fmt.Println(indent(e.ResponseBody, "      < "))
	}
}

func printReport(rep update.ProbeReport) {
	for _, s := range rep.Steps {
		mark := "✓"
		switch {
		case s.Skipped:
			mark = "–"
		case !s.OK:
			mark = "✗"
		}
		line := fmt.Sprintf("%s %-22s %-5s %s", mark, s.Name, s.Method, s.Path)
		if !s.Skipped {
			line += fmt.Sprintf("  (%dms)", s.DurationMs)
		}
		fmt.Println(line)
		switch {
		case s.Error != "":
			fmt.Println("    " + s.Error)
		case s.Detail != "":
			fmt.Println("    " + s.Detail)
		}
	}
	fmt.Println()
	if rep.OK {
		fmt.Println("All checks passed.")
	} else {
		fmt.Println("Some checks failed.")
	}
}

func printHeaders(h map[string]string, prefix string) {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%s%s: %s\n", prefix, k, h[k])
	}
}

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}
