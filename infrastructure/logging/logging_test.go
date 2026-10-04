package logging

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

// CONTRACT — infrastructure/logging (SPEC §9 L01/L02/L04, L07-L09, L01GO-L04GO).
//
// The developer must define the following in package `logging` (file
// infrastructure/logging/logging.go). This test file is written FIRST (TDD, red
// state) and will not compile until these exist. The exact contract:
//
//	// Options maps the §9 table onto logger construction. Mode is "http"|"stdio";
//	// Level is a logrus level string ("" in stdio means "disabled", L02); Format
//	// is "text"|"json" (default "text", L04); Filename is the stdio log file.
//	type Options struct {
//		Mode     string
//		Level    string
//		Format   string
//		Filename string
//	}
//
//	// New builds a logrus logger per L01/L02/L04:
//	//   - Mode "http"  -> Out = os.Stdout, always enabled, default level "info"
//	//                     (overridable via Level).
//	//   - Mode "stdio" -> Out = file at Filename (chmod 0600); if Level == "" the
//	//                     logger is DISABLED (nothing is emitted, L02); if
//	//                     Filename == "" it is an error.
//	//   - Format "text" -> TextFormatter; "json" -> JSONFormatter.
//	//   - Any other Mode / Format / unparseable Level is an error.
//	func New(opts Options) (*logrus.Logger, error)
//
//	// WithRequestID stores the request id in ctx; RequestIDFromContext retrieves
//	// it, returning "" when absent (L09/L04GO).
//	func WithRequestID(ctx context.Context, id string) context.Context
//	func RequestIDFromContext(ctx context.Context) string
//
//	// WithContext returns a logrus.Entry bound to ctx; when ctx carries a
//	// request_id, the entry has a "request_id" field set (L09).
//	func WithContext(l *logrus.Logger, ctx context.Context) *logrus.Entry
//
//	// NewSlogHandler returns a slog.Handler that forwards records into the given
//	// logrus logger, honouring the given level for slog-level filtering (L07/L03GO).
//	func NewSlogHandler(l *logrus.Logger, level slog.Level) slog.Handler
//
//	// LogToolCall emits the per-request tool-call access log at info level (L08):
//	// structured fields tool, args (already redacted), source, duration, outcome.
//	func LogToolCall(entry *logrus.Entry, tool string, redactedArgs map[string]any, source string, duration time.Duration, outcome string)
//
// DESIGN DECISIONS (documented):
//   - Mode/Level/Format/Filename are all strings in Options to mirror the env
//     surface of §9 directly. Mode is supplied by the caller (from the `-mode`
//     flag in cmd/mcp-miniflux), not from Config.
//   - stdio + empty Level == "disabled": the logger is still constructed (so
//     New always returns a usable handle) but its level is raised so nothing is
//     emitted and its output is discarded (L02). Verified by
//     TestNewStdioModeDisabledWithoutLevel.
//   - The slog handler owns slog-level filtering; logrus-level filtering is left
//     to the wrapped logger.

// captureStdout redirects os.Stdout to a pipe for the duration of the test and
// returns a function that closes the pipe writer and returns what was written.
func captureStdout(t *testing.T) (read func() []byte) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = old
		w.Close()
		r.Close()
	})
	return func() []byte {
		w.Close()
		data, _ := io.ReadAll(r)
		return data
	}
}

