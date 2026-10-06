package miniflux

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// CONTRACT — package `miniflux` (SPEC §6.1, S02). The developer must define
// the following types and methods. This test file is written FIRST (TDD, red
// state) and will not compile until they exist. Exact contract:
//
//	type CategoryRef struct {
//		ID    int    `json:"id"`
//		Title string `json:"title"`
//	}
//
//	type Feed struct {
//		ID         int         `json:"id"`
//		UserID     int         `json:"user_id"`
//		FeedURL    string      `json:"feed_url"`
//		SiteURL    string      `json:"site_url"`
//		Title      string      `json:"title"`
//		Category   CategoryRef `json:"category"`
//		Status     string      `json:"status"`
//		ErrorCount int         `json:"error_count"`
//		Username   string      `json:"username" secret:"true"`
//		Password   string      `json:"password" secret:"true"`
//	}
//
//	// Redacted returns a copy of the Feed with every secret:"true" field
//	// zeroed. The receiver is NOT mutated (value semantics).
//	func (f Feed) Redacted() Feed
//
//	type Category struct {
//		ID         int    `json:"id"`
//		Title      string `json:"title"`
//		FeedCount  int    `json:"feed_count"`
//		EntryCount int    `json:"entry_count"`
//	}
//
//	type Entry struct {
//		ID           int       `json:"id"`
//		UserID       int       `json:"user_id"`
//		FeedID       int       `json:"feed_id"`
//		Status       string    `json:"status"`
//		Starred      bool      `json:"starred"`
//		Title        string    `json:"title"`
//		URL          string    `json:"url"`
//		CommentsURL  string    `json:"comments_url"`
//		PublishedAt  time.Time `json:"published_at"`
//		CreatedAt    time.Time `json:"created_at"`
//		Content      string    `json:"content"`
//	}
//
//	type CounterTotals struct {
//		Unread int `json:"unread"`
//		Read   int `json:"read"`
//	}
//
//	type Counters struct {
//		Feeds  map[string]int `json:"feeds"`
//		Totals CounterTotals  `json:"totals"`
//	}
//
//	type Me struct {
//		ID       int    `json:"id"`
//		Username string `json:"username"`
//		IsAdmin  bool   `json:"is_admin"`
//		Theme    string `json:"theme"`
//	}
//
//	type FeedEntries struct {
//		Total   int     `json:"total"`
//		Entries []Entry `json:"entries"`
//	}
//
//	type Categories []Category
//	type Feeds      []Feed
//
// Decision notes:
//   - Username is annotated `secret:"true"` alongside Password because HTTP
//     basic-auth credentials for a feed are returned by Miniflux and both are
//     sensitive; the S02 redaction helper zeroes both. (Task: Redacted() must
//     clear both Password and Username.)
//   - Feed.Redacted uses value receiver + returns a value to guarantee the
//     original is untouched (value semantics).

var (
	testFeedJSON = `{
		"id": 42,
		"user_id": 7,
		"feed_url": "https://example.com/feed.xml",
		"site_url": "https://example.com/",
		"title": "Example Feed",
		"category": {"id": 3, "title": "Tech"},
		"status": "active",
		"error_count": 0,
		"username": "svc-account",
		"password": "super-secret-pw"
	}`

	testEntryTime   = time.Date(2026, 10, 1, 12, 30, 45, 0, time.UTC)
	testCreatedTime = time.Date(2026, 9, 30, 8, 15, 0, 0, time.UTC)

	testEntryJSON = `{
		"id": 1001,
		"user_id": 7,
		"feed_id": 42,
		"status": "unread",
		"starred": false,
		"title": "Hello world",
		"url": "https://example.com/post/1",
		"comments_url": "https://example.com/post/1#comments",
		"published_at": "2026-10-01T12:30:45Z",
		"created_at": "2026-09-30T08:15:00Z",
		"content": "<p>Hello</p>"
	}`
)

func TestFeedJSONRoundTrip(t *testing.T) {
	var f Feed
	if err := json.Unmarshal([]byte(testFeedJSON), &f); err != nil {
		t.Fatalf("unmarshal Feed: %v", err)
	}

	// Every public field round-trips from the representative payload.
	if f.ID != 42 || f.UserID != 7 {
		t.Errorf("ID/UserID = %d/%d, want 42/7", f.ID, f.UserID)
	}
	if f.FeedURL != "https://example.com/feed.xml" || f.SiteURL != "https://example.com/" {
		t.Errorf("URLs wrong: %q / %q", f.FeedURL, f.SiteURL)
	}
	if f.Title != "Example Feed" {
		t.Errorf("Title = %q", f.Title)
	}
	if f.Category != (CategoryRef{ID: 3, Title: "Tech"}) {
		t.Errorf("Category = %+v, want {3 Tech}", f.Category)
	}
	if f.Status != "active" || f.ErrorCount != 0 {
		t.Errorf("Status/ErrorCount = %q/%d", f.Status, f.ErrorCount)
	}
	if f.Username != "svc-account" || f.Password != "super-secret-pw" {
		t.Errorf("credentials not round-tripped: %q/%q", f.Username, f.Password)
	}

	// Marshal back and confirm the JSON keys are the Miniflux API names.
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal Feed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("re-unmarshal Feed to map: %v", err)
	}
	for _, key := range []string{"id", "user_id", "feed_url", "site_url", "title",
		"category", "status", "error_count", "username", "password"} {
		if _, ok := m[key]; !ok {
			t.Errorf("marshaled Feed missing JSON key %q (got keys %v)", key, keysOfMap(m))
		}
	}
}

