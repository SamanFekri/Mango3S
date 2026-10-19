package mongox

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/logging"
)

// Step is one stage of opening a connection, reported live to the UI so a
// failure shows exactly where it happened. Details never contain secrets.
type Step struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	State      string `json:"state"` // pending | running | ok | warning | failed | skipped
	Detail     string `json:"detail,omitempty"`
	Error      string `json:"error,omitempty"`
	Code       string `json:"code,omitempty"`
	DurationMs int64  `json:"durationMs"`
}

// Reporter receives every step update (nil = nobody is watching).
type Reporter func(Step)

type stepper struct {
	rep   Reporter
	steps []Step
}

func (s *stepper) emit(st Step) {
	for i := range s.steps {
		if s.steps[i].ID == st.ID {
			s.steps[i] = st
		}
	}
	if s.rep != nil {
		s.rep(st)
	}
}

// plan announces the steps that will run, so the UI can list them up front.
func (s *stepper) plan(steps ...Step) {
	for _, st := range steps {
		st.State = "pending"
		s.steps = append(s.steps, st)
		if s.rep != nil {
			s.rep(st)
		}
	}
}

func (s *stepper) label(id string) string {
	for _, st := range s.steps {
		if st.ID == id {
			return st.Label
		}
	}
	return id
}

// run executes one step; a failure marks every later step as skipped.
func (s *stepper) run(id string, f func() (detail string, warn bool, err error)) error {
	st := Step{ID: id, Label: s.label(id), State: "running"}
	s.emit(st)
	start := time.Now()
	detail, warn, err := f()
	st.DurationMs = time.Since(start).Milliseconds()
	st.Detail = logging.Redact(detail)
	switch {
	case err != nil:
		ae := apperr.From(err)
		st.State, st.Error, st.Code = "failed", logging.Redact(ae.Message), string(ae.Code)
		if ae.Detail != "" && ae.Detail != ae.Message {
			st.Detail = strings.TrimSpace(st.Detail + "\n" + logging.Redact(ae.Detail))
		}
		s.emit(st)
		s.skipRest()
		return err
	case warn:
		st.State = "warning"
	default:
		st.State = "ok"
	}
	s.emit(st)
	return nil
}

func (s *stepper) skipRest() {
	for _, st := range s.steps {
		if st.State == "pending" {
			st.State = "skipped"
			s.emit(st)
		}
	}
}

