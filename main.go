package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpserver "github.com/takutakahashi/awesome-mcp-proxy/server"
)

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on (e.g., :8080)")
	flag.Parse()

	// Create MCP server
	mcpServer := mcpserver.NewMCPServer()

	// Create HTTP handler with streamable transport
	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return mcpServer.GetServer()
		},
		nil,
	)

	// Set up HTTP server
	http.Handle("/mcp", handler)

	log.Printf("MCP HTTP Server starting on %s/mcp", *addr)
	log.Printf("Using official MCP Go SDK with Streamable HTTP transport")

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
