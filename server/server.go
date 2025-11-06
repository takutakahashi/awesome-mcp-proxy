package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer represents an MCP server with HTTP transport
type MCPServer struct {
	server *mcp.Server
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer() *MCPServer {
	// Create MCP server with basic information
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "awesome-mcp-proxy",
			Version: "0.1.0",
		},
		nil,
	)

	mcpServer := &MCPServer{
		server: server,
	}

	// Register example tools
	mcpServer.registerTools()

	// Register gateway tools
	mcpServer.registerGatewayTools()

	// Register example resources
	mcpServer.registerResources()

	// Register example prompts
	mcpServer.registerPrompts()

	return mcpServer
}

// Tool parameter types
type EchoParams struct {
	Message string `json:"message" jsonschema:"the message to echo back"`
}

type AddParams struct {
	A float64 `json:"a" jsonschema:"first number"`
	B float64 `json:"b" jsonschema:"second number"`
}

// Gateway tool parameter types
type GatewayListToolsParams struct {
	RemoteURL string `json:"remote_url" jsonschema:"URL of the remote MCP server (e.g., http://localhost:8080/mcp)"`
}

type GatewayDescribeToolParams struct {
	RemoteURL string `json:"remote_url" jsonschema:"URL of the remote MCP server"`
	ToolName  string `json:"tool_name" jsonschema:"name of the tool to describe"`
}

type GatewayExecuteToolParams struct {
	RemoteURL string                 `json:"remote_url" jsonschema:"URL of the remote MCP server"`
	ToolName  string                 `json:"tool_name" jsonschema:"name of the tool to execute"`
	Arguments map[string]interface{} `json:"arguments" jsonschema:"arguments to pass to the tool"`
}

// registerTools registers example tools
func (s *MCPServer) registerTools() {
	// Example: Echo tool
	echoTool := &mcp.Tool{
		Name:        "echo",
		Description: "Echoes back the input message",
	}

	mcp.AddTool(s.server, echoTool, func(ctx context.Context, request *mcp.CallToolRequest, params EchoParams) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Echo: %s", params.Message),
				},
			},
		}, nil, nil
	})

	// Example: Add numbers tool
	addTool := &mcp.Tool{
		Name:        "add",
		Description: "Adds two numbers together",
	}

	mcp.AddTool(s.server, addTool, func(ctx context.Context, request *mcp.CallToolRequest, params AddParams) (*mcp.CallToolResult, any, error) {
		result := params.A + params.B
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Result: %.2f", result),
				},
			},
		}, nil, nil
	})

	log.Println("Registered tools: echo, add")
}

