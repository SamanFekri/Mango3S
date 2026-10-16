package mongox

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"
)

// echoServer accepts TCP connections and echoes what it reads.
func echoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	return ln.Addr().String()
}

func pipe(a, b net.Conn) {
	go func() { _, _ = io.Copy(a, b); a.Close() }()
	_, _ = io.Copy(b, a)
	b.Close()
}

type sshServer struct {
	addr        string
	fingerprint string
	clientKey   ssh.Signer   // authorized client key
	forwards    atomic.Int32 // direct-tcpip channels opened
}

// newSSHServer starts an SSH server that accepts password "pw" for user
// "me" (and clientKey), and forwards direct-tcpip channels.
func newSSHServer(t *testing.T) *sshServer {
	t.Helper()
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostPriv)
	_, cliPriv, _ := ed25519.GenerateKey(rand.Reader)
	cliSigner, _ := ssh.NewSignerFromKey(cliPriv)
	s := &sshServer{fingerprint: ssh.FingerprintSHA256(hostSigner.PublicKey()), clientKey: cliSigner}
	var cfg *ssh.ServerConfig
	cfg = &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if c.User() == "me" && string(pw) == "pw" {
				return nil, nil
			}
			return nil, io.EOF
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			if string(k.Marshal()) == string(s.clientKey.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, io.EOF
		},
	}
	cfg.AddHostKey(hostSigner)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	s.addr = ln.Addr().String()
	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(nc, cfg)
		}
	}()
	return s
}

func (s *sshServer) serve(nc net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(nc, cfg)
	if err != nil {
		nc.Close()
		return
	}
	go func() {
		for r := range reqs {
			if r.WantReply {
				_ = r.Reply(true, nil)
			}
		}
	}()
	for nch := range chans {
		if nch.ChannelType() != "direct-tcpip" {
			_ = nch.Reject(ssh.UnknownChannelType, "no")
			continue
		}
		var p struct {
			Host       string
			Port       uint32
			OriginHost string
			OriginPort uint32
		}
		if err := ssh.Unmarshal(nch.ExtraData(), &p); err != nil {
			_ = nch.Reject(ssh.ConnectionFailed, "bad payload")
			continue
		}
		target, err := net.Dial("tcp", net.JoinHostPort(p.Host, strconv.Itoa(int(p.Port))))
		if err != nil {
			_ = nch.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}
		ch, creqs, err := nch.Accept()
		if err != nil {
			target.Close()
			continue
		}
		s.forwards.Add(1)
		go ssh.DiscardRequests(creqs)
		go func() {
			go func() { _, _ = io.Copy(ch, target); ch.Close() }()
			_, _ = io.Copy(target, ch)
			target.Close()
		}()
	}
}

// writeKey writes the server's authorized client key as an OpenSSH PEM.
func (s *sshServer) writeKey(t *testing.T, passphrase string) string {
	t.Helper()
	// Recreate a private key whose public half the server authorizes.
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(priv)
	s.clientKey = signer
	var block *pem.Block
	var err error
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "test")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "test", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_test")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// socks5Server is a minimal SOCKS5 server (CONNECT, optional user/pass).
func socks5Server(t *testing.T, user, pass string, hits *atomic.Int32) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				hdr := make([]byte, 2)
				if _, err := io.ReadFull(r, hdr); err != nil {
					return
				}
				methods := make([]byte, hdr[1])
				_, _ = io.ReadFull(r, methods)
				if user != "" {
					_, _ = c.Write([]byte{5, 2})
					v := make([]byte, 2)
					_, _ = io.ReadFull(r, v)
					u := make([]byte, v[1])
					_, _ = io.ReadFull(r, u)
					pl := make([]byte, 1)
					_, _ = io.ReadFull(r, pl)
					p := make([]byte, pl[0])
					_, _ = io.ReadFull(r, p)
					if string(u) != user || string(p) != pass {
						_, _ = c.Write([]byte{1, 1})
						return
					}
					_, _ = c.Write([]byte{1, 0})
				} else {
					_, _ = c.Write([]byte{5, 0})
				}
				req := make([]byte, 4)
				if _, err := io.ReadFull(r, req); err != nil {
					return
				}
				var host string
				switch req[3] {
				case 1:
					ip := make([]byte, 4)
					_, _ = io.ReadFull(r, ip)
					host = net.IP(ip).String()
				case 3:
					l := make([]byte, 1)
					_, _ = io.ReadFull(r, l)
					h := make([]byte, l[0])
					_, _ = io.ReadFull(r, h)
					host = string(h)
				default:
					return
				}
				pb := make([]byte, 2)
				_, _ = io.ReadFull(r, pb)
				port := binary.BigEndian.Uint16(pb)
				target, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(port))))
				if err != nil {
					_, _ = c.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
					return
				}
				hits.Add(1)
				_, _ = c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
				pipe(&readerConn{Conn: c, r: r}, target)
			}(c)
		}
	}()
	return ln.Addr().String()
}

type readerConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *readerConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// httpProxy is a minimal HTTP CONNECT proxy with optional Basic auth.
func httpProxy(t *testing.T, user, pass string, hits *atomic.Int32) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				r := bufio.NewReader(c)
				req, err := http.ReadRequest(r)
				if err != nil || req.Method != http.MethodConnect {
					c.Close()
					return
				}
				if user != "" {
					u, p, ok := parseBasic(req.Header.Get("Proxy-Authorization"))
					if !ok || u != user || p != pass {
						_, _ = c.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n"))
						c.Close()
						return
					}
				}
				target, err := net.Dial("tcp", req.Host)
				if err != nil {
					_, _ = c.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
					c.Close()
					return
				}
				hits.Add(1)
				_, _ = c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
				pipe(&readerConn{Conn: c, r: r}, target)
			}(c)
		}
	}()
	return ln.Addr().String()
}

func parseBasic(h string) (string, string, bool) {
	r := &http.Request{Header: http.Header{"Authorization": []string{h}}}
	return r.BasicAuth()
}

func hostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(p)
	return h, n
}
