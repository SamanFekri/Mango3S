// Package apperr defines the typed application errors that cross the
// backend/frontend boundary. Every error returned to the UI is converted to an
// *Error so the frontend can show a friendly message and react to the code.
package apperr

import (
	"context"
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
