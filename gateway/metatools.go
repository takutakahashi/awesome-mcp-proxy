package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MetaToolHandler handles the three meta-tools for the gateway
type MetaToolHandler struct {
	backendManager *BackendManager
	routingTable   *RoutingTable
}

// NewMetaToolHandler creates a new meta-tool handler
func NewMetaToolHandler(backendManager *BackendManager, routingTable *RoutingTable) *MetaToolHandler {
	return &MetaToolHandler{
		backendManager: backendManager,
		routingTable:   routingTable,
	}
}

// ListToolsParams represents parameters for list_tools meta-tool
type ListToolsParams struct{}

// DescribeToolParams represents parameters for describe_tool meta-tool
type DescribeToolParams struct {
	ToolName string `json:"tool_name" jsonschema:"required,description=The name of the tool to describe"`
}

// CallToolParams represents parameters for call_tool meta-tool
type CallToolParams struct {
	ToolName  string                 `json:"tool_name" jsonschema:"required,description=The name of the tool to call"`
	Arguments map[string]interface{} `json:"arguments" jsonschema:"required,description=The arguments to pass to the tool"`
}

// GetMetaTools returns the three meta-tools definitions
func (mth *MetaToolHandler) GetMetaTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "list_tools",
			Description: "バックエンドから利用可能なツールの名前一覧を取得",
			// InputSchema will be set by the SDK based on ListToolsParams
		},
		{
			Name:        "describe_tool",
			Description: "指定したツールの詳細情報（説明、引数仕様）を取得",
			// InputSchema will be set by the SDK based on DescribeToolParams
		},
		{
			Name:        "call_tool",
			Description: "実際のツール実行を行う",
			// InputSchema will be set by the SDK based on CallToolParams
		},
	}
}

// HandleListTools implements the list_tools meta-tool
func (mth *MetaToolHandler) HandleListTools(ctx context.Context, request *mcp.CallToolRequest, params ListToolsParams) (*mcp.CallToolResult, interface{}, error) {
	// Get all available tools from routing table
	tools := mth.routingTable.GetAllTools()

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Available tools: %v", tools),
			},
		},
	}, tools, nil
}

// HandleDescribeTool implements the describe_tool meta-tool
func (mth *MetaToolHandler) HandleDescribeTool(ctx context.Context, request *mcp.CallToolRequest, params DescribeToolParams) (*mcp.CallToolResult, interface{}, error) {
	// Find backend that provides this tool
	backendName, exists := mth.routingTable.FindToolBackend(params.ToolName)
	if !exists {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Tool '%s' not found", params.ToolName),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("tool '%s' not found", params.ToolName)
	}

	// Get backend
	backend, exists := mth.backendManager.GetBackend(backendName)
	if !exists {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Backend '%s' not available", backendName),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("backend '%s' not available", backendName)
	}

	// Get tools list from backend to find the specific tool description
	response, err := backend.SendRequest(ctx, "tools/list", struct{}{})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Failed to get tools from backend: %v", err),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("failed to get tools from backend: %w", err)
	}

	var toolsResponse struct {
		Tools []mcp.Tool `json:"tools"`
	}

	if err := json.Unmarshal(*response, &toolsResponse); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Failed to parse tools response: %v", err),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	// Find the specific tool
	for _, tool := range toolsResponse.Tools {
		if tool.Name == params.ToolName {
			// Return the tool description
			toolData, err := json.Marshal(tool)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{
							Text: fmt.Sprintf("Failed to serialize tool description: %v", err),
						},
					},
					IsError: true,
				}, nil, fmt.Errorf("failed to serialize tool description: %w", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: string(toolData),
					},
				},
			}, tool, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Tool '%s' not found in backend '%s'", params.ToolName, backendName),
			},
		},
		IsError: true,
	}, nil, fmt.Errorf("tool '%s' not found in backend '%s'", params.ToolName, backendName)
}

// HandleCallTool implements the call_tool meta-tool
func (mth *MetaToolHandler) HandleCallTool(ctx context.Context, request *mcp.CallToolRequest, params CallToolParams) (*mcp.CallToolResult, interface{}, error) {
	// Find backend that provides this tool
	backendName, exists := mth.routingTable.FindToolBackend(params.ToolName)
	if !exists {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Tool '%s' not found", params.ToolName),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("tool '%s' not found", params.ToolName)
	}

	// Get backend
	backend, exists := mth.backendManager.GetBackend(backendName)
	if !exists {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Backend '%s' not available", backendName),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("backend '%s' not available", backendName)
	}

	// Check backend health
	if !backend.IsHealthy() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Backend '%s' is not healthy", backendName),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("backend '%s' is not healthy", backendName)
	}

	// Prepare the tool call request for the backend
	toolCallParams := struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}{
		Name:      params.ToolName,
		Arguments: params.Arguments,
	}

	// Send the tool call to the backend
	response, err := backend.SendRequest(ctx, "tools/call", toolCallParams)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Failed to call tool on backend: %v", err),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("failed to call tool on backend: %w", err)
	}

	// Parse the response from backend
	var toolResult mcp.CallToolResult
	if err := json.Unmarshal(*response, &toolResult); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Failed to parse tool response: %v", err),
				},
			},
			IsError: true,
		}, nil, fmt.Errorf("failed to parse tool response: %w", err)
	}

	// Return the result from backend
	return &toolResult, nil, nil
}

// ValidateMetaToolCall checks if a tool call is for a meta-tool and validates it
func (mth *MetaToolHandler) ValidateMetaToolCall(toolName string) (bool, error) {
	switch toolName {
	case "list_tools", "describe_tool", "call_tool":
		return true, nil
	default:
		// This is a direct backend tool call, which is prohibited
		return false, fmt.Errorf("direct tool calls are prohibited. Use meta-tools instead. Requested tool: %s", toolName)
	}
}

// IsMetaTool checks if a given tool name is a meta-tool
func IsMetaTool(toolName string) bool {
	return toolName == "list_tools" || toolName == "describe_tool" || toolName == "call_tool"
}
