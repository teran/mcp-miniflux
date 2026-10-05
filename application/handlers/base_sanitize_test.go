package handlers

import (
	"strings"
	"testing"
)

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "hello world", "hello world"},
		{"tab preserved", "a\tb", "a\tb"},
		{"newline preserved", "a\nb", "a\nb"},
		{"other control stripped", "a\x00b\x01c", "abc"},
		{"DEL stripped", "a\x7fb", "ab"}, // 0x7f (DEL) is a control char (S09)
		{"bare ESC stripped", "a\x1bb", "ab"},
		{"CSI sequence stripped", "a\x1b[31mred\x1b[0mb", "aredb"},
		{"OSC hyperlink stripped", "a\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\b", "alinkb"},
		{"OSC BEL terminated stripped", "a\x1b]0;title\x07b", "ab"},
		{"utf8 preserved", "héllo → world", "héllo → world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeText(tt.in); got != tt.want {
				t.Errorf("sanitizeText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestJSONResultSanitized ensures jsonResult produces output with no raw
// ANSI/control bytes: the JSON encoder escapes control characters as \uXXXX
// (safe at the transport level), and sanitizeText additionally strips any raw
// escape sequences that survive (S09/N23).
func TestJSONResultSanitized(t *testing.T) {
	res, err := jsonResult(map[string]any{"title": "a\x1b[31mred\x1b[0mb"})
	if err != nil {
		t.Fatalf("jsonResult: %v", err)
	}
	got := textOf(t, res)
	if strings.Contains(got, "\x1b") {
		t.Errorf("jsonResult output contains a raw ESC byte: %q", got)
	}
	// The control bytes must not appear verbatim anywhere in the text output.
	if strings.Contains(got, "\x00") || strings.Contains(got, "\x07") {
		t.Errorf("jsonResult output contains a raw control byte: %q", got)
	}
}

// TestUpstreamErrSanitized ensures upstream error messages are sanitized in the
// result content (S09/N23).
func TestUpstreamErrSanitized(t *testing.T) {
	res := upstreamErr(errT("boom\x1b[0m"))
	got := textOf(t, res)
	if got != "boom" {
		t.Errorf("upstreamErr content = %q, want %q", got, "boom")
	}
	if !res.IsError {
		t.Error("upstreamErr should set IsError")
	}
}

type errT string

func (e errT) Error() string { return string(e) }
