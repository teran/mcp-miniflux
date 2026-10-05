package miniflux

import (
	"context"
	"errors"
	"testing"
)

// fakeKindError is a minimal KindError implementation used to exercise the
// interface contract and the ErrorKind taxonomy.
type fakeKindError struct {
	kind ErrorKind
	code int
	msg  string
}

func (f fakeKindError) Error() string        { return f.msg }
func (f fakeKindError) ErrorKind() ErrorKind { return f.kind }
func (f fakeKindError) StatusCode() int      { return f.code }

func TestErrorKindDistinctValues(t *testing.T) {
	if ErrorNotFound == ErrorAuth || ErrorAuth == ErrorTransient || ErrorNotFound == ErrorTransient {
		t.Fatalf("ErrorKind constants must be distinct: %d/%d/%d",
			ErrorNotFound, ErrorAuth, ErrorTransient)
	}
	// iota order: NotFound < Auth < Transient.
	if !(ErrorNotFound < ErrorAuth && ErrorAuth < ErrorTransient) {
		t.Errorf("iota order violated: %d < %d < %d expected",
			ErrorNotFound, ErrorAuth, ErrorTransient)
	}
	if ErrorNotFound != 0 || ErrorAuth != 1 || ErrorTransient != 2 {
		t.Errorf("explicit iota values = %d/%d/%d, want 0/1/2",
			ErrorNotFound, ErrorAuth, ErrorTransient)
	}
}

func TestKindErrorIsAnError(t *testing.T) {
	e := fakeKindError{kind: ErrorNotFound, code: 404, msg: "not found"}
	var err error = e // must satisfy error
	if err.Error() != "not found" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !errors.Is(err, err) {
		t.Errorf("errors.Is(self) = false")
	}
}

func TestKindErrorAccessors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kind     ErrorKind
		code     int
		wantKind ErrorKind
		wantCode int
	}{
		{"notfound", ErrorNotFound, 404, ErrorNotFound, 404},
		{"auth", ErrorAuth, 403, ErrorAuth, 403},
		{"transient", ErrorTransient, 429, ErrorTransient, 429},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := fakeKindError{kind: tc.kind, code: tc.code}
			if e.ErrorKind() != tc.wantKind {
				t.Errorf("ErrorKind() = %d, want %d", e.ErrorKind(), tc.wantKind)
			}
			if e.StatusCode() != tc.wantCode {
				t.Errorf("StatusCode() = %d, want %d", e.StatusCode(), tc.wantCode)
			}
		})
	}
}

// --- Token context -----------------------------------------------------------

func TestWithTokenAndTokenFromContext(t *testing.T) {
	ctx := WithToken(context.Background(), "secret-token")
	if got := TokenFromContext(ctx); got != "secret-token" {
		t.Errorf("TokenFromContext = %q, want %q", got, "secret-token")
	}
}

func TestTokenFromContextAbsentReturnsEmpty(t *testing.T) {
	if got := TokenFromContext(context.Background()); got != "" {
		t.Errorf("TokenFromContext(empty ctx) = %q, want \"\"", got)
	}
}

func TestTokenFromContextNilReturnsEmpty(t *testing.T) {
	if got := TokenFromContext(nil); got != "" {
		t.Errorf("TokenFromContext(nil) = %q, want \"\"", got)
	}
}

func TestWithTokenDoesNotAffectParent(t *testing.T) {
	parent := context.Background()
	ctx := WithToken(parent, "tok")
	if got := TokenFromContext(parent); got != "" {
		t.Errorf("parent context unexpectedly carries token: %q", got)
	}
	if got := TokenFromContext(ctx); got != "tok" {
		t.Errorf("derived context lost token: %q", got)
	}
}

func TestTokenFromContextWrongTypeIsIgnored(t *testing.T) {
	// A context carrying the same key type but a non-string value must not panic
	// and must return "" (defensive against key collision misuse).
	ctx := context.WithValue(context.Background(), tokenCtxKey{}, 123)
	if got := TokenFromContext(ctx); got != "" {
		t.Errorf("TokenFromContext(wrong-type) = %q, want \"\"", got)
	}
}
