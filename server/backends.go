package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// HTTPBackend represents an HTTP-based MCP backend
type HTTPBackend struct {
	config Backend
	client *http.Client
}

// NewHTTPBackend creates a new HTTP backend
func NewHTTPBackend(config Backend) (*HTTPBackend, error) {
	return &HTTPBackend{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Initialize initializes the HTTP backend
func (h *HTTPBackend) Initialize() error {
	// Send initialize request to the backend
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
				"prompts":   map[string]interface{}{},
			},
			"clientInfo": map[string]interface{}{
				"name":    "awesome-mcp-proxy",
				"version": "0.1.0",
			},
		},
		"id": 1,
	}
	
	_, err := h.sendRequest(initRequest)
	return err
}

// ListTools lists available tools from the backend
func (h *HTTPBackend) ListTools() ([]Tool, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
		"id":      1,
	}
	
	response, err := h.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}
	
	toolsData, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format")
	}
	
	var tools []Tool
	for _, toolData := range toolsData {
		if toolMap, ok := toolData.(map[string]interface{}); ok {
			tool := Tool{
				Name:        getString(toolMap, "name"),
				Description: getString(toolMap, "description"),
				InputSchema: toolMap["inputSchema"],
			}
			tools = append(tools, tool)
		}
	}
	
	return tools, nil
}

// ListResources lists available resources from the backend
func (h *HTTPBackend) ListResources() ([]Resource, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "resources/list",
		"id":      1,
	}
	
	response, err := h.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}
	
	resourcesData, ok := result["resources"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid resources format")
	}
	
	var resources []Resource
	for _, resourceData := range resourcesData {
		if resourceMap, ok := resourceData.(map[string]interface{}); ok {
			resource := Resource{
				URI:         getString(resourceMap, "uri"),
				Name:        getString(resourceMap, "name"),
				Description: getString(resourceMap, "description"),
				MimeType:    getString(resourceMap, "mimeType"),
			}
			resources = append(resources, resource)
		}
	}
	
	return resources, nil
}

// ListPrompts lists available prompts from the backend
func (h *HTTPBackend) ListPrompts() ([]Prompt, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "prompts/list",
		"id":      1,
	}
	
	response, err := h.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}
	
	promptsData, ok := result["prompts"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid prompts format")
	}
	
	var prompts []Prompt
	for _, promptData := range promptsData {
		if promptMap, ok := promptData.(map[string]interface{}); ok {
			prompt := Prompt{
				Name:        getString(promptMap, "name"),
				Description: getString(promptMap, "description"),
				Arguments:   promptMap["arguments"],
			}
			prompts = append(prompts, prompt)
		}
	}
	
	return prompts, nil
}

// CallTool calls a tool on the backend
func (h *HTTPBackend) CallTool(name string, arguments map[string]interface{}) (interface{}, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
		"id": 1,
	}
	
	response, err := h.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	return response["result"], nil
}

// ReadResource reads a resource from the backend
func (h *HTTPBackend) ReadResource(uri string) (interface{}, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": uri,
		},
		"id": 1,
	}
	
	response, err := h.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	return response["result"], nil
}

// GetPrompt gets a prompt from the backend
func (h *HTTPBackend) GetPrompt(name string, arguments map[string]interface{}) (interface{}, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "prompts/get",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
		"id": 1,
	}
	
	response, err := h.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	return response["result"], nil
}

// Close closes the HTTP backend
func (h *HTTPBackend) Close() error {
	// HTTP connections are automatically managed
	return nil
}