func TestFeedRedactedZeroesSecrets(t *testing.T) {
	f := Feed{
		ID:       42,
		Title:    "Example Feed",
		Username: "svc-account",
		Password: "super-secret-pw",
	}

	got := f.Redacted()

	// Secrets are zeroed in the returned copy.
	if got.Password != "" {
		t.Errorf("Redacted().Password = %q, want empty", got.Password)
	}
	if got.Username != "" {
		t.Errorf("Redacted().Username = %q, want empty", got.Username)
	}

	// Non-secret fields are preserved.
	if got.ID != 42 || got.Title != "Example Feed" {
		t.Errorf("Redacted() dropped non-secret fields: %+v", got)
	}

	// The original must NOT be mutated (value semantics).
	if f.Password != "super-secret-pw" || f.Username != "svc-account" {
		t.Errorf("Redacted() mutated the original: %+v", f)
	}
}

// TestFeedRedactedSecretNeverInOutput: the marshaled form of a Redacted feed
// must never contain a secret value.
func TestFeedRedactedSecretNeverInOutput(t *testing.T) {
	f := Feed{Username: "u", Password: "topsecret", Title: "T"}
	b, err := json.Marshal(f.Redacted())
	if err != nil {
		t.Fatalf("marshal redacted Feed: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "topsecret") || strings.Contains(s, "\"u\"") {
		t.Fatalf("secret leaked into redacted output: %s", s)
	}
}

// TestFeedRedactedTagIsPresent locks the S02 annotation on both credential
// fields.
func TestFeedRedactedTagIsPresent(t *testing.T) {
	for _, name := range []string{"Password", "Username"} {
		sf, ok := reflect.TypeOf(Feed{}).FieldByName(name)
		if !ok {
			t.Fatalf("Feed.%s field not found", name)
		}
		if got := sf.Tag.Get("secret"); got != "true" {
			t.Errorf("Feed.%s secret tag = %q, want \"true\"", name, got)
		}
	}
}

func TestCategoryJSONRoundTrip(t *testing.T) {
	j := `{"id": 3, "title": "Tech", "feed_count": 5, "entry_count": 120}`
	var c Category
	if err := json.Unmarshal([]byte(j), &c); err != nil {
		t.Fatalf("unmarshal Category: %v", err)
	}
	if c.ID != 3 || c.Title != "Tech" || c.FeedCount != 5 || c.EntryCount != 120 {
		t.Errorf("Category = %+v", c)
	}

	b, _ := json.Marshal(c)
	for _, key := range []string{"id", "title", "feed_count", "entry_count"} {
		if !strings.Contains(string(b), key) {
			t.Errorf("marshaled Category missing %q in %s", key, b)
		}
	}
}

func TestEntryJSONRoundTrip(t *testing.T) {
	var e Entry
	if err := json.Unmarshal([]byte(testEntryJSON), &e); err != nil {
		t.Fatalf("unmarshal Entry: %v", err)
	}
	if e.ID != 1001 || e.UserID != 7 || e.FeedID != 42 {
		t.Errorf("ids wrong: %+v", e)
	}
	if e.Status != "unread" || e.Starred {
		t.Errorf("status/starred wrong: %+v", e)
	}
	if e.Title != "Hello world" || e.URL != "https://example.com/post/1" {
		t.Errorf("title/url wrong: %+v", e)
	}
	if !e.PublishedAt.Equal(testEntryTime) {
		t.Errorf("PublishedAt = %v, want %v", e.PublishedAt, testEntryTime)
	}
	if !e.CreatedAt.Equal(testCreatedTime) {
		t.Errorf("CreatedAt = %v, want %v", e.CreatedAt, testCreatedTime)
	}

	// time.Time must serialize in RFC3339 form (SDK default marshaling).
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal Entry: %v", err)
	}
	if !strings.Contains(string(b), `"published_at":"2026-10-01T12:30:45Z"`) {
		t.Errorf("published_at not RFC3339 in %s", b)
	}
}

