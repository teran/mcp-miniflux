package handlers

import (
	"testing"
)

// expectedOrder mirrors domain/tools.expectedToolOrder (S03): read →
// write/update → delete.
var expectedOrder = []string{
	"list_feeds", "get_feed", "list_categories", "list_entries", "get_entry",
	"get_feed_entries", "get_counters", "get_me", "export_opml",
	"discover_subscriptions",
	"create_feed", "update_feed", "refresh_feed", "create_category",
	"update_category", "refresh_category", "mark_feed_entries_read",
	"mark_category_entries_read", "update_entries", "toggle_entry_bookmark",
	"update_entry", "import_opml",
	"delete_feed", "delete_category", "flush_history",
}

func TestAllOrderAndCount(t *testing.T) {
	tools := All(&fakeClient{})
	if len(tools) != len(expectedOrder) {
		t.Fatalf("All() returned %d tools, want %d", len(tools), len(expectedOrder))
	}
	for i, tl := range tools {
		if tl.Def.Name() != expectedOrder[i] {
			t.Errorf("tool[%d].Name() = %q, want %q (S03 order)", i, tl.Def.Name(), expectedOrder[i])
		}
		if tl.Handler == nil {
			t.Errorf("%s: handler must be wired", tl.Def.Name())
		}
		if tl.Def.InputSchema() == nil {
			t.Errorf("%s: input schema must be non-nil", tl.Def.Name())
		}
	}
}

func TestAllAnnotationsAndInstructions(t *testing.T) {
	for _, tl := range All(&fakeClient{}) {
		name := tl.Def.Name()
		a := tl.Def.Annotations()
		if a.Title == "" {
			t.Errorf("%s: annotations title empty", name)
		}
		if tl.Def.Instructions() == "" {
			t.Errorf("%s: instructions empty (M04/M05)", name)
		}
		switch name {
		case "list_feeds", "get_feed", "list_categories", "list_entries", "get_entry",
			"get_feed_entries", "get_counters", "get_me", "export_opml",
			"discover_subscriptions":
			if !a.ReadOnlyHint {
				t.Errorf("%s: expected readOnlyHint", name)
			}
		case "delete_feed", "delete_category", "flush_history":
			if a.DestructiveHint == nil || !*a.DestructiveHint {
				t.Errorf("%s: expected destructiveHint (S12)", name)
			}
		}
	}
}
