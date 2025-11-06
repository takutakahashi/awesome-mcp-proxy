package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServer represents an MCP server with HTTP transport
type MCPServer struct {
	server *server.MCPServer
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer() *MCPServer {
	// Create MCP server with basic information
	s := server.NewMCPServer(
		"awesome-mcp-proxy",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
		server.WithPromptCapabilities(true),
	)

	mcpServer := &MCPServer{
		server: s,
	}

	// Register example tools
	mcpServer.registerTools()

	// Register example resources
	mcpServer.registerResources()

	// Register example prompts
	mcpServer.registerPrompts()

	return mcpServer
}

// registerTools registers example tools
func (s *MCPServer) registerTools() {
	// Example: Echo tool
	echoTool := mcp.NewTool("echo",
		mcp.WithDescription("Echoes back the input message"),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("The message to echo back"),
		),
	)

	s.server.AddTool(echoTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		message, err := request.RequireString("message")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Echo: %s", message)), nil
	})

	// Example: Add numbers tool
	addTool := mcp.NewTool("add",
		mcp.WithDescription("Adds two numbers together"),
		mcp.WithNumber("a",
			mcp.Required(),
			mcp.Description("First number"),
		),
		mcp.WithNumber("b",
			mcp.Required(),
			mcp.Description("Second number"),
		),
	)

	s.server.AddTool(addTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, err := request.RequireFloat("a")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, err := request.RequireFloat("b")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result := a + b
		return mcp.NewToolResultText(fmt.Sprintf("Result: %.2f", result)), nil
	})

	log.Println("Registered tools: echo, add")
}

// registerResources registers example resources
func (s *MCPServer) registerResources() {
	// Example: Info resource
	infoResource := mcp.NewResource(
		"info://server",
		"Server Information",
		mcp.WithResourceDescription("General information about this MCP server"),
		mcp.WithMIMEType("text/plain"),
	)

	s.server.AddResource(infoResource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "info://server",
				MIMEType: "text/plain",
				Text:     "This is an awesome MCP proxy server with HTTP transport support!",
			},
		}, nil
	})

	// Example: Status resource
	statusResource := mcp.NewResource(
		"status://health",
		"Health Status",
		mcp.WithResourceDescription("Current health status of the server"),
		mcp.WithMIMEType("application/json"),
	)

	s.server.AddResource(statusResource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		status := map[string]interface{}{
			"status":  "healthy",
			"version": "0.1.0",
		}
		data, err := json.Marshal(status)
		if err != nil {
			return nil, err
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "status://health",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	})

	log.Println("Registered resources: info://server, status://health")
}

// registerPrompts registers example prompts
func (s *MCPServer) registerPrompts() {
	// Example: Greeting prompt
	greetingPrompt := mcp.NewPrompt("greeting",
		mcp.WithPromptDescription("A friendly greeting prompt"),
		mcp.WithArgument("name",
			mcp.ArgumentDescription("The name of the person to greet"),
			mcp.RequiredArgument(),
		),
	)

	s.server.AddPrompt(greetingPrompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		name, ok := request.Params.Arguments["name"]
		if !ok {
			name = "there"
		}

		return mcp.NewGetPromptResult(
			"A personalized greeting",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser,
					mcp.NewTextContent(
						fmt.Sprintf("Hello, %s! Welcome to the awesome MCP proxy server!", name),
					),
				),
			},
		), nil
	})

	log.Println("Registered prompts: greeting")
}

// GetServer returns the underlying MCP server
func (s *MCPServer) GetServer() *server.MCPServer {
	return s.server
}
