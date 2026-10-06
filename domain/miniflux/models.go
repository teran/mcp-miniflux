// Package miniflux holds the domain models for the Miniflux RSS reader API
// (SPEC §6.1, S02). These are pure types with no dependencies. Fields that may
// hold secrets are annotated `secret:"true"` so the S02 redaction helper can
// strip them from tool output and logs.
package miniflux

import (
	"encoding/json"
	"time"
)

// CategoryRef is a lightweight reference to a category embedded in a Feed.
type CategoryRef struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// Feed models a Miniflux feed subscription. Username and Password hold HTTP
// basic-auth credentials and are annotated `secret:"true"` (S02).
type Feed struct {
	ID         int         `json:"id"`
	UserID     int         `json:"user_id"`
	FeedURL    string      `json:"feed_url"`
	SiteURL    string      `json:"site_url"`
	Title      string      `json:"title"`
	Category   CategoryRef `json:"category"`
	Status     string      `json:"status"`
	ErrorCount int         `json:"error_count"`
	Username   string      `json:"username" secret:"true"`
	Password   string      `json:"password" secret:"true"`
}

// Redacted returns a copy of the Feed with every `secret:"true"` field zeroed.
// It uses a value receiver and returns a value, so the original is never
// mutated (value semantics).
func (f Feed) Redacted() Feed {
	f.Username = ""
	f.Password = ""
	return f
}

// Category models a Miniflux category with its per-category entry counts.
type Category struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	FeedCount  int    `json:"feed_count"`
	EntryCount int    `json:"entry_count"`
}

// Entry models a single Miniflux entry.
type Entry struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	FeedID      int       `json:"feed_id"`
	Status      string    `json:"status"`
	Starred     bool      `json:"starred"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	CommentsURL string    `json:"comments_url"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	Content     string    `json:"content"`
}

// CounterTotals aggregates the account-wide unread/read counters.
type CounterTotals struct {
	Unread int `json:"unread"`
	Read   int `json:"read"`
}

// Counters holds per-feed read/unread counts plus account totals.
type Counters struct {
	Feeds  map[string]CounterTotals `json:"feeds"`
	Totals CounterTotals            `json:"totals"`
}

// UnmarshalJSON parses the real Miniflux GET /v1/feeds/counters wire shape:
// `{"reads":{feed_id:count,...},"unreads":{feed_id:count,...}}`. It folds the
// reads/unreads maps into the per-feed Feeds map (feed id -> {read, unread})
// and sums the account-wide Totals. When reads/unreads are absent or empty,
// Feeds is still initialized to a non-nil empty map so it marshals to `{}` (an
// object) rather than `null` — the typed output schema declares feeds as
// `{"type":"object"}`, which rejects null.
func (c *Counters) UnmarshalJSON(data []byte) error {
	var wire struct {
		Reads   map[string]int `json:"reads"`
		Unreads map[string]int `json:"unreads"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	feeds := make(map[string]CounterTotals, len(wire.Reads))
	readTotal, unreadTotal := 0, 0
	for id, n := range wire.Reads {
		ct := feeds[id]
		ct.Read = n
		feeds[id] = ct
		readTotal += n
	}
	for id, n := range wire.Unreads {
		ct := feeds[id]
		ct.Unread = n
		feeds[id] = ct
		unreadTotal += n
	}

	c.Feeds = feeds
	c.Totals = CounterTotals{Read: readTotal, Unread: unreadTotal}
	return nil
}

// Me models the authenticated user's profile.
type Me struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	Theme    string `json:"theme"`
}

// FeedEntries is a paginated wrapper of entries for a single feed.
type FeedEntries struct {
	Total   int     `json:"total"`
	Entries []Entry `json:"entries"`
}

// Categories is a list wrapper for categories.
type Categories []Category

// Feeds is a list wrapper for feeds.
type Feeds []Feed

// EntryFilter carries the optional query parameters for listing entries
// (SPEC §4.1). Pointer fields distinguish "unset" from a zero value so that
// absent filters are omitted from the upstream query string.
type EntryFilter struct {
	Status     string     `json:"status,omitempty"`
	Order      string     `json:"order,omitempty"`
	Direction  string     `json:"direction,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
	Search     string     `json:"search,omitempty"`
	Starred    *bool      `json:"starred,omitempty"`
	CategoryID *int       `json:"category_id,omitempty"`
	Before     *time.Time `json:"before,omitempty"`
	After      *time.Time `json:"after,omitempty"`
}

// DiscoveryResult models a single candidate feed returned by the Miniflux
// discovery endpoint (SPEC §4.1). The payload is untrusted external data
// (S07/N20) and is returned structurally.
type DiscoveryResult struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// CreateFeedRequest is the body of POST /v1/feeds (SPEC §4.2). Password and
// Username are HTTP basic-auth credentials annotated `secret:"true"` (S02).
type CreateFeedRequest struct {
	FeedURL    string `json:"feed_url"`
	CategoryID *int   `json:"category_id,omitempty"`
	Title      string `json:"title,omitempty"`
	Username   string `json:"username,omitempty" secret:"true"`
	Password   string `json:"password,omitempty" secret:"true"`
}

// UpdateFeedRequest is the body of PUT /v1/feeds/{feedID} (SPEC §4.2). Only
// the provided fields are sent; credentials are annotated `secret:"true"`.
type UpdateFeedRequest struct {
	Title        string `json:"title,omitempty"`
	SiteURL      string `json:"site_url,omitempty"`
	CategoryID   *int   `json:"category_id,omitempty"`
	Username     string `json:"username,omitempty" secret:"true"`
	Password     string `json:"password,omitempty" secret:"true"`
	UserAgent    string `json:"user_agent,omitempty"`
	ScraperRules string `json:"scraper_rules,omitempty"`
	RewriteRules string `json:"rewrite_rules,omitempty"`
	Crawler      *bool  `json:"crawler,omitempty"`
}

// UpdateEntriesRequest is the body of PUT /v1/entries (SPEC §4.2) for the
// bulk status/starred update.
type UpdateEntriesRequest struct {
	EntryIDs []int  `json:"entry_ids"`
	Status   string `json:"status,omitempty"`
	Starred  *bool  `json:"starred,omitempty"`
}

// UpdateEntryRequest is the body of PUT /v1/entries/{entryID} (SPEC §4.2) for
// editing an entry's title/content/url.
type UpdateEntryRequest struct {
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
	URL     string `json:"url,omitempty"`
}
