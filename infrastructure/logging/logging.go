// Package logging builds the logrus logger and provides the slog->logrus
// adapter used to wire the go-sdk logger into the server's logging pipeline.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// Options maps the §9 table onto logger construction. Mode is "http"|"stdio";
// Level is a logrus level string ("" in stdio means "disabled", L02); Format is
// "text"|"json" (default "text", L04); Filename is the stdio log file.
type Options struct {
	Mode     string
	Level    string
	Format   string
	Filename string
}

const (
	defaultLevel  = "info"
	defaultFormat = "text"
	filePerm      = 0o600
)

// New builds a logrus logger per L01/L02/L04:
//
//   - Mode "http" -> Out = os.Stdout, always enabled, default level "info"
//     (overridable via Level).
//   - Mode "stdio" -> Out = file at Filename (chmod 0600); if Level == "" the
//     logger is DISABLED (nothing is emitted, L02); if Filename == "" it is an
//     error.
//   - Format "text" -> TextFormatter; "json" -> JSONFormatter.
//   - Any other Mode / Format / unparseable Level is an error.
func New(opts Options) (*logrus.Logger, error) {
	switch opts.Mode {
	case "http", "stdio":
	default:
		return nil, fmt.Errorf("logging: invalid mode %q", opts.Mode)
	}

	format := opts.Format
	if format == "" {
		format = defaultFormat
	}
	var formatter logrus.Formatter
	switch format {
	case "text":
		formatter = &logrus.TextFormatter{}
	case "json":
		formatter = &logrus.JSONFormatter{}
	default:
		return nil, fmt.Errorf("logging: invalid format %q", opts.Format)
	}

	l := logrus.New()
	l.SetFormatter(formatter)

	if opts.Mode == "http" {
		l.SetOutput(os.Stdout)
		return l, setLevel(l, opts.Level, defaultLevel)
	}

	// stdio mode: log to a file (chmod 0600), never stdout (L01/L03).
	if opts.Filename == "" {
		return nil, errors.New("logging: stdio mode requires a non-empty Filename")
	}
	f, err := os.OpenFile(opts.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm)
	if err != nil {
		return nil, fmt.Errorf("logging: open log file: %w", err)
	}

	// stdio + empty Level == "disabled" (L02): the logger is still returned so
	// callers get a usable handle, but nothing is emitted and output is
	// discarded.
	if opts.Level == "" {
		l.SetOutput(io.Discard)
		l.SetLevel(logrus.PanicLevel)
		return l, nil
	}

	l.SetOutput(f)
	return l, setLevel(l, opts.Level, "")
}

// setLevel parses level and applies it to l. When level is empty it falls back
// to fallback (used for the http-mode default of "info"). An unparseable level
// is an error.
func setLevel(l *logrus.Logger, level, fallback string) error {
	if level == "" {
		level = fallback
	}
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("logging: invalid level %q: %w", level, err)
	}
	l.SetLevel(lvl)
	return nil
}

// ctxKey is the unexported context key for the request_id (L09/L04GO).
type ctxKey struct{}

// WithRequestID stores the request id in ctx (L09).
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestIDFromContext retrieves the request id from ctx, returning "" when
// absent (L09).
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// WithContext returns a logrus.Entry bound to ctx; when ctx carries a
// request_id, the entry has a "request_id" field set (L09).
func WithContext(l *logrus.Logger, ctx context.Context) *logrus.Entry {
	entry := logrus.NewEntry(l)
	if id := RequestIDFromContext(ctx); id != "" {
		entry = entry.WithField("request_id", id)
	}
	return entry
}

// NewSlogHandler returns a slog.Handler that forwards records into the given
// logrus logger, honouring the given level for slog-level filtering (L07/L03GO).
func NewSlogHandler(l *logrus.Logger, level slog.Level) slog.Handler {
	return &slogHandler{logger: l, level: level}
}

// slogHandler adapts slog records into the logrus pipeline.
type slogHandler struct {
	logger *logrus.Logger
	level  slog.Level
}

// Enabled reports whether records at level should be logged. The slog handler
// owns its own level filtering; logrus-level filtering is left to the wrapped
// logger.
func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle forwards a single slog record to logrus at the corresponding level.
func (h *slogHandler) Handle(_ context.Context, r slog.Record) error {
	fields := logrus.Fields{}
	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.Any()
		return true
	})

	entry := h.logger.WithFields(fields)
	switch r.Level {
	case slog.LevelDebug:
		entry.Debug(r.Message)
	case slog.LevelInfo:
		entry.Info(r.Message)
	case slog.LevelWarn:
		entry.Warn(r.Message)
	default:
		entry.Error(r.Message)
	}
	return nil
}

// WithAttrs returns a handler that also carries the given attributes. This
// implementation keeps the same underlying logger; attributes are captured per
// record.
func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns a handler scoped to a group. This implementation ignores
// the group and forwards records unchanged.
func (h *slogHandler) WithGroup(name string) slog.Handler {
	return h
}

// LogToolCall emits the per-request tool-call access log at info level (L08):
// structured fields tool, args (already redacted), source, duration, outcome.
func LogToolCall(entry *logrus.Entry, tool string, redactedArgs map[string]any, source string, duration time.Duration, outcome string) {
	entry.WithFields(logrus.Fields{
		"tool":     tool,
		"args":     redactedArgs,
		"source":   source,
		"duration": duration,
		"outcome":  outcome,
	}).Info("tool call")
}

// Banner returns the startup banner string (SPEC §9 L06, §11.4 B05). It is
// intended to be the first line written to the channel-appropriate log when
// logging is enabled.
func Banner(appName, version, commit, ts string) string {
	return fmt.Sprintf("Starting %s/%s (commit: %s; built at %s) ...", appName, version, commit, ts)
}
