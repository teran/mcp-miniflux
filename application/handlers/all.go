package handlers

import (
	"github.com/teran/mcp-miniflux/domain/app"
	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
	"github.com/teran/mcp-miniflux/domain/tools"
)

// All returns every tool (definition + use-case handler) in registration order
// read → write/update → delete (SPEC §4, S03). The order must match
// domain/tools.expectedToolOrder. The composition root passes the result to
// application.RegisterTools.
func All(client dmf.Client) []app.Tool {
	return []app.Tool{
		// Read (readOnlyHint:true).
		{Def: &tools.ListFeeds{}, Handler: ListFeedsHandler{Client: client}},
		{Def: &tools.GetFeed{}, Handler: GetFeedHandler{Client: client}},
		{Def: &tools.ListCategories{}, Handler: ListCategoriesHandler{Client: client}},
		{Def: &tools.ListEntries{}, Handler: ListEntriesHandler{Client: client}},
		{Def: &tools.GetEntry{}, Handler: GetEntryHandler{Client: client}},
		{Def: &tools.GetFeedEntries{}, Handler: GetFeedEntriesHandler{Client: client}},
		{Def: &tools.GetCounters{}, Handler: GetCountersHandler{Client: client}},
		{Def: &tools.GetMe{}, Handler: GetMeHandler{Client: client}},
		{Def: &tools.ExportOPML{}, Handler: ExportOPMLHandler{Client: client}},
		{Def: &tools.DiscoverSubscriptions{}, Handler: DiscoverSubscriptionsHandler{Client: client}},
		// Write / update.
		{Def: &tools.CreateFeed{}, Handler: CreateFeedHandler{Client: client}},
		{Def: &tools.UpdateFeed{}, Handler: UpdateFeedHandler{Client: client}},
		{Def: &tools.RefreshFeed{}, Handler: RefreshFeedHandler{Client: client}},
		{Def: &tools.CreateCategory{}, Handler: CreateCategoryHandler{Client: client}},
		{Def: &tools.UpdateCategory{}, Handler: UpdateCategoryHandler{Client: client}},
		{Def: &tools.RefreshCategory{}, Handler: RefreshCategoryHandler{Client: client}},
		{Def: &tools.MarkFeedEntriesRead{}, Handler: MarkFeedEntriesReadHandler{Client: client}},
		{Def: &tools.MarkCategoryEntriesRead{}, Handler: MarkCategoryEntriesReadHandler{Client: client}},
		{Def: &tools.UpdateEntries{}, Handler: UpdateEntriesHandler{Client: client}},
		{Def: &tools.ToggleEntryBookmark{}, Handler: ToggleEntryBookmarkHandler{Client: client}},
		{Def: &tools.UpdateEntry{}, Handler: UpdateEntryHandler{Client: client}},
		{Def: &tools.ImportOPML{}, Handler: ImportOPMLHandler{Client: client}},
		// Delete (destructiveHint:true, HITL S12).
		{Def: &tools.DeleteFeed{}, Handler: DeleteFeedHandler{Client: client}},
		{Def: &tools.DeleteCategory{}, Handler: DeleteCategoryHandler{Client: client}},
		{Def: &tools.FlushHistory{}, Handler: FlushHistoryHandler{Client: client}},
	}
}
