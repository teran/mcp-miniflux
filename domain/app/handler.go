// Package app holds the application-layer contracts shared by the tool
// registry (application root) and the use-case handlers (application/handlers).
//
// go-arch-lint treats application/application/handlers and the domain
// subpackages as distinct components that may not import each other, so the
// types they must share live here in the domain leaf (SPEC §6.1). The
// composition root (cmd) assembles the ordered []Tool and injects it into the
// registry.
package app

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Definition is the structural subset of domain/tools.Handler that the
// registry needs to build an *mcp.Tool (name, schemas, annotations,
// instructions). domain/tools tool structs satisfy it without any import, so
// this package stays free of a domain→domain/tools edge.
type Definition interface {
	Name() string
	InputSchema() *map[string]any
	OutputSchema() *map[string]any
	Annotations() mcp.ToolAnnotations
	Instructions() string
}

// Handler is implemented by each tool's application-layer use-case handler. It
// mirrors the domain/tools.Handler.Call shape: the registry parses the raw
// request arguments into a map, threads the inbound token into ctx and calls
// Call, then logs the access line (L08).
type Handler interface {
	Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error)
}

// ToolLogger emits the per-request tool-call access log (L08) at info level
// with already-redacted args (L05). The composition root wires it to
// infrastructure/logging (LogToolCall + WithContext for request_id
// correlation).
type ToolLogger func(ctx context.Context, tool string, redactedArgs map[string]any, source string, duration time.Duration, outcome string)

// Tool pairs a tool definition (schemas, annotations, instructions) with its
// application-layer use-case handler.
type Tool struct {
	Def     Definition
	Handler Handler
}
