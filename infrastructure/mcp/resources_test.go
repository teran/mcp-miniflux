package mcp

// Resources capability tests (M08/M09).
//
// These tests are written first (TDD) to pin the contract of the NEW
// read-only self-describing guidance resources. They drive the real protocol
// path (tool-handler -> go-sdk) through an in-memory client session, so a
// broken RegisterResources implementation cannot pass by only returning static
// data — resources/list and resources/read must actually be served over the
// wire.
//
// Contract pinned here:
//   - RegisterResources(s *mcp.Server) registers exactly four guidance
//     resources under the namespaced scheme mcp-miniflux://guidance/<name>.
//   - Each resource carries a non-empty Name/Title/Description and
//     MIMEType == "text/markdown".
//   - resources/list returns exactly those four URIs.
//   - resources/read on a known URI returns the markdown text content
//     (non-empty, ANSI/control-clean).
//   - resources/read on an UNKNOWN URI returns a not-found error
//     (mcp.CodeResourceNotFound).
//   - ResourcesInstructions() returns server-level instructions that include a
//     "Resources" index pointing at all four guidance URIs (M08).

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The exact guidance URIs the server must expose. Kept in one place so a typo
// cannot silently diverge between the list and read assertions.
const (
	uriOverview   = "mcp-miniflux://guidance/overview"
	uriTools      = "mcp-miniflux://guidance/tools"
	uriSecurity   = "mcp-miniflux://guidance/security"
	uriOperations = "mcp-miniflux://guidance/operations"
)

// guidanceURIs is the authoritative set of guidance URIs (M08 index / M09).
func guidanceURIs() []string {
	return []string{uriOverview, uriTools, uriSecurity, uriOperations}
}

