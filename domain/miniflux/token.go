package miniflux

import "context"

// tokenCtxKey is the context key for the per-call inbound X-Auth-Token.
//
// It lives in the domain layer (mirroring domain/requestid) so both the
// application layer (which threads the inbound token before invoking a use
// case) and the infrastructure client (which reads it back) can share the same
// key without an application→infrastructure dependency edge (SPEC §3.1, §6.1).
type tokenCtxKey struct{}

// WithToken carries the inbound X-Auth-Token through ctx so per-call methods
// honour the pass-through (SPEC §3.1). Mirrors domain/requestid.WithRequestID.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenCtxKey{}, token)
}

// TokenFromContext reads the inbound X-Auth-Token stored by WithToken,
// returning "" when absent.
func TokenFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(tokenCtxKey{}).(string)
	return v
}
