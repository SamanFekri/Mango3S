package dump

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"mango3s/internal/config"
	"mango3s/internal/mongox"
)

// ToolDump runs mongodump. It writes a single .archive file (or a folder
// when Archive is empty and Out is set) and is the only way to include users
// and roles or a point-in-time oplog.
type ToolDump struct {
	Archive            string   `json:"archive"` // file path; empty → Out folder
	Out                string   `json:"out"`
	Gzip               bool     `json:"gzip"`
	DB                 string   `json:"db"`         // empty: all databases
	Collection         string   `json:"collection"` // requires DB
	ExcludeCollections []string `json:"excludeCollections"`
	Query              string   `json:"query"`         // Extended JSON filter; requires Collection
	UsersAndRoles      bool     `json:"usersAndRoles"` // requires DB
	Oplog              bool     `json:"oplog"`         // all databases on a replica set
	ViewsAsCollections bool     `json:"viewsAsCollections"`
}

// ToolRestore runs mongorestore.
type ToolRestore struct {
	Archive          string   `json:"archive"` // .archive file; empty → Dir
	Dir              string   `json:"dir"`
	Gzip             bool     `json:"gzip"`
	NsInclude        []string `json:"nsInclude"`
	NsExclude        []string `json:"nsExclude"`
	NsFrom           []string `json:"nsFrom"`
	NsTo             []string `json:"nsTo"`
	Drop             bool     `json:"drop"`
	NoIndexRestore   bool     `json:"noIndexRestore"`
	NoOptionsRestore bool     `json:"noOptionsRestore"`
	StopOnError      bool     `json:"stopOnError"`
	BypassValidation bool     `json:"bypassValidation"`
	UsersAndRoles    bool     `json:"usersAndRoles"`
	OplogReplay      bool     `json:"oplogReplay"`
	DryRun           bool     `json:"dryRun"`
}

// ToolArgs is a prepared tool invocation. Secrets are only in the config
// file (mode 0600, removed by Cleanup), never on the command line where other
// processes could read them.
type ToolArgs struct {
	Args    []string
	config  string
	secrets []string
}

// Cleanup removes the temporary config file.
func (t *ToolArgs) Cleanup() {
	if t.config != "" {
		os.Remove(t.config)
		t.config = ""
	}
}

// Scrub removes any secret from a line of tool output.
func (t *ToolArgs) Scrub(s string) string {
	for _, sec := range t.secrets {
		if sec != "" {
			s = strings.ReplaceAll(s, sec, "*****")
		}
	}
	return s
}

// CanUseTools reports why a profile cannot be used with the external tools.
func CanUseTools(p config.Profile) error {
	p = p.Normalize()
	if p.SSH.Enabled {
		return errors.New("this connection uses an SSH tunnel, which the MongoDB Database Tools cannot use; choose the built-in backup format (a folder that mongorestore can also read)")
	}
	if p.Proxy.Type != config.ProxyNone {
		return errors.New("this connection uses a proxy, which the MongoDB Database Tools cannot use; choose the built-in backup format (a folder that mongorestore can also read)")
	}
	if p.AuthMechanism == config.MechAWS && p.HasSecret(config.SecretAWSSessionToken) {
		return errors.New("AWS authentication with a session token can't be passed to the tools securely; choose the built-in backup format")
	}
	return nil
}

