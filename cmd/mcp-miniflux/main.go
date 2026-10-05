// Command mcp-miniflux is a stateless Remote (HTTP) MCP server that exposes
// the Miniflux RSS reader API as strongly-typed MCP tools.
//
// This is the composition root (SPEC §6.6): it parses the launch mode flag,
// loads config, builds the logger, emits the startup banner (B05/L06), starts
// the observability server (O01), builds the Miniflux client and the MCP server
// (transport + tools), and serves. Startup performs LOCAL wiring only (A02/N35):
// no outbound/upstream request is ever made here.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/teran/mcp-miniflux/application"
	"github.com/teran/mcp-miniflux/application/handlers"
	"github.com/teran/mcp-miniflux/infrastructure/config"
	"github.com/teran/mcp-miniflux/infrastructure/logging"
	mcpmcp "github.com/teran/mcp-miniflux/infrastructure/mcp"
	"github.com/teran/mcp-miniflux/infrastructure/miniflux"
	"github.com/teran/mcp-miniflux/infrastructure/observability"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Build metadata, injected by goreleaser via ldflags (SPEC §11.4 B02).
var (
	appName       = "mcp-miniflux"
	appVersion    = "dev"
	appCommitHash = "none"
	appTimestamp  = "unknown"
)

// serverInstructions is the server-level instructions string exposed to
// connected clients (SPEC §6.3).
const serverInstructions = "mcp-miniflux exposes the Miniflux RSS reader API as strongly-typed MCP tools. " +
	"Tools are grouped read → write/update → delete; destructive tools require human confirmation. " +
	"Authentication is passed through via the inbound X-Auth-Token header."

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		// The token and other secrets are never logged/printed (L05).
		fmt.Fprintln(os.Stderr, "mcp-miniflux:", err)
		os.Exit(1)
	}
}

// run is the testable composition root. It returns an error to fail fast on any
// wiring error (config, logger, client) without leaking secrets.
func run(ctx context.Context, args []string) error {
	mode, err := parseMode(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load(mode)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := buildLogger(mode, cfg.LogLevel, cfg.LogFormat, cfg.LogFilename)
	if err != nil {
		return err
	}

	// Startup banner is the FIRST log line when logging is enabled (B05/L06).
	// When logging is disabled (stdio + unset LOG_LEVEL, L02) nothing is emitted.
	emitBanner(log, appName, appVersion, appCommitHash, appTimestamp)

	// Internal observability endpoint (metrics + pprof + probes) is ALWAYS
	// present for a Remote server (O01/N32); it runs on its own listener and is
	// shut down gracefully when ctx is cancelled (SIGTERM/SIGINT).
	go func() {
		if err := observability.RunObservabilityServer(ctx, cfg.InternalAddr, log); err != nil {
			log.Errorf("observability server: %v", err)
		}
	}()

	// Construct the Miniflux client (A02/N35: this is local wiring only — the
	// client performs no outbound request at construction).
	// O03 upstream metrics: the recorder is passed as the miniflux.MetricsRecorder
	// interface (not a concrete observability type) to keep the composition-root
	// wiring layer-clean (SPEC §6.1, C07GO).
	var upstreamMetrics miniflux.MetricsRecorder = observability.NewUpstreamMetrics("mcp_miniflux", "upstream")
	client, err := miniflux.New(cfg.MinifluxAPIURL,
		miniflux.WithDefaultToken(cfg.MinifluxAPIToken),
		miniflux.WithMetrics(upstreamMetrics), // O03 upstream metrics
		miniflux.WithLogger(log),              // L09/L04GO outbound request log
		miniflux.WithTimeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("build miniflux client: %w", err)
	}

	// Compose the application layer (SPEC §6.2): client port, access-log logger
	// and the ordered (read → write/update → delete, S03) tool list. The client
	// is passed as the dmf.Client port (via miniflux.AsPort) so no concrete
	// infrastructure type crosses into the application layer (SPEC §6.1, C07GO).
	clientPort := miniflux.AsPort(client)
	deps := application.Deps{
		Client: clientPort,
		Logger: makeToolLogger(log),
		Tools:  handlers.All(clientPort),
	}

	// Build the go-sdk server, register the tools, and serve over the selected
	// transport. The slog→logrus adapter (L07/L03GO) is built here because the
	// mcp package may not import other infrastructure subpackages (arch-lint).
	sdkLogger := slog.New(logging.NewSlogHandler(log, slog.LevelInfo))
	srv := mcpmcp.NewServer(serverImplementation(), sdkLogger, serverInstructions)
	application.RegisterTools(srv, deps)

	if mode == "http" {
		handler := mcpmcp.NewStreamableHandler(srv, sdkLogger)
		return serveHTTP(ctx, handler, cfg.ListenAddr, log)
	}
	return mcpmcp.RunStdio(ctx, srv)
}

// serverImplementation returns the MCP server identity (SPEC §6.3).
func serverImplementation() *mcp.Implementation {
	return &mcp.Implementation{
		Name:        appName,
		Title:       "mcp-miniflux",
		Description: "Miniflux RSS reader as strongly-typed MCP tools.",
		Version:     appVersion,
	}
}

// parseMode extracts the -mode flag from args. It defaults to "stdio" (M06);
// the deployed Remote mode is "http". Any other value is rejected.
func parseMode(args []string) (string, error) {
	fs := flag.NewFlagSet("mcp-miniflux", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var mode string
	fs.StringVar(&mode, "mode", "stdio", "launch mode: http (deployed Remote) or stdio (local debug)")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if mode != "http" && mode != "stdio" {
		return "", fmt.Errorf("invalid -mode %q: want http or stdio", mode)
	}
	return mode, nil
}

// buildLogger constructs the logrus logger per L01/L02/L04 from the config
// surface. In stdio mode with no LOG_LEVEL the returned logger is disabled
// (L02); callers must tolerate it.
func buildLogger(mode, level, format, filename string) (*logrus.Logger, error) {
	l, err := logging.New(logging.Options{
		Mode:     mode,
		Level:    level,
		Format:   format,
		Filename: filename,
	})
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return l, nil
}

// emitBanner writes the startup banner as the first log line (B05/L06) when
// logging is enabled. It is a no-op for a disabled logger.
func emitBanner(l *logrus.Logger, name, version, commit, ts string) {
	if l == nil {
		return
	}
	l.Info(logging.Banner(name, version, commit, ts))
}

// makeToolLogger adapts the logrus pipeline to the application's app.ToolLogger
// port (L08): each access-log line is emitted on an entry bound to ctx so the
// request_id (L09) is attached, with the already-redacted args. When l is nil
// (logging disabled) it returns nil so the registry skips access logging.
func makeToolLogger(l *logrus.Logger) func(ctx context.Context, tool string, args map[string]any, source string, duration time.Duration, outcome string) {
	if l == nil {
		return nil
	}
	return func(ctx context.Context, tool string, args map[string]any, source string, duration time.Duration, outcome string) {
		logging.LogToolCall(logging.WithContext(l, ctx), tool, args, source, duration, outcome)
	}
}

// serveHTTP runs the Streamable HTTP handler on addr with graceful shutdown:
// it blocks until ctx is cancelled (SIGINT/SIGTERM wired in main), then drains
// in-flight requests via http.Server.Shutdown.
func serveHTTP(ctx context.Context, handler http.Handler, addr string, log *logrus.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("mcp streamable http server listening", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Info("shutting down mcp streamable http server")
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("mcp http server: %w", err)
	}
}
