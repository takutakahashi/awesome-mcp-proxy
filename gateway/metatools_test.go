package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMetaToolHandler_GetMetaTools(t *testing.T) {
	manager := NewBackendManager()
	rt := NewRoutingTable()
	handler := NewMetaToolHandler(manager, rt)

	tools := handler.GetMetaTools()
	if len(tools) != 3 {
		t.Fatalf("Expected 3 meta-tools, got %d", len(tools))
	}

	// Check tool names
	expectedTools := map[string]bool{
		"list_tools":    false,
		"describe_tool": false,
		"call_tool":     false,
	}

	for _, tool := range tools {
		if _, exists := expectedTools[tool.Name]; !exists {
			t.Errorf("Unexpected tool: %s", tool.Name)
		}
		expectedTools[tool.Name] = true
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Missing expected tool: %s", name)
		}
	}
}

func TestMetaToolHandler_HandleListTools(t *testing.T) {
	manager := NewBackendManager()
	rt := NewRoutingTable()
	rt.ToolsMap["tool1"] = "backend1"
	rt.ToolsMap["tool2"] = "backend2"

	handler := NewMetaToolHandler(manager, rt)

	ctx := context.Background()
	result, data, err := handler.HandleListTools(ctx, &mcp.CallToolRequest{}, ListToolsParams{})

	if err != nil {
		t.Fatalf("HandleListTools failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Check that tools are returned in data
	tools, ok := data.([]string)
	if !ok {
		t.Fatal("Data should be []string")
	}

	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
}

func TestMetaToolHandler_HandleDescribeTool_NotFound(t *testing.T) {
	manager := NewBackendManager()
	rt := NewRoutingTable()
	handler := NewMetaToolHandler(manager, rt)

	ctx := context.Background()
	params := DescribeToolParams{
		ToolName: "non_existent_tool",
	}

	result, _, err := handler.HandleDescribeTool(ctx, &mcp.CallToolRequest{}, params)

	if err == nil {
		t.Fatal("Expected error for non-existent tool")
	}

	if result == nil || !result.IsError {
		t.Error("Result should indicate error")
	}
}

func TestMetaToolHandler_HandleCallTool_NotFound(t *testing.T) {
	manager := NewBackendManager()
	rt := NewRoutingTable()
	handler := NewMetaToolHandler(manager, rt)

	ctx := context.Background()
	params := CallToolParams{
		ToolName:  "non_existent_tool",
		Arguments: map[string]interface{}{},
	}

	result, _, err := handler.HandleCallTool(ctx, &mcp.CallToolRequest{}, params)

	if err == nil {
		t.Fatal("Expected error for non-existent tool")
	}

	if result == nil || !result.IsError {
		t.Error("Result should indicate error")
	}
}

func TestMetaToolHandler_ValidateMetaToolCall(t *testing.T) {
	handler := &MetaToolHandler{}

	tests := []struct {
		name        string
		toolName    string
		expectMeta  bool
		expectError bool
	}{
		{"list_tools is meta", "list_tools", true, false},
		{"describe_tool is meta", "describe_tool", true, false},
		{"call_tool is meta", "call_tool", true, false},
		{"regular tool is not meta", "echo", false, true},
		{"another regular tool", "add", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isMeta, err := handler.ValidateMetaToolCall(tt.toolName)

			if isMeta != tt.expectMeta {
				t.Errorf("Expected isMeta=%v, got %v", tt.expectMeta, isMeta)
			}

			if (err != nil) != tt.expectError {
				t.Errorf("Expected error=%v, got %v", tt.expectError, err != nil)
			}
		})
	}
}

func TestIsMetaTool(t *testing.T) {
	tests := []struct {
		toolName string
		expected bool
	}{
		{"list_tools", true},
		{"describe_tool", true},
		{"call_tool", true},
		{"echo", false},
		{"add", false},
		{"random_tool", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := IsMetaTool(tt.toolName)
			if result != tt.expected {
				t.Errorf("IsMetaTool(%s) = %v, expected %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestDescribeToolParams_Structure(t *testing.T) {
	params := DescribeToolParams{
		ToolName: "test_tool",
	}

	// Test JSON marshaling
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal DescribeToolParams: %v", err)
	}

	var unmarshaled DescribeToolParams
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal DescribeToolParams: %v", err)
	}

	if unmarshaled.ToolName != "test_tool" {
		t.Errorf("Expected tool_name to be test_tool, got %s", unmarshaled.ToolName)
	}
}

func TestCallToolParams_Structure(t *testing.T) {
	params := CallToolParams{
		ToolName: "test_tool",
		Arguments: map[string]interface{}{
			"arg1": "value1",
			"arg2": 42,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal CallToolParams: %v", err)
	}

	var unmarshaled CallToolParams
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal CallToolParams: %v", err)
	}

	if unmarshaled.ToolName != "test_tool" {
		t.Errorf("Expected tool_name to be test_tool, got %s", unmarshaled.ToolName)
	}

	if len(unmarshaled.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(unmarshaled.Arguments))
	}

	if unmarshaled.Arguments["arg1"] != "value1" {
		t.Errorf("Expected arg1 to be value1, got %v", unmarshaled.Arguments["arg1"])
	}
}