// open connects step by step: settings, DNS seed list, SSH tunnel or proxy,
// reachability of each host, TLS handshake, sign-in and server info.
func open(ctx context.Context, p config.Profile, c Credentials, rep Reporter) (*mongo.Client, func(), TestResult, error) {
	p = p.Normalize()
	s := &stepper{rep: rep}
	srv := strings.HasPrefix(p.URI, "mongodb+srv://")
	plan := []Step{{ID: "settings", Label: "Check connection settings"}}
	if srv {
		plan = append(plan, Step{ID: "dns", Label: "Look up the DNS seed list (SRV)"})
	}
	if p.SSH.Enabled {
		label := fmt.Sprintf("Open SSH tunnel to %s@%s:%d", p.SSH.Username, p.SSH.Host, p.SSH.Port)
		if p.Proxy.Type != config.ProxyNone {
			label += fmt.Sprintf(" via %s proxy %s:%d", p.Proxy.Type, p.Proxy.Host, p.Proxy.Port)
		}
		plan = append(plan, Step{ID: "ssh", Label: label})
	} else if p.Proxy.Type != config.ProxyNone {
		plan = append(plan, Step{ID: "proxy", Label: fmt.Sprintf("Reach the %s proxy %s:%d", p.Proxy.Type, p.Proxy.Host, p.Proxy.Port)})
	}
	plan = append(plan, Step{ID: "reach", Label: "Reach the server"})
	tlsOn := p.TLS.Enabled || srv || strings.Contains(strings.ToLower(p.URI), "tls=true") || strings.Contains(strings.ToLower(p.URI), "ssl=true")
	if tlsOn {
		plan = append(plan, Step{ID: "tls", Label: "TLS handshake"})
	}
	auth := "Connect"
	if p.Username != "" || (p.AuthMechanism != "" && p.AuthMechanism != config.MechDefault) {
		who := p.Username
		if who == "" {
			who = "the configured identity"
		}
		mech := p.AuthMechanism
		if mech == "" {
			mech = "SCRAM"
		}
		auth = fmt.Sprintf("Connect and sign in as %s (%s)", who, mech)
	}
	plan = append(plan, Step{ID: "auth", Label: auth}, Step{ID: "info", Label: "Read server information"})
	s.plan(plan...)

	fail := func(err error) (*mongo.Client, func(), TestResult, error) { return nil, nil, TestResult{}, err }

	var opts *options.ClientOptions
	if err := s.run("settings", func() (string, bool, error) {
		if srv {
			// The driver resolves SRV while parsing; that is reported as its own step.
			return describeProfile(p), false, nil
		}
		o, err := BuildClientOptions(p, c)
		if err != nil {
			return "", false, err
		}
		opts = o
		return describeProfile(p), false, nil
	}); err != nil {
		return fail(err)
	}
	if srv {
		if err := s.run("dns", func() (string, bool, error) {
			host := srvHost(p.URI)
			_, addrs, err := net.DefaultResolver.LookupSRV(ctx, "mongodb", "tcp", host)
			if err != nil {
				return "", false, apperr.Wrap(apperr.CodeUnreachable, err, fmt.Sprintf("DNS: no SRV record _mongodb._tcp.%s. Check the cluster host name, or use a standard mongodb:// connection string.", host))
			}
			hosts := make([]string, len(addrs))
			for i, a := range addrs {
				hosts[i] = net.JoinHostPort(strings.TrimSuffix(a.Target, "."), fmt.Sprint(a.Port))
			}
			detail := fmt.Sprintf("%d host(s): %s", len(hosts), strings.Join(hosts, ", "))
			if txt, err := net.DefaultResolver.LookupTXT(ctx, host); err == nil && len(txt) > 0 {
				detail += "\nTXT options: " + strings.Join(txt, " ")
			}
			o, err := BuildClientOptions(p, c)
			if err != nil {
				return detail, false, err
			}
			opts = o
			return detail, false, nil
		}); err != nil {
			return fail(err)
		}
	}

	closeTunnel := func() {}
	var dialer contextDialer = &net.Dialer{Timeout: time.Duration(p.ConnectTimeoutSeconds) * time.Second}
	if p.SSH.Enabled || p.Proxy.Type != config.ProxyNone {
		id := "ssh"
		if !p.SSH.Enabled {
			id = "proxy"
		}
		if err := s.run(id, func() (string, bool, error) {
			d, cl, err := buildDialer(ctx, p, c)
			if err != nil {
				return "", false, err
			}
			closeTunnel = cl
			dialer = d.(contextDialer)
			if !p.SSH.Enabled {
				// Prove the proxy is reachable before going through it.
				conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(p.Proxy.Host, fmt.Sprint(p.Proxy.Port)))
				if err != nil {
					return "", false, apperr.Wrap(apperr.CodeProxy, err, "Proxy: could not reach the proxy.")
				}
				conn.Close()
				return "The proxy accepts connections; MongoDB traffic goes through it.", false, nil
			}
			detail := "Signed in to the SSH server; MongoDB connections go through the tunnel."
			if p.SSH.HostKey != "" {
				detail += "\nHost key " + p.SSH.HostKey + " (pinned)."
			}
			return detail, false, nil
		}); err != nil {
			return fail(err)
		}
		opts.SetDialer(dialer)
	}

	var reachable string
	if err := s.run("reach", func() (string, bool, error) {
		hosts := opts.Hosts
		if len(hosts) == 0 {
			hosts = []string{"localhost:27017"}
		}
		type res struct {
			host string
			ms   int64
			err  error
		}
		out := make([]res, len(hosts))
		var wg sync.WaitGroup
		for i, h := range hosts {
			wg.Add(1)
			go func(i int, h string) {
				defer wg.Done()
				cctx, cancel := context.WithTimeout(ctx, minDur(time.Duration(p.ConnectTimeoutSeconds)*time.Second, 10*time.Second))
				defer cancel()
				start := time.Now()
				conn, err := dialer.DialContext(cctx, "tcp", h)
				if err == nil {
					conn.Close()
				}
				out[i] = res{h, time.Since(start).Milliseconds(), err}
			}(i, h)
		}
		wg.Wait()
		var lines []string
		ok := 0
		for _, r := range out {
			if r.err != nil {
				lines = append(lines, fmt.Sprintf("✗ %s — %s", r.host, shortErr(r.err)))
				continue
			}
			ok++
			if reachable == "" {
				reachable = r.host
			}
			lines = append(lines, fmt.Sprintf("✓ %s (%d ms)", r.host, r.ms))
		}
		detail := strings.Join(lines, "\n")
		switch {
		case ok == 0:
			return detail, false, apperr.New(apperr.CodeUnreachable, "No server could be reached. Check the host names, ports, firewall%s.", tunnelHint(p))
		case ok < len(out):
			return detail, true, nil
		}
		return detail, false, nil
	}); err != nil {
		closeTunnel()
		return fail(err)
	}

	if tlsOn {
		if err := s.run("tls", func() (string, bool, error) {
			cfg := opts.TLSConfig
			if cfg == nil {
				cfg = &tls.Config{}
			}
			cfg = cfg.Clone()
			host, _, _ := net.SplitHostPort(reachable)
			if cfg.ServerName == "" {
				cfg.ServerName = host
			}
			cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			conn, err := dialer.DialContext(cctx, "tcp", reachable)
			if err != nil {
				return "", false, err
			}
			defer conn.Close()
			tc := tls.Client(conn, cfg)
			if err := tc.HandshakeContext(cctx); err != nil {
				// A server without TLS (MongoDB 8 and later) closes the connection
				// on the TLS hello instead of answering in plain text.
				if errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) {
					return "", false, apperr.Wrap(apperr.CodeTLS, err, "TLS: the server closed the connection during the TLS handshake. It probably does not use TLS: turn TLS off on the SSL / TLS tab. If it does use TLS, it may require a client certificate.")
				}
				return "", false, err
			}
			cs := tc.ConnectionState()
			detail := tlsVersion(cs.Version)
			if len(cs.PeerCertificates) > 0 {
				detail += " · " + certSummary(cs.PeerCertificates[0])
			}
			if cfg.InsecureSkipVerify && cfg.VerifyConnection == nil {
				return detail + "\nCertificate verification is OFF (allow invalid certificates).", true, nil
			}
			if cfg.VerifyConnection != nil {
				return detail + "\nHost name check is off (allow invalid host names); the chain was verified.", true, nil
			}
			return detail, false, nil
		}); err != nil {
			closeTunnel()
			return fail(err)
		}
	}

	var client *mongo.Client
	var res TestResult
	if err := s.run("auth", func() (string, bool, error) {
		cl, err := mongo.Connect(opts)
		if err != nil {
			return "", false, apperr.From(err)
		}
		start := time.Now()
		if err := cl.Ping(ctx, readpref.Nearest()); err != nil {
			_ = cl.Disconnect(context.Background())
			return "", false, apperr.From(err)
		}
		client = cl
		res.LatencyMs = time.Since(start).Milliseconds()
		detail := fmt.Sprintf("Ping %d ms", res.LatencyMs)
		if opts.Auth != nil && opts.Auth.Username != "" {
			db := opts.Auth.AuthSource
			if db == "" {
				db = "admin"
			}
			detail = fmt.Sprintf("Signed in as %s on %s · ping %d ms", opts.Auth.Username, db, res.LatencyMs)
		}
		return detail, false, nil
	}); err != nil {
		closeTunnel()
		return fail(err)
	}
	_ = s.run("info", func() (string, bool, error) {
		res.ServerVersion, res.Topology = describe(ctx, client)
		return fmt.Sprintf("MongoDB %s · %s", orUnknown(res.ServerVersion), res.Topology), res.ServerVersion == "", nil
	})
	res.Via = via(p)
	return client, closeTunnel, res, nil
}