// connArgs prepares the connection part of a tool command line.
func connArgs(p config.Profile, c mongox.Credentials) (*ToolArgs, error) {
	if err := CanUseTools(p); err != nil {
		return nil, err
	}
	p = p.Normalize()
	// Not net/url: it rejects several hosts, as in every replica set string.
	u, err := config.ParseConnString(p.URI)
	if err != nil {
		return nil, errors.New("the connection string is not valid")
	}
	t := &ToolArgs{}
	cfg := map[string]string{}
	if u.HasPass {
		// Credentials embedded in the connection string stay in the config file.
		pw, _ := url.PathUnescape(u.Password)
		t.secrets = append(t.secrets, pw, u.Password, url.QueryEscape(pw), url.PathEscape(pw))
	}
	if p.Username != "" {
		// The form's user wins over one in the URI, like BuildClientOptions.
		u.User, u.Password, u.HasPass = "", "", false
		t.Args = append(t.Args, "--username="+p.Username)
		if c.Password != "" {
			cfg["password"] = c.Password
			t.secrets = append(t.secrets, c.Password)
		}
	}
	q := u.Query
	if p.AuthSource != "" {
		q.Del("authSource")
		t.Args = append(t.Args, "--authenticationDatabase="+p.AuthSource)
	}
	if p.AuthMechanism != "" {
		q.Del("authMechanism")
		t.Args = append(t.Args, "--authenticationMechanism="+p.AuthMechanism)
	}
	if p.Advanced.AppName == "" && q.Get("appName") == "" {
		q.Set("appName", "Mango3S")
	}
	if p.TLS.Enabled {
		q.Set("tls", "true")
		if p.TLS.CAFile != "" {
			q.Set("tlsCAFile", p.TLS.CAFile)
		}
		if p.TLS.CertKeyFile != "" {
			q.Set("tlsCertificateKeyFile", p.TLS.CertKeyFile)
			if c.TLSKeyPassword != "" {
				cfg["sslPEMKeyPassword"] = c.TLSKeyPassword
				t.secrets = append(t.secrets, c.TLSKeyPassword)
			}
		}
		if p.TLS.AllowInvalidCertificates || p.TLS.AllowInvalidHostnames {
			q.Set("tlsInsecure", "true")
		}
	}
	if p.Advanced.ReadPreference != "" {
		q.Set("readPreference", p.Advanced.ReadPreference)
	}
	u.Query = q
	cfg["uri"] = u.String()
	f, err := os.CreateTemp("", "mango3s-tool-*.yaml")
	if err != nil {
		return nil, err
	}
	t.config = f.Name()
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		t.Cleanup()
		return nil, err
	}
	var b strings.Builder
	for _, k := range []string{"uri", "password", "sslPEMKeyPassword"} {
		if v, ok := cfg[k]; ok {
			// A JSON string is a valid YAML double-quoted scalar.
			j, _ := json.Marshal(v)
			fmt.Fprintf(&b, "%s: %s\n", k, j)
		}
	}
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		t.Cleanup()
		return nil, err
	}
	if err := f.Close(); err != nil {
		t.Cleanup()
		return nil, err
	}
	t.Args = append([]string{"--config=" + t.config}, t.Args...)
	return t, nil
}

// DumpArgs builds a mongodump invocation.
func DumpArgs(p config.Profile, c mongox.Credentials, o ToolDump) (*ToolArgs, error) {
	if o.Archive == "" && o.Out == "" {
		return nil, errors.New("choose an archive file or an output folder")
	}
	if o.Collection != "" && o.DB == "" {
		return nil, errors.New("a collection needs its database")
	}
	if o.Query != "" && o.Collection == "" {
		return nil, errors.New("a query filter needs a single collection")
	}
	if o.UsersAndRoles && (o.DB == "" || o.DB == "admin") {
		return nil, errors.New("users and roles can be included only when backing up one database other than admin")
	}
	if o.Oplog && o.DB != "" {
		return nil, errors.New("a point-in-time oplog needs a backup of all databases")
	}
	if o.Archive != "" {
		if _, err := os.Stat(o.Archive); err == nil {
			return nil, fmt.Errorf("%s already exists; choose a new file name so no backup is overwritten", o.Archive)
		}
	} else if err := prepareDir(o.Out); err != nil {
		return nil, err
	}
	t, err := connArgs(p, c)
	if err != nil {
		return nil, err
	}
	if o.Archive != "" {
		t.Args = append(t.Args, "--archive="+o.Archive)
	} else {
		t.Args = append(t.Args, "--out="+o.Out)
	}
	if o.Gzip {
		t.Args = append(t.Args, "--gzip")
	}
	if o.DB != "" {
		t.Args = append(t.Args, "--db="+o.DB)
	}
	if o.Collection != "" {
		t.Args = append(t.Args, "--collection="+o.Collection)
	}
	for _, x := range o.ExcludeCollections {
		if x = strings.TrimSpace(x); x != "" {
			t.Args = append(t.Args, "--excludeCollection="+x)
		}
	}
	if o.Query != "" {
		t.Args = append(t.Args, "--query="+o.Query)
	}
	if o.UsersAndRoles {
		t.Args = append(t.Args, "--dumpDbUsersAndRoles")
	}
	if o.Oplog {
		t.Args = append(t.Args, "--oplog")
	}
	if o.ViewsAsCollections {
		t.Args = append(t.Args, "--viewsAsCollections")
	}
	return t, nil
}

