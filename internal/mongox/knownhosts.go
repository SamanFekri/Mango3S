package mongox

import (
	"bufio"
	"crypto/ed25519"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// AppKnownHostsFile is the app's own known_hosts file (in its data folder),
// trusted after ~/.ssh/known_hosts. Connections synced from another computer
// bring the lines that vouch for their SSH server here, so the user's own
// file is never edited. Set once at startup; empty disables it.
var AppKnownHostsFile string

var appKnownHostsMu sync.Mutex

// knownHostsFiles lists the known_hosts files that exist, user's first.
func knownHostsFiles() []string {
	var files []string
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files, filepath.Join(home, ".ssh", "known_hosts"))
	}
	if AppKnownHostsFile != "" {
		files = append(files, AppKnownHostsFile)
	}
	out := files[:0]
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			out = append(out, f)
		}
	}
	return out
}

func knownHostsCallback() ssh.HostKeyCallback {
	files := knownHostsFiles()
	if len(files) == 0 {
		return nil
	}
	cb, err := knownhosts.New(files...)
	if err != nil {
		return nil
	}
	return cb
}

// KnownHostLines returns the known_hosts lines (from the user's file and the
// app's own) that list a key for host:port, exactly as written, so another
// computer can trust the same server keys.
func KnownHostLines(host string, port int) []string {
	cb := knownHostsCallback()
	if cb == nil || host == "" {
		return nil
	}
	if port == 0 {
		port = 22
	}
	// A key nobody has: the answer lists every key known for the host.
	probe, err := ssh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		return nil
	}
	err = cb(net.JoinHostPort(host, strconv.Itoa(port)), &net.TCPAddr{IP: net.IPv4zero, Port: port}, probe)
	var ke *knownhosts.KeyError
	if !errors.As(err, &ke) || len(ke.Want) == 0 {
		return nil
	}
	want := map[string]map[int]bool{}
	for _, k := range ke.Want {
		if want[k.Filename] == nil {
			want[k.Filename] = map[int]bool{}
		}
		want[k.Filename][k.Line] = true
	}
	var lines []string
	seen := map[string]bool{}
	for _, file := range knownHostsFiles() {
		nums := want[file]
		if len(nums) == 0 {
			continue
		}
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for n := 1; sc.Scan(); n++ {
			if l := strings.TrimSpace(sc.Text()); nums[n] && !seen[l] {
				seen[l] = true
				lines = append(lines, l)
			}
		}
		f.Close()
	}
	return lines
}

// AddKnownHostLines appends lines to the app's known_hosts file, skipping
// ones it already has and ones that don't parse.
func AddKnownHostLines(lines []string) error {
	if AppKnownHostsFile == "" || len(lines) == 0 {
		return nil
	}
	appKnownHostsMu.Lock()
	defer appKnownHostsMu.Unlock()
	have := map[string]bool{}
	if data, err := os.ReadFile(AppKnownHostsFile); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			have[strings.TrimSpace(l)] = true
		}
	}
	var add []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || have[l] || strings.ContainsAny(l, "\r\n") {
			continue
		}
		if _, _, _, _, _, err := ssh.ParseKnownHosts([]byte(l)); err != nil {
			continue
		}
		have[l] = true
		add = append(add, l)
	}
	if len(add) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(AppKnownHostsFile), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(AppKnownHostsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strings.Join(add, "\n") + "\n")
	return err
}
