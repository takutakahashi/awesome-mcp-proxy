package gateway

import (
	"context"
	"testing"

	"github.com/takutakahashi/awesome-mcp-proxy/config"
)

func TestNewGateway(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			Host:     "localhost",
			Port:     8080,
			Endpoint: "/mcp",
		},
		Groups: []config.Group{
			{
				Name: "test-group",
				Backends: map[string]config.Backend{
					"test-backend": {
						Name:      "test-backend",
						Transport: "http",
						Endpoint:  "http://localhost:3000/mcp",
					},
				},
			},
		},
	}

	gateway, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	if gateway == nil {
		t.Fatal("Gateway should not be nil")
	}

	// Check that backend was added
	backends := gateway.GetBackendManager().GetAllBackends()
	if len(backends) != 1 {
		t.Errorf("Expected 1 backend, got %d", len(backends))
	}

	// Close gateway
	if err := gateway.Close(); err != nil {
		t.Errorf("Failed to close gateway: %v", err)
	}
}

func TestNewGateway_UnsupportedTransport(t *testing.T) {
	cfg := &config.Config{
		Groups: []config.Group{
			{
				Name: "test-group",
				Backends: map[string]config.Backend{
					"test-backend": {
						Name:      "test-backend",
						Transport: "unsupported",
						Endpoint:  "http://localhost:3000/mcp",
					},
				},
			},
		},
	}

	_, err := NewGateway(cfg)
	if err == nil {
		t.Fatal("Expected error for unsupported transport")
	}
}

func TestGateway_Initialize_NoBackends(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			Host:     "localhost",
			Port:     8080,
			Endpoint: "/mcp",
		},
		Groups: []config.Group{},
	}

	gateway, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	ctx := context.Background()
	err = gateway.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize should succeed with no backends: %v", err)
	}

	// Capabilities should all be false with no backends
	capabilities := gateway.GetCapabilities()
	if capabilities.Tools || capabilities.Resources || capabilities.Prompts {
		t.Error("All capabilities should be false with no backends")
	}

	// Server should be created
	if gateway.GetServer() == nil {
		t.Error("Server should be created even with no backends")
	}
}

func TestGateway_GetCapabilities(t *testing.T) {
	gateway := &Gateway{
		capabilities: GatewayCapabilities{
			Tools:     true,
			Resources: false,
			Prompts:   true,
		},
	}

	caps := gateway.GetCapabilities()
	if !caps.Tools {
		t.Error("Tools capability should be true")
	}
	if caps.Resources {
		t.Error("Resources capability should be false")
	}
	if !caps.Prompts {
		t.Error("Prompts capability should be true")
	}
}

func TestGateway_GetServer(t *testing.T) {
	cfg := &config.Config{
		Groups: []config.Group{},
	}

	gateway, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Server should be nil before initialization
	if gateway.GetServer() != nil {
		t.Error("Server should be nil before initialization")
	}

	ctx := context.Background()
	if err := gateway.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize gateway: %v", err)
	}

	// Server should not be nil after initialization
	if gateway.GetServer() == nil {
		t.Error("Server should not be nil after initialization")
	}
}

func TestGateway_GetBackendManager(t *testing.T) {
	cfg := &config.Config{
		Groups: []config.Group{
			{
				Name: "test-group",
				Backends: map[string]config.Backend{
					"backend1": {
						Name:      "backend1",
						Transport: "http",
						Endpoint:  "http://localhost:3001/mcp",
					},
					"backend2": {
						Name:      "backend2",
						Transport: "http",
						Endpoint:  "http://localhost:3002/mcp",
					},
				},
			},
		},
	}

	gateway, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	manager := gateway.GetBackendManager()
	if manager == nil {
		t.Fatal("Backend manager should not be nil")
	}

	backends := manager.GetAllBackends()
	if len(backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(backends))
	}
}

func TestGateway_GetRoutingTable(t *testing.T) {
	cfg := &config.Config{
		Groups: []config.Group{},
	}

	gateway, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	rt := gateway.GetRoutingTable()
	if rt == nil {
		t.Fatal("Routing table should not be nil")
	}

	// Initially, routing table should be empty
	tools := rt.GetAllTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools in routing table, got %d", len(tools))
	}
}

func TestGateway_MultipleGroups(t *testing.T) {
	cfg := &config.Config{
		Groups: []config.Group{
			{
				Name: "group1",
				Backends: map[string]config.Backend{
					"backend1": {
						Name:      "backend1",
						Transport: "http",
						Endpoint:  "http://localhost:3001/mcp",
					},
				},
			},
			{
				Name: "group2",
				Backends: map[string]config.Backend{
					"backend2": {
						Name:      "backend2",
						Transport: "stdio",
						Command:   "mcp-server",
						Args:      []string{"--test"},
					},
				},
			},
		},
	}

	gateway, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	backends := gateway.GetBackendManager().GetAllBackends()
	if len(backends) != 2 {
		t.Errorf("Expected 2 backends from 2 groups, got %d", len(backends))
	}

	// Check that both transport types are present
	var hasHTTP, hasStdio bool
	for _, backend := range backends {
		info := backend.GetInfo()
		if info.Transport == "http" {
			hasHTTP = true
		} else if info.Transport == "stdio" {
			hasStdio = true
		}
	}

	if !hasHTTP {
		t.Error("Expected HTTP backend in the mix")
	}
	if !hasStdio {
		t.Error("Expected Stdio backend in the mix")
	}
}

func TestGateway_Close(t *testing.T) {
	cfg := &config.Config{
		Groups: []config.Group{
			{
				Name: "test-group",
				Backends: map[string]config.Backend{
					"backend1": {
						Name:      "backend1",
						Transport: "http",
						Endpoint:  "http://localhost:3001/mcp",
					},
				},
			},
		},
	}

	gateway, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Close should not error
	if err := gateway.Close(); err != nil {
		t.Errorf("Close should not return error: %v", err)
	}

	// Multiple closes should be safe
	if err := gateway.Close(); err != nil {
		t.Errorf("Second close should not return error: %v", err)
	}
}