func describeProfile(p config.Profile) string {
	var parts []string
	if cs, err := connstring.Parse(p.URI); err == nil && !strings.HasPrefix(p.URI, "mongodb+srv://") {
		parts = append(parts, "Hosts: "+strings.Join(cs.Hosts, ", "))
	}
	parts = append(parts, "Connection string: "+logging.Redact(p.URI))
	if p.Username != "" {
		parts = append(parts, "User: "+p.Username)
	}
	if p.TLS.Enabled {
		parts = append(parts, "TLS on")
	}
	parts = append(parts, fmt.Sprintf("Connect timeout %ds", p.ConnectTimeoutSeconds))
	return strings.Join(parts, "\n")
}

func srvHost(uri string) string {
	h := strings.TrimPrefix(uri, "mongodb+srv://")
	if i := strings.LastIndex(h, "@"); i >= 0 {
		h = h[i+1:]
	}
	if i := strings.IndexAny(h, "/?"); i >= 0 {
		h = h[:i]
	}
	return h
}

func tunnelHint(p config.Profile) string {
	if p.SSH.Enabled {
		return " (as seen from the SSH server)"
	}
	return " and VPN"
}

func shortErr(err error) string {
	msg := err.Error()
	if ae, ok := err.(*apperr.Error); ok {
		msg = ae.Message
		if ae.Detail != "" {
			msg += " (" + ae.Detail + ")"
		}
	}
	for _, x := range []string{"connection refused", "no such host", "i/o timeout", "network is unreachable", "deadline exceeded"} {
		if strings.Contains(msg, x) {
			return x
		}
	}
	return logging.Redact(msg)
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func tlsVersion(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	}
	return fmt.Sprintf("TLS 0x%x", v)
}

func certSummary(c *x509.Certificate) string {
	return fmt.Sprintf("certificate %s, issued by %s, valid until %s", c.Subject.CommonName, c.Issuer.CommonName, c.NotAfter.Format("2006-01-02"))
}

func orUnknown(s string) string {
	if s == "" {
		return "(version unknown)"
	}
	return s
}
