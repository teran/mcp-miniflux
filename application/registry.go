package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-miniflux/domain/app"
	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// source is the access-log source identifier for tool calls (L08).
const source = "mcp"

// RegisterTools declares and registers all tools in deps.Tools on the go-sdk
// server in the order they are supplied (read → write/update → delete, S03;
// SPEC §6.2). The composition root supplies the ordered list (built from the
// use-case handlers) together with the Miniflux port and logger.
func RegisterTools(server *mcp.Server, deps Deps) {
	for _, rt := range deps.Tools {
		registerTool(server, rt, deps.Logger)
	}
}

// registerTool builds an *mcp.Tool from the definition and registers it with a
// ToolHandler that delegates to the use-case handler.
func registerTool(server *mcp.Server, rt app.Tool, logger app.ToolLogger) {
	server.AddTool(buildTool(rt.Def), toolCallHandler(rt.Def.Name(), rt.Handler, logger))
}

// buildTool maps a tool definition onto the go-sdk *mcp.Tool (name, schemas,
// annotations, description from the instructions).
func buildTool(def app.Definition) *mcp.Tool {
	ann := def.Annotations()
	return &mcp.Tool{
		Name:         def.Name(),
		Description:  def.Instructions(),
		Title:        ann.Title,
		Annotations:  &ann,
		InputSchema:  def.InputSchema(),
		OutputSchema: def.OutputSchema(),
	}
}

// toolCallHandler wraps a use-case handler with the per-call cross-cutting
// concerns: token pass-through, argument parsing, access logging (L08) and
// duration/outcome bookkeeping.
func toolCallHandler(name string, h app.Handler, logger app.ToolLogger) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()

		// SPEC §3.1 pass-through: thread the inbound X-Auth-Token into ctx so
		// the client's ResolveToken honours it.
		if tok := inboundToken(req); tok != "" {
			ctx = dmf.WithToken(ctx, tok)
		}

		args, err := parseArgs(req)
		if err != nil {
			return nil, err
		}
		redacted := dmf.RedactMap(args)

		res, callErr := h.Call(ctx, args)
		outcome := "ok"
		if callErr != nil || (res != nil && res.IsError) {
			outcome = "error"
		}
		if logger != nil {
			logger(ctx, name, redacted, source, time.Since(start), outcome)
		}
		if callErr != nil {
			return nil, callErr
		}
		return res, nil
	}
}

// inboundToken extracts the inbound X-Auth-Token from the request headers
// (pass-through, SPEC §3.1). Returns "" when the request carries none (e.g. in
// stdio mode or tests without headers).
func inboundToken(req *mcp.CallToolRequest) string {
	if req == nil || req.Extra == nil || req.Extra.Header == nil {
		return ""
	}
	return req.Extra.Header.Get("X-Auth-Token")
}

// parseArgs unmarshals the raw tool arguments into a map. Empty or absent
// arguments become an empty map.
func parseArgs(req *mcp.CallToolRequest) (map[string]any, error) {
	if req == nil || req.Params == nil {
		return map[string]any{}, nil
	}
	raw := req.Params.Arguments
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "invalid arguments: " + err.Error()}
	}
	return args, nil
}
