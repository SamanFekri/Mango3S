package apperr

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestCheckNilIsUntypedNil(t *testing.T) {
	var err error = Check(nil)
	if err != nil {
		t.Fatalf("Check(nil) must be a nil interface, got %#v", err)
	}
	if From(nil) != nil {
		t.Fatal("From(nil) must be nil")
	}
}

func TestClassification(t *testing.T) {
	cases := []struct {
		err  error
		want Code
	}{
		{context.Canceled, CodeCanceled},
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), CodeTimeout},
		{errors.New("server selection error: connection refused"), CodeUnreachable},
		{errors.New("auth error: sasl conversation error: Authentication failed."), CodeAuthFailed},
		{errors.New("something odd"), CodeInternal},
		{Invalid("bad"), CodeInvalidInput},
	}
	for _, c := range cases {
		if got := From(c.err).Code; got != c.want {
			t.Errorf("%v: got %s, want %s", c.err, got, c.want)
		}
	}
	e := Wrap(CodeMongo, context.Canceled, "msg")
	if !errors.Is(e, context.Canceled) || !Is(e, CodeMongo) {
		t.Fatal("Wrap must keep the cause and code")
	}
}
