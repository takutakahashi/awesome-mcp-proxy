package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// NewGateway creates a new gateway instance
func NewGateway(config *GatewayConfig) *Gateway {
	return &Gateway{
		config:       config,
		groups:       config.Groups,
		toolsMap:     make(map[string]BackendRef),
		resourcesMap: make(map[string]BackendRef),
		promptsMap:   make(map[string]BackendRef),
		backends:     make(map[string]BackendConnection),
		cache:        NewCache(),
	}
}

// Initialize initializes the gateway and discovers capabilities from all backends
func (g *Gateway) Initialize() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, group := range g.groups {
		for _, backend := range group.Backends {
			backendKey := fmt.Sprintf("%s/%s", group.Name, backend.Name)
			
			var conn BackendConnection
			var err error
			
			switch backend.Transport {
			case "http":
				conn, err = NewHTTPBackend(backend)
			case "stdio":
				conn, err = NewStdioBackend(backend)
			default:
				log.Printf("Unsupported transport: %s for backend %s", backend.Transport, backendKey)
				continue
			}
			
			if err != nil {
				log.Printf("Failed to create backend %s: %v", backendKey, err)
				continue
			}
			
			// Initialize connection
			if err := conn.Initialize(); err != nil {
				log.Printf("Failed to initialize backend %s: %v", backendKey, err)
				continue
			}
			
			g.backends[backendKey] = conn
			
			// Discover capabilities
			ref := BackendRef{
				GroupName:   group.Name,
				BackendName: backend.Name,
			}
			
			// Discover tools
			if tools, err := conn.ListTools(); err == nil {
				for _, tool := range tools {
					g.toolsMap[tool.Name] = ref
					log.Printf("Registered tool '%s' from backend %s", tool.Name, backendKey)
				}
			} else {
				log.Printf("Failed to list tools from %s: %v", backendKey, err)
			}
			
			// Discover resources
			if resources, err := conn.ListResources(); err == nil {
				for _, resource := range resources {
					g.resourcesMap[resource.URI] = ref
					log.Printf("Registered resource '%s' from backend %s", resource.URI, backendKey)
				}
			} else {
				log.Printf("Failed to list resources from %s: %v", backendKey, err)
			}
			
			// Discover prompts
			if prompts, err := conn.ListPrompts(); err == nil {
				for _, prompt := range prompts {
					g.promptsMap[prompt.Name] = ref
					log.Printf("Registered prompt '%s' from backend %s", prompt.Name, backendKey)
				}
			} else {
				log.Printf("Failed to list prompts from %s: %v", backendKey, err)
			}
		}
	}
	
	log.Printf("Gateway initialized with %d backends, %d tools, %d resources, %d prompts",
		len(g.backends), len(g.toolsMap), len(g.resourcesMap), len(g.promptsMap))
	
	return nil
}

// GetBackend retrieves a backend by reference
func (g *Gateway) GetBackend(ref BackendRef) BackendConnection {
	backendKey := fmt.Sprintf("%s/%s", ref.GroupName, ref.BackendName)
	return g.backends[backendKey]
}

// HandleMCPRequest handles incoming MCP requests and routes them to appropriate backends
func (g *Gateway) HandleMCPRequest(w http.ResponseWriter, r *http.Request) {
	var request map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		g.writeJSONRPCError(w, nil, -32700, "Parse error")
		return
	}
	
	method, ok := request["method"].(string)
	if !ok {
		g.writeJSONRPCError(w, request["id"], -32600, "Invalid Request")
		return
	}
	
	switch method {
	case "initialize":
		g.handleInitialize(w, request)
	case "tools/list":
		g.handleToolsList(w, request)
	case "tools/call":
		g.handleToolsCall(w, request)
	case "resources/list":
		g.handleResourcesList(w, request)
	case "resources/read":
		g.handleResourcesRead(w, request)
	case "prompts/list":
		g.handlePromptsList(w, request)
	case "prompts/get":
		g.handlePromptsGet(w, request)
	default:
		g.writeJSONRPCError(w, request["id"], -32601, "Method not found")
	}
}

// handleInitialize returns gateway capabilities (union of all backends)
func (g *Gateway) handleInitialize(w http.ResponseWriter, request map[string]interface{}) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request["id"],
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
				"prompts":   map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "awesome-mcp-proxy",
				"version": "0.1.0",
			},
		},
	}
	
	g.writeJSONResponse(w, response)
}

// handleToolsList aggregates tools from all backends
func (g *Gateway) handleToolsList(w http.ResponseWriter, request map[string]interface{}) {
	cacheKey := "tools/list"
	if cached := g.cache.Get(cacheKey); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}
	
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	allTools := []Tool{}
	
	for _, backendConn := range g.backends {
		if tools, err := backendConn.ListTools(); err == nil {
			allTools = append(allTools, tools...)
		}
	}
	
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request["id"],
		"result": map[string]interface{}{
			"tools": allTools,
		},
	}
	
	responseData, _ := json.Marshal(response)
	g.cache.Set(cacheKey, responseData, 5*time.Minute)
	
	g.writeJSONResponse(w, response)
}

// handleToolsCall routes tool calls to the appropriate backend
func (g *Gateway) handleToolsCall(w http.ResponseWriter, request map[string]interface{}) {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		g.writeJSONRPCError(w, request["id"], -32602, "Invalid params")
		return
	}
	
	toolName, ok := params["name"].(string)
	if !ok {
		g.writeJSONRPCError(w, request["id"], -32602, "Missing tool name")
		return
	}
	
	g.mu.RLock()
	ref, exists := g.toolsMap[toolName]
	g.mu.RUnlock()
	
	if !exists {
		g.writeJSONRPCError(w, request["id"], -32602, "Unknown tool: "+toolName)
		return
	}
	
	backend := g.GetBackend(ref)
	if backend == nil {
		g.writeJSONRPCError(w, request["id"], -32603, "Backend not available")
		return
	}
	
	arguments, _ := params["arguments"].(map[string]interface{})
	result, err := backend.CallTool(toolName, arguments)
	if err != nil {
		g.writeJSONRPCError(w, request["id"], -32603, err.Error())
		return
	}
	
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request["id"],
		"result":  result,
	}
	
	g.writeJSONResponse(w, response)
}

