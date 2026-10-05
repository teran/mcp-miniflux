package miniflux

// ErrorKind classifies upstream failures into the SPEC §6.5 taxonomy. It is
// declared in the domain layer (with the models) so the application layer can
// map upstream errors to MCP/JSON-RPC outcomes without an
// application→infrastructure dependency edge.
type ErrorKind int

const (
	// ErrorNotFound maps to an upstream 404.
	ErrorNotFound ErrorKind = iota
	// ErrorAuth maps to an upstream 401/403.
	ErrorAuth
	// ErrorTransient maps to upstream 429/5xx, network errors or timeouts.
	ErrorTransient
)

// KindError is implemented by infrastructure errors (e.g. the Miniflux
// client's APIError) to expose the §6.5 taxonomy and upstream status code.
// Its Message must never contain secrets (L05).
type KindError interface {
	error
	ErrorKind() ErrorKind
	StatusCode() int
}