// sendRequest sends an HTTP request to the backend
func (h *HTTPBackend) sendRequest(request map[string]interface{}) (map[string]interface{}, error) {
	reqData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest("POST", h.config.Endpoint, bytes.NewBuffer(reqData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Add custom headers
	for key, value := range h.config.Headers {
		req.Header.Set(key, value)
	}
	
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}
	
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	
	// Check for JSON-RPC error
	if errorData, exists := response["error"]; exists {
		return nil, fmt.Errorf("JSON-RPC error: %v", errorData)
	}
	
	return response, nil
}

// StdioBackend represents a stdio-based MCP backend
type StdioBackend struct {
	config  Backend
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	decoder *json.Decoder
	encoder *json.Encoder
}

// NewStdioBackend creates a new stdio backend
func NewStdioBackend(config Backend) (*StdioBackend, error) {
	return &StdioBackend{
		config: config,
	}, nil
}

// Initialize initializes the stdio backend
func (s *StdioBackend) Initialize() error {
	// Set up environment variables
	env := os.Environ()
	for key, value := range s.config.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	
	// Create command
	s.cmd = exec.Command(s.config.Command, s.config.Args...)
	s.cmd.Env = env
	
	// Set up pipes
	var err error
	s.stdin, err = s.cmd.StdinPipe()
	if err != nil {
		return err
	}
	
	s.stdout, err = s.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	
	s.stderr, err = s.cmd.StderrPipe()
	if err != nil {
		return err
	}
	
	// Start the process
	if err := s.cmd.Start(); err != nil {
		return err
	}
	
	// Set up JSON encoder/decoder
	s.encoder = json.NewEncoder(s.stdin)
	s.decoder = json.NewDecoder(s.stdout)
	
	// Send initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
				"prompts":   map[string]interface{}{},
			},
			"clientInfo": map[string]interface{}{
				"name":    "awesome-mcp-proxy",
				"version": "0.1.0",
			},
		},
		"id": 1,
	}
	
	_, err = s.sendRequest(initRequest)
	return err
}

// ListTools lists available tools from the backend
func (s *StdioBackend) ListTools() ([]Tool, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
		"id":      1,
	}
	
	response, err := s.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}
	
	toolsData, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format")
	}
	
	var tools []Tool
	for _, toolData := range toolsData {
		if toolMap, ok := toolData.(map[string]interface{}); ok {
			tool := Tool{
				Name:        getString(toolMap, "name"),
				Description: getString(toolMap, "description"),
				InputSchema: toolMap["inputSchema"],
			}
			tools = append(tools, tool)
		}
	}
	
	return tools, nil
}

// ListResources lists available resources from the backend
func (s *StdioBackend) ListResources() ([]Resource, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "resources/list",
		"id":      1,
	}
	
	response, err := s.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}
	
	resourcesData, ok := result["resources"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid resources format")
	}
	
	var resources []Resource
	for _, resourceData := range resourcesData {
		if resourceMap, ok := resourceData.(map[string]interface{}); ok {
			resource := Resource{
				URI:         getString(resourceMap, "uri"),
				Name:        getString(resourceMap, "name"),
				Description: getString(resourceMap, "description"),
				MimeType:    getString(resourceMap, "mimeType"),
			}
			resources = append(resources, resource)
		}
	}
	
	return resources, nil
}

// ListPrompts lists available prompts from the backend
func (s *StdioBackend) ListPrompts() ([]Prompt, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "prompts/list",
		"id":      1,
	}
	
	response, err := s.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}
	
	promptsData, ok := result["prompts"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid prompts format")
	}
	
	var prompts []Prompt
	for _, promptData := range promptsData {
		if promptMap, ok := promptData.(map[string]interface{}); ok {
			prompt := Prompt{
				Name:        getString(promptMap, "name"),
				Description: getString(promptMap, "description"),
				Arguments:   promptMap["arguments"],
			}
			prompts = append(prompts, prompt)
		}
	}
	
	return prompts, nil
}

// CallTool calls a tool on the backend
func (s *StdioBackend) CallTool(name string, arguments map[string]interface{}) (interface{}, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
		"id": 1,
	}
	
	response, err := s.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	return response["result"], nil
}

// ReadResource reads a resource from the backend
func (s *StdioBackend) ReadResource(uri string) (interface{}, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": uri,
		},
		"id": 1,
	}
	
	response, err := s.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	return response["result"], nil
}

// GetPrompt gets a prompt from the backend
func (s *StdioBackend) GetPrompt(name string, arguments map[string]interface{}) (interface{}, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "prompts/get",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
		"id": 1,
	}
	
	response, err := s.sendRequest(request)
	if err != nil {
		return nil, err
	}
	
	return response["result"], nil
}

// Close closes the stdio backend
func (s *StdioBackend) Close() error {
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}
	if s.stderr != nil {
		s.stderr.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

// sendRequest sends a request to the stdio backend
func (s *StdioBackend) sendRequest(request map[string]interface{}) (map[string]interface{}, error) {
	// Send request
	if err := s.encoder.Encode(request); err != nil {
		return nil, err
	}
	
	// Read response
	var response map[string]interface{}
	if err := s.decoder.Decode(&response); err != nil {
		return nil, err
	}
	
	// Check for JSON-RPC error
	if errorData, exists := response["error"]; exists {
		return nil, fmt.Errorf("JSON-RPC error: %v", errorData)
	}
	
	return response, nil
}

// getString safely gets a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}