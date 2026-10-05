package handlers

import (
	"testing"
)

func TestAsIntFractional(t *testing.T) {
	if _, err := asInt("feed_id", 1.5); err == nil {
		t.Error("expected InvalidParams for fractional integer")
	}
	if _, err := asInt("feed_id", "x"); err == nil {
		t.Error("expected InvalidParams for non-number")
	}
	if v, err := asInt("feed_id", 42.0); err != nil || v != 42 {
		t.Errorf("asInt(42.0) = %d, %v", v, err)
	}
}

func TestStringArgWrongType(t *testing.T) {
	if _, err := stringArg("title", map[string]any{"title": 123}); err == nil {
		t.Error("expected InvalidParams for non-string arg")
	}
	if v, err := stringArg("title", map[string]any{}); err != nil || v != "" {
		t.Errorf("stringArg absent = %q, %v", v, err)
	}
}

func TestBoolArgWrongType(t *testing.T) {
	if _, err := boolArg("starred", map[string]any{"starred": "yes"}); err == nil {
		t.Error("expected InvalidParams for non-boolean arg")
	}
	if v, err := boolArg("starred", map[string]any{}); err != nil || v != nil {
		t.Errorf("boolArg absent = %v, %v", v, err)
	}
}

func TestTimeArgError(t *testing.T) {
	if _, err := timeArg("before", map[string]any{"before": "not-a-time"}); err == nil {
		t.Error("expected InvalidParams for malformed timestamp")
	}
	if v, err := timeArg("before", map[string]any{}); err != nil || v != nil {
		t.Errorf("timeArg absent = %v, %v", v, err)
	}
}

func TestEntryFilterBranches(t *testing.T) {
	f, err := entryFilter(map[string]any{
		"status":      "read",
		"order":       "published_at",
		"direction":   "desc",
		"search":      "go",
		"category_id": float64(4),
		"before":      "2024-01-01T00:00:00Z",
		"after":       "2023-01-01T00:00:00Z",
		"offset":      float64(10),
		"starred":     true,
	}, true)
	if err != nil {
		t.Fatalf("entryFilter error: %v", err)
	}
	if f.Status != "read" || f.Order != "published_at" || f.Direction != "desc" || f.Search != "go" ||
		f.Offset != 10 || f.CategoryID == nil || *f.CategoryID != 4 || f.Before == nil || f.After == nil ||
		f.Starred == nil || !*f.Starred {
		t.Errorf("entryFilter not fully populated: %+v", f)
	}
}

func TestEntryFilterBadCategory(t *testing.T) {
	if _, err := entryFilter(map[string]any{"category_id": "x"}, true); err == nil {
		t.Error("expected InvalidParams for bad category_id")
	}
}

func TestEntryFilterBadBefore(t *testing.T) {
	if _, err := entryFilter(map[string]any{"before": "nope"}, false); err == nil {
		t.Error("expected InvalidParams for bad before")
	}
}

func TestJSONResultMarshalError(t *testing.T) {
	if _, err := jsonResult(map[string]any{"ch": make(chan int)}); err == nil {
		t.Error("expected marshal error for channel value")
	}
}

func TestAbsoluteHTTPURLEdgeCases(t *testing.T) {
	if err := absoluteHTTPURL("http://"); err == nil {
		t.Error("expected error for empty host")
	}
	if err := absoluteHTTPURL("http://[::1"); err == nil {
		t.Error("expected error for malformed URL")
	}
	if err := absoluteHTTPURL("https://example.com/x"); err != nil {
		t.Errorf("unexpected error for valid URL: %v", err)
	}
}

func TestInvalidParamsBuildsError(t *testing.T) {
	err := invalidParams("%s required", "feed_id")
	if err == nil || err.Error() != "feed_id required" {
		t.Errorf("invalidParams = %v", err)
	}
}

func TestRequireConfirmRefusesNonBool(t *testing.T) {
	if err := requireConfirm(map[string]any{"confirm": "yes"}); err == nil {
		t.Error("expected refusal for non-true confirm")
	}
}

func TestOKResult(t *testing.T) {
	res := okResult()
	if len(res.Content) == 0 {
		t.Error("okResult should have content")
	}
}
