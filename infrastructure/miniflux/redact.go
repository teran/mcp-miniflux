package miniflux

import (
	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// Redact returns a deep copy of v in which every struct field annotated
// `secret:"true"` is zeroed (S02/L05). The implementation lives in the domain
// layer (next to the models) so the application layer can use it without an
// application→infrastructure edge; this package re-exports it for callers that
// already depend on infrastructure.
func Redact(v any) any {
	return dmf.Redact(v)
}

// RedactMap returns a deep copy of m in which every key whose lowercased name
// is in the secret-name set is removed (L05). See the domain implementation.
func RedactMap(m map[string]any) map[string]any {
	return dmf.RedactMap(m)
}
