package handlers

import (
	"context"
	"testing"
	"time"
)

func TestDeleteFeedHandlerConfirm(t *testing.T) {
	var gotID int
	var called bool
	c := &fakeClient{deleteFeed: func(_ context.Context, id int) error {
		called = true
		gotID = id
		return nil
	}}
	h := DeleteFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(3), "confirm": true})
	okRes(t, res, err)
	if !called || gotID != 3 {
		t.Errorf("deleteFeed called=%v id=%d", called, gotID)
	}
}

func TestDeleteFeedHandlerRefusesWithoutConfirm(t *testing.T) {
	c := &fakeClient{deleteFeed: func(context.Context, int) error {
		t.Error("DeleteFeed must not run without confirm (S12)")
		return nil
	}}
	h := DeleteFeedHandler{Client: c}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": float64(3)}); err == nil {
		t.Error("expected refusal (InvalidParams) without confirm")
	}
}

func TestDeleteFeedHandlerBadID(t *testing.T) {
	h := DeleteFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": "x", "confirm": true}); err == nil {
		t.Error("expected InvalidParams for bad feed_id")
	}
}

func TestDeleteFeedHandlerUpstream(t *testing.T) {
	c := &fakeClient{deleteFeed: func(context.Context, int) error { return errSentinel }}
	h := DeleteFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1), "confirm": true})
	if err != nil {
		t.Fatalf("want IsError, got %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestDeleteCategoryHandlerConfirm(t *testing.T) {
	var gotID int
	var called bool
	c := &fakeClient{deleteCategory: func(_ context.Context, id int) error {
		called = true
		gotID = id
		return nil
	}}
	h := DeleteCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"category_id": float64(2), "confirm": true})
	okRes(t, res, err)
	if !called || gotID != 2 {
		t.Errorf("deleteCategory called=%v id=%d", called, gotID)
	}
}

func TestDeleteCategoryHandlerRefusesWithoutConfirm(t *testing.T) {
	c := &fakeClient{deleteCategory: func(context.Context, int) error {
		t.Error("DeleteCategory must not run without confirm (S12)")
		return nil
	}}
	h := DeleteCategoryHandler{Client: c}
	if _, err := h.Call(context.Background(), map[string]any{"category_id": float64(2)}); err == nil {
		t.Error("expected refusal without confirm")
	}
}

func TestDeleteCategoryHandlerUpstream(t *testing.T) {
	c := &fakeClient{deleteCategory: func(context.Context, int) error { return errSentinel }}
	h := DeleteCategoryHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"category_id": float64(1), "confirm": true})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestFlushHistoryHandlerConfirm(t *testing.T) {
	var gotBefore *time.Time
	var called bool
	c := &fakeClient{flushHistory: func(_ context.Context, before *time.Time) error {
		called = true
		gotBefore = before
		return nil
	}}
	h := FlushHistoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"before": "2024-01-01T00:00:00Z", "confirm": true})
	okRes(t, res, err)
	if !called || gotBefore == nil {
		t.Errorf("flushHistory called=%v before=%v", called, gotBefore)
	}
}

func TestFlushHistoryHandlerNilBefore(t *testing.T) {
	var gotBefore *time.Time
	c := &fakeClient{flushHistory: func(_ context.Context, before *time.Time) error {
		gotBefore = before
		return nil
	}}
	h := FlushHistoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"confirm": true})
	okRes(t, res, err)
	if gotBefore != nil {
		t.Errorf("before = %v, want nil", gotBefore)
	}
}

func TestFlushHistoryHandlerRefusesWithoutConfirm(t *testing.T) {
	c := &fakeClient{flushHistory: func(context.Context, *time.Time) error {
		t.Error("FlushHistory must not run without confirm (S12)")
		return nil
	}}
	h := FlushHistoryHandler{Client: c}
	if _, err := h.Call(context.Background(), map[string]any{}); err == nil {
		t.Error("expected refusal without confirm")
	}
}

func TestFlushHistoryHandlerBadBefore(t *testing.T) {
	h := FlushHistoryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"before": "not-a-time", "confirm": true}); err == nil {
		t.Error("expected InvalidParams for malformed before")
	}
}

func TestFlushHistoryHandlerUpstream(t *testing.T) {
	c := &fakeClient{flushHistory: func(context.Context, *time.Time) error { return errSentinel }}
	h := FlushHistoryHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"confirm": true})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}
