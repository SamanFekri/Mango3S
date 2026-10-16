// Package apperr defines the typed application errors that cross the
// backend/frontend boundary. Every error returned to the UI is converted to an
// *Error so the frontend can show a friendly message and react to the code.
package apperr

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Code is a stable, machine-readable error category.
type Code string

const (
	CodeInvalidInput  Code = "INVALID_INPUT"
	CodeNotFound      Code = "NOT_FOUND"
	CodeNotConnected  Code = "NOT_CONNECTED"
	CodeAuthFailed    Code = "AUTH_FAILED"
	CodeUnreachable   Code = "UNREACHABLE"
	CodeTimeout       Code = "TIMEOUT"
	CodeCanceled      Code = "CANCELED"
	CodeReadOnly      Code = "READ_ONLY"
	CodeConflict      Code = "CONFLICT"
	CodeMongo         Code = "MONGO_ERROR"
	CodeUnsupported   Code = "UNSUPPORTED"
	CodeInternal      Code = "INTERNAL"
	CodeSecretBackend Code = "SECRET_BACKEND"
	CodeTLS           Code = "TLS_ERROR"
	CodeSSH           Code = "SSH_ERROR"
	// CodeSSHHostKey: the SSH server's key is not trusted yet; Detail holds
	// its SHA256 fingerprint so the UI can ask the user to verify and pin it.
	CodeSSHHostKey Code = "SSH_HOST_KEY"
	CodeProxy      Code = "PROXY_ERROR"
)

// Error is a user-presentable error.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	cause   error
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// New creates an error with the given code and message.
func New(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap creates an error with the given code that keeps cause for errors.Is/As.
func Wrap(code Code, cause error, message string) *Error {
	e := &Error{Code: code, Message: message, cause: cause}
	if cause != nil {
		e.Detail = cause.Error()
	}
	return e
}

// Invalid is a shorthand for CodeInvalidInput.
func Invalid(format string, args ...any) *Error {
	return New(CodeInvalidInput, format, args...)
}

// Is reports whether err is an *Error with the given code.
func Is(err error, code Code) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// Check is From for call sites that return a plain error: it returns an
// untyped nil for a nil input, avoiding a non-nil interface holding a nil *Error.
func Check(err error) error {
	if err == nil {
		return nil
	}
	return From(err)
}

// From classifies any error into an *Error. Already-typed errors pass through.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	switch {
	case errors.Is(err, context.Canceled):
		return Wrap(CodeCanceled, err, "The operation was canceled.")
	case errors.Is(err, context.DeadlineExceeded) || mongo.IsTimeout(err):
		return Wrap(CodeTimeout, err, "The operation timed out. Narrow the query or raise the timeout in Settings.")
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	var srvErr mongo.ServerError
	if errors.As(err, &srvErr) {
		if srvErr.HasErrorCode(18) || srvErr.HasErrorCode(8000) {
			return Wrap(CodeAuthFailed, err, "Authentication failed. Check the username, password and auth database.")
		}
		if srvErr.HasErrorCode(13) {
			return Wrap(CodeMongo, err, "Not authorized to perform this operation.")
		}
		if mongo.IsDuplicateKeyError(err) {
			return Wrap(CodeConflict, err, "A document with the same unique key already exists.")
		}
		return Wrap(CodeMongo, err, firstLine(msg))
	}

	var unknownAuth x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	switch {
	case errors.As(err, &unknownAuth):
		return Wrap(CodeTLS, err, "TLS: the server certificate is signed by an unknown authority. Set the CA file on the SSL / TLS tab (or allow invalid certificates for testing only).")
	case errors.As(err, &hostErr):
		return Wrap(CodeTLS, err, "TLS: the server certificate is not valid for "+hostErr.Host+". Connect with the host name in the certificate, or allow invalid host names.")
	case errors.As(err, &invalid):
		return Wrap(CodeTLS, err, "TLS: the server certificate is invalid ("+invalid.Error()+").")
	case strings.Contains(lower, "first record does not look like a tls handshake"):
		return Wrap(CodeTLS, err, "TLS: the server does not speak TLS. Turn TLS off on the SSL / TLS tab.")
	case strings.Contains(lower, "x509: certificate signed by unknown authority"):
		return Wrap(CodeTLS, err, "TLS: the server certificate is signed by an unknown authority. Set the CA file on the SSL / TLS tab (or allow invalid certificates for testing only).")
	case strings.Contains(lower, "x509: certificate is valid for") || strings.Contains(lower, "x509: certificate is not valid for any names"):
		return Wrap(CodeTLS, err, "TLS: the server certificate does not match the host name. Connect with a host name listed in the certificate, or allow invalid host names.")
	case strings.Contains(lower, "x509: certificate has expired"):
		return Wrap(CodeTLS, err, "TLS: the server certificate has expired or is not yet valid.")
	case strings.Contains(lower, "remote error: tls:") || strings.Contains(lower, "tls: "):
		return Wrap(CodeTLS, err, "TLS handshake failed. If the server requires a client certificate, set it on the SSL / TLS tab.")
	}

	switch {
	case strings.Contains(lower, "authentication failed") || strings.Contains(lower, "auth error"):
		return Wrap(CodeAuthFailed, err, "Authentication failed. Check the username, password and auth database.")
	case strings.Contains(lower, "server selection") || strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") || strings.Contains(lower, "i/o timeout"):
		return Wrap(CodeUnreachable, err, "Could not reach the MongoDB server. Check the host, port, network and TLS settings.")
	case errors.Is(err, mongo.ErrClientDisconnected):
		return Wrap(CodeNotConnected, err, "The connection is closed. Reconnect and try again.")
	}
	return Wrap(CodeInternal, err, firstLine(msg))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
