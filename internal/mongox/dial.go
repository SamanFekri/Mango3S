package mongox

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/net/proxy"

	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// contextDialer is what the driver, SOCKS5 and SSH all agree on.
type contextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// buildDialer returns the dialer chain for a profile: direct → proxy → SSH.
// When SSH is enabled the proxy (if any) carries the SSH connection, and
// MongoDB connections — including replica set members the driver discovers —
// are opened through the tunnel. It returns nil when no custom dialer is
// needed. close must be called when the client is disconnected.
func buildDialer(ctx context.Context, p config.Profile, c Credentials) (d options.ContextDialer, closeFn func(), err error) {
	p = p.Normalize()
	if !p.SSH.Enabled && (p.Proxy.Type == config.ProxyNone || p.Proxy.Type == "") {
		return nil, func() {}, nil
	}
	timeout := time.Duration(p.ConnectTimeoutSeconds) * time.Second
	var chain contextDialer = &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	switch p.Proxy.Type {
	case config.ProxySOCKS5:
		var auth *proxy.Auth
		if p.Proxy.Username != "" {
			auth = &proxy.Auth{User: p.Proxy.Username, Password: c.ProxyPassword}
		}
		sd, err := proxy.SOCKS5("tcp", net.JoinHostPort(p.Proxy.Host, strconv.Itoa(p.Proxy.Port)), auth, chain.(*net.Dialer))
		if err != nil {
			return nil, nil, apperr.Wrap(apperr.CodeProxy, err, "Proxy: invalid SOCKS5 settings.")
		}
		chain = proxyErrors{sd.(proxy.ContextDialer), "SOCKS5"}
	case config.ProxyHTTP:
		chain = proxyErrors{&httpConnect{
			addr: net.JoinHostPort(p.Proxy.Host, strconv.Itoa(p.Proxy.Port)), user: p.Proxy.Username, pass: c.ProxyPassword, forward: chain,
		}, "HTTP"}
	}
	if !p.SSH.Enabled {
		return chain, func() {}, nil
	}
	cfg, err := sshClientConfig(p.SSH, c, timeout)
	if err != nil {
		return nil, nil, err
	}
	t := &sshTunnel{addr: net.JoinHostPort(p.SSH.Host, strconv.Itoa(p.SSH.Port)), cfg: cfg, forward: chain}
	// Connect now so SSH problems are reported as such, not as a vague
	// server selection timeout later.
	if _, err := t.get(ctx); err != nil {
		return nil, nil, err
	}
	return t, t.Close, nil
}

// proxyErrors labels proxy failures so they aren't mistaken for MongoDB ones.
type proxyErrors struct {
	d    contextDialer
	kind string
}

func (p proxyErrors) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := p.d.DialContext(ctx, network, address)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) {
			return nil, err
		}
		return nil, apperr.Wrap(apperr.CodeProxy, err, fmt.Sprintf("Proxy: the %s proxy could not connect to %s.", p.kind, address))
	}
	return conn, nil
}

// httpConnect tunnels TCP through an HTTP proxy with the CONNECT method.
type httpConnect struct {
	addr, user, pass string
	forward          contextDialer
}

