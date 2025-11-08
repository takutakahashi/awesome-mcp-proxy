package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/takutakahashi/awesome-mcp-proxy/server"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server in standalone mode",
	Long: `Start the MCP server in standalone mode with built-in tools, resources, and prompts.

This mode runs a single MCP server instance that provides:
- Echo and Add tools for demonstration
- Server information and health status resources  
- Greeting prompts

The server supports both HTTP streaming and SSE endpoints for MCP communication.`,
	Run: func(cmd *cobra.Command, args []string) {
		runStandaloneMode()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Local flags for serve command
	serveCmd.Flags().StringP("endpoint", "e", "/mcp", "MCP endpoint path")
	serveCmd.Flags().StringP("sse-endpoint", "s", "/sse", "SSE endpoint path")

	// Bind flags to viper
	viper.BindPFlag("serve.endpoint", serveCmd.Flags().Lookup("endpoint"))
	viper.BindPFlag("serve.sse-endpoint", serveCmd.Flags().Lookup("sse-endpoint"))
}

func runStandaloneMode() {
	addr := viper.GetString("addr")
	endpoint := viper.GetString("serve.endpoint")
	sseEndpoint := viper.GetString("serve.sse-endpoint")
	verbose := viper.GetBool("verbose")

	if verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Verbose logging enabled")
	}

	log.Printf("Starting MCP server in standalone mode on %s", addr)
	log.Printf("MCP endpoint: %s", endpoint)
	log.Printf("SSE endpoint: %s", sseEndpoint)

	// Create MCP server
	mcpServer := server.NewMCPServer()

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
	mux := http.NewServeMux()
	mux.Handle(endpoint, streamHandler)
	mux.Handle(sseEndpoint, sseHandler)

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"status": "healthy",
			"mode": "standalone",
			"version": "0.1.0",
			"timestamp": "%s"
		}`, time.Now().UTC().Format(time.RFC3339))
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	log.Printf("Server started successfully")
	log.Printf("Using official MCP Go SDK with Streamable HTTP transport")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("Server shutdown complete")
	}
}