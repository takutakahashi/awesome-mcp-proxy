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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/takutakahashi/awesome-mcp-proxy/server"
)

// gatewayCmd represents the gateway command
var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the MCP proxy in gateway mode",
	Long: `Start the MCP proxy in gateway mode to aggregate multiple backend MCP servers.

This mode runs as a proxy/gateway that:
- Automatically discovers capabilities from configured backend servers
- Routes requests to appropriate backends based on tool/resource/prompt names
- Aggregates responses from multiple backends
- Provides caching and middleware support
- Supports both HTTP and stdio backend transports

Configuration is loaded from a YAML file that defines backend groups and settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		runGatewayMode()
	},
}

func init() {
	rootCmd.AddCommand(gatewayCmd)

	// Local flags for gateway command
	gatewayCmd.Flags().StringP("endpoint", "e", "/mcp", "MCP endpoint path")
	gatewayCmd.Flags().StringP("config-file", "c", "config.yaml", "Gateway configuration file")
	gatewayCmd.Flags().Bool("use-default-config", false, "Use default configuration instead of loading from file")

	// Bind flags to viper
	viper.BindPFlag("gateway.endpoint", gatewayCmd.Flags().Lookup("endpoint"))
	viper.BindPFlag("gateway.config-file", gatewayCmd.Flags().Lookup("config-file"))
	viper.BindPFlag("gateway.use-default-config", gatewayCmd.Flags().Lookup("use-default-config"))
}

func runGatewayMode() {
	addr := viper.GetString("addr")
	endpoint := viper.GetString("gateway.endpoint")
	configFile := viper.GetString("gateway.config-file")
	useDefaultConfig := viper.GetBool("gateway.use-default-config")
	verbose := viper.GetBool("verbose")

	if verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Verbose logging enabled")
	}

	log.Printf("Starting MCP proxy in gateway mode on %s", addr)
	log.Printf("MCP endpoint: %s", endpoint)

	// Load configuration
	var config *server.GatewayConfig
	var err error

	if useDefaultConfig {
		log.Println("Using default gateway configuration")
		config = server.DefaultGatewayConfig()
	} else {
		log.Printf("Loading gateway configuration from: %s", configFile)
		config, err = server.LoadGatewayConfig(configFile)
		if err != nil {
			log.Fatalf("Failed to load gateway config: %v", err)
		}
	}

	// Override config with command line flags
	config.Gateway.Host = extractHost(addr)
	config.Gateway.Port = extractPort(addr)
	config.Gateway.Endpoint = endpoint

	log.Printf("Gateway configuration loaded: %d groups, %d total backends",
		len(config.Groups), getTotalBackends(config))

	// Create gateway
	gateway := server.NewGateway(config)

	// Initialize gateway and discover backend capabilities
	log.Println("Initializing gateway and discovering backend capabilities...")
	if err := gateway.Initialize(); err != nil {
		log.Fatalf("Failed to initialize gateway: %v", err)
	}

	// Set up HTTP server
	mux := http.NewServeMux()

	// Gateway MCP endpoint
	mux.HandleFunc(endpoint, gateway.HandleMCPRequest)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"status": "healthy",
			"mode": "gateway",
			"version": "0.1.0",
			"timestamp": "%s",
			"backends": %d
		}`, time.Now().UTC().Format(time.RFC3339), getTotalBackends(config))
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Gateway server failed to start: %v", err)
		}
	}()

	log.Printf("Gateway server started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gateway...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Close gateway connections
	if err := gateway.Close(); err != nil {
		log.Printf("Error closing gateway: %v", err)
	}

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Gateway forced to shutdown: %v", err)
	} else {
		log.Println("Gateway shutdown complete")
	}
}

// Helper functions
func extractHost(addr string) string {
	if addr[0] == ':' {
		return "0.0.0.0"
	}
	// Simple extraction - for production use proper URL parsing
	return "0.0.0.0"
}

func extractPort(addr string) int {
	if addr == ":8080" {
		return 8080
	}
	// For simplicity, return default - for production use proper parsing
	return 8080
}

func getTotalBackends(config *server.GatewayConfig) int {
	total := 0
	for _, group := range config.Groups {
		total += len(group.Backends)
	}
	return total
}