// TestNewHTTPModeWritesToStdout locks L01 (HTTP -> stdout) and L02 (HTTP mode is
// always enabled at default level "info", so an info line is emitted while a
// debug line is filtered out).
func TestNewHTTPModeWritesToStdout(t *testing.T) {
	read := captureStdout(t)

	l, err := New(Options{Mode: "http", Level: "", Format: "text"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if l.Out != os.Stdout {
		t.Errorf("http-mode logger Out = %T, want os.Stdout", l.Out)
	}

	l.Info("hello-stdout")
	l.Debug("hello-debug") // below default info -> must NOT appear

	data := read()
	if !bytes.Contains(data, []byte("hello-stdout")) {
		t.Errorf("stdout missing info line; got %q", data)
	}
	if bytes.Contains(data, []byte("hello-debug")) {
		t.Errorf("stdout unexpectedly contains debug line (default level must be info); got %q", data)
	}
}

// TestNewHTTPModeJSONFormat locks L04: Format "json" switches the formatter to
// JSON output.
func TestNewHTTPModeJSONFormat(t *testing.T) {
	read := captureStdout(t)

	l, err := New(Options{Mode: "http", Level: "info", Format: "json"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.Info("json-hello")

	data := read()
	if !bytes.Contains(data, []byte(`"msg":"json-hello"`)) {
		t.Errorf("stdout missing JSON msg field; got %q", data)
	}
	if !bytes.Contains(data, []byte(`"level":"info"`)) {
		t.Errorf("stdout missing JSON level field; got %q", data)
	}
}

// TestNewHTTPModeTextFormat locks L04 default: Format "text" (or empty) uses the
// plain text formatter, not JSON.
func TestNewHTTPModeTextFormat(t *testing.T) {
	read := captureStdout(t)

	l, err := New(Options{Mode: "http", Level: "info", Format: "text"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.Info("plain-hello")

	data := read()
	if bytes.Contains(data, []byte(`"msg":"plain-hello"`)) {
		t.Errorf("expected text (non-JSON) output, got JSON-looking line %q", data)
	}
	if !bytes.Contains(data, []byte("plain-hello")) {
		t.Errorf("missing plain-hello in text output; got %q", data)
	}
}

// TestNewStdioModeWritesToFile locks L01/L03: stdio mode writes to the named
// file (chmod 0600), never stdout.
func TestNewStdioModeWritesToFile(t *testing.T) {
	fname := filepath.Join(t.TempDir(), "server.log")

	l, err := New(Options{Mode: "stdio", Level: "info", Format: "text", Filename: fname})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if l.Out == os.Stdout {
		t.Errorf("stdio-mode logger must not write to stdout")
	}

	l.Info("file-hello")

	data, err := os.ReadFile(fname)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !bytes.Contains(data, []byte("file-hello")) {
		t.Errorf("log file missing message; got %q", data)
	}

	info, err := os.Stat(fname)
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("log file mode = %o, want 0600", got)
	}
}

// TestNewStdioModeMissingFilenameError locks L01/L03: stdio mode requires a
// Filename; an empty one is an error.
func TestNewStdioModeMissingFilenameError(t *testing.T) {
	_, err := New(Options{Mode: "stdio", Level: "info", Format: "text", Filename: ""})
	if err == nil {
		t.Fatal("New() = nil error with empty Filename in stdio mode, want error")
	}
}

// TestNewStdioModeDisabledWithoutLevel locks L02: stdio mode with Level unset
// disables logging entirely — even an info/error entry produces no record.
func TestNewStdioModeDisabledWithoutLevel(t *testing.T) {
	fname := filepath.Join(t.TempDir(), "disabled.log")

	l, err := New(Options{Mode: "stdio", Level: "", Format: "text", Filename: fname})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	hook := &test.Hook{}
	l.AddHook(hook)

	l.Info("should-not-log")
	l.Error("should-not-log-2")

	if len(hook.Entries) != 0 {
		t.Fatalf("disabled logger emitted %d entries, want 0 (L02)", len(hook.Entries))
	}
}

// TestNewInvalidModeError locks the Mode contract: anything other than http/stdio
// is rejected.
func TestNewInvalidModeError(t *testing.T) {
	_, err := New(Options{Mode: "bogus"})
	if err == nil {
		t.Fatal("New() = nil error for invalid Mode, want error")
	}
}

// TestNewInvalidFormatError locks the Format contract: anything other than
// text/json is rejected.
func TestNewInvalidFormatError(t *testing.T) {
	l, err := New(Options{Mode: "http", Level: "info", Format: "bogus"})
	if err == nil {
		t.Fatal("New() = nil error for invalid Format, want error")
	}
	if l != nil {
		t.Errorf("New() returned non-nil logger on error, want nil")
	}
}

// TestNewInvalidLevelError locks Level parsing: an unparseable level is rejected.
func TestNewInvalidLevelError(t *testing.T) {
	_, err := New(Options{Mode: "http", Level: "bogus", Format: "text"})
	if err == nil {
		t.Fatal("New() = nil error for invalid Level, want error")
	}
}

// TestWithRequestIDRoundTrip locks L09: WithRequestID stores a value that
// RequestIDFromContext retrieves unchanged.
func TestWithRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-abc-123")
	if got := RequestIDFromContext(ctx); got != "req-abc-123" {
		t.Errorf("RequestIDFromContext = %q, want %q", got, "req-abc-123")
	}
}

// TestRequestIDFromContextEmpty locks L09: a context without a request_id yields
// the empty string (never a panic).
func TestRequestIDFromContextEmpty(t *testing.T) {
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("RequestIDFromContext(background) = %q, want empty", got)
	}
}

// newHookedLogger returns a logrus logger enabled at Debug with a test.Hook
// attached, ready for assertion.
func newHookedLogger() (*logrus.Logger, *test.Hook) {
	l := logrus.New()
	l.SetLevel(logrus.DebugLevel)
	h := &test.Hook{}
	l.AddHook(h)
	return l, h
}

// TestWithContextSetsRequestID locks the entry-builder: when ctx carries a
// request_id, the produced entry carries it on every emitted record.
func TestWithContextSetsRequestID(t *testing.T) {
	l, hook := newHookedLogger()
	ctx := WithRequestID(context.Background(), "req-42")

	WithContext(l, ctx).Info("hello")

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no log entry captured")
	}
	if got, ok := e.Data["request_id"].(string); !ok || got != "req-42" {
		t.Errorf("entry request_id = %v, want %q", e.Data["request_id"], "req-42")
	}
}

// TestWithContextNoRequestID locks the entry-builder: when ctx has no request_id,
// the emitted record must NOT carry a request_id field at all.
func TestWithContextNoRequestID(t *testing.T) {
	l, hook := newHookedLogger()

	WithContext(l, context.Background()).Info("hello")

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no log entry captured")
	}
	if _, ok := e.Data["request_id"]; ok {
		t.Errorf("entry unexpectedly has request_id %v", e.Data["request_id"])
	}
}

// TestSlogHandlerForwardsToLogrus locks L07/L03GO: a record logged through the
// slog adapter reaches the logrus hook with its message, level and structured
// attribute intact.
func TestSlogHandlerForwardsToLogrus(t *testing.T) {
	l, hook := newHookedLogger()
	h := NewSlogHandler(l, slog.LevelInfo)
	sl := slog.New(h)

	sl.Info("slog-msg", "key", "value")

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("slog record not forwarded to logrus")
	}
	if e.Message != "slog-msg" {
		t.Errorf("message = %q, want %q", e.Message, "slog-msg")
	}
	if e.Level != logrus.InfoLevel {
		t.Errorf("level = %v, want info", e.Level)
	}
	if e.Data["key"] != "value" {
		t.Errorf("structured attribute key = %v, want %q", e.Data["key"], "value")
	}
}

