// Package mcp wires the go-sdk server and transports (Streamable HTTP for the
// deployed mode, stdio for local debugging) onto the mcp-miniflux handlers.
//
// The go-sdk is blank-imported here only to pin the exact dependency version
// in go.mod while the package skeleton is empty; real usage lands with the
// transport wiring in a later phase.
package mcp

import _ "github.com/modelcontextprotocol/go-sdk/mcp"
