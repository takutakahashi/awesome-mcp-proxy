package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

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

// Gateway configuration structures

type GatewayConfig struct {
	Gateway    GatewaySettings  `yaml:"gateway"`
	Groups     []Group          `yaml:"groups"`
	Middleware MiddlewareConfig `yaml:"middleware"`
}

type GatewaySettings struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Endpoint string `yaml:"endpoint"`
	Timeout  string `yaml:"timeout"`
}

type Group struct {
	Name     string    `yaml:"name"`
	Backends []Backend `yaml:"backends"`
}

type Backend struct {
	Name      string            `yaml:"name"`
	Transport string            `yaml:"transport"`
	Command   string            `yaml:"command,omitempty"`
	Args      []string          `yaml:"args,omitempty"`
	Endpoint  string            `yaml:"endpoint,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
}

type MiddlewareConfig struct {
	Logging LoggingConfig `yaml:"logging"`
	CORS    CORSConfig    `yaml:"cors"`
	Caching CachingConfig `yaml:"caching"`
}

type LoggingConfig struct {
	Enabled bool   `yaml:"enabled"`
	Level   string `yaml:"level"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type CachingConfig struct {
	Enabled bool   `yaml:"enabled"`
	TTL     string `yaml:"ttl"`
}

// BackendRef represents a reference to a specific backend
type BackendRef struct {
	GroupName   string
	BackendName string
}

// Gateway represents the MCP gateway/proxy
type Gateway struct {
	config       *GatewayConfig
	groups       []Group
	toolsMap     map[string]BackendRef
	resourcesMap map[string]BackendRef
	promptsMap   map[string]BackendRef
	backends     map[string]BackendConnection
	cache        *Cache
	mu           sync.RWMutex
}

// BackendConnection represents a connection to a backend MCP server
type BackendConnection interface {
	Initialize() error
	ListTools() ([]Tool, error)
	ListResources() ([]Resource, error)
	ListPrompts() ([]Prompt, error)
	CallTool(name string, arguments map[string]interface{}) (interface{}, error)
	ReadResource(uri string) (interface{}, error)
	GetPrompt(name string, arguments map[string]interface{}) (interface{}, error)
	Close() error
}

// Tool represents an MCP tool
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Prompt represents an MCP prompt
type Prompt struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Arguments   interface{} `json:"arguments,omitempty"`
}

// Cache represents a simple in-memory cache
type Cache struct {
	data map[string]CacheEntry
	mu   sync.RWMutex
}

type CacheEntry struct {
	Value     []byte
	ExpiresAt time.Time
}