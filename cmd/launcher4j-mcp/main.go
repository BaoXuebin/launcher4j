// Launcher4j MCP Server
//
// A Model Context Protocol (MCP) server that exposes Java/Spring Boot project
// management operations as AI-callable tools. Communicates via JSON-RPC over
// standard input/output.
//
// Usage:
//
//	launcher4j-mcp
//
// Add to a harness agent's MCP configuration to enable AI-driven project control.
package main

import (
	"fmt"
	"os"

	"github.com/baoxuebin/launcher4j/internal/mcp"
)

func main() {
	handler := mcp.NewHandler()
	server := mcp.NewServer(handler)

	fmt.Fprintf(os.Stderr, "Launcher4j MCP Server v0.2.0 starting...\n")

	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