// hasForbiddenControl reports whether s contains an ANSI escape (ESC) or any
// control character other than the whitespace markdown is expected to use.
func hasForbiddenControl(s string) bool {
	for _, r := range s {
		switch r {
		case '\n', '\t', '\r':
			continue
		}
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// newResourcesServer builds the repo server (NewServer) with the guidance
// instructions wired and RegisterResources applied, ready for connection.
func newResourcesServer() *mcp.Server {
	return NewServer(
		&mcp.Implementation{Name: "mcp-miniflux", Version: "test"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		ResourcesInstructions(),
	)
}

// newResourceClient connects an in-memory client session to a server that has
// RegisterResources applied. It returns the client session and a cleanup that
// closes both ends.
func newResourceClient(t *testing.T, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// listGuidance lists the resources and returns them keyed by URI.
func listGuidance(t *testing.T, cs *mcp.ClientSession) map[string]*mcp.Resource {
	t.Helper()

	res, err := cs.ListResources(context.Background(), &mcp.ListResourcesParams{})
	if err != nil {
		t.Fatalf("resources/list: %v", err)
	}
	byURI := make(map[string]*mcp.Resource, len(res.Resources))
	for _, r := range res.Resources {
		if _, dup := byURI[r.URI]; dup {
			t.Errorf("resources/list returned duplicate URI %q", r.URI)
		}
		byURI[r.URI] = r
	}
	return byURI
}

// readGuidance reads a single resource over the wire and returns its contents.
func readGuidance(t *testing.T, cs *mcp.ClientSession, uri string) *mcp.ReadResourceResult {
	t.Helper()

	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("resources/read %q: %v", uri, err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("resources/read %q: got %d contents, want 1", uri, len(res.Contents))
	}
	return res
}

// TestRegisterResourcesListsExactlyGuidanceSet verifies resources/list returns
// exactly the four namespaced guidance resources, each with the required
// self-describing metadata and markdown MIME type (M08/M09).
func TestRegisterResourcesListsExactlyGuidanceSet(t *testing.T) {
	srv := newResourcesServer()
	RegisterResources(srv)
	cs := newResourceClient(t, srv)

	byURI := listGuidance(t, cs)

	if len(byURI) != len(guidanceURIs()) {
		t.Errorf("resources/list returned %d resources, want %d", len(byURI), len(guidanceURIs()))
	}

	for _, uri := range guidanceURIs() {
		r, ok := byURI[uri]
		if !ok {
			t.Errorf("resources/list missing %q", uri)
			continue
		}
		if !strings.HasPrefix(r.URI, "mcp-miniflux://guidance/") {
			t.Errorf("%q: URI is not namespaced under mcp-miniflux://guidance/", r.URI)
		}
		if r.Name == "" {
			t.Errorf("%q: empty Name", uri)
		}
		if r.Title == "" {
			t.Errorf("%q: empty Title", uri)
		}
		if r.Description == "" {
			t.Errorf("%q: empty Description", uri)
		}
		if r.MIMEType != "text/markdown" {
			t.Errorf("%q: MIMEType = %q, want text/markdown", uri, r.MIMEType)
		}
	}
}

// TestRegisterResourcesReadsEachKnown verifies resources/read returns the
// markdown text for every known guidance URI, with the URI/MIME round-tripped
// and content non-empty and ANSI/control-clean (M09).
func TestRegisterResourcesReadsEachKnown(t *testing.T) {
	srv := newResourcesServer()
	RegisterResources(srv)
	cs := newResourceClient(t, srv)

	for _, uri := range guidanceURIs() {
		uri := uri
		t.Run(uri, func(t *testing.T) {
			res := readGuidance(t, cs, uri)
			c := res.Contents[0]
			if c.URI != uri {
				t.Errorf("content URI = %q, want %q", c.URI, uri)
			}
			if c.MIMEType != "text/markdown" {
				t.Errorf("content MIMEType = %q, want text/markdown", c.MIMEType)
			}
			if c.Text == "" {
				t.Fatal("content text is empty")
			}
			if strings.TrimSpace(c.Text) == "" {
				t.Error("content text is only whitespace")
			}
			if hasForbiddenControl(c.Text) {
				t.Errorf("content contains ANSI/control characters: %q", c.Text)
			}
		})
	}
}

// TestRegisterResourcesReadUnknownReturnsNotFound verifies resources/read on an
// unregistered URI returns a not-found error with code CodeResourceNotFound,
// both for an unknown guidance name and for a foreign scheme (namespace
// isolation).
func TestRegisterResourcesReadUnknownReturnsNotFound(t *testing.T) {
	srv := newResourcesServer()
	RegisterResources(srv)
	cs := newResourceClient(t, srv)

	for _, uri := range []string{
		"mcp-miniflux://guidance/does-not-exist",
		"file:///etc/passwd",
		"mcp-miniflux://other/overview",
	} {
		uri := uri
		t.Run(uri, func(t *testing.T) {
			_, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
			if err == nil {
				t.Fatal("resources/read on unknown URI succeeded, want not-found error")
			}
			var rpcErr *jsonrpc.Error
			if !errors.As(err, &rpcErr) {
				t.Fatalf("resources/read error type = %T, want *jsonrpc.Error", err)
			}
			if rpcErr.Code != mcp.CodeResourceNotFound {
				t.Errorf("resources/read error code = %d, want %d (ResourceNotFound)", rpcErr.Code, mcp.CodeResourceNotFound)
			}
			if rpcErr.Message == "" {
				t.Error("resources/read error message is empty")
			}
		})
	}
}

// TestResourcesContentANSICleanAcrossAll pins M09 for every guidance resource:
// the full markdown body returned over the wire must be free of ANSI/control
// sequences and non-trivial. This is asserted across the whole set, not just a
// single representative.
func TestResourcesContentANSICleanAcrossAll(t *testing.T) {
	srv := newResourcesServer()
	RegisterResources(srv)
	cs := newResourceClient(t, srv)

	for _, uri := range guidanceURIs() {
		res := readGuidance(t, cs, uri)
		if hasForbiddenControl(res.Contents[0].Text) {
			t.Errorf("%q: content has ANSI/control characters", uri)
		}
	}
}

// TestResourcesInstructionsIndexesGuidance verifies the server-level
// instructions include a "Resources" index that points to all four guidance
// URIs (M08), so clients can discover them without calling resources/list.
func TestResourcesInstructionsIndexesGuidance(t *testing.T) {
	instructions := ResourcesInstructions()

	if instructions == "" {
		t.Fatal("ResourcesInstructions() returned empty instructions")
	}
	if !strings.Contains(instructions, "Resources") {
		t.Error("instructions do not mention a Resources index")
	}
	for _, uri := range guidanceURIs() {
		if !strings.Contains(instructions, uri) {
			t.Errorf("instructions do not index guidance URI %q", uri)
		}
	}
}
