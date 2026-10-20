package miniflux

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// CONTRACT — feed freshness signal (checked_at / next_check_at).
//
// Miniflux returns these two fields in GET /v1/feeds, GET /v1/feeds/{id},
// PUT /v1/feeds/{id} and in the nested `feed` object of every entry. The Feed
// model MUST carry them through to tool output so clients can observe when a
// feed was last checked and when it is scheduled to be checked next.
//
// Type decision (fixed here): both fields are `time.Time` (non-pointer),
// consistent with Entry.PublishedAt/CreatedAt. A `null` or
// "0001-01-01T00:00:00Z" next_check_at (a never-scheduled feed) normalizes to
// the zero time and re-marshals as the RFC3339 zero STRING — never JSON null —
// so the typed output schema (string / format date-time) keeps accepting it
// (S09/M07, additionalProperties:false).
//
// These are plain metadata, NOT credentials: they must never be annotated
// `secret:"true"` and must survive the S02 redaction path intact.

func TestFeedUnmarshalFreshnessFields(t *testing.T) {
	const j = `{
		"id": 42,
		"user_id": 7,
		"feed_url": "https://example.com/feed.xml",
		"site_url": "https://example.com/",
		"title": "Example Feed",
		"category": {"id": 3, "title": "Tech"},
		"status": "active",
		"error_count": 0,
		"checked_at": "2026-10-01T12:30:45Z",
		"next_check_at": "2026-10-02T00:00:00Z"
	}`
	var f Feed
	if err := json.Unmarshal([]byte(j), &f); err != nil {
		t.Fatalf("unmarshal Feed: %v", err)
	}

	wantChecked := time.Date(2026, 10, 1, 12, 30, 45, 0, time.UTC)
	wantNext := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	if !f.CheckedAt.Equal(wantChecked) {
		t.Errorf("CheckedAt = %v, want %v", f.CheckedAt, wantChecked)
	}
	if !f.NextCheckAt.Equal(wantNext) {
		t.Errorf("NextCheckAt = %v, want %v", f.NextCheckAt, wantNext)
	}

	// Marshal back: both fields appear under the Miniflux API JSON keys as
	// RFC3339 strings.
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal Feed: %v", err)
	}
	for _, key := range []string{"checked_at", "next_check_at"} {
		if !strings.Contains(string(b), key) {
			t.Errorf("marshaled Feed missing JSON key %q in %s", key, b)
		}
	}
	if !strings.Contains(string(b), `"checked_at":"2026-10-01T12:30:45Z"`) {
		t.Errorf("checked_at not RFC3339 in %s", b)
	}
}

// TestFeedUnmarshalZeroNextCheckAt pins the semantics for a feed that has not
// been scheduled yet. Miniflux may send either "0001-01-01T00:00:00Z" or
// `null` for next_check_at. Both must normalize to the zero time and re-marshal
// as the RFC3339 zero STRING (schema-conformant), never as JSON null.
func TestFeedUnmarshalZeroNextCheckAt(t *testing.T) {
	for name, nextVal := range map[string]string{
		"zeroTime": `"0001-01-01T00:00:00Z"`,
		"null":     `null`,
	} {
		t.Run(name, func(t *testing.T) {
			j := `{"id":1,"checked_at":"2026-10-01T12:30:45Z","next_check_at":` + nextVal + `}`
			var f Feed
			if err := json.Unmarshal([]byte(j), &f); err != nil {
				t.Fatalf("unmarshal Feed: %v", err)
			}
			if !f.NextCheckAt.IsZero() {
				t.Errorf("NextCheckAt = %v, want zero time", f.NextCheckAt)
			}
			b, err := json.Marshal(f)
			if err != nil {
				t.Fatalf("marshal Feed: %v", err)
			}
			if !strings.Contains(string(b), `"next_check_at":"0001-01-01T00:00:00Z"`) {
				t.Errorf("next_check_at must marshal to the zero RFC3339 string, got %s", b)
			}
		})
	}
}

// TestFeedFreshnessFieldsNotSecret locks S02: checked_at / next_check_at carry
// NO `secret:"true"` tag — they are plain metadata, never credentials.
func TestFeedFreshnessFieldsNotSecret(t *testing.T) {
	for _, name := range []string{"CheckedAt", "NextCheckAt"} {
		sf, ok := reflect.TypeOf(Feed{}).FieldByName(name)
		if !ok {
			t.Fatalf("Feed.%s field not found — add it to the Feed model", name)
		}
		if got := sf.Tag.Get("secret"); got == "true" {
			t.Errorf("Feed.%s must NOT be secret-tagged, got secret=%q", name, got)
		}
	}
}

// TestFeedFreshnessFieldsSurviveRedaction: the S02 redaction path must preserve
// the freshness fields (they are not secrets) in both Feed.Redacted() and the
// generic Redact() helper.
func TestFeedFreshnessFieldsSurviveRedaction(t *testing.T) {
	checked := time.Date(2026, 10, 1, 12, 30, 45, 0, time.UTC)
	f := Feed{CheckedAt: checked, NextCheckAt: checked, Password: "secret"}

	got := f.Redacted()
	if !got.CheckedAt.Equal(checked) || !got.NextCheckAt.Equal(checked) {
		t.Errorf("Redacted() dropped freshness fields: %+v", got)
	}

	redacted := Redact(map[string]any{"feed": f})
	feedOut, ok := redacted.(map[string]any)["feed"].(Feed)
	if !ok {
		t.Fatalf("Redact() returned unexpected type %T", redacted)
	}
	if !feedOut.CheckedAt.Equal(checked) || !feedOut.NextCheckAt.Equal(checked) {
		t.Errorf("Redact() dropped freshness fields: %+v", feedOut)
	}
	b, err := json.Marshal(feedOut)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"checked_at":"2026-10-01T12:30:45Z"`) {
		t.Errorf("redacted output missing checked_at: %s", b)
	}
}
