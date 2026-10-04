// Package requestid provides the canonical request_id context helpers (L09).
//
// Both the logging pipeline and the Miniflux client depend on this package so
// the correlation id can be threaded through context and propagated outbound
// as X-Request-ID without an infrastructure -> infrastructure edge (SPEC §6.1,
// C07GO). The context key is unexported to this package so it cannot be
// guessed or clobbered from outside.
package requestid

import "context"

// ctxKey is the unexported context key for the request_id (L09/L04GO).
type ctxKey struct{}

// WithRequestID stores the request id in ctx (L09).
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestIDFromContext retrieves the request id from ctx, returning "" when
// absent (L09).
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}