// RestoreArgs builds a mongorestore invocation.
func RestoreArgs(p config.Profile, c mongox.Credentials, o ToolRestore) (*ToolArgs, error) {
	if o.Archive == "" && o.Dir == "" {
		return nil, errors.New("choose an archive file or a backup folder")
	}
	if len(o.NsFrom) != len(o.NsTo) {
		return nil, errors.New("each rename needs both a source and a target pattern")
	}
	t, err := connArgs(p, c)
	if err != nil {
		return nil, err
	}
	if o.Archive != "" {
		t.Args = append(t.Args, "--archive="+o.Archive)
	} else {
		t.Args = append(t.Args, "--dir="+o.Dir)
	}
	flags := []struct {
		on   bool
		flag string
	}{
		{o.Gzip, "--gzip"}, {o.Drop, "--drop"}, {o.NoIndexRestore, "--noIndexRestore"}, {o.NoOptionsRestore, "--noOptionsRestore"},
		{o.StopOnError, "--stopOnError"}, {o.BypassValidation, "--bypassDocumentValidation"}, {o.UsersAndRoles, "--restoreDbUsersAndRoles"},
		{o.OplogReplay, "--oplogReplay"}, {o.DryRun, "--dryRun"}, {true, "--objcheck"},
	}
	for _, f := range flags {
		if f.on {
			t.Args = append(t.Args, f.flag)
		}
	}
	for _, x := range o.NsInclude {
		t.Args = append(t.Args, "--nsInclude="+x)
	}
	for _, x := range o.NsExclude {
		t.Args = append(t.Args, "--nsExclude="+x)
	}
	for i := range o.NsFrom {
		t.Args = append(t.Args, "--nsFrom="+o.NsFrom[i], "--nsTo="+o.NsTo[i])
	}
	return t, nil
}

var (
	reDone     = regexp.MustCompile(`done dumping (\S+) \((\d+) documents?\)`)
	reRestored = regexp.MustCompile(`(\d+) document\(s\) restored successfully\. (\d+) document\(s\) failed to restore\.`)
)

// ToolSummary is parsed from the tool's output.
type ToolSummary struct {
	Documents int64 `json:"documents"`
	Failed    int64 `json:"failed"`
}

// RunTool runs a prepared invocation, streaming scrubbed output lines to log.
// The context cancels the process.
func RunTool(ctx context.Context, path string, t *ToolArgs, log Logger) (ToolSummary, error) {
	defer t.Cleanup()
	var sum ToolSummary
	cmd := exec.CommandContext(ctx, path, t.Args...)
	cmd.Env = append(os.Environ(), "MONGODB_URI=") // never inherit a URI from the environment
	out, err := cmd.StderrPipe()
	if err != nil {
		return sum, err
	}
	cmd.Stdout = cmd.Stderr
	log.Log("Running %s", toolDisplay(path, t.Args))
	if err := cmd.Start(); err != nil {
		return sum, err
	}
	var mu sync.Mutex
	var lastErr string
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := t.Scrub(sc.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		log.Log("%s", line)
		mu.Lock()
		if m := reDone.FindStringSubmatch(line); m != nil {
			var n int64
			fmt.Sscan(m[2], &n)
			sum.Documents += n
			log.Add(n)
		}
		if m := reRestored.FindStringSubmatch(line); m != nil {
			fmt.Sscan(m[1], &sum.Documents)
			fmt.Sscan(m[2], &sum.Failed)
		}
		if strings.Contains(strings.ToLower(line), "error") || strings.Contains(line, "Failed:") {
			lastErr = line
		}
		mu.Unlock()
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		log.Log("reading tool output: %v", err)
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return sum, ctx.Err()
	}
	if err != nil {
		if lastErr != "" {
			return sum, fmt.Errorf("%s failed: %s", toolName(path), lastErr)
		}
		return sum, fmt.Errorf("%s failed: %v", toolName(path), err)
	}
	if sum.Failed > 0 {
		return sum, fmt.Errorf("%d documents failed to restore; see the job log", sum.Failed)
	}
	return sum, nil
}

func toolName(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		path = path[i+1:]
	}
	return strings.TrimSuffix(path, ".exe")
}

// toolDisplay is the command line for the log; the config file is named but
// its contents (the connection string and secrets) are not shown.
func toolDisplay(path string, args []string) string {
	shown := make([]string, 0, len(args))
	for _, a := range args {
		if strings.HasPrefix(a, "--config=") {
			a = "--config=<private connection settings>"
		}
		shown = append(shown, a)
	}
	return toolName(path) + " " + strings.Join(shown, " ")
}
