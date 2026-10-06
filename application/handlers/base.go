// Package handlers implements the application-layer use-case handlers, one
// per MCP tool, orchestrating domain types and the domain Miniflux port.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// invalidParams builds a JSON-RPC InvalidParams error for a tool argument that
// fails validation (SPEC §6.5 S08). It becomes a protocol error (no upstream
// call is made).
func invalidParams(format string, args ...any) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: fmt.Sprintf(format, args...)}
}

// asInt validates that v is a JSON integer (decoded as float64) and returns it
// as an int. A non-integer value yields an InvalidParams error.
func asInt(name string, v any) (int, error) {
	f, ok := v.(float64)
	if !ok {
		return 0, invalidParams("%s must be an integer", name)
	}
	if f != float64(int64(f)) {
		return 0, invalidParams("%s must be an integer", name)
	}
	return int(f), nil
}

// intArg returns the value of the optional integer argument name, or def when
// absent.
func intArg(name string, args map[string]any, def int) (int, error) {
	v, ok := args[name]
	if !ok {
		return def, nil
	}
	return asInt(name, v)
}

// stringArg returns the string value of argument name, or "" when absent.
func stringArg(name string, args map[string]any) (string, error) {
	v, ok := args[name]
	if !ok {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", invalidParams("%s must be a string", name)
	}
	return s, nil
}

// boolArg returns the pointer to the boolean value of argument name, or nil
// when absent. Used for optional boolean filters.
func boolArg(name string, args map[string]any) (*bool, error) {
	v, ok := args[name]
	if !ok {
		return nil, nil
	}
	b, ok := v.(bool)
	if !ok {
		return nil, invalidParams("%s must be a boolean", name)
	}
	return &b, nil
}

// timeArg parses argument name (an RFC3339 string) into a time, or nil when
// absent. A malformed timestamp yields an InvalidParams error.
func timeArg(name string, args map[string]any) (*time.Time, error) {
	s, err := stringArg(name, args)
	if err != nil || s == "" {
		return nil, err
	}
	t, perr := time.Parse(time.RFC3339, s)
	if perr != nil {
		return nil, invalidParams("%s must be an RFC3339 timestamp", name)
	}
	return &t, nil
}

// entryFilter builds an EntryFilter from the shared list_entries /
// get_feed_entries filter arguments (SPEC §4.1).
func entryFilter(args map[string]any, withCategory bool) (dmf.EntryFilter, error) {
	var f dmf.EntryFilter

	if v, err := stringArg("status", args); err != nil {
		return f, err
	} else {
		f.Status = v
	}
	if v, err := stringArg("order", args); err != nil {
		return f, err
	} else {
		f.Order = v
	}
	if v, err := stringArg("direction", args); err != nil {
		return f, err
	} else {
		f.Direction = v
	}
	if v, err := intArg("limit", args, 100); err != nil {
		return f, err
	} else {
		f.Limit = v
	}
	if f.Limit > 1000 {
		return f, invalidParams("limit must not exceed 1000")
	}
	if v, err := intArg("offset", args, 0); err != nil {
		return f, err
	} else {
		f.Offset = v
	}
	if v, err := stringArg("search", args); err != nil {
		return f, err
	} else {
		f.Search = v
	}
	if v, err := boolArg("starred", args); err != nil {
		return f, err
	} else {
		f.Starred = v
	}
	if withCategory {
		if v, ok := args["category_id"]; ok {
			id, err := asInt("category_id", v)
			if err != nil {
				return f, err
			}
			f.CategoryID = &id
		}
	}
	if v, err := timeArg("before", args); err != nil {
		return f, err
	} else {
		f.Before = v
	}
	if v, err := timeArg("after", args); err != nil {
		return f, err
	} else {
		f.After = v
	}
	return f, nil
}

// marshalJSON marshals v (with secret fields redacted, S02) to JSON bytes.
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(dmf.Redact(v))
}

// sanitizeText strips ANSI/control escape sequences from tool text output so no
// terminal escape or control character reaches the client (S09/N23). It removes
// bytes < 0x20 except \t (0x09) and \n (0x0A), and strips CSI/OSC escape
// sequences (ESC [ ... / ESC ] ... BEL or ST). It never panics on invalid UTF-8.
func sanitizeText(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	// Iterate over bytes: escape-sequence state machine plus control-byte filter.
	in := []byte(s)
	i := 0
	for i < len(in) {
		c := in[i]
		switch {
		case c == 0x1b: // ESC: consume a full escape sequence if it is one.
			j := i + 1
			// CSI: ESC [ ... final byte in 0x40–0x7e; OSCI: ESC ] ... until
			// BEL (0x07) or ST (ESC \).
			if j < len(in) && (in[j] == '[' || in[j] == ']') {
				k := j + 1
				if in[j] == '[' {
					for k < len(in) {
						f := in[k]
						if f >= 0x40 && f <= 0x7e {
							break
						}
						k++
					}
					if k < len(in) {
						i = k + 1 // consumed the CSI sequence
						continue
					}
					i = j // malformed/incomplete: drop the ESC, keep the rest
					continue
				}
				// OSCI: ESC ] ... until BEL or ST (ESC \).
				for k < len(in) {
					if in[k] == 0x07 {
						i = k + 1
						break
					}
					if k+1 < len(in) && in[k] == 0x1b && in[k+1] == '\\' {
						i = k + 2
						break
					}
					k++
				}
				if i > j {
					continue
				}
				i = j
				continue
			}
			// Bare ESC: drop it.
			i++
			continue
		case c < 0x20 && c != '\t' && c != '\n':
			// Control character (other than tab/newline): drop it.
			i++
			continue
		case c == 0x7f:
			// DEL (0x7f) is a control character: drop it.
			i++
			continue
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// textContent builds a TextContent with the given (already redacted) text run
// through the ANSI/control sanitizer (S09/N23).
func textContent(text string) *mcp.TextContent {
	return &mcp.TextContent{Text: sanitizeText(text)}
}

// jsonResult marshals v (with secret fields redacted, S02) into a text content
// result and populates StructuredContent with the same redacted value
// (SEP-2106: tools declaring an outputSchema must return structured content).
func jsonResult(v any) (*mcp.CallToolResult, error) {
	redacted := dmf.Redact(v)
	data, err := json.Marshal(redacted)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{textContent(string(data))},
		StructuredContent: redacted,
	}, nil
}

// okResult returns a minimal success result for tools with no meaningful
// payload.
func okResult() *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content:           []mcp.Content{textContent(`{"ok":true}`)},
		StructuredContent: map[string]any{"ok": true},
	}
}

// upstreamErr maps an upstream failure into an IsError result so the model sees
// a sanitized message (S09; the error message is already sanitized by the
// infrastructure client — no secrets, L05).
func upstreamErr(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{textContent(err.Error())},
	}
}

// absoluteHTTPURL validates that s is an absolute http(s) URL (S11), used by
// the open-world discover_subscriptions tool before any upstream call.
func absoluteHTTPURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return invalidParams("url must be a valid absolute http(s) URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return invalidParams("url must use http or https scheme")
	}
	if u.Host == "" {
		return invalidParams("url must be an absolute URL")
	}
	return nil
}

// requireConfirm enforces the HITL confirmation flag (S12) on destructive
// tools: the caller must pass confirm:true, otherwise the call is refused.
func requireConfirm(args map[string]any) error {
	if v, ok := args["confirm"]; !ok || v != true {
		return invalidParams("this is a destructive, irreversible operation; pass confirm:true to proceed (S12)")
	}
	return nil
}
