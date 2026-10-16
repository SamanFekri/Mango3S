// Package logging configures structured logging and guarantees credentials
// never reach log output: every string attribute and message passes through
// Redact before it is written.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// scheme://user:pass@host -> scheme://user:****@host (mongodb, postgres, mysql, ...)
	uriUserInfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^:@/\s]*):([^@/\s]*)@`)
	// Go MySQL driver DSN: user:pass@tcp(host)/db
	mysqlDSN = regexp.MustCompile(`(^|[\s"'=])([^:@/\s"'=]+):([^@/\s]*)@(tcp|unix)\(`)
	// password=..., pwd: ..., "password": "..."
	keyValue = regexp.MustCompile(`(?i)("?(?:password|passwd|pwd|secret|token)"?\s*[:=]\s*"?)([^"&,\s}]+)`)
)

// Redact masks passwords embedded in connection strings and key/value text.
func Redact(s string) string {
	s = uriUserInfo.ReplaceAllString(s, "$1$2:****@")
	s = mysqlDSN.ReplaceAllString(s, "$1$2:****@$4(")
	return keyValue.ReplaceAllString(s, "$1****")
}

// ParseLevel maps a settings string to a slog level.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return slog.LevelInfo
}

// Setup creates a logger that writes JSON lines to <dir>/logs/mango3s.log and
// text to stderr. The returned LevelVar can change verbosity at runtime.
func Setup(dir string, level slog.Level) (*slog.Logger, *slog.LevelVar, string, error) {
	lv := new(slog.LevelVar)
	lv.Set(level)
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return slog.New(NewRedactingHandler(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lv}))), lv, "", err
	}
	path := filepath.Join(logDir, "mango3s.log")
	rotate(path, 5<<20)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	var w io.Writer = os.Stderr
	if err == nil {
		w = io.MultiWriter(f, os.Stderr)
	}
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv})
	return slog.New(NewRedactingHandler(h)), lv, path, err
}

func rotate(path string, max int64) {
	if info, err := os.Stat(path); err == nil && info.Size() > max {
		_ = os.Rename(path, path+".1")
	}
}

// RedactingHandler wraps a handler and redacts messages and string attributes.
type RedactingHandler struct{ next slog.Handler }

// NewRedactingHandler wraps next.
func NewRedactingHandler(next slog.Handler) *RedactingHandler { return &RedactingHandler{next: next} }

func (h *RedactingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, Redact(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, nr)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return &RedactingHandler{next: h.next.WithAttrs(out)}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{next: h.next.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	if isSecretKey(a.Key) {
		return slog.String(a.Key, "****")
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, Redact(v.String()))
	case slog.KindGroup:
		g := v.Group()
		out := make([]any, len(g))
		for i, ga := range g {
			out[i] = redactAttr(ga)
		}
		return slog.Group(a.Key, out...)
	case slog.KindAny:
		if err, ok := v.Any().(error); ok {
			return slog.String(a.Key, Redact(err.Error()))
		}
	}
	return a
}

func isSecretKey(k string) bool {
	k = strings.ToLower(k)
	return k == "password" || k == "pwd" || k == "secret" || k == "token"
}
