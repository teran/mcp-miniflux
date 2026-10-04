// Package miniflux holds the domain models for the Miniflux RSS reader API
// (SPEC §6.1, S02). These are pure types with no dependencies. Fields that may
// hold secrets are annotated `secret:"true"` so the S02 redaction helper can
// strip them from tool output and logs.
package miniflux

import "time"

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

// Counters holds per-feed unread counts plus account totals.
type Counters struct {
	Feeds  map[string]int `json:"feeds"`
	Totals CounterTotals  `json:"totals"`
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
