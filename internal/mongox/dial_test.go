package mongox

import (
	"context"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// roundTrip dials addr through d and checks that bytes come back.
func roundTrip(t *testing.T, d contextDialer, addr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("echo: %q %v", buf, err)
	}
}

func sshProfile(t *testing.T, s *sshServer) config.Profile {
	h, p := hostPort(t, s.addr)
	return config.Profile{Name: "t", URI: "mongodb://db.internal:27017", ConnectTimeoutSeconds: 5,
		SSH: config.SSHConfig{Enabled: true, Host: h, Port: p, Username: "me", AuthMethod: config.SSHAuthPassword, HostKey: s.fingerprint}}
}

func dialer(t *testing.T, p config.Profile, c Credentials) (contextDialer, error) {
	t.Helper()
	d, closeFn, err := buildDialer(context.Background(), p, c)
	if err != nil {
		return nil, err
	}
	t.Cleanup(closeFn)
	return d.(contextDialer), nil
}

func TestNoCustomDialerByDefault(t *testing.T) {
	d, closeFn, err := buildDialer(context.Background(), config.Profile{URI: "mongodb://h"}, Credentials{})
	if err != nil || d != nil {
		t.Fatalf("plain profiles must use the driver's dialer: %v %v", d, err)
	}
	closeFn()
}

func TestSSHTunnelWithPasswordAndPinnedKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no known_hosts
	echo := echoServer(t)
	s := newSSHServer(t)
	d, err := dialer(t, sshProfile(t, s), Credentials{SSHPassword: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, d, echo)
	roundTrip(t, d, echo) // several MongoDB connections share one SSH connection
	if s.forwards.Load() != 2 {
		t.Fatalf("expected 2 forwarded channels, got %d", s.forwards.Load())
	}
}

func TestSSHPrivateKeyWithPassphrase(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	echo := echoServer(t)
	s := newSSHServer(t)
	p := sshProfile(t, s)
	p.SSH.AuthMethod = config.SSHAuthKey
	p.SSH.PrivateKeyFile = s.writeKey(t, "open sesame")

	if _, err := dialer(t, p, Credentials{}); err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("missing passphrase should be explained, got %v", err)
	}
	if _, err := dialer(t, p, Credentials{SSHPassphrase: "nope"}); err == nil || !strings.Contains(err.Error(), "passphrase is wrong") {
		t.Fatalf("wrong passphrase should be explained, got %v", err)
	}
	d, err := dialer(t, p, Credentials{SSHPassphrase: "open sesame"})
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, d, echo)
}

func TestSSHHostKeyVerification(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := newSSHServer(t)
	p := sshProfile(t, s)

	p.SSH.HostKey = ""
	_, err := dialer(t, p, Credentials{SSHPassword: "pw"})
	var ae *apperr.Error
	if !asAppErr(err, &ae) || ae.Code != apperr.CodeSSHHostKey || ae.Detail != s.fingerprint {
		t.Fatalf("unknown host key must report its fingerprint for verification, got %#v", err)
	}

	p.SSH.HostKey = "SHA256:somethingElse"
	_, err = dialer(t, p, Credentials{SSHPassword: "pw"})
	if !asAppErr(err, &ae) || ae.Code != apperr.CodeSSH || !strings.Contains(ae.Message, "changed") {
		t.Fatalf("a changed host key must fail hard, got %v", err)
	}
}

func TestSSHAuthFailureAndUnreachable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := newSSHServer(t)
	if _, err := dialer(t, sshProfile(t, s), Credentials{SSHPassword: "wrong"}); !apperr.Is(err, apperr.CodeAuthFailed) || !strings.Contains(err.Error(), "SSH") {
		t.Fatalf("bad SSH password: %v", err)
	}
	p := sshProfile(t, s)
	p.SSH.Port = freePort(t)
	if _, err := dialer(t, p, Credentials{SSHPassword: "pw"}); !apperr.Is(err, apperr.CodeSSH) {
		t.Fatalf("unreachable SSH server: %v", err)
	}
	if _, err := dialer(t, sshProfile(t, s), Credentials{}); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("missing SSH password must be reported before dialing: %v", err)
	}
}

func TestSOCKS5Proxy(t *testing.T) {
	echo := echoServer(t)
	var hits atomic.Int32
	h, port := hostPort(t, socks5Server(t, "u", "p", &hits))
	p := config.Profile{URI: "mongodb://x", Proxy: config.ProxyConfig{Type: config.ProxySOCKS5, Host: h, Port: port, Username: "u"}}
	d, err := dialer(t, p, Credentials{ProxyPassword: "p"})
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, d, echo)
	if hits.Load() != 1 {
		t.Fatal("traffic did not go through the proxy")
	}
	bad, _ := dialer(t, p, Credentials{ProxyPassword: "x"})
	if _, err := bad.DialContext(context.Background(), "tcp", echo); !apperr.Is(err, apperr.CodeProxy) {
		t.Fatalf("bad proxy password: %v", err)
	}
}

func TestHTTPConnectProxy(t *testing.T) {
	echo := echoServer(t)
	var hits atomic.Int32
	h, port := hostPort(t, httpProxy(t, "u", "p", &hits))
	p := config.Profile{URI: "mongodb://x", Proxy: config.ProxyConfig{Type: config.ProxyHTTP, Host: h, Port: port, Username: "u"}}
	d, err := dialer(t, p, Credentials{ProxyPassword: "p"})
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, d, echo)
	bad, _ := dialer(t, p, Credentials{ProxyPassword: "x"})
	if _, err := bad.DialContext(context.Background(), "tcp", echo); !apperr.Is(err, apperr.CodeProxy) || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("407 must be explained: %v", err)
	}
}

func TestSSHThroughSOCKS5(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	echo := echoServer(t)
	s := newSSHServer(t)
	var hits atomic.Int32
	h, port := hostPort(t, socks5Server(t, "", "", &hits))
	p := sshProfile(t, s)
	p.Proxy = config.ProxyConfig{Type: config.ProxySOCKS5, Host: h, Port: port}
	d, err := dialer(t, p, Credentials{SSHPassword: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, d, echo)
	if hits.Load() != 1 || s.forwards.Load() != 1 {
		t.Fatalf("expected proxy → SSH → target, got proxy=%d ssh=%d", hits.Load(), s.forwards.Load())
	}
}

func freePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, p := hostPort(t, ln.Addr().String())
	ln.Close()
	return p
}

func asAppErr(err error, target **apperr.Error) bool {
	e, ok := err.(*apperr.Error)
	if ok {
		*target = e
	}
	return ok
}

func TestTunneledConnectionsSupportDeadlines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	echo := echoServer(t)
	s := newSSHServer(t)
	d, err := dialer(t, sshProfile(t, s), Credentials{SSHPassword: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := d.DialContext(context.Background(), "tcp", echo)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.SetDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("the driver sets deadlines on every connection: %v", err)
	}
	buf := make([]byte, 1)
	_, err = c.Read(buf) // nothing sent: must time out, not hang
	if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("expected a timeout, got %v", err)
	}
	_ = c.SetDeadline(time.Time{})
	roundTripConn(t, c)
}

func roundTripConn(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte("pong")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "pong" {
		t.Fatalf("after a timeout the connection must still work: %q %v", buf, err)
	}
}
