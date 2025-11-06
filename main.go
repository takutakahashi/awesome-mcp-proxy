package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"
	mcpserver "github.com/takutakahashi/awesome-mcp-proxy/server"
)

func main() {
	port := flag.String("port", ":8080", "Port to listen on (e.g., :8080)")
	flag.Parse()

	// Create MCP server
	mcpServer := mcpserver.NewMCPServer()

	// Create HTTP server with streamable transport
	httpServer := server.NewStreamableHTTPServer(mcpServer.GetServer())

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	log.Printf("MCP HTTP Server starting on %s/mcp", *port)
	if err := httpServer.Start(*port); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}
