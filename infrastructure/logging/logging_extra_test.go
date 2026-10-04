package logging

import (
	"context"
	"log/slog"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestRequestIDFromContextNil guards the nil-context path (L09): a nil context
// must yield "" without panicking.
func TestRequestIDFromContextNil(t *testing.T) {
	if got := RequestIDFromContext(nil); got != "" {
		t.Errorf("RequestIDFromContext(nil) = %q, want empty", got)
	}
}

// TestRequestIDFromContextWrongType guards against a non-string stored value.
func TestRequestIDFromContextWrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey{}, 42)
	if got := RequestIDFromContext(ctx); got != "" {
		t.Errorf("RequestIDFromContext(wrong type) = %q, want empty", got)
	}
}

// TestSlogHandlerWarnForwarding locks the warn-level mapping in the adapter.
func TestSlogHandlerWarnForwarding(t *testing.T) {
	l, hook := newHookedLogger()
	sl := slog.New(NewSlogHandler(l, slog.LevelWarn))

	sl.Warn("warn-msg", "k", "v")

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no warn record captured")
	}
	if e.Level != logrus.WarnLevel || e.Message != "warn-msg" {
		t.Errorf("got level=%v message=%q, want level=warn message=%q", e.Level, e.Message, "warn-msg")
	}
}

// TestSlogHandlerErrorForwarding locks the error-level mapping.
func TestSlogHandlerErrorForwarding(t *testing.T) {
	l, hook := newHookedLogger()
	sl := slog.New(NewSlogHandler(l, slog.LevelError))

	sl.Error("err-msg")

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no error record captured")
	}
	if e.Level != logrus.ErrorLevel || e.Message != "err-msg" {
		t.Errorf("got level=%v message=%q, want level=error message=%q", e.Level, e.Message, "err-msg")
	}
}

// TestSlogHandlerWithAttrsAndGroup verifies the adapter remains usable through
// the slog composition methods (they do not nil-out or panic).
func TestSlogHandlerWithAttrsAndGroup(t *testing.T) {
	l, hook := newHookedLogger()
	h := NewSlogHandler(l, slog.LevelInfo)
	h = h.WithAttrs([]slog.Attr{slog.String("static", "x")}).WithGroup("grp")

	slog.New(h).Info("composed-msg", "dyn", 1)

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no composed record captured")
	}
	if e.Message != "composed-msg" {
		t.Errorf("message = %q, want %q", e.Message, "composed-msg")
	}
	if got, ok := e.Data["dyn"].(int64); !ok || got != 1 {
		t.Errorf("dynamic attr dyn = %v, want int64 1", e.Data["dyn"])
	}
}
