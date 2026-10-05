package handlers

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// textOf concatenates the text of a result's content blocks.
func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil {
		t.Fatal("result is nil")
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// okRes checks the result is not an error and returns its text.
func okRes(t *testing.T, res *mcp.CallToolResult, err error) string {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError result: %s", textOf(t, res))
	}
	return textOf(t, res)
}
