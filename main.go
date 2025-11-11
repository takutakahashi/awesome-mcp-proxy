package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/takutakahashi/awesome-mcp-proxy/config"
	"github.com/takutakahashi/awesome-mcp-proxy/gateway"
	mcpserver "github.com/takutakahashi/awesome-mcp-proxy/server"
)

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on (e.g., :8080)")
	configPath := flag.String("config", "", "Path to gateway configuration file")
	flag.Parse()

	// Check if gateway mode is requested
	if *configPath != "" {
		runGateway(*addr, *configPath)
	} else {
		runStandaloneServer(*addr)
	}
}

func runGateway(addr, configPath string) {
	log.Printf("Starting MCP Gateway with config: %s", configPath)

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Use configured host and port if provided
	if cfg.Gateway.Host != "" && cfg.Gateway.Port != 0 {
		addr = fmt.Sprintf("%s:%d", cfg.Gateway.Host, cfg.Gateway.Port)
	}

	// Create gateway
	gatewayServer, err := gateway.NewGateway(cfg)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}
	defer func() { _ = gatewayServer.Close() }()

	// Initialize gateway
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := gatewayServer.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize gateway: %v", err)
	}

	// Create HTTP handlers
	streamHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return gatewayServer.GetServer()
		},
		nil,
	)

	sseHandler := mcp.NewSSEHandler(
		func(r *http.Request) *mcp.Server {
			return gatewayServer.GetServer()
		},
		nil,
	)

	// Set up HTTP server
	http.Handle("/mcp", streamHandler)
	http.Handle("/sse", sseHandler)

	log.Printf("MCP Gateway starting on %s/mcp", addr)
	log.Printf("Capabilities: %+v", gatewayServer.GetCapabilities())

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runStandaloneServer(addr string) {
	log.Println("Starting standalone MCP Server")

	// Create MCP server
	mcpServer := mcpserver.NewMCPServer()

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

	log.Printf("MCP HTTP Server starting on %s/mcp", addr)
	log.Printf("Using official MCP Go SDK with Streamable HTTP transport")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