// TestSlogHandlerLevelFiltering locks that the adapter honours its slog level: a
// record below the handler's level is filtered and never reaches logrus.
func TestSlogHandlerLevelFiltering(t *testing.T) {
	l, hook := newHookedLogger()
	h := NewSlogHandler(l, slog.LevelInfo)
	sl := slog.New(h)

	sl.Debug("debug-msg") // below Info handler level -> filtered

	if e := hook.LastEntry(); e != nil {
		t.Errorf("handler at Info level logged a Debug record: %v", e.Message)
	}
}

// TestSlogHandlerDebugAllowedAtDebug locks that a lower handler level does let
// the matching record through (guards against a handler that filters everything).
func TestSlogHandlerDebugAllowedAtDebug(t *testing.T) {
	l, hook := newHookedLogger()
	h := NewSlogHandler(l, slog.LevelDebug)
	sl := slog.New(h)

	sl.Debug("debug-msg")

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("handler at Debug level did not forward a Debug record")
	}
	if e.Message != "debug-msg" || e.Level != logrus.DebugLevel {
		t.Errorf("got message=%q level=%v, want message=%q level=debug", e.Message, e.Level, "debug-msg")
	}
}

// TestLogToolCall locks L08: the tool-call access log emits a record at info
// level carrying all five structured fields with correct values, and threads the
// request_id through the supplied entry.
func TestLogToolCall(t *testing.T) {
	l, hook := newHookedLogger()
	ctx := WithRequestID(context.Background(), "req-99")
	entry := WithContext(l, ctx)

	args := map[string]any{"feed_id": 7, "password": "REDACTED"}
	dur := 15 * time.Millisecond
	LogToolCall(entry, "get_feed", args, "STDIO", dur, "ok")

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no access-log record captured")
	}
	if e.Level != logrus.InfoLevel {
		t.Errorf("access log level = %v, want info (L08)", e.Level)
	}
	if e.Data["tool"] != "get_feed" {
		t.Errorf("tool = %v, want %q", e.Data["tool"], "get_feed")
	}
	if !reflect.DeepEqual(e.Data["args"], args) {
		t.Errorf("args = %v, want %v", e.Data["args"], args)
	}
	if e.Data["source"] != "STDIO" {
		t.Errorf("source = %v, want %q", e.Data["source"], "STDIO")
	}
	if e.Data["duration"] != dur {
		t.Errorf("duration = %v, want %v", e.Data["duration"], dur)
	}
	if e.Data["outcome"] != "ok" {
		t.Errorf("outcome = %v, want %q", e.Data["outcome"], "ok")
	}
	if e.Data["request_id"] != "req-99" {
		t.Errorf("request_id = %v, want %q (L09 threading)", e.Data["request_id"], "req-99")
	}
}

// TestLogToolCallErrorOutcome locks the error-path of the access log: the outcome
// field is reported as "error" rather than "ok".
func TestLogToolCallErrorOutcome(t *testing.T) {
	l, hook := newHookedLogger()

	LogToolCall(WithContext(l, context.Background()), "list_feeds", nil, "127.0.0.1", time.Millisecond, "error")

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no access-log record captured")
	}
	if e.Data["outcome"] != "error" {
		t.Errorf("outcome = %v, want %q", e.Data["outcome"], "error")
	}
}
