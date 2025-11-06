package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GatewayHTTPHandler wraps the MCP handler to intercept and route gateway tools
type GatewayHTTPHandler struct {
	baseHandler http.Handler
	mcpServer   *MCPServer
}

// NewGatewayHTTPHandler creates a new gateway handler
func NewGatewayHTTPHandler(baseHandler http.Handler, mcpServer *MCPServer) http.Handler {
	return &GatewayHTTPHandler{
		baseHandler: baseHandler,
		mcpServer:   mcpServer,
	}
}

// ServeHTTP implements http.Handler
func (h *GatewayHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Try to parse as JSON-RPC request
	var jsonRPCReq struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	if err := json.Unmarshal(body, &jsonRPCReq); err == nil {
		// Handle specific methods
		switch jsonRPCReq.Method {
		case "tools/list":
			// Restore body for base handler call
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			h.handleToolsList(w, r, jsonRPCReq.ID)
			return
		case "tools/call":
			h.handleToolsCall(w, r, jsonRPCReq.ID, jsonRPCReq.Params)
			return
		}
	}

	// For all other requests, restore body and pass through to base handler
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	h.baseHandler.ServeHTTP(w, r)
}

// handleToolsList handles tools/list requests
func (h *GatewayHTTPHandler) handleToolsList(w http.ResponseWriter, r *http.Request, id interface{}) {
	ctx := r.Context()

	// Create a response recorder to capture the base handler's response
	recorder := &responseRecorder{
		body: &strings.Builder{},
	}

	// Call base handler to get local tools
	h.baseHandler.ServeHTTP(recorder, r)

	// Debug: log the recorder output
	recorderOutput := recorder.body.String()
	log.Printf("DEBUG: recorder output: %s", recorderOutput)

	// Parse the response
	var localTools []mcp.Tool
	lines := strings.Split(recorderOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")
			log.Printf("DEBUG: parsing JSON data: %s", jsonData)
			var resp struct {
				Result struct {
					Tools []mcp.Tool `json:"tools"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(jsonData), &resp); err == nil {
				localTools = resp.Result.Tools
				log.Printf("DEBUG: found %d local tools", len(localTools))
				break
			} else {
				log.Printf("DEBUG: failed to unmarshal: %v", err)
			}
		}
	}

	// Get gateway tools
	gatewayTools, err := h.mcpServer.GetAllGatewayTools(ctx)
	if err != nil {
		// Just log the error and continue with local tools only
		log.Printf("Warning: Failed to get gateway tools: %v", err)
	}
	log.Printf("DEBUG: found %d gateway tools", len(gatewayTools))

	// Merge tools
	allTools := append(localTools, gatewayTools...)
	log.Printf("DEBUG: total tools: %d", len(allTools))

	result := map[string]interface{}{
		"tools": allTools,
	}

	h.sendSSEResult(w, id, result)
}

// responseRecorder records HTTP response
type responseRecorder struct {
	statusCode int
	header     http.Header
	body       *strings.Builder
}

func (r *responseRecorder) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

// handleToolsCall handles tools/call requests
func (h *GatewayHTTPHandler) handleToolsCall(w http.ResponseWriter, r *http.Request, id interface{}, params json.RawMessage) {
	ctx := r.Context()

	// Parse params
	var callParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(params, &callParams); err != nil {
		h.sendSSEError(w, id, -32602, "Invalid params")
		return
	}

	// Check if this is a gateway tool (has a known prefix)
	client, originalName, err := h.mcpServer.FindGatewayForTool(callParams.Name)
	if err != nil {
		// Not a gateway tool, pass through to base handler
		// Restore the request body and call base handler
		body, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "tools/call",
			"params":  callParams,
		})
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		h.baseHandler.ServeHTTP(w, r)
		return
	}

	// Call the remote tool
	result, err := client.CallTool(ctx, originalName, callParams.Arguments)
	if err != nil {
		h.sendSSEError(w, id, -32603, fmt.Sprintf("Failed to call remote tool: %v", err))
		return
	}

	h.sendSSEResult(w, id, result)
}

// sendSSEResult sends a JSON-RPC success response in SSE format
func (h *GatewayHTTPHandler) sendSSEResult(w http.ResponseWriter, id interface{}, result interface{}) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		h.sendSSEError(w, id, -32603, "Failed to marshal response")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(jsonData))
}

// sendSSEError sends a JSON-RPC error response in SSE format
func (h *GatewayHTTPHandler) sendSSEError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		// Fallback error
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%v,\"error\":{\"code\":-32603,\"message\":\"Internal error\"}}\n\n", id)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(jsonData))
}

// GetAllToolsIncludingLocal gets all tools including local and gateway tools
func (h *GatewayHTTPHandler) GetAllToolsIncludingLocal(ctx context.Context) ([]mcp.Tool, error) {
	// Get local tools
	localTools := []mcp.Tool{}
	// TODO: Get local tools from base server
	// This requires accessing the MCP server's tool registry

	// Get gateway tools
	gatewayTools, err := h.mcpServer.GetAllGatewayTools(ctx)
	if err != nil {
		return localTools, err
	}

	// Merge tools
	allTools := append(localTools, gatewayTools...)
	return allTools, nil
}
