// Package scriptrun runs JavaScript through the installed mongosh, for the
// shell tab's "mongosh (full JavaScript)" mode. The connection — password
// included — goes into a private temporary launcher script that connects and
// then load()s the user's script, so credentials never appear on the command
// line, in the environment or in the output, and error line numbers still
// refer to the user's script.
package scriptrun

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
	"path/filepath"
	"strings"
	"sync"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/mongox"
)

// CanRun explains why a connection can't use mongosh, or returns nil.
func CanRun(p config.Profile) error {
	if p.SSH.Enabled {
		return apperr.New(apperr.CodeUnsupported, "This connection uses an SSH tunnel, which mongosh can't use. Use the built-in shell mode, or open the tunnel yourself and add a direct connection.")
	}
	if p.Proxy.Type != "" && p.Proxy.Type != config.ProxyNone {
		return apperr.New(apperr.CodeUnsupported, "This connection goes through a proxy, which mongosh can't use. Use the built-in shell mode.")
	}
	return nil
}

// ConnectionURI builds the connection string mongosh connects with,
// credentials included, for database db. secrets lists every form in which
// a secret could appear, for scrubbing output.
func ConnectionURI(p config.Profile, c mongox.Credentials, db string) (uri string, secrets []string, err error) {
	p = p.Normalize()
	u, err := config.ParseConnString(p.URI)
	if err != nil {
		return "", nil, err
	}
	addSecret := func(s string) {
		if s != "" {
			secrets = append(secrets, s, url.QueryEscape(s), url.PathEscape(s))
		}
	}
	if u.HasPass {
		if dec, err := url.PathUnescape(u.Password); err == nil {
			addSecret(dec)
		}
		addSecret(u.Password)
	}
	if p.Username != "" { // the form's user wins over one in the URI
		u.User, u.Password, u.HasPass = config.EscapeUserinfo(p.Username), "", false
		if c.Password != "" {
			u.Password, u.HasPass = config.EscapeUserinfo(c.Password), true
			addSecret(c.Password)
			addSecret(u.Password)
		}
	}
	q := u.Query
	if p.AuthSource != "" {
		q.Set("authSource", p.AuthSource)
	}
	if p.AuthMechanism != "" {
		q.Set("authMechanism", p.AuthMechanism)
	}
	if p.AuthProperties != "" {
		q.Set("authMechanismProperties", p.AuthProperties)
	}
	if q.Get("appName") == "" {
		name := p.Advanced.AppName
		if name == "" {
			name = "Mango3S"
		}
		q.Set("appName", name)
	}
	if p.TLS.Enabled {
		q.Set("tls", "true")
		if p.TLS.CAFile != "" {
			q.Set("tlsCAFile", p.TLS.CAFile)
		}
		if p.TLS.CertKeyFile != "" {
			q.Set("tlsCertificateKeyFile", p.TLS.CertKeyFile)
			if c.TLSKeyPassword != "" {
				q.Set("tlsCertificateKeyFilePassword", c.TLSKeyPassword)
				addSecret(c.TLSKeyPassword)
			}
		}
		if p.TLS.AllowInvalidCertificates {
			q.Set("tlsAllowInvalidCertificates", "true")
		}
		if p.TLS.AllowInvalidHostnames {
			q.Set("tlsAllowInvalidHostnames", "true")
		}
	}
	if p.Advanced.ReadPreference != "" {
		q.Set("readPreference", p.Advanced.ReadPreference)
	}
	if db != "" {
		u.Path = "/" + url.PathEscape(db)
	}
	u.Query = q
	return u.String(), secrets, nil
}

// Line is one line of mongosh output.
type Line struct {
	Stream string `json:"stream"` // stdout | stderr
	Text   string `json:"text"`
}

// Run executes script with mongosh against uri, sending output lines to out
// as they appear. It returns mongosh's exit code. Canceling ctx stops it.
func Run(ctx context.Context, mongosh, uri, script string, secrets []string, out func(Line)) (int, error) {
	dir, err := os.MkdirTemp("", "mango3s-mongosh-*")
	if err != nil {
		return -1, err
	}
	defer os.RemoveAll(dir) // the launcher holds the password: never leave it behind
	if err := os.Chmod(dir, 0o700); err != nil {
		return -1, err
	}
	scriptPath := filepath.Join(dir, "script.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return -1, err
	}
	u, _ := json.Marshal(uri)
	sp, _ := json.Marshal(scriptPath)
	launcher := fmt.Sprintf("db = connect(%s);\nload(%s);\n", u, sp)
	launcherPath := filepath.Join(dir, "run.js")
	if err := os.WriteFile(launcherPath, []byte(launcher), 0o600); err != nil {
		return -1, err
	}

	scrub := func(s string) string {
		for _, sec := range secrets {
			if len(sec) >= 3 {
				s = strings.ReplaceAll(s, sec, "*****")
			}
		}
		// The temporary paths mean nothing to the user.
		s = strings.ReplaceAll(s, scriptPath, "<script>")
		return strings.ReplaceAll(s, launcherPath, "<script>")
	}

	cmd := exec.CommandContext(ctx, mongosh, "--nodb", "--quiet", "--norc", launcherPath)
	cmd.Env = append(os.Environ(), "MONGOSH_NO_UPDATE_CHECK=1", "NO_COLOR=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}
	if err := cmd.Start(); err != nil {
		return -1, apperr.Wrap(apperr.CodeInternal, err, "mongosh could not be started.")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	pump := func(r io.Reader, stream string) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 4<<20)
		for sc.Scan() {
			mu.Lock()
			out(Line{Stream: stream, Text: scrub(sc.Text())})
			mu.Unlock()
		}
	}
	wg.Add(2)
	go pump(stdout, "stdout")
	go pump(stderr, "stderr")
	wg.Wait()
	err = cmd.Wait()
	if ctx.Err() != nil {
		return -1, apperr.From(ctx.Err())
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}