// TestEntryRealWireDatesNonZero pins BUG 4. The real Miniflux wire format for
// published_at/created_at is an RFC3339 STRING (verified against
// https://miniflux.app/docs/api.html, e.g. "2016-12-12T16:15:19Z" and
// "2023-10-07T03:52:50.013556Z" — the latter carries fractional seconds).
// Current code parses these correctly into time.Time; this regression test
// documents the correct behaviour so a stale build or a mistaken integer/unix
// wire format can never silently regress to the zero time 0001-01-01T00:00:00Z.
func TestEntryRealWireDatesNonZero(t *testing.T) {
	j := `{
		"id": 888,
		"user_id": 123,
		"feed_id": 42,
		"title": "Entry Title",
		"url": "http://example.org/article.html",
		"published_at": "2016-12-12T16:15:19Z",
		"created_at": "2023-10-07T03:52:50.013556Z",
		"status": "unread",
		"starred": false
	}`
	var e Entry
	if err := json.Unmarshal([]byte(j), &e); err != nil {
		t.Fatalf("unmarshal Entry (real wire format): %v", err)
	}
	if e.PublishedAt.IsZero() {
		t.Error("BUG4: PublishedAt is zero — real wire format is an RFC3339 string")
	}
	if e.CreatedAt.IsZero() {
		t.Error("BUG4: CreatedAt is zero — real wire format is an RFC3339 string")
	}
	if want := time.Date(2016, 12, 12, 16, 15, 19, 0, time.UTC); !e.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", e.PublishedAt, want)
	}
	if e.CreatedAt.Year() != 2023 || e.CreatedAt.Month() != time.October {
		t.Errorf("CreatedAt = %v, want 2023-10-07", e.CreatedAt)
	}
}

// TestCountersRealWireShape pins BUG 1. Miniflux GET /v1/feeds/counters returns
// {"reads":{...},"unreads":{...}} — NOT {"feeds":...,"totals":...}
// (https://miniflux.app/docs/api.html#endpoint-counters). The Counters model
// must parse the real wire shape into a non-nil per-feed Feeds map (feed id ->
// {read, unread}) and sum the account totals. This is the contract @developer
// must implement (custom UnmarshalJSON over reads/unreads).
func TestCountersRealWireShape(t *testing.T) {
	j := `{"reads":{"1":12,"3":1},"unreads":{"1":7,"3":99}}`
	var c Counters
	if err := json.Unmarshal([]byte(j), &c); err != nil {
		t.Fatalf("unmarshal Counters: %v", err)
	}
	if c.Feeds == nil {
		t.Fatal("BUG1: Counters.Feeds is nil after unmarshaling the real {reads,unreads} wire shape")
	}
	if f := c.Feeds["1"]; f.Read != 12 || f.Unread != 7 {
		t.Errorf("Feeds[\"1\"] = %+v, want {Read:12 Unread:7}", f)
	}
	if f := c.Feeds["3"]; f.Read != 1 || f.Unread != 99 {
		t.Errorf("Feeds[\"3\"] = %+v, want {Read:1 Unread:99}", f)
	}
	if c.Totals.Read != 13 || c.Totals.Unread != 106 {
		t.Errorf("Totals = %+v, want {Read:13 Unread:106}", c.Totals)
	}

	// Output shape: feeds object keyed by feed id with per-feed read/unread,
	// plus totals — matching GetCounters.OutputSchema countersProps.
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal Counters: %v", err)
	}
	for _, key := range []string{"feeds", "totals", "read", "unread"} {
		if !strings.Contains(string(b), key) {
			t.Errorf("marshaled Counters missing %q in %s", key, b)
		}
	}
}

func TestMeJSONRoundTrip(t *testing.T) {
	j := `{"id": 7, "username": "alice", "is_admin": true, "theme": "system"}`
	var m Me
	if err := json.Unmarshal([]byte(j), &m); err != nil {
		t.Fatalf("unmarshal Me: %v", err)
	}
	if m.ID != 7 || m.Username != "alice" || !m.IsAdmin || m.Theme != "system" {
		t.Errorf("Me = %+v", m)
	}
}

func TestFeedEntriesJSONRoundTrip(t *testing.T) {
	j := `{"total": 1, "entries": [` + testEntryJSON + `]}`
	var fe FeedEntries
	if err := json.Unmarshal([]byte(j), &fe); err != nil {
		t.Fatalf("unmarshal FeedEntries: %v", err)
	}
	if fe.Total != 1 || len(fe.Entries) != 1 || fe.Entries[0].ID != 1001 {
		t.Errorf("FeedEntries = %+v", fe)
	}
}

func TestListWrappersRoundTrip(t *testing.T) {
	catJSON := `[{"id":1,"title":"A","feed_count":0,"entry_count":0},{"id":2,"title":"B","feed_count":1,"entry_count":2}]`
	var cats Categories
	if err := json.Unmarshal([]byte(catJSON), &cats); err != nil {
		t.Fatalf("unmarshal Categories: %v", err)
	}
	if len(cats) != 2 || cats[0].Title != "A" || cats[1].Title != "B" {
		t.Errorf("Categories = %+v", cats)
	}

	feedJSON := `[` + testFeedJSON + `]`
	var feeds Feeds
	if err := json.Unmarshal([]byte(feedJSON), &feeds); err != nil {
		t.Fatalf("unmarshal Feeds: %v", err)
	}
	if len(feeds) != 1 || feeds[0].ID != 42 {
		t.Errorf("Feeds = %+v", feeds)
	}
}

func keysOfMap(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
