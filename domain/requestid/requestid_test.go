package requestid

import (
	"context"
	"testing"
)

func TestWithRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")
	if got := RequestIDFromContext(ctx); got != "req-123" {
		t.Fatalf("RequestIDFromContext = %q, want %q", got, "req-123")
	}
}

func TestRequestIDFromContextEmptyWhenAbsent(t *testing.T) {
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("RequestIDFromContext(empty) = %q, want empty", got)
	}
}

func TestRequestIDFromContextNil(t *testing.T) {
	if got := RequestIDFromContext(nil); got != "" {
		t.Fatalf("RequestIDFromContext(nil) = %q, want empty", got)
	}
}

func TestRequestIDFromContextWrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey{}, 42)
	if got := RequestIDFromContext(ctx); got != "" {
		t.Fatalf("RequestIDFromContext(wrong type) = %q, want empty", got)
	}
}

func TestWithRequestIDDoesNotClobberParentValue(t *testing.T) {
	parent := WithRequestID(context.Background(), "parent")
	child := WithRequestID(parent, "child")

	if got := RequestIDFromContext(child); got != "child" {
		t.Fatalf("child = %q, want %q", got, "child")
	}
	if got := RequestIDFromContext(parent); got != "parent" {
		t.Fatalf("parent = %q, want %q", got, "parent")
	}
}
