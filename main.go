package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/takutakahashi/awesome-mcp-proxy/config"
	mcpserver "github.com/takutakahashi/awesome-mcp-proxy/server"
)

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on (e.g., :8080)")
	configFile := flag.String("config", "", "Path to config file (optional)")
	flag.Parse()

	// Load config if provided
	var cfg *config.Config
	var err error
	if *configFile != "" {
		cfg, err = config.LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		log.Printf("Loaded config from %s", *configFile)
	} else {
		// Check for default config file
		defaultConfig := "config.yaml"
		if _, err := os.Stat(defaultConfig); err == nil {
			cfg, err = config.LoadConfig(defaultConfig)
			if err != nil {
				log.Printf("Warning: Failed to load default config: %v", err)
			} else {
				log.Printf("Loaded config from %s", defaultConfig)
			}
		}
	}

	// Create MCP server
	mcpServer := mcpserver.NewMCPServer(cfg)

	// Create HTTP handler with streamable transport
	streamHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return mcpServer.GetServer()
		},
		nil,
	)

	// Create SSE handler for testing and compatibility
	sseHandler := mcp.NewSSEHandler(
		func(r *http.Request) *mcp.Server {
			return mcpServer.GetServer()
		},
		nil,
	)

	// Set up HTTP server
	http.Handle("/mcp", streamHandler)
	http.Handle("/sse", sseHandler)

	log.Printf("MCP HTTP Server starting on %s/mcp", *addr)
	log.Printf("Using official MCP Go SDK with Streamable HTTP transport")
	if cfg != nil {
		log.Printf("Gateway mode enabled with %d remote server(s)", len(cfg.Gateways))
	}

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
