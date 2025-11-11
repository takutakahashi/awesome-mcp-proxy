package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/takutakahashi/awesome-mcp-proxy/config"
)

// Backend represents a connection to a backend MCP server
type Backend interface {
	// Initialize sends the initialize request to the backend
	Initialize(ctx context.Context, req interface{}) (*mcp.InitializeResult, error)

	// SendRequest sends an arbitrary JSON-RPC request to the backend
	SendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error)

	// GetInfo returns backend information
	GetInfo() BackendInfo

	// Close closes the backend connection
	Close() error

	// IsHealthy returns the health status of the backend
	IsHealthy() bool
}

// BackendInfo contains metadata about a backend
type BackendInfo struct {
	Name      string
	Transport string
	Group     string
}

// HTTPBackend implements Backend interface for HTTP transport
type HTTPBackend struct {
	info     BackendInfo
	config   config.Backend
	client   *http.Client
	endpoint string
	healthy  bool
	mu       sync.RWMutex
}

// NewHTTPBackend creates a new HTTP backend
func NewHTTPBackend(cfg config.Backend, groupName string) *HTTPBackend {
	return &HTTPBackend{
		info: BackendInfo{
			Name:      cfg.Name,
			Transport: "http",
			Group:     groupName,
		},
		config:   cfg,
		endpoint: cfg.Endpoint,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		healthy: true,
	}
}

func (b *HTTPBackend) Initialize(ctx context.Context, req interface{}) (*mcp.InitializeResult, error) {
	response, err := b.sendJSONRPC(ctx, "initialize", req)
	if err != nil {
		b.setHealthy(false)
		return nil, err
	}

	var result *mcp.InitializeResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal initialize response: %w", err)
	}

	b.setHealthy(true)
	return result, nil
}

func (b *HTTPBackend) SendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	return b.sendJSONRPC(ctx, method, params)
}

func (b *HTTPBackend) sendJSONRPC(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Set custom headers
	for key, value := range b.config.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		b.setHealthy(false)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.setHealthy(false)
		return nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var jsonRPCResponse map[string]*json.RawMessage
	if err := json.Unmarshal(body, &jsonRPCResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if errorData, exists := jsonRPCResponse["error"]; exists && errorData != nil {
		return nil, fmt.Errorf("JSON-RPC error: %s", string(*errorData))
	}

	result, exists := jsonRPCResponse["result"]
	if !exists {
		return nil, fmt.Errorf("no result in response")
	}

	b.setHealthy(true)
	return result, nil
}

func (b *HTTPBackend) GetInfo() BackendInfo {
	return b.info
}

func (b *HTTPBackend) Close() error {
	// HTTP clients don't need explicit closing
	return nil
}

func (b *HTTPBackend) IsHealthy() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.healthy
}

func (b *HTTPBackend) setHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.healthy = healthy
}

// StdioBackend implements Backend interface for stdio transport
type StdioBackend struct {
	info    BackendInfo
	config  config.Backend
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	healthy bool
	mu      sync.RWMutex
	reqID   int64
}

// NewStdioBackend creates a new stdio backend
func NewStdioBackend(cfg config.Backend, groupName string) *StdioBackend {
	return &StdioBackend{
		info: BackendInfo{
			Name:      cfg.Name,
			Transport: "stdio",
			Group:     groupName,
		},
		config:  cfg,
		healthy: true,
		reqID:   1,
	}
}

func (b *StdioBackend) Initialize(ctx context.Context, req interface{}) (*mcp.InitializeResult, error) {
	if err := b.start(); err != nil {
		return nil, err
	}

	response, err := b.sendJSONRPC(ctx, "initialize", req)
	if err != nil {
		b.setHealthy(false)
		return nil, err
	}

	var result *mcp.InitializeResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal initialize response: %w", err)
	}

	b.setHealthy(true)
	return result, nil
}

func (b *StdioBackend) start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cmd != nil {
		return nil // Already started
	}

	b.cmd = exec.Command(b.config.Command, b.config.Args...)

	// Set environment variables
	if len(b.config.Env) > 0 {
		env := b.cmd.Environ()
		for key, value := range b.config.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		b.cmd.Env = env
	}

	stdin, err := b.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	b.stdin = stdin

	stdout, err := b.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	b.stdout = stdout

	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	return nil
}

func (b *StdioBackend) SendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	return b.sendJSONRPC(ctx, method, params)
}

func (b *StdioBackend) sendJSONRPC(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	b.mu.Lock()
	currentID := b.reqID
	b.reqID++
	b.mu.Unlock()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      currentID,
		"method":  method,
		"params":  params,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Add newline for stdio transport
	jsonData = append(jsonData, '\n')

	// Send request
	if _, err := b.stdin.Write(jsonData); err != nil {
		b.setHealthy(false)
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}

	// Read response
	decoder := json.NewDecoder(b.stdout)
	var jsonRPCResponse map[string]*json.RawMessage
	if err := decoder.Decode(&jsonRPCResponse); err != nil {
		b.setHealthy(false)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if errorData, exists := jsonRPCResponse["error"]; exists && errorData != nil {
		return nil, fmt.Errorf("JSON-RPC error: %s", string(*errorData))
	}

	result, exists := jsonRPCResponse["result"]
	if !exists {
		return nil, fmt.Errorf("no result in response")
	}

	b.setHealthy(true)
	return result, nil
}

func (b *StdioBackend) GetInfo() BackendInfo {
	return b.info
}

func (b *StdioBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stdin != nil {
		b.stdin.Close()
	}
	if b.stdout != nil {
		b.stdout.Close()
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
		_ = b.cmd.Wait()
	}
	return nil
}

func (b *StdioBackend) IsHealthy() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.healthy
}

func (b *StdioBackend) setHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.healthy = healthy
}

// BackendManager manages multiple backends
type BackendManager struct {
	backends map[string]Backend
	mu       sync.RWMutex
}

// NewBackendManager creates a new backend manager
func NewBackendManager() *BackendManager {
	return &BackendManager{
		backends: make(map[string]Backend),
	}
}

// AddBackend adds a backend to the manager
func (bm *BackendManager) AddBackend(backend Backend) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	info := backend.GetInfo()
	bm.backends[info.Name] = backend
}

// GetBackend returns a backend by name
func (bm *BackendManager) GetBackend(name string) (Backend, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	backend, exists := bm.backends[name]
	return backend, exists
}

// GetAllBackends returns all backends
func (bm *BackendManager) GetAllBackends() []Backend {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	backends := make([]Backend, 0, len(bm.backends))
	for _, backend := range bm.backends {
		backends = append(backends, backend)
	}
	return backends
}

// GetHealthyBackends returns only healthy backends
func (bm *BackendManager) GetHealthyBackends() []Backend {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var backends []Backend
	for _, backend := range bm.backends {
		if backend.IsHealthy() {
			backends = append(backends, backend)
		}
	}
	return backends
}

// Close closes all backends
func (bm *BackendManager) Close() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for _, backend := range bm.backends {
		if err := backend.Close(); err != nil {
			// Log error but continue closing others
			fmt.Printf("Error closing backend %s: %v\n", backend.GetInfo().Name, err)
		}
	}
	return nil
}
