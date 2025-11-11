package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// MockBackend implements Backend interface for testing
type MockBackend struct {
	name       string
	group      string
	healthy    bool
	tools      []string
	resources  []string
	prompts    []string
	shouldFail bool
}

func (m *MockBackend) Initialize(ctx context.Context, req interface{}) (*MockInitializeResult, error) {
	if m.shouldFail {
		return nil, errTest
	}

	capabilities := make(map[string]interface{})
	if len(m.tools) > 0 {
		capabilities["tools"] = map[string]interface{}{}
	}
	if len(m.resources) > 0 {
		capabilities["resources"] = map[string]interface{}{}
	}
	if len(m.prompts) > 0 {
		capabilities["prompts"] = map[string]interface{}{}
	}

	return &MockInitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    capabilities,
	}, nil
}

func (m *MockBackend) SendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	if m.shouldFail {
		return nil, errTest
	}

	var response interface{}

	switch method {
	case "tools/list":
		tools := make([]map[string]string, len(m.tools))
		for i, tool := range m.tools {
			tools[i] = map[string]string{"name": tool, "description": "Mock tool"}
		}
		response = map[string]interface{}{"tools": tools}
	case "resources/list":
		resources := make([]map[string]string, len(m.resources))
		for i, resource := range m.resources {
			resources[i] = map[string]string{"uri": resource, "name": "Mock resource"}
		}
		response = map[string]interface{}{"resources": resources}
	case "prompts/list":
		prompts := make([]map[string]string, len(m.prompts))
		for i, prompt := range m.prompts {
			prompts[i] = map[string]string{"name": prompt, "description": "Mock prompt"}
		}
		response = map[string]interface{}{"prompts": prompts}
	default:
		response = map[string]interface{}{}
	}

	data, _ := json.Marshal(response)
	raw := json.RawMessage(data)
	return &raw, nil
}

func (m *MockBackend) GetInfo() BackendInfo {
	return BackendInfo{
		Name:      m.name,
		Transport: "mock",
		Group:     m.group,
	}
}

func (m *MockBackend) Close() error {
	return nil
}

func (m *MockBackend) IsHealthy() bool {
	return m.healthy
}

// MockInitializeResult for testing
type MockInitializeResult struct {
	ProtocolVersion string
	Capabilities    map[string]interface{}
}

var errTest = fmt.Errorf("test error")

func TestRoutingTable_FindToolBackend(t *testing.T) {
	rt := NewRoutingTable()
	rt.ToolsMap["test_tool"] = "backend1"
	rt.ToolsMap["another_tool"] = "backend2"

	backend, exists := rt.FindToolBackend("test_tool")
	if !exists {
		t.Fatal("Tool should exist")
	}
	if backend != "backend1" {
		t.Errorf("Expected backend1, got %s", backend)
	}

	_, exists = rt.FindToolBackend("non_existent")
	if exists {
		t.Error("Non-existent tool should not exist")
	}
}

func TestRoutingTable_FindResourceBackend(t *testing.T) {
	rt := NewRoutingTable()
	rt.ResourcesMap["resource://test"] = "backend1"
	rt.ResourcesMap["resource://another"] = "backend2"

	backend, exists := rt.FindResourceBackend("resource://test")
	if !exists {
		t.Fatal("Resource should exist")
	}
	if backend != "backend1" {
		t.Errorf("Expected backend1, got %s", backend)
	}

	_, exists = rt.FindResourceBackend("resource://non_existent")
	if exists {
		t.Error("Non-existent resource should not exist")
	}
}

func TestRoutingTable_FindPromptBackend(t *testing.T) {
	rt := NewRoutingTable()
	rt.PromptsMap["test_prompt"] = "backend1"
	rt.PromptsMap["another_prompt"] = "backend2"

	backend, exists := rt.FindPromptBackend("test_prompt")
	if !exists {
		t.Fatal("Prompt should exist")
	}
	if backend != "backend1" {
		t.Errorf("Expected backend1, got %s", backend)
	}

	_, exists = rt.FindPromptBackend("non_existent")
	if exists {
		t.Error("Non-existent prompt should not exist")
	}
}

func TestRoutingTable_GetAllTools(t *testing.T) {
	rt := NewRoutingTable()
	rt.ToolsMap["tool1"] = "backend1"
	rt.ToolsMap["tool2"] = "backend2"
	rt.ToolsMap["tool3"] = "backend1"

	tools := rt.GetAllTools()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}

	// Check that all tools are present
	toolSet := make(map[string]bool)
	for _, tool := range tools {
		toolSet[tool] = true
	}

	if !toolSet["tool1"] || !toolSet["tool2"] || !toolSet["tool3"] {
		t.Error("Not all tools are present in the result")
	}
}

func TestRoutingTable_GetAllResources(t *testing.T) {
	rt := NewRoutingTable()
	rt.ResourcesMap["res1"] = "backend1"
	rt.ResourcesMap["res2"] = "backend2"

	resources := rt.GetAllResources()
	if len(resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resources))
	}
}

func TestRoutingTable_GetAllPrompts(t *testing.T) {
	rt := NewRoutingTable()
	rt.PromptsMap["prompt1"] = "backend1"
	rt.PromptsMap["prompt2"] = "backend2"
	rt.PromptsMap["prompt3"] = "backend3"

	prompts := rt.GetAllPrompts()
	if len(prompts) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(prompts))
	}
}

func TestGatewayCapabilities_Structure(t *testing.T) {
	caps := GatewayCapabilities{
		Tools:     true,
		Resources: false,
		Prompts:   true,
	}

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

func TestRoutingTable_ConcurrentAccess(t *testing.T) {
	rt := NewRoutingTable()

	// Test concurrent writes and reads
	done := make(chan bool, 3)

	// Writer 1 - use proper locking
	go func() {
		for i := 0; i < 100; i++ {
			rt.mu.Lock()
			rt.ToolsMap[fmt.Sprintf("tool%d", i)] = fmt.Sprintf("backend%d", i)
			rt.mu.Unlock()
		}
		done <- true
	}()

	// Writer 2 - use proper locking
	go func() {
		for i := 0; i < 100; i++ {
			rt.mu.Lock()
			rt.ResourcesMap[fmt.Sprintf("res%d", i)] = fmt.Sprintf("backend%d", i)
			rt.mu.Unlock()
		}
		done <- true
	}()

	// Reader - uses the public methods which already have locking
	go func() {
		for i := 0; i < 100; i++ {
			_ = rt.GetAllTools()
			_ = rt.GetAllResources()
			rt.FindToolBackend(fmt.Sprintf("tool%d", i))
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify data integrity
	tools := rt.GetAllTools()
	if len(tools) != 100 {
		t.Errorf("Expected 100 tools after concurrent writes, got %d", len(tools))
	}

	resources := rt.GetAllResources()
	if len(resources) != 100 {
		t.Errorf("Expected 100 resources after concurrent writes, got %d", len(resources))
	}
}