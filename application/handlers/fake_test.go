package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// errSentinel is returned by fake client methods that have no behaviour set.
var errSentinel = errors.New("boom")

// fakeClient is an in-memory implementation of the dmf.Client port. Each field
// holds the behaviour for the matching method; when nil, the method returns
// errSentinel. The token threaded through ctx is captured on every call.
type fakeClient struct {
	token string

	listFeeds       func(ctx context.Context, categoryID *int, limit, offset int) ([]dmf.Feed, error)
	getFeed         func(ctx context.Context, id int) (*dmf.Feed, error)
	listCategories  func(ctx context.Context) ([]dmf.Category, error)
	listEntries     func(ctx context.Context, filter dmf.EntryFilter) (dmf.FeedEntries, error)
	getEntry        func(ctx context.Context, id int) (*dmf.Entry, error)
	getFeedEntries  func(ctx context.Context, feedID int, filter dmf.EntryFilter) (dmf.FeedEntries, error)
	getCounters     func(ctx context.Context) (*dmf.Counters, error)
	getMe           func(ctx context.Context) (*dmf.Me, error)
	exportOPML      func(ctx context.Context) (string, error)
	discover        func(ctx context.Context, url string) ([]dmf.DiscoveryResult, error)
	createFeed      func(ctx context.Context, req dmf.CreateFeedRequest) (*dmf.Feed, error)
	updateFeed      func(ctx context.Context, id int, req dmf.UpdateFeedRequest) (*dmf.Feed, error)
	refreshFeed     func(ctx context.Context, id int) error
	createCategory  func(ctx context.Context, title string) (*dmf.Category, error)
	updateCategory  func(ctx context.Context, id int, title string) error
	refreshCategory func(ctx context.Context, id int) error
	markFeedRead    func(ctx context.Context, feedID int) error
	markCatRead     func(ctx context.Context, categoryID int) error
	updateEntries   func(ctx context.Context, req dmf.UpdateEntriesRequest) error
	toggleBookmark  func(ctx context.Context, entryID int) error
	updateEntry     func(ctx context.Context, id int, req dmf.UpdateEntryRequest) (*dmf.Entry, error)
	importOPML      func(ctx context.Context, opml string) error
	deleteFeed      func(ctx context.Context, id int) error
	deleteCategory  func(ctx context.Context, id int) error
	flushHistory    func(ctx context.Context, before *time.Time) error
}

func (f *fakeClient) capture(ctx context.Context) {
	f.token = dmf.TokenFromContext(ctx)
}

func (f *fakeClient) ListFeeds(ctx context.Context, categoryID *int, limit, offset int) ([]dmf.Feed, error) {
	f.capture(ctx)
	if f.listFeeds == nil {
		return nil, errSentinel
	}
	return f.listFeeds(ctx, categoryID, limit, offset)
}

func (f *fakeClient) GetFeed(ctx context.Context, id int) (*dmf.Feed, error) {
	f.capture(ctx)
	if f.getFeed == nil {
		return nil, errSentinel
	}
	return f.getFeed(ctx, id)
}

func (f *fakeClient) ListCategories(ctx context.Context) ([]dmf.Category, error) {
	f.capture(ctx)
	if f.listCategories == nil {
		return nil, errSentinel
	}
	return f.listCategories(ctx)
}

func (f *fakeClient) ListEntries(ctx context.Context, filter dmf.EntryFilter) (dmf.FeedEntries, error) {
	f.capture(ctx)
	if f.listEntries == nil {
		return dmf.FeedEntries{}, errSentinel
	}
	return f.listEntries(ctx, filter)
}

func (f *fakeClient) GetEntry(ctx context.Context, id int) (*dmf.Entry, error) {
	f.capture(ctx)
	if f.getEntry == nil {
		return nil, errSentinel
	}
	return f.getEntry(ctx, id)
}

func (f *fakeClient) GetFeedEntries(ctx context.Context, feedID int, filter dmf.EntryFilter) (dmf.FeedEntries, error) {
	f.capture(ctx)
	if f.getFeedEntries == nil {
		return dmf.FeedEntries{}, errSentinel
	}
	return f.getFeedEntries(ctx, feedID, filter)
}

func (f *fakeClient) GetCounters(ctx context.Context) (*dmf.Counters, error) {
	f.capture(ctx)
	if f.getCounters == nil {
		return nil, errSentinel
	}
	return f.getCounters(ctx)
}

func (f *fakeClient) GetMe(ctx context.Context) (*dmf.Me, error) {
	f.capture(ctx)
	if f.getMe == nil {
		return nil, errSentinel
	}
	return f.getMe(ctx)
}

