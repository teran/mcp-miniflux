// Command mcp-miniflux is a stateless Remote (HTTP) MCP server that exposes
// the Miniflux RSS reader API as strongly-typed MCP tools.
//
// This is the composition root: in later phases it wires config -> logger ->
// layers -> transports. For now it is a thin stub that compiles so the build
// and lint gates pass on the package skeleton.
package main

import "fmt"

func main() {
	fmt.Println("mcp-miniflux: composition root not yet wired")
}
