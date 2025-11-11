package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/takutakahashi/awesome-mcp-proxy/config"
)

// MockHTTPServer creates a test HTTP server for backend testing
func MockHTTPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		method, _ := request["method"].(string)

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      request["id"],
		}

		switch method {
		case "initialize":
			response["result"] = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			}
		case "tools/list":
			response["result"] = map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "test_tool",
						"description": "A test tool",
					},
				},
			}
		case "tools/call":
			params, _ := request["params"].(map[string]interface{})
			toolName, _ := params["name"].(string)
			response["result"] = map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Tool " + toolName + " executed",
					},
				},
			}
		default:
			response["error"] = map[string]interface{}{
				"code":    -32601,
				"message": "Method not found",
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func TestHTTPBackend_Initialize(t *testing.T) {
	server := MockHTTPServer(t)
	defer server.Close()

	backend := NewHTTPBackend(config.Backend{
		Name:      "test-backend",
		Transport: "http",
		Endpoint:  server.URL,
		Headers:   map[string]string{"X-Test": "test-header"},
	}, "test-group")

	ctx := context.Background()
	initReq := struct {
		ProtocolVersion string                 `json:"protocolVersion"`
		Capabilities    map[string]interface{} `json:"capabilities"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]interface{}{},
		ClientInfo: struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	result, err := backend.Initialize(ctx, initReq)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if result == nil {
		t.Fatal("Initialize result is nil")
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version 2024-11-05, got %s", result.ProtocolVersion)
	}

	if !backend.IsHealthy() {
		t.Error("Backend should be healthy after successful initialization")
	}
}

func TestHTTPBackend_SendRequest(t *testing.T) {
	server := MockHTTPServer(t)
	defer server.Close()

	backend := NewHTTPBackend(config.Backend{
		Name:      "test-backend",
		Transport: "http",
		Endpoint:  server.URL,
	}, "test-group")

	ctx := context.Background()
	response, err := backend.SendRequest(ctx, "tools/list", struct{}{})
	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(*response, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	tools, ok := result["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Errorf("Expected 1 tool in response, got %v", result)
	}
}

func TestHTTPBackend_GetInfo(t *testing.T) {
	backend := NewHTTPBackend(config.Backend{
		Name:      "test-backend",
		Transport: "http",
		Endpoint:  "http://localhost:8080",
	}, "test-group")

	info := backend.GetInfo()
	if info.Name != "test-backend" {
		t.Errorf("Expected name test-backend, got %s", info.Name)
	}
	if info.Transport != "http" {
		t.Errorf("Expected transport http, got %s", info.Transport)
	}
	if info.Group != "test-group" {
		t.Errorf("Expected group test-group, got %s", info.Group)
	}
}

func TestHTTPBackend_HealthCheck(t *testing.T) {
	server := MockHTTPServer(t)
	defer server.Close()

	backend := NewHTTPBackend(config.Backend{
		Name:      "test-backend",
		Transport: "http",
		Endpoint:  server.URL,
	}, "test-group")

	// Initially healthy
	if !backend.IsHealthy() {
		t.Error("Backend should be initially healthy")
	}

	// Make a failed request (stop server)
	server.Close()
	ctx := context.Background()
	_, _ = backend.SendRequest(ctx, "test", struct{}{})

	// Should be unhealthy after failed request
	if backend.IsHealthy() {
		t.Error("Backend should be unhealthy after failed request")
	}
}

func TestBackendManager_AddAndGetBackend(t *testing.T) {
	manager := NewBackendManager()

	backend := NewHTTPBackend(config.Backend{
		Name:      "test-backend",
		Transport: "http",
		Endpoint:  "http://localhost:8080",
	}, "test-group")

	manager.AddBackend(backend)

	retrieved, exists := manager.GetBackend("test-backend")
	if !exists {
		t.Fatal("Backend should exist after adding")
	}

	if retrieved.GetInfo().Name != "test-backend" {
		t.Error("Retrieved backend has wrong name")
	}

	_, exists = manager.GetBackend("non-existent")
	if exists {
		t.Error("Non-existent backend should not exist")
	}
}

func TestBackendManager_GetAllBackends(t *testing.T) {
	manager := NewBackendManager()

	backend1 := NewHTTPBackend(config.Backend{
		Name: "backend1",
	}, "group1")
	backend2 := NewHTTPBackend(config.Backend{
		Name: "backend2",
	}, "group2")

	manager.AddBackend(backend1)
	manager.AddBackend(backend2)

	backends := manager.GetAllBackends()
	if len(backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(backends))
	}
}

func TestBackendManager_GetHealthyBackends(t *testing.T) {
	manager := NewBackendManager()

	// Create a healthy backend
	server := MockHTTPServer(t)
	defer server.Close()

	healthyBackend := NewHTTPBackend(config.Backend{
		Name:      "healthy",
		Transport: "http",
		Endpoint:  server.URL,
	}, "test-group")

	// Create an unhealthy backend (invalid endpoint)
	unhealthyBackend := NewHTTPBackend(config.Backend{
		Name:      "unhealthy",
		Transport: "http",
		Endpoint:  "http://invalid:99999",
	}, "test-group")

	// Make unhealthy backend actually unhealthy
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _ = unhealthyBackend.SendRequest(ctx, "test", struct{}{})

	manager.AddBackend(healthyBackend)
	manager.AddBackend(unhealthyBackend)

	healthyBackends := manager.GetHealthyBackends()
	if len(healthyBackends) != 1 {
		t.Errorf("Expected 1 healthy backend, got %d", len(healthyBackends))
	}

	if len(healthyBackends) > 0 && healthyBackends[0].GetInfo().Name != "healthy" {
		t.Error("Wrong backend marked as healthy")
	}
}

func TestBackendManager_Close(t *testing.T) {
	manager := NewBackendManager()

	backend := NewHTTPBackend(config.Backend{
		Name: "test-backend",
	}, "test-group")

	manager.AddBackend(backend)

	err := manager.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

// Test InitializeResult structure compatibility
func TestInitializeResult_MCPCompatibility(t *testing.T) {
	// This test ensures our InitializeResult can be properly unmarshaled
	resultJSON := `{
		"protocolVersion": "2024-11-05",
		"capabilities": {
			"tools": {},
			"resources": {},
			"prompts": {}
		},
		"serverInfo": {
			"name": "test-server",
			"version": "1.0.0"
		}
	}`

	var result mcp.InitializeResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		t.Fatalf("Failed to unmarshal InitializeResult: %v", err)
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version 2024-11-05, got %s", result.ProtocolVersion)
	}

	if result.ServerInfo == nil || result.ServerInfo.Name != "test-server" {
		t.Error("ServerInfo not properly unmarshaled")
	}
}