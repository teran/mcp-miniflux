package tools

import "testing"

// CONTRACT — feed freshness in the output schemas (S09/M07).
//
// Every tool outputSchema is strict (additionalProperties:false). A Feed object
// now carries checked_at / next_check_at (freshness signal from Miniflux), so
// feedProps() MUST declare both keys — otherwise a strict MCP client rejects the
// handler's output with "data must NOT have additional properties" (the exact
// bug class pinned in output_schema_test.go). Both are typed `string` with
// `format: date-time`, matching how time.Time marshals (RFC3339).

func TestFeedPropsIncludesFreshness(t *testing.T) {
	props := feedProps()
	for _, key := range []string{"checked_at", "next_check_at"} {
		p, ok := props[key].(map[string]any)
		if !ok {
			t.Errorf("feedProps() must declare %q (S09), got props: %v", key, props)
			continue
		}
		if p["type"] != "string" {
			t.Errorf("feedProps.%s.type = %v, want string", key, p["type"])
		}
		if p["format"] != "date-time" {
			t.Errorf("feedProps.%s.format = %v, want date-time", key, p["format"])
		}
	}
}

// TestFeedSchemaAcceptsFreshness is the strongest guard: the resolved output
// schema of get_feed (a bare Feed) and list_feeds (feeds[]) must ACCEPT a
// representative feed object that carries checked_at / next_check_at. Today
// additionalProperties:false rejects those keys, so this fails until the schema
// declares them.
func TestFeedSchemaAcceptsFreshness(t *testing.T) {
	feedWithFreshness := map[string]any{
		"id":            1,
		"user_id":       2,
		"feed_url":      "https://example.com/feed.xml",
		"site_url":      "https://example.com",
		"title":         "Example",
		"category":      map[string]any{"id": 3, "title": "tech"},
		"status":        "subscribed",
		"error_count":   0,
		"username":      "",
		"password":      "",
		"checked_at":    "2026-10-01T12:30:45Z",
		"next_check_at": "0001-01-01T00:00:00Z",
	}

	cases := []struct {
		name   string
		h      Handler
		output any
	}{
		{name: "get_feed", h: &GetFeed{}, output: feedWithFreshness},
		{name: "list_feeds", h: &ListFeeds{}, output: map[string]any{"feeds": []any{feedWithFreshness}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := asObject(t, tc.output)
			if err := resolveSchema(t, tc.h.OutputSchema()).Validate(obj); err != nil {
				t.Errorf("%s: output schema must accept a feed carrying checked_at/next_check_at (S09), got: %v", tc.name, err)
			}
		})
	}
}

// TestFeedSchemaIncludesFreshness checks the two tools that emit a Feed object
// (get_feed directly; list_feeds under feeds.items) declare the keys.
func TestFeedSchemaIncludesFreshness(t *testing.T) {
	cases := []struct {
		name string
		obj  *map[string]any
	}{
		{"get_feed", new(GetFeed).OutputSchema()},
		{"list_feeds", new(ListFeeds).OutputSchema()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := *tc.obj
			props, ok := m["properties"].(map[string]any)
			if !ok {
				t.Fatalf("%s: output schema must declare properties: %v", tc.name, m)
			}
			target := props
			if feeds, ok := props["feeds"].(map[string]any); ok {
				if items, ok := feeds["items"].(map[string]any); ok {
					if ip, ok := items["properties"].(map[string]any); ok {
						target = ip
					}
				}
			}
			for _, key := range []string{"checked_at", "next_check_at"} {
				if _, ok := target[key]; !ok {
					t.Errorf("%s: feed object schema must declare %q (S09)", tc.name, key)
				}
			}
		})
	}
}
