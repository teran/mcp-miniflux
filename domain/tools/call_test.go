package tools

import (
	"context"
	"testing"
)

// TestCallStubsReturnNotImplemented exercises every tool's Call method. In the
// domain layer Call is a stub (execution is wired in the application layer);
// this test locks the stub behaviour so the contract stays type-correct and the
// zero-value tools are callable.
func TestCallStubsReturnNotImplemented(t *testing.T) {
	ctx := context.Background()
	for _, h := range allTools() {
		h := h
		t.Run(h.Name(), func(t *testing.T) {
			res, err := h.Call(ctx, map[string]any{})
			if res != nil {
				t.Errorf("%s: Call returned a non-nil result from the stub: %v", h.Name(), res)
			}
			if err != errNotImplemented {
				t.Errorf("%s: Call error = %v, want errNotImplemented", h.Name(), err)
			}
		})
	}
}