func (h *httpConnect) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := h.forward.DialContext(ctx, "tcp", h.addr)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeProxy, err, "Proxy: could not reach the HTTP proxy at "+h.addr+".")
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	req := "CONNECT " + address + " HTTP/1.1\r\nHost: " + address + "\r\n"
	if h.user != "" {
		req += "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(h.user+":"+h.pass)) + "\r\n"
	}
	req += "\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, apperr.Wrap(apperr.CodeProxy, err, "Proxy: the HTTP proxy closed the connection.")
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		conn.Close()
		return nil, apperr.Wrap(apperr.CodeProxy, err, "Proxy: the HTTP proxy sent an invalid response.")
	}
	resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusProxyAuthRequired:
		conn.Close()
		return nil, apperr.New(apperr.CodeProxy, "Proxy: the HTTP proxy requires authentication (or rejected the username and password).")
	case resp.StatusCode != http.StatusOK:
		conn.Close()
		return nil, apperr.New(apperr.CodeProxy, "Proxy: the HTTP proxy refused the tunnel to %s (%s).", address, resp.Status)
	}
	_ = conn.SetDeadline(time.Time{})
	if br.Buffered() > 0 {
		return &bufferedConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (b *bufferedConn) Read(p []byte) (int, error) { return b.r.Read(p) }

// sshTunnel opens MongoDB connections through an SSH server and reconnects
// to it if the SSH connection drops.
type sshTunnel struct {
	addr    string
	cfg     *ssh.ClientConfig
	forward contextDialer

	mu     sync.Mutex
	client *ssh.Client
	closed bool
}

func (t *sshTunnel) get(ctx context.Context) (*ssh.Client, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil, apperr.New(apperr.CodeNotConnected, "The SSH tunnel is closed.")
	}
	if t.client != nil {
		return t.client, nil
	}
	conn, err := t.forward.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) {
			return nil, err
		}
		return nil, apperr.Wrap(apperr.CodeSSH, err, "SSH: could not reach the SSH server at "+t.addr+".")
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	cc, chans, reqs, err := ssh.NewClientConn(conn, t.addr, t.cfg)
	if err != nil {
		conn.Close()
		return nil, classifySSH(err)
	}
	_ = conn.SetDeadline(time.Time{})
	client := ssh.NewClient(cc, chans, reqs)
	t.client = client
	go func() {
		_ = client.Wait()
		t.mu.Lock()
		if t.client == client {
			t.client = nil
		}
		t.mu.Unlock()
	}()
	go keepAlive(client)
	return client, nil
}

func keepAlive(c *ssh.Client) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for range tick.C {
		if _, _, err := c.SendRequest("keepalive@openssh.com", true, nil); err != nil {
			return
		}
	}
}

// DialContext implements the driver's ContextDialer.
func (t *sshTunnel) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	for attempt := 0; ; attempt++ {
		client, err := t.get(ctx)
		if err != nil {
			return nil, err
		}
		conn, err := client.DialContext(ctx, "tcp", address)
		if err == nil {
			return withDeadlines(conn), nil
		}
		var open *ssh.OpenChannelError
		if errors.As(err, &open) || attempt > 0 || ctx.Err() != nil {
			return nil, apperr.Wrap(apperr.CodeSSH, err, "SSH: the SSH server could not connect to "+address+". Check the MongoDB host as seen from the SSH server.")
		}
		// The SSH connection probably died; drop it and retry once.
		t.mu.Lock()
		if t.client == client {
			t.client = nil
		}
		t.mu.Unlock()
		client.Close()
	}
}

// withDeadlines bridges an SSH channel (which has no read/write deadlines)
// through net.Pipe, whose end supports them, because the driver relies on
// deadlines for timeouts. An expired deadline returns a timeout error
// without tearing the channel down, matching TCP semantics.
func withDeadlines(ch net.Conn) net.Conn {
	local, remote := net.Pipe()
	go func() {
		_, _ = io.Copy(ch, remote)
		ch.Close()
	}()
	go func() {
		_, _ = io.Copy(remote, ch)
		remote.Close()
	}()
	return &tunneledConn{Conn: local, ch: ch}
}

// tunneledConn reports the channel's addresses and closes both ends.
type tunneledConn struct {
	net.Conn
	ch net.Conn
}

func (c *tunneledConn) Close() error {
	c.ch.Close()
	return c.Conn.Close()
}
func (c *tunneledConn) LocalAddr() net.Addr  { return c.ch.LocalAddr() }
func (c *tunneledConn) RemoteAddr() net.Addr { return c.ch.RemoteAddr() }

// Close shuts the tunnel down.
func (t *sshTunnel) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	if t.client != nil {
		t.client.Close()
		t.client = nil
	}
}

