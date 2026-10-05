// Package application contains the use-case layer: one handler per tool and
// the tool registry that declares and registers tools on the go-sdk server.
package application

import (
	"github.com/teran/mcp-miniflux/domain/app"
	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// Deps bundles the dependencies the tool registry needs (SPEC §6.2). The
// composition root injects the concrete Miniflux client (infrastructure), a
// logger and the ordered tool list, so the registry never constructs
// infrastructure itself (DIP) and stays free of an application→handlers edge.
type Deps struct {
	// Client is the Miniflux port implementation.
	Client dmf.Client
	// Logger emits the access log; nil disables logging.
	Logger app.ToolLogger
	// Tools is the ordered (read → write/update → delete, S03) tool list.
	Tools []app.Tool
}
