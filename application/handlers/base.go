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

// entryFilter argument bounds (mnd-friendly named constants).
const (
	defaultEntryLimit = 100 // default list_entries/get_feed_entries limit
	maxEntryLimit     = 1000
)

// ANSI/control byte constants used by sanitizeText (mnd-friendly).
const (
	escByte = 0x1b // ESC control byte
	delByte = 0x7f // DEL control byte
	belByte = 0x07 // BEL control byte
	// stLen is the byte length of the OSC string-terminator sequence (ESC \).
	stLen = 2
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
		// Absent optional argument: return a nil *bool with no error (no
		// filter). A named nil is used so nilnil's (nil, nil) check stays
		// clean while the behaviour is identical to returning literal nil,nil.
		var absent *bool

		return absent, nil
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

	var err error
	if f.Status, err = stringArg("status", args); err != nil {
		return f, err
	}
	if f.Order, err = stringArg("order", args); err != nil {
		return f, err
	}
	if f.Direction, err = stringArg("direction", args); err != nil {
		return f, err
	}
	if f.Limit, err = intArg("limit", args, defaultEntryLimit); err != nil {
		return f, err
	}
	if f.Limit > maxEntryLimit {
		return f, invalidParams("limit must not exceed %d", maxEntryLimit)
	}
	if f.Offset, err = intArg("offset", args, 0); err != nil {
		return f, err
	}
	if f.Search, err = stringArg("search", args); err != nil {
		return f, err
	}
	if f.Starred, err = boolArg("starred", args); err != nil {
		return f, err
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
	if f.Before, err = timeArg("before", args); err != nil {
		return f, err
	}
	if f.After, err = timeArg("after", args); err != nil {
		return f, err
	}

	return f, nil
}

// marshalJSON marshals v (with secret fields redacted, S02) to JSON bytes.
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(dmf.Redact(v))
}

// consumeEscape parses an ANSI escape sequence starting at in[i], where in[i]
// is ESC, and returns the index just past the consumed bytes. A full CSI
// (ESC [ ... ) or OSC (ESC ] ... BEL|ST) sequence is consumed; a bare ESC or a
// truncated/malformed sequence consumes just the ESC byte (the rest is kept).
func consumeEscape(in []byte, i int) int {
	j := i + 1
	if j >= len(in) || (in[j] != '[' && in[j] != ']') {
		return i + 1
	}
	if in[j] == '[' {
		// CSI: ESC [ ... final byte in 0x40–0x7e.
		for k := j + 1; k < len(in); k++ {
			if in[k] >= 0x40 && in[k] <= 0x7e {
				return k + 1
			}
		}

		return j // truncated: drop the ESC, keep the rest
	}
	// OSC: ESC ] ... until BEL (0x07) or ST (ESC \).
	for k := j + 1; k < len(in); k++ {
		if in[k] == belByte {
			return k + 1
		}
		if in[k] == escByte && k+1 < len(in) && in[k+1] == '\\' {
			return k + stLen
		}
	}

	return j // truncated: drop the ESC, keep the rest
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
		case c == escByte:
			i = consumeEscape(in, i)
		case c < 0x20 && c != '\t' && c != '\n':
			// Control character (other than tab/newline): drop it.
			i++
		case c == delByte:
			// DEL (0x7f) is a control character: drop it.
			i++
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
	parsedURL, err := url.Parse(s)
	if err != nil {
		return invalidParams("url must be a valid absolute http(s) URL")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return invalidParams("url must use http or https scheme")
	}
	if parsedURL.Host == "" {
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