// handleResourcesList aggregates resources from all backends
func (g *Gateway) handleResourcesList(w http.ResponseWriter, request map[string]interface{}) {
	cacheKey := "resources/list"
	if cached := g.cache.Get(cacheKey); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}
	
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	allResources := []Resource{}
	
	for _, backendConn := range g.backends {
		if resources, err := backendConn.ListResources(); err == nil {
			allResources = append(allResources, resources...)
		}
	}
	
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request["id"],
		"result": map[string]interface{}{
			"resources": allResources,
		},
	}
	
	responseData, _ := json.Marshal(response)
	g.cache.Set(cacheKey, responseData, 5*time.Minute)
	
	g.writeJSONResponse(w, response)
}

// handleResourcesRead routes resource reads to the appropriate backend
func (g *Gateway) handleResourcesRead(w http.ResponseWriter, request map[string]interface{}) {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		g.writeJSONRPCError(w, request["id"], -32602, "Invalid params")
		return
	}
	
	uri, ok := params["uri"].(string)
	if !ok {
		g.writeJSONRPCError(w, request["id"], -32602, "Missing resource URI")
		return
	}
	
	g.mu.RLock()
	ref := g.findBackendForResource(uri)
	g.mu.RUnlock()
	
	if ref == (BackendRef{}) {
		g.writeJSONRPCError(w, request["id"], -32602, "No backend for resource: "+uri)
		return
	}
	
	backend := g.GetBackend(ref)
	if backend == nil {
		g.writeJSONRPCError(w, request["id"], -32603, "Backend not available")
		return
	}
	
	result, err := backend.ReadResource(uri)
	if err != nil {
		g.writeJSONRPCError(w, request["id"], -32603, err.Error())
		return
	}
	
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request["id"],
		"result":  result,
	}
	
	g.writeJSONResponse(w, response)
}

// handlePromptsList aggregates prompts from all backends
func (g *Gateway) handlePromptsList(w http.ResponseWriter, request map[string]interface{}) {
	cacheKey := "prompts/list"
	if cached := g.cache.Get(cacheKey); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cached)
		return
	}
	
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	allPrompts := []Prompt{}
	
	for _, backendConn := range g.backends {
		if prompts, err := backendConn.ListPrompts(); err == nil {
			allPrompts = append(allPrompts, prompts...)
		}
	}
	
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request["id"],
		"result": map[string]interface{}{
			"prompts": allPrompts,
		},
	}
	
	responseData, _ := json.Marshal(response)
	g.cache.Set(cacheKey, responseData, 5*time.Minute)
	
	g.writeJSONResponse(w, response)
}

// handlePromptsGet routes prompt gets to the appropriate backend
func (g *Gateway) handlePromptsGet(w http.ResponseWriter, request map[string]interface{}) {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		g.writeJSONRPCError(w, request["id"], -32602, "Invalid params")
		return
	}
	
	promptName, ok := params["name"].(string)
	if !ok {
		g.writeJSONRPCError(w, request["id"], -32602, "Missing prompt name")
		return
	}
	
	g.mu.RLock()
	ref, exists := g.promptsMap[promptName]
	g.mu.RUnlock()
	
	if !exists {
		g.writeJSONRPCError(w, request["id"], -32602, "Unknown prompt: "+promptName)
		return
	}
	
	backend := g.GetBackend(ref)
	if backend == nil {
		g.writeJSONRPCError(w, request["id"], -32603, "Backend not available")
		return
	}
	
	arguments, _ := params["arguments"].(map[string]interface{})
	result, err := backend.GetPrompt(promptName, arguments)
	if err != nil {
		g.writeJSONRPCError(w, request["id"], -32603, err.Error())
		return
	}
	
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request["id"],
		"result":  result,
	}
	
	g.writeJSONResponse(w, response)
}

// findBackendForResource finds the backend that can handle a given resource URI
func (g *Gateway) findBackendForResource(uri string) BackendRef {
	// Exact match first
	if ref, exists := g.resourcesMap[uri]; exists {
		return ref
	}
	
	// Pattern matching (simple prefix matching for now)
	for pattern, ref := range g.resourcesMap {
		if strings.HasPrefix(uri, strings.TrimSuffix(pattern, "*")) {
			return ref
		}
	}
	
	return BackendRef{}
}

// writeJSONResponse writes a JSON response
func (g *Gateway) writeJSONResponse(w http.ResponseWriter, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// writeJSONRPCError writes a JSON-RPC error response
func (g *Gateway) writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors are still HTTP 200
	json.NewEncoder(w).Encode(response)
}

// NewCache creates a new cache instance
func NewCache() *Cache {
	return &Cache{
		data: make(map[string]CacheEntry),
	}
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.data[key]
	if !exists || time.Now().After(entry.ExpiresAt) {
		return nil
	}
	
	return entry.Value
}

// Set stores a value in the cache
func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data[key] = CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Close closes the gateway and all backend connections
func (g *Gateway) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	for name, backend := range g.backends {
		if err := backend.Close(); err != nil {
			log.Printf("Error closing backend %s: %v", name, err)
		}
	}
	
	return nil
}