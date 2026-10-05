package miniflux

import (
	"context"
	"time"
)

// mockClient is a minimal in-package implementation of the Client port. It
// exists purely so that `var _ Client = (*mockClient)(nil)` compiles: if the
// port's method set or signatures ever change, this file stops compiling, which
// locks the port surface (SPEC §6.1, DIP). The real DIP guard against the
// infrastructure HTTP client lives in the infrastructure package (which may
// import domain, not the reverse — go-arch-lint: domain → (nothing)).
type mockClient struct{}

func (mockClient) ListFeeds(ctx context.Context, categoryID *int, limit, offset int) ([]Feed, error) {
	return nil, nil
}
func (mockClient) GetFeed(ctx context.Context, id int) (*Feed, error)     { return nil, nil }
func (mockClient) ListCategories(ctx context.Context) ([]Category, error) { return nil, nil }
func (mockClient) ListEntries(ctx context.Context, filter EntryFilter) (FeedEntries, error) {
	return FeedEntries{}, nil
}
func (mockClient) GetEntry(ctx context.Context, id int) (*Entry, error) { return nil, nil }
func (mockClient) GetFeedEntries(ctx context.Context, feedID int, filter EntryFilter) (FeedEntries, error) {
	return FeedEntries{}, nil
}
func (mockClient) GetCounters(ctx context.Context) (*Counters, error) { return nil, nil }
func (mockClient) GetMe(ctx context.Context) (*Me, error)             { return nil, nil }
func (mockClient) ExportOPML(ctx context.Context) (string, error)     { return "", nil }
func (mockClient) Discover(ctx context.Context, url string) ([]DiscoveryResult, error) {
	return nil, nil
}
func (mockClient) CreateFeed(ctx context.Context, req CreateFeedRequest) (*Feed, error) {
	return nil, nil
}
func (mockClient) UpdateFeed(ctx context.Context, id int, req UpdateFeedRequest) (*Feed, error) {
	return nil, nil
}
func (mockClient) RefreshFeed(ctx context.Context, id int) error { return nil }
func (mockClient) CreateCategory(ctx context.Context, title string) (*Category, error) {
	return nil, nil
}
func (mockClient) UpdateCategory(ctx context.Context, id int, title string) error { return nil }
func (mockClient) RefreshCategory(ctx context.Context, id int) error              { return nil }
func (mockClient) MarkFeedEntriesRead(ctx context.Context, feedID int) error      { return nil }
func (mockClient) MarkCategoryEntriesRead(ctx context.Context, categoryID int) error {
	return nil
}
func (mockClient) UpdateEntries(ctx context.Context, req UpdateEntriesRequest) error { return nil }
func (mockClient) ToggleEntryBookmark(ctx context.Context, entryID int) error        { return nil }
func (mockClient) UpdateEntry(ctx context.Context, id int, req UpdateEntryRequest) (*Entry, error) {
	return nil, nil
}
func (mockClient) ImportOPML(ctx context.Context, opml string) error         { return nil }
func (mockClient) DeleteFeed(ctx context.Context, id int) error              { return nil }
func (mockClient) DeleteCategory(ctx context.Context, id int) error          { return nil }
func (mockClient) FlushHistory(ctx context.Context, before *time.Time) error { return nil }

// Compile-time assertion that mockClient satisfies the full Client port.
var _ Client = (*mockClient)(nil)