// registerGatewayTools registers gateway tools for proxying to remote MCP servers
func (s *MCPServer) registerGatewayTools() {
	// Gateway: List tools from remote server
	listToolsTool := &mcp.Tool{
		Name:        "gateway-list-tools",
		Description: "Lists all available tools from a remote MCP server",
	}

	mcp.AddTool(s.server, listToolsTool, func(ctx context.Context, request *mcp.CallToolRequest, params GatewayListToolsParams) (*mcp.CallToolResult, any, error) {
		if params.RemoteURL == "" {
			return nil, nil, fmt.Errorf("remote_url is required")
		}

		client := NewRemoteMCPClient(params.RemoteURL)
		tools, err := client.ListTools(ctx)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error listing tools: %v", err),
					},
				},
				IsError: true,
			}, nil, nil
		}

		// Format tools as JSON
		toolsJSON, err := json.MarshalIndent(tools, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal tools: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Remote MCP server tools:\n%s", string(toolsJSON)),
				},
			},
		}, nil, nil
	})

	// Gateway: Describe a specific tool
	describeToolTool := &mcp.Tool{
		Name:        "gateway-describe-tool",
		Description: "Gets detailed information about a specific tool from a remote MCP server",
	}

	mcp.AddTool(s.server, describeToolTool, func(ctx context.Context, request *mcp.CallToolRequest, params GatewayDescribeToolParams) (*mcp.CallToolResult, any, error) {
		if params.RemoteURL == "" {
			return nil, nil, fmt.Errorf("remote_url is required")
		}
		if params.ToolName == "" {
			return nil, nil, fmt.Errorf("tool_name is required")
		}

		client := NewRemoteMCPClient(params.RemoteURL)
		tools, err := client.ListTools(ctx)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error listing tools: %v", err),
					},
				},
				IsError: true,
			}, nil, nil
		}

		// Find the requested tool
		var targetTool *mcp.Tool
		for _, tool := range tools {
			if tool.Name == params.ToolName {
				targetTool = &tool
				break
			}
		}

		if targetTool == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Tool '%s' not found on remote server", params.ToolName),
					},
				},
				IsError: true,
			}, nil, nil
		}

		// Format tool details as JSON
		toolJSON, err := json.MarshalIndent(targetTool, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal tool: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Tool details:\n%s", string(toolJSON)),
				},
			},
		}, nil, nil
	})

	// Gateway: Execute a tool on remote server
	executeToolTool := &mcp.Tool{
		Name:        "gateway-execute-tool",
		Description: "Executes a tool on a remote MCP server and returns the result",
	}

	mcp.AddTool(s.server, executeToolTool, func(ctx context.Context, request *mcp.CallToolRequest, params GatewayExecuteToolParams) (*mcp.CallToolResult, any, error) {
		if params.RemoteURL == "" {
			return nil, nil, fmt.Errorf("remote_url is required")
		}
		if params.ToolName == "" {
			return nil, nil, fmt.Errorf("tool_name is required")
		}

		client := NewRemoteMCPClient(params.RemoteURL)
		result, err := client.CallTool(ctx, params.ToolName, params.Arguments)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error executing tool: %v", err),
					},
				},
				IsError: true,
			}, nil, nil
		}

		return result, nil, nil
	})

	log.Println("Registered gateway tools: gateway-list-tools, gateway-describe-tool, gateway-execute-tool")
}

// registerResources registers example resources
func (s *MCPServer) registerResources() {
	// Example: Info resource
	s.server.AddResource(&mcp.Resource{
		URI:         "info://server",
		Name:        "Server Information",
		Description: "General information about this MCP server",
		MIMEType:    "text/plain",
	}, func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "info://server",
					MIMEType: "text/plain",
					Text:     "This is an awesome MCP proxy server with HTTP transport support!",
				},
			},
		}, nil
	})

	// Example: Status resource
	s.server.AddResource(&mcp.Resource{
		URI:         "status://health",
		Name:        "Health Status",
		Description: "Current health status of the server",
		MIMEType:    "application/json",
	}, func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		status := map[string]interface{}{
			"status":  "healthy",
			"version": "0.1.0",
		}
		data, err := json.Marshal(status)
		if err != nil {
			return nil, err
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "status://health",
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	})

	log.Println("Registered resources: info://server, status://health")
}

// Prompt parameter types
type GreetingParams struct {
	Name string `json:"name" jsonschema:"the name of the person to greet"`
}

// registerPrompts registers example prompts
func (s *MCPServer) registerPrompts() {
	// Example: Greeting prompt
	greetingPrompt := &mcp.Prompt{
		Name:        "greeting",
		Description: "A friendly greeting prompt",
	}

	s.server.AddPrompt(greetingPrompt, func(ctx context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		name, ok := request.Params.Arguments["name"]
		if !ok || name == "" {
			name = "there"
		}

		return &mcp.GetPromptResult{
			Description: "A personalized greeting",
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: fmt.Sprintf("Hello, %s! Welcome to the awesome MCP proxy server!", name),
					},
				},
			},
		}, nil
	})

	log.Println("Registered prompts: greeting")
}

// GetServer returns the underlying MCP server
func (s *MCPServer) GetServer() *mcp.Server {
	return s.server
}
