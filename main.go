package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpserver "github.com/takutakahashi/awesome-mcp-proxy/server"
)

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on (e.g., :8080)")
	configFile := flag.String("config", "", "Path to gateway configuration file (YAML)")
	gatewayMode := flag.Bool("gateway", false, "Run in gateway/proxy mode")
	flag.Parse()

	if *gatewayMode {
		runGatewayMode(*addr, *configFile)
	} else {
		runStandaloneMode(*addr)
	}
}

func runGatewayMode(addr, configFile string) {
	log.Println("Starting MCP Gateway/Proxy mode")

	var config *mcpserver.GatewayConfig
	var err error

	if configFile != "" {
		// Load configuration from file
		config, err = mcpserver.LoadGatewayConfig(configFile)
		if err != nil {
			log.Fatalf("Failed to load config file %s: %v", configFile, err)
		}
		log.Printf("Loaded configuration from %s", configFile)
	} else {
		// Use default configuration
		config = mcpserver.DefaultGatewayConfig()
		log.Println("Using default configuration")
	}

	// Create and initialize gateway
	gateway := mcpserver.NewGateway(config)
	if err := gateway.Initialize(); err != nil {
		log.Fatalf("Failed to initialize gateway: %v", err)
	}

	// Set up graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down gateway...")
		gateway.Close()
		os.Exit(0)
	}()

	// Set up HTTP server
	http.HandleFunc(config.Gateway.Endpoint, gateway.HandleMCPRequest)
	
	// Add health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","mode":"gateway"}`)
	})

	serverAddr := fmt.Sprintf("%s:%d", config.Gateway.Host, config.Gateway.Port)
	log.Printf("MCP Gateway starting on %s%s", serverAddr, config.Gateway.Endpoint)
	log.Printf("Health check available at %s/health", serverAddr)

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runStandaloneMode(addr string) {
	log.Println("Starting MCP Standalone mode")

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
	
	// Add health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","mode":"standalone"}`)
	})

	log.Printf("MCP HTTP Server starting on %s/mcp", addr)
	log.Printf("SSE endpoint available at %s/sse", addr)
	log.Printf("Health check available at %s/health", addr)
	log.Printf("Using official MCP Go SDK with Streamable HTTP transport")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}