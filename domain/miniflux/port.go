package miniflux

import (
	"context"
	"time"
)

// Reader is the read-only half of the Miniflux port (SPEC §4.1): listing and
// fetching feeds, categories, entries, counters and the user profile, plus feed
// discovery.
type Reader interface {
	ListFeeds(ctx context.Context, categoryID *int, limit, offset int) ([]Feed, error)
	GetFeed(ctx context.Context, id int) (*Feed, error)
	ListCategories(ctx context.Context) ([]Category, error)
	ListEntries(ctx context.Context, filter EntryFilter) (FeedEntries, error)
	GetEntry(ctx context.Context, id int) (*Entry, error)
	GetFeedEntries(ctx context.Context, feedID int, filter EntryFilter) (FeedEntries, error)
	GetCounters(ctx context.Context) (*Counters, error)
	GetMe(ctx context.Context) (*Me, error)
	ExportOPML(ctx context.Context) (string, error)
	Discover(ctx context.Context, url string) ([]DiscoveryResult, error)
}

// FeedWriter is the feed/category-subscription write half of the port
// (SPEC §4.2): creating/updating/refreshing feeds and categories, and OPML
// import.
type FeedWriter interface {
	CreateFeed(ctx context.Context, req CreateFeedRequest) (*Feed, error)
	UpdateFeed(ctx context.Context, id int, req UpdateFeedRequest) (*Feed, error)
	RefreshFeed(ctx context.Context, id int) error
	CreateCategory(ctx context.Context, title string) (*Category, error)
	UpdateCategory(ctx context.Context, id int, title string) error
	RefreshCategory(ctx context.Context, id int) error
	ImportOPML(ctx context.Context, opml string) error
}

// EntryWriter is the entry-state write half of the port (SPEC §4.2): bulk
// marking read and updating/toggling individual entries.
type EntryWriter interface {
	MarkFeedEntriesRead(ctx context.Context, feedID int) error
	MarkCategoryEntriesRead(ctx context.Context, categoryID int) error
	UpdateEntries(ctx context.Context, req UpdateEntriesRequest) error
	ToggleEntryBookmark(ctx context.Context, entryID int) error
	UpdateEntry(ctx context.Context, id int, req UpdateEntryRequest) (*Entry, error)
}

// Deleter is the destructive half of the port (SPEC §4.3).
type Deleter interface {
	DeleteFeed(ctx context.Context, id int) error
	DeleteCategory(ctx context.Context, id int) error
	FlushHistory(ctx context.Context, before *time.Time) error
}

// Client is the port the application layer depends on to talk to the Miniflux
// upstream. It is implemented by the infrastructure HTTP client (SPEC §6.1):
// defining the port in the domain keeps the application→domain dependency edge
// clean while letting the composition root wire in the concrete implementation
// (DIP).
//
// It is the composition of the smaller Reader/FeedWriter/EntryWriter/Deleter
// interfaces so the application layer can depend on the whole surface or (when
// desired) only the slice a handler needs.
//
// Every method maps 1:1 to a single upstream request (X01). The inbound
// X-Auth-Token is threaded through ctx with WithToken; the implementation
// resolves the effective token via its ResolveToken pass-through (SPEC §3.1).
type Client interface {
	Reader
	FeedWriter
	EntryWriter
	Deleter
}
