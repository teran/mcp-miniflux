// Package mcp wires the go-sdk server and transports (Streamable HTTP for the
// deployed Remote mode, stdio for local debugging) onto the mcp-miniflux
// handlers (SPEC §6.3, M02/M06).
//
// This package is transport-only: it owns how a *mcp.Server is constructed
// (with the slog→logrus logger wired, L07/L03GO), how the Streamable HTTP
// handler is built and wrapped with the request_id middleware (L09/L04GO), and
// how the stdio transport is run. The composition root (cmd/mcp-miniflux) is
// responsible for registering the tools (application.RegisterTools) and for
// wiring the concrete Miniflux client and logger, because this package must not
// depend on application (see .go-arch-lint.yml: infrastructure → domain only).
package mcp
