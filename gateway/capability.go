package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GatewayCapabilities represents the aggregated capabilities of all backends
type GatewayCapabilities struct {
	Tools     bool `json:"tools,omitempty"`
	Resources bool `json:"resources,omitempty"`
	Prompts   bool `json:"prompts,omitempty"`
}

// RoutingTable manages routing information for tools, resources, and prompts
type RoutingTable struct {
	ToolsMap     map[string]string // tool name -> backend name
	ResourcesMap map[string]string // resource URI pattern -> backend name
	PromptsMap   map[string]string // prompt name -> backend name
	mu           sync.RWMutex
}

// NewRoutingTable creates a new routing table
func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		ToolsMap:     make(map[string]string),
		ResourcesMap: make(map[string]string),
		PromptsMap:   make(map[string]string),
	}
}

// CapabilityDiscoverer handles capability discovery and routing table construction
type CapabilityDiscoverer struct {
	backendManager *BackendManager
	routingTable   *RoutingTable
}

// NewCapabilityDiscoverer creates a new capability discoverer
func NewCapabilityDiscoverer(backendManager *BackendManager) *CapabilityDiscoverer {
	return &CapabilityDiscoverer{
		backendManager: backendManager,
		routingTable:   NewRoutingTable(),
	}
}

// DiscoverCapabilities performs capability discovery on all backends
func (cd *CapabilityDiscoverer) DiscoverCapabilities(ctx context.Context) (GatewayCapabilities, error) {
	capabilities := GatewayCapabilities{}
	backends := cd.backendManager.GetHealthyBackends()

	for _, backend := range backends {
		backendInfo := backend.GetInfo()
		log.Printf("Discovering capabilities for backend: %s", backendInfo.Name)

		// Initialize backend - simplified call
		initReq := struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    map[string]interface{} `json:"capabilities"`
			ClientInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"clientInfo"`
		}{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}{
				Name:    "mcp-gateway",
				Version: "1.0.0",
			},
		}

		initResp, err := backend.Initialize(ctx, initReq)
		if err != nil {
			log.Printf("Backend %s initialization failed: %v", backendInfo.Name, err)
			continue
		}

		// Check and aggregate capabilities
		if initResp.Capabilities != nil {
			if initResp.Capabilities.Tools != nil {
				capabilities.Tools = true
				if err := cd.discoverTools(ctx, backend); err != nil {
					log.Printf("Failed to discover tools for backend %s: %v", backendInfo.Name, err)
				}
			}

			if initResp.Capabilities.Resources != nil {
				capabilities.Resources = true
				if err := cd.discoverResources(ctx, backend); err != nil {
					log.Printf("Failed to discover resources for backend %s: %v", backendInfo.Name, err)
				}
			}

			if initResp.Capabilities.Prompts != nil {
				capabilities.Prompts = true
				if err := cd.discoverPrompts(ctx, backend); err != nil {
					log.Printf("Failed to discover prompts for backend %s: %v", backendInfo.Name, err)
				}
			}
		}
	}

	return capabilities, nil
}

// discoverTools discovers and maps tools from a backend
func (cd *CapabilityDiscoverer) discoverTools(ctx context.Context, backend Backend) error {
	response, err := backend.SendRequest(ctx, "tools/list", struct{}{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	var toolsResponse struct {
		Tools []mcp.Tool `json:"tools"`
	}

	if err := json.Unmarshal(*response, &toolsResponse); err != nil {
		return fmt.Errorf("failed to unmarshal tools response: %w", err)
	}

	backendInfo := backend.GetInfo()
	cd.routingTable.mu.Lock()
	defer cd.routingTable.mu.Unlock()

	for _, tool := range toolsResponse.Tools {
		cd.routingTable.ToolsMap[tool.Name] = backendInfo.Name
		log.Printf("Mapped tool %s to backend %s", tool.Name, backendInfo.Name)
	}

	return nil
}

// discoverResources discovers and maps resources from a backend
func (cd *CapabilityDiscoverer) discoverResources(ctx context.Context, backend Backend) error {
	response, err := backend.SendRequest(ctx, "resources/list", struct{}{})
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	var resourcesResponse struct {
		Resources []mcp.Resource `json:"resources"`
	}

	if err := json.Unmarshal(*response, &resourcesResponse); err != nil {
		return fmt.Errorf("failed to unmarshal resources response: %w", err)
	}

	backendInfo := backend.GetInfo()
	cd.routingTable.mu.Lock()
	defer cd.routingTable.mu.Unlock()

	for _, resource := range resourcesResponse.Resources {
		cd.routingTable.ResourcesMap[resource.URI] = backendInfo.Name
		log.Printf("Mapped resource %s to backend %s", resource.URI, backendInfo.Name)
	}

	return nil
}

// discoverPrompts discovers and maps prompts from a backend
func (cd *CapabilityDiscoverer) discoverPrompts(ctx context.Context, backend Backend) error {
	response, err := backend.SendRequest(ctx, "prompts/list", struct{}{})
	if err != nil {
		return fmt.Errorf("failed to list prompts: %w", err)
	}

	var promptsResponse struct {
		Prompts []mcp.Prompt `json:"prompts"`
	}

	if err := json.Unmarshal(*response, &promptsResponse); err != nil {
		return fmt.Errorf("failed to unmarshal prompts response: %w", err)
	}

	backendInfo := backend.GetInfo()
	cd.routingTable.mu.Lock()
	defer cd.routingTable.mu.Unlock()

	for _, prompt := range promptsResponse.Prompts {
		cd.routingTable.PromptsMap[prompt.Name] = backendInfo.Name
		log.Printf("Mapped prompt %s to backend %s", prompt.Name, backendInfo.Name)
	}

	return nil
}

// GetRoutingTable returns the current routing table
func (cd *CapabilityDiscoverer) GetRoutingTable() *RoutingTable {
	return cd.routingTable
}

// FindToolBackend finds the backend that provides a specific tool
func (rt *RoutingTable) FindToolBackend(toolName string) (string, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	backendName, exists := rt.ToolsMap[toolName]
	return backendName, exists
}

// FindResourceBackend finds the backend that provides a specific resource
func (rt *RoutingTable) FindResourceBackend(resourceURI string) (string, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// Exact match first
	if backendName, exists := rt.ResourcesMap[resourceURI]; exists {
		return backendName, true
	}

	// Pattern matching could be implemented here for more complex URI matching
	// For now, we use exact matching
	return "", false
}

// FindPromptBackend finds the backend that provides a specific prompt
func (rt *RoutingTable) FindPromptBackend(promptName string) (string, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	backendName, exists := rt.PromptsMap[promptName]
	return backendName, exists
}

// GetAllTools returns all available tools from all backends
func (rt *RoutingTable) GetAllTools() []string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	tools := make([]string, 0, len(rt.ToolsMap))
	for tool := range rt.ToolsMap {
		tools = append(tools, tool)
	}
	return tools
}

// GetAllResources returns all available resources from all backends
func (rt *RoutingTable) GetAllResources() []string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	resources := make([]string, 0, len(rt.ResourcesMap))
	for resource := range rt.ResourcesMap {
		resources = append(resources, resource)
	}
	return resources
}

// GetAllPrompts returns all available prompts from all backends
func (rt *RoutingTable) GetAllPrompts() []string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	prompts := make([]string, 0, len(rt.PromptsMap))
	for prompt := range rt.PromptsMap {
		prompts = append(prompts, prompt)
	}
	return prompts
}
