package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Guidance resources (M08/M09).
//
// mcp-miniflux exposes a small set of read-only, self-describing guidance
// resources under the namespaced scheme mcp-miniflux://guidance/<name>. Each
// resource carries static markdown that orients a connected client: an
// overview of the server, the tool-grouping model, the security model and the
// operational notes. The content is authored here (never derived from the
// environment) so it is deterministic, free of secrets and free of
// ANSI/control sequences.
//
// The URIs, their display metadata and their markdown bodies are declared in
// the single authoritative map below so the index and the read path cannot
// silently diverge.
const guidanceScheme = "mcp-miniflux://guidance/"

const (
	uriGuidanceOverview   = guidanceScheme + "overview"
	uriGuidanceTools      = guidanceScheme + "tools"
	uriGuidanceSecurity   = guidanceScheme + "security"
	uriGuidanceOperations = guidanceScheme + "operations"
)

// The markdown bodies are static, self-contained and free of secrets and
// ANSI/control sequences (M09). They are authored here rather than loaded from
// the environment to keep the server deterministic and stateless.

const overviewMarkdown = `# mcp-miniflux Overview

mcp-miniflux is a stateless Remote MCP server that exposes the Miniflux RSS
reader API (https://miniflux.app) as a set of specific, strongly-typed MCP
tools. It runs as a sidecar next to a Miniflux instance and forwards the
client's requests to it.

Miniflux owns all state: feeds, categories, entries, counters and OPML. The
server itself holds no persistent state.

- Transport: Streamable HTTP on port 8080.
- Auth: the inbound X-Auth-Token header is passed straight through to Miniflux.
- Tools: read → write/update → delete, with destructive tools requiring human
  confirmation.

Read the tools, security and operations resources for the details.
`

const toolsMarkdown = `# mcp-miniflux Tools

Tools are grouped by their effect on Miniflux state:

- Read tools: fetch feeds, categories, entries and counters without changing
  anything.
- Write/update tools: create or modify state (for example adding a feed).
- Delete tools: remove state. Destructive tools require human confirmation
  before they execute.

Every tools/call validates its arguments against the tool's input schema before
execution; invalid arguments are rejected before any upstream request is made.
A single tool call maps to a single upstream Miniflux request.
`

const securityMarkdown = `# mcp-miniflux Security

Authentication is delegated to Miniflux:

- The inbound X-Auth-Token header is forwarded verbatim to Miniflux as
  X-Auth-Token.
- If no inbound header is present, the MINIFLUX_API_TOKEN environment variable
  is used.

Secret-bearing fields are marked and stripped from tool output and logs by a
single redaction helper. Credentials and tokens are never logged. TLS is
terminated by the reverse proxy, never inside the server.
`

const operationsMarkdown = `# mcp-miniflux Operations

Observability runs on a separate internal address (default port 8081) serving
Prometheus metrics, pprof and health probes.

- Access log: each tool call is logged at info level with the tool name, source,
  duration and outcome (args are redacted).
- Request correlation: every request carries a request_id that is threaded
  through logs and propagated to Miniflux as X-Request-ID.
- The server is stateless and performs no outbound requests at startup.
`

// guidanceResources is the authoritative set of guidance resources exposed by
// the server. It is the single source of truth for both resources/list and
// resources/read.
var guidanceResources = []guidanceResource{
	{
		uri:         uriGuidanceOverview,
		name:        "mcp-miniflux overview",
		title:       "mcp-miniflux Overview",
		description: "Overview of the mcp-miniflux MCP server and how it maps the Miniflux RSS reader API to MCP tools.",
		markdown:    overviewMarkdown,
	},
	{
		uri:         uriGuidanceTools,
		name:        "mcp-miniflux tools",
		title:       "mcp-miniflux Tools",
		description: "The tool grouping model: read, write/update and delete tools, and how destructive tools require confirmation.",
		markdown:    toolsMarkdown,
	},
	{
		uri:         uriGuidanceSecurity,
		name:        "mcp-miniflux security",
		title:       "mcp-miniflux Security",
		description: "The security model: authentication pass-through, secret handling and how credentials are never exposed.",
		markdown:    securityMarkdown,
	},
	{
		uri:         uriGuidanceOperations,
		name:        "mcp-miniflux operations",
		title:       "mcp-miniflux Operations",
		description: "Operational notes: observability endpoints, logging and how requests are correlated.",
		markdown:    operationsMarkdown,
	},
}

// guidanceResource describes one self-describing guidance resource.
type guidanceResource struct {
	uri         string
	name        string
	title       string
	description string
	markdown    string
}

// RegisterResources registers the read-only self-describing guidance resources
// on the given server. Each resource maps 1:1 to its markdown body; reading an
// unknown URI returns mcp.ResourceNotFoundError. It is safe to call more than
// once (the SDK replaces a resource with the same URI).
func RegisterResources(s *mcp.Server) {
	for _, r := range guidanceResources {
		s.AddResource(&mcp.Resource{
			URI:         r.uri,
			Name:        r.name,
			Title:       r.title,
			Description: r.description,
			MIMEType:    "text/markdown",
		}, readGuidanceResource(r))
	}
}

// readGuidanceResource returns the ResourceHandler for a single guidance
// resource: it returns the resource's own markdown body, or a
// mcp.ResourceNotFoundError for any other URI.
func readGuidanceResource(r guidanceResource) mcp.ResourceHandler {
	return func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if req.Params.URI != r.uri {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      r.uri,
					MIMEType: "text/markdown",
					Text:     r.markdown,
				},
			},
		}, nil
	}
}

// ResourcesInstructions returns the server-level instructions string. It orients
// a connected client about the tool model and includes a "Resources" index
// pointing at all four guidance resources so clients can discover them without
// calling resources/list (M08). It is separate from RegisterResources because
// the SDK only exposes Instructions through ServerOptions at NewServer time.
func ResourcesInstructions() string {
	return "mcp-miniflux exposes the Miniflux RSS reader API as strongly-typed MCP tools. " +
		"Tools are grouped read → write/update → delete; destructive tools require human confirmation. " +
		"Authentication is passed through via the inbound X-Auth-Token header.\n\n" +
		"## Resources\n\n" +
		"Self-describing guidance is available as read-only markdown resources:\n\n" +
		"- " + uriGuidanceOverview + " — overview of the server and how the Miniflux API maps to tools\n" +
		"- " + uriGuidanceTools + " — the tool grouping model (read → write/update → delete)\n" +
		"- " + uriGuidanceSecurity + " — the security model: auth pass-through and secret handling\n" +
		"- " + uriGuidanceOperations + " — operational notes: observability and request correlation"
}