func sshClientConfig(s config.SSHConfig, c Credentials, timeout time.Duration) (*ssh.ClientConfig, error) {
	var auth []ssh.AuthMethod
	switch s.AuthMethod {
	case config.SSHAuthPassword:
		if c.SSHPassword == "" {
			return nil, apperr.Invalid("SSH: enter the SSH password.")
		}
		pw := c.SSHPassword
		auth = append(auth, ssh.Password(pw), ssh.KeyboardInteractive(func(_, _ string, qs []string, echos []bool) ([]string, error) {
			answers := make([]string, len(qs))
			for i := range qs {
				if !echos[i] {
					answers[i] = pw
				}
			}
			return answers, nil
		}))
	case config.SSHAuthKey:
		signer, err := loadKey(s.PrivateKeyFile, c.SSHPassphrase)
		if err != nil {
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	case config.SSHAuthAgent:
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			if runtime.GOOS == "windows" {
				return nil, apperr.New(apperr.CodeUnsupported, "SSH: the Windows SSH agent (named pipe) is not supported. Use a private key file instead.")
			}
			return nil, apperr.Invalid("SSH: no SSH agent is running (SSH_AUTH_SOCK is not set).")
		}
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeSSH, err, "SSH: could not talk to the SSH agent.")
		}
		ag := agent.NewClient(conn)
		auth = append(auth, ssh.PublicKeysCallback(ag.Signers))
	default:
		return nil, apperr.Invalid("SSH: unsupported authentication method %q.", s.AuthMethod)
	}
	return &ssh.ClientConfig{
		User:            s.Username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback(s.HostKey),
		Timeout:         timeout,
	}, nil
}

func loadKey(path, passphrase string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperr.Invalid("SSH: the private key file %s does not exist.", path)
		}
		return nil, apperr.Wrap(apperr.CodeSSH, err, "SSH: the private key file can't be read.")
	}
	signer, err := ssh.ParsePrivateKey(data)
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		if passphrase == "" {
			return nil, apperr.Invalid("SSH: the private key is protected by a passphrase. Enter it.")
		}
		signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err != nil {
			return nil, apperr.Invalid("SSH: the key passphrase is wrong.")
		}
	} else if err != nil {
		return nil, apperr.Wrap(apperr.CodeSSH, err, "SSH: the private key file is not a supported key (OpenSSH, PEM RSA/ECDSA/Ed25519).")
	}
	return signer, nil
}

// hostKeyCallback accepts keys listed in ~/.ssh/known_hosts (or the app's
// own known_hosts, see AppKnownHostsFile) or the key the user pinned after
// verifying its fingerprint. Unknown keys fail with CodeSSHHostKey
// (Detail = fingerprint); changed keys fail hard.
func hostKeyCallback(pinned string) ssh.HostKeyCallback {
	known := knownHostsCallback()
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		if pinned != "" {
			if fp == pinned {
				return nil
			}
			e := apperr.New(apperr.CodeSSH, "SSH: the host key of %s changed (now %s). This can mean someone is intercepting the connection. Verify the new key with the server administrator before trusting it.", hostname, fp)
			e.Detail = fp
			return e
		}
		if known != nil {
			err := known(hostname, remote, key)
			if err == nil {
				return nil
			}
			var ke *knownhosts.KeyError
			if errors.As(err, &ke) && len(ke.Want) > 0 {
				return apperr.New(apperr.CodeSSH, "SSH: the host key of %s does not match ~/.ssh/known_hosts. This can mean someone is intercepting the connection.", hostname)
			}
		}
		e := apperr.New(apperr.CodeSSHHostKey, "SSH: the host key of %s is not known. Check that its fingerprint is %s, then trust it.", hostname, fp)
		e.Detail = fp
		return e
	}
}

func classifySSH(err error) error {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	msg := err.Error()
	switch {
	case containsAny(msg, "unable to authenticate", "no supported methods remain"):
		return apperr.Wrap(apperr.CodeAuthFailed, err, "SSH: authentication failed. Check the SSH username, password or key.")
	case containsAny(msg, "i/o timeout", "deadline exceeded"):
		return apperr.Wrap(apperr.CodeTimeout, err, "SSH: the SSH server did not respond in time.")
	}
	return apperr.Wrap(apperr.CodeSSH, err, "SSH: the SSH connection failed.")
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}
