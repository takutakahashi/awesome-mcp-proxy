package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RemoteMCPClient manages communication with a remote MCP server
type RemoteMCPClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	session    *remoteSession
}

type remoteSession struct {
	initialized bool
	serverInfo  *mcp.Implementation
}

// MCPRequest represents a JSON-RPC request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewRemoteMCPClient creates a new remote MCP client
func NewRemoteMCPClient(baseURL string) *RemoteMCPClient {
	return &RemoteMCPClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		session: &remoteSession{},
	}
}

// makeRequest makes an HTTP request to the remote MCP server
func (c *RemoteMCPClient) makeRequest(ctx context.Context, method string, params interface{}) (*MCPResponse, error) {
	reqBody := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle SSE format (lines starting with "data: ")
	bodyStr := string(body)
	if strings.HasPrefix(bodyStr, "data: ") {
		lines := strings.Split(bodyStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				body = []byte(strings.TrimPrefix(line, "data: "))
				break
			}
		}
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("remote error [%d]: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}

	return &mcpResp, nil
}

// Initialize initializes the connection with the remote server
func (c *RemoteMCPClient) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session.initialized {
		return nil
	}

	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "awesome-mcp-proxy-gateway",
			"version": "0.1.0",
		},
	}

	resp, err := c.makeRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	var initResult struct {
		ServerInfo *mcp.Implementation `json:"serverInfo"`
	}

	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}

	c.session.initialized = true
	c.session.serverInfo = initResult.ServerInfo

	return nil
}

// ListTools retrieves the list of tools from the remote server
func (c *RemoteMCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	resp, err := c.makeRequest(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	var result struct {
		Tools []mcp.Tool `json:"tools"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}

	return result.Tools, nil
}

// CallTool calls a tool on the remote server
func (c *RemoteMCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	params := map[string]interface{}{
		"name":      toolName,
		"arguments": arguments,
	}

	resp, err := c.makeRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}

	var result mcp.CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool call result: %w", err)
	}

	return &result, nil
}