func (f *fakeClient) ExportOPML(ctx context.Context) (string, error) {
	f.capture(ctx)
	if f.exportOPML == nil {
		return "", errSentinel
	}
	return f.exportOPML(ctx)
}

func (f *fakeClient) Discover(ctx context.Context, url string) ([]dmf.DiscoveryResult, error) {
	f.capture(ctx)
	if f.discover == nil {
		return nil, errSentinel
	}
	return f.discover(ctx, url)
}

func (f *fakeClient) CreateFeed(ctx context.Context, req dmf.CreateFeedRequest) (*dmf.Feed, error) {
	f.capture(ctx)
	if f.createFeed == nil {
		return nil, errSentinel
	}
	return f.createFeed(ctx, req)
}

func (f *fakeClient) UpdateFeed(ctx context.Context, id int, req dmf.UpdateFeedRequest) (*dmf.Feed, error) {
	f.capture(ctx)
	if f.updateFeed == nil {
		return nil, errSentinel
	}
	return f.updateFeed(ctx, id, req)
}

func (f *fakeClient) RefreshFeed(ctx context.Context, id int) error {
	f.capture(ctx)
	if f.refreshFeed == nil {
		return errSentinel
	}
	return f.refreshFeed(ctx, id)
}

func (f *fakeClient) CreateCategory(ctx context.Context, title string) (*dmf.Category, error) {
	f.capture(ctx)
	if f.createCategory == nil {
		return nil, errSentinel
	}
	return f.createCategory(ctx, title)
}

func (f *fakeClient) UpdateCategory(ctx context.Context, id int, title string) error {
	f.capture(ctx)
	if f.updateCategory == nil {
		return errSentinel
	}
	return f.updateCategory(ctx, id, title)
}

func (f *fakeClient) RefreshCategory(ctx context.Context, id int) error {
	f.capture(ctx)
	if f.refreshCategory == nil {
		return errSentinel
	}
	return f.refreshCategory(ctx, id)
}

func (f *fakeClient) MarkFeedEntriesRead(ctx context.Context, feedID int) error {
	f.capture(ctx)
	if f.markFeedRead == nil {
		return errSentinel
	}
	return f.markFeedRead(ctx, feedID)
}

func (f *fakeClient) MarkCategoryEntriesRead(ctx context.Context, categoryID int) error {
	f.capture(ctx)
	if f.markCatRead == nil {
		return errSentinel
	}
	return f.markCatRead(ctx, categoryID)
}

func (f *fakeClient) UpdateEntries(ctx context.Context, req dmf.UpdateEntriesRequest) error {
	f.capture(ctx)
	if f.updateEntries == nil {
		return errSentinel
	}
	return f.updateEntries(ctx, req)
}

func (f *fakeClient) ToggleEntryBookmark(ctx context.Context, entryID int) error {
	f.capture(ctx)
	if f.toggleBookmark == nil {
		return errSentinel
	}
	return f.toggleBookmark(ctx, entryID)
}

func (f *fakeClient) UpdateEntry(ctx context.Context, id int, req dmf.UpdateEntryRequest) (*dmf.Entry, error) {
	f.capture(ctx)
	if f.updateEntry == nil {
		return nil, errSentinel
	}
	return f.updateEntry(ctx, id, req)
}

func (f *fakeClient) ImportOPML(ctx context.Context, opml string) error {
	f.capture(ctx)
	if f.importOPML == nil {
		return errSentinel
	}
	return f.importOPML(ctx, opml)
}

func (f *fakeClient) DeleteFeed(ctx context.Context, id int) error {
	f.capture(ctx)
	if f.deleteFeed == nil {
		return errSentinel
	}
	return f.deleteFeed(ctx, id)
}

func (f *fakeClient) DeleteCategory(ctx context.Context, id int) error {
	f.capture(ctx)
	if f.deleteCategory == nil {
		return errSentinel
	}
	return f.deleteCategory(ctx, id)
}

func (f *fakeClient) FlushHistory(ctx context.Context, before *time.Time) error {
	f.capture(ctx)
	if f.flushHistory == nil {
		return errSentinel
	}
	return f.flushHistory(ctx, before)
}

// withToken threads a token into ctx for pass-through verification.
func withToken(ctx context.Context, tok string) context.Context {
	return dmf.WithToken(ctx, tok)
}

func tokCtx(t *testing.T) context.Context {
	t.Helper()
	return withToken(context.Background(), "tok-123")
}
