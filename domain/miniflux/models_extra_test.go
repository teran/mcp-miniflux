package miniflux

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Tests written by @developer for the new domain request/filter types (S02).

func TestEntryFilterJSONTags(t *testing.T) {
	starred := true
	cat := 7
	before := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := EntryFilter{
		Status:     "unread",
		Order:      "published_at",
		Direction:  "desc",
		Limit:      50,
		Offset:     10,
		Search:     "golang",
		Starred:    &starred,
		CategoryID: &cat,
		Before:     &before,
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal EntryFilter: %v", err)
	}
	for _, key := range []string{
		"status", "order", "direction", "limit", "offset",
		"search", "starred", "category_id", "before",
	} {
		if !strings.Contains(string(b), key) {
			t.Errorf("EntryFilter JSON missing %q: %s", key, b)
		}
	}
}

func TestDiscoveryResultJSON(t *testing.T) {
	j := `{"url":"https://example.com/feed.xml","title":"T","type":"rss"}`
	var d DiscoveryResult
	if err := json.Unmarshal([]byte(j), &d); err != nil {
		t.Fatalf("unmarshal DiscoveryResult: %v", err)
	}
	if d.URL != "https://example.com/feed.xml" || d.Title != "T" || d.Type != "rss" {
		t.Errorf("DiscoveryResult = %+v", d)
	}
}

func TestCreateFeedRequestJSON(t *testing.T) {
	cat := 3
	req := CreateFeedRequest{FeedURL: "https://example.com/feed.xml", CategoryID: &cat, Title: "T", Password: "pw"}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal CreateFeedRequest: %v", err)
	}
	for _, key := range []string{"feed_url", "category_id", "title", "password"} {
		if !strings.Contains(string(b), key) {
			t.Errorf("CreateFeedRequest JSON missing %q: %s", key, b)
		}
	}
	// secret:true annotations present (S02)
	for _, name := range []string{"Username", "Password"} {
		sf, ok := reflect.TypeOf(CreateFeedRequest{}).FieldByName(name)
		if !ok {
			t.Fatalf("CreateFeedRequest.%s missing", name)
		}
		if sf.Tag.Get("secret") != "true" {
			t.Errorf("CreateFeedRequest.%s secret tag = %q", name, sf.Tag.Get("secret"))
		}
	}
}

func TestUpdateFeedRequestSecretTags(t *testing.T) {
	for _, name := range []string{"Username", "Password"} {
		sf, ok := reflect.TypeOf(UpdateFeedRequest{}).FieldByName(name)
		if !ok {
			t.Fatalf("UpdateFeedRequest.%s missing", name)
		}
		if sf.Tag.Get("secret") != "true" {
			t.Errorf("UpdateFeedRequest.%s secret tag = %q", name, sf.Tag.Get("secret"))
		}
	}
}

func TestUpdateEntriesRequestJSON(t *testing.T) {
	starred := true
	req := UpdateEntriesRequest{EntryIDs: []int{1, 2}, Status: "read", Starred: &starred}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal UpdateEntriesRequest: %v", err)
	}
	if !strings.Contains(string(b), `"entry_ids":[1,2]`) {
		t.Errorf("UpdateEntriesRequest entry_ids wrong: %s", b)
	}
}

func TestUpdateEntryRequestJSON(t *testing.T) {
	req := UpdateEntryRequest{Title: "T", Content: "<p>c</p>", URL: "https://example.com"}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal UpdateEntryRequest: %v", err)
	}
	for _, key := range []string{"title", "content", "url"} {
		if !strings.Contains(string(b), key) {
			t.Errorf("UpdateEntryRequest JSON missing %q: %s", key, b)
		}
	}
}
