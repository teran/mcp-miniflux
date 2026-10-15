package requestid

import (
	"context"
	"testing"
)

func TestWithRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")
	if got := FromContext(ctx); got != "req-123" {
		t.Fatalf("FromContext = %q, want %q", got, "req-123")
	}
}

func TestRequestIDFromContextEmptyWhenAbsent(t *testing.T) {
	if got := FromContext(context.Background()); got != "" {
		t.Fatalf("FromContext(empty) = %q, want empty", got)
	}
}

func TestRequestIDFromContextNil(t *testing.T) {
	if got := FromContext(nil); got != "" {
		t.Fatalf("FromContext(nil) = %q, want empty", got)
	}
}

func TestRequestIDFromContextWrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey{}, 42)
	if got := FromContext(ctx); got != "" {
		t.Fatalf("FromContext(wrong type) = %q, want empty", got)
	}
}

func TestWithRequestIDDoesNotClobberParentValue(t *testing.T) {
	parent := WithRequestID(context.Background(), "parent")
	child := WithRequestID(parent, "child")

	if got := FromContext(child); got != "child" {
		t.Fatalf("child = %q, want %q", got, "child")
	}
	if got := FromContext(parent); got != "parent" {
		t.Fatalf("parent = %q, want %q", got, "parent")
	}
}
