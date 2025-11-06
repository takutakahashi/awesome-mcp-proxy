package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/takutakahashi/awesome-mcp-proxy/config"
)

// MCPServer represents an MCP server with HTTP transport
type MCPServer struct {
	server  *mcp.Server
	config  *config.Config
	clients map[string]*RemoteMCPClient
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(cfg *config.Config) *MCPServer {
	// Create MCP server with basic information
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "awesome-mcp-proxy",
			Version: "0.1.0",
		},
		nil,
	)

	mcpServer := &MCPServer{
		server:  server,
		config:  cfg,
		clients: make(map[string]*RemoteMCPClient),
	}

	// Initialize remote clients
	if cfg != nil {
		for _, gw := range cfg.Gateways {
			mcpServer.clients[gw.Name] = NewRemoteMCPClient(gw.URL)
			log.Printf("Initialized gateway client for %s (%s)", gw.Name, gw.URL)
		}
	}

	// Register example tools
	mcpServer.registerTools()

	// Register gateway tools from config
	if cfg != nil {
		mcpServer.registerGatewayToolsFromConfig()
	}

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

// registerGatewayToolsFromConfig registers tools from configured gateways with prefixes
func (s *MCPServer) registerGatewayToolsFromConfig() {
	if s.config == nil || len(s.config.Gateways) == 0 {
		log.Println("No gateways configured")
		return
	}

	// Use a context with timeout to avoid hanging forever
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	totalTools := 0

	for _, gw := range s.config.Gateways {
		client := s.clients[gw.Name]
		if client == nil {
			log.Printf("Warning: client not initialized for gateway %s", gw.Name)
			continue
		}

		log.Printf("Fetching tools from gateway %s at %s...", gw.Name, gw.URL)

		// Get tools from remote server
		tools, err := client.ListTools(ctx)
		if err != nil {
			log.Printf("Warning: failed to list tools from gateway %s: %v", gw.Name, err)
			continue
		}

		// Register each tool with prefix
		for _, remoteTool := range tools {
			s.registerGatewayTool(gw, remoteTool)
			totalTools++
		}

		log.Printf("Registered %d tools from gateway %s with prefix: %s", len(tools), gw.Name, gw.Prefix)
	}

	if totalTools > 0 {
		log.Printf("Successfully registered %d gateway tools from %d gateway(s)", totalTools, len(s.config.Gateways))
	} else {
		log.Printf("No gateway tools registered (0 from %d gateway(s))", len(s.config.Gateways))
	}
}

// GatewayToolParams represents parameters for a gateway tool (dynamic arguments)
type GatewayToolParams struct {
	// Arguments will be passed as-is to the remote tool
	Arguments map[string]interface{} `json:"-"`
}

// registerGatewayTool registers a single tool from a gateway
func (s *MCPServer) registerGatewayTool(gw config.Gateway, remoteTool mcp.Tool) {
	gatewayName := gw.Name
	prefix := gw.Prefix
	client := s.clients[gatewayName]

	// Create tool with prefixed name
	tool := &mcp.Tool{
		Name:        prefix + remoteTool.Name,
		Description: fmt.Sprintf("[Gateway: %s] %s", gatewayName, remoteTool.Description),
		InputSchema: remoteTool.InputSchema,
	}

	// Use a dynamic handler that forwards to the remote server
	originalToolName := remoteTool.Name
	mcp.AddTool(s.server, tool, func(ctx context.Context, request *mcp.CallToolRequest, params map[string]interface{}) (*mcp.CallToolResult, any, error) {
		// Forward the call to the remote server
		result, err := client.CallTool(ctx, originalToolName, params)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error calling remote tool: %v", err),
					},
				},
				IsError: true,
			}, nil, nil
		}
		return result, nil, nil
	})
}

// GetAllGatewayTools retrieves all tools from all configured gateways with prefixes
func (s *MCPServer) GetAllGatewayTools(ctx context.Context) ([]mcp.Tool, error) {
	var allTools []mcp.Tool

	if s.config == nil {
		return allTools, nil
	}

	for _, gw := range s.config.Gateways {
		client := s.clients[gw.Name]
		if client == nil {
			continue
		}

		tools, err := client.ListTools(ctx)
		if err != nil {
			log.Printf("Warning: failed to list tools from gateway %s: %v", gw.Name, err)
			continue
		}

		// Add prefix to each tool name
		for _, tool := range tools {
			tool.Name = gw.Prefix + tool.Name
			allTools = append(allTools, tool)
		}
	}

	return allTools, nil
}

// FindGatewayForTool finds the gateway and original tool name for a prefixed tool
func (s *MCPServer) FindGatewayForTool(toolName string) (*RemoteMCPClient, string, error) {
	if s.config == nil {
		return nil, "", fmt.Errorf("no gateways configured")
	}

	for _, gw := range s.config.Gateways {
		if len(toolName) > len(gw.Prefix) && toolName[:len(gw.Prefix)] == gw.Prefix {
			// Found matching prefix
			originalName := toolName[len(gw.Prefix):]
			client := s.clients[gw.Name]
			if client == nil {
				return nil, "", fmt.Errorf("gateway client not initialized: %s", gw.Name)
			}
			return client, originalName, nil
		}
	}

	return nil, "", fmt.Errorf("no gateway found for tool: %s", toolName)
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
