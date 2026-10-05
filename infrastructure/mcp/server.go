package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-miniflux/domain/requestid"
)

// NewServer builds a bare go-sdk *mcp.Server with the given slog logger wired
// into ServerOptions.Logger (L07/L03GO) and the given server-level instructions.
// The slog→logrus adapter is supplied by the composition root (which may import
// infrastructure/logging); this package must not depend on other infrastructure
// subpackages (see .go-arch-lint.yml: infrastructure → domain only). The server
// has no tools until the composition root registers them via
// application.RegisterTools.
func NewServer(impl *mcp.Implementation, logger *slog.Logger, instructions string) *mcp.Server {
	return mcp.NewServer(impl, &mcp.ServerOptions{
		Instructions: instructions,
		Logger:       logger,
	})
}

// NewStreamableHandler builds the Streamable HTTP handler for a stateless
// Remote server (M02/M06) and wraps it with the request_id middleware (L09):
// a request_id is taken from the inbound X-Request-ID header when present, else
// generated, threaded through ctx via requestid.WithRequestID, and then the
// streamable handler runs with that ctx so every downstream log record and the
// outbound Miniflux X-Request-ID correlation (L09/C07GO) share the same id.
func NewStreamableHandler(s *mcp.Server, logger *slog.Logger) http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{
			Stateless:    true, // stateless Remote server (A02/N35)
			JSONResponse: true, // prefer application/json responses
			Logger:       logger,
		},
	)
	return withRequestID(streamable)
}

// withRequestIDCtx returns a ctx derived from r's ctx carrying the request_id:
// the inbound X-Request-ID header when present, else a freshly generated id
// (L09/L04GO).
func withRequestIDCtx(r *http.Request) context.Context {
	id := r.Header.Get("X-Request-ID")
	if id == "" {
		id = newRequestID()
	}
	return requestid.WithRequestID(r.Context(), id)
}

// withRequestID is the request_id middleware (L09/L04GO): it threads the
// request_id through ctx via requestid.WithRequestID and calls next with that
// ctx. It is separated from NewStreamableHandler so it can be unit-tested in
// isolation.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(withRequestIDCtx(r)))
	})
}

// RunStdio runs the server over the stdio transport (local debugging, M06),
// blocking until the client terminates the connection or ctx is cancelled.
func RunStdio(ctx context.Context, s *mcp.Server) error {
	return s.Run(ctx, &mcp.StdioTransport{})
}

// newRequestID generates a random hex request identifier. On the (practically
// impossible) entropy failure it falls back to a fixed id so the request still
// carries a non-empty correlation id.
func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return "request"
}
