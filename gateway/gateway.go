package gateway

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/takutakahashi/awesome-mcp-proxy/config"
)

// Gateway represents the MCP Gateway server
type Gateway struct {
	config             *config.Config
	backendManager     *BackendManager
	capabilityDiscover *CapabilityDiscoverer
	metaToolHandler    *MetaToolHandler
	routingTable       *RoutingTable
	capabilities       GatewayCapabilities
	server             *mcp.Server
}

// NewGateway creates a new Gateway instance
func NewGateway(cfg *config.Config) (*Gateway, error) {
	// Create backend manager
	backendManager := NewBackendManager()

	// Initialize backends from config
	for _, group := range cfg.Groups {
		for _, backendCfg := range group.Backends {
			var backend Backend

			switch backendCfg.Transport {
			case "http":
				backend = NewHTTPBackend(backendCfg, group.Name)
			case "stdio":
				backend = NewStdioBackend(backendCfg, group.Name)
			default:
				return nil, fmt.Errorf("unsupported transport type: %s", backendCfg.Transport)
			}

			backendManager.AddBackend(backend)
			log.Printf("Added %s backend: %s (group: %s)", backendCfg.Transport, backendCfg.Name, group.Name)
		}
	}

	// Create capability discoverer
	capabilityDiscover := NewCapabilityDiscoverer(backendManager)

	// Create gateway
	gateway := &Gateway{
		config:             cfg,
		backendManager:     backendManager,
		capabilityDiscover: capabilityDiscover,
		routingTable:       capabilityDiscover.GetRoutingTable(),
	}

	// Create meta-tool handler
	gateway.metaToolHandler = NewMetaToolHandler(backendManager, gateway.routingTable)

	return gateway, nil
}

// Initialize initializes the gateway and discovers backend capabilities
func (g *Gateway) Initialize(ctx context.Context) error {
	log.Println("Initializing MCP Gateway...")

	// Discover capabilities from all backends
	capabilities, err := g.capabilityDiscover.DiscoverCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover capabilities: %w", err)
	}

	g.capabilities = capabilities
	log.Printf("Gateway capabilities: tools=%t, resources=%t, prompts=%t",
		capabilities.Tools, capabilities.Resources, capabilities.Prompts)

	// Create MCP server
	g.server = mcp.NewServer(
		&mcp.Implementation{
			Name:    "mcp-gateway",
			Version: "1.0.0",
		},
		nil,
	)

	// Register meta-tools if tools capability is enabled
	if capabilities.Tools {
		g.registerMetaTools()
	}

	// Register resource and prompt handlers
	// TODO: Implement dynamic resource and prompt aggregation
	// For now, focus on meta-tools functionality
	// if capabilities.Resources {
	// 	g.registerResourceHandlers()
	// }

	// if capabilities.Prompts {
	// 	g.registerPromptHandlers()
	// }

	return nil
}

// registerMetaTools registers the three meta-tools
func (g *Gateway) registerMetaTools() {
	// Register list_tools meta-tool
	listToolsTool := &mcp.Tool{
		Name:        "list_tools",
		Description: "バックエンドから利用可能なツールの名前一覧を取得",
	}
	mcp.AddTool(g.server, listToolsTool, g.metaToolHandler.HandleListTools)

	// Register describe_tool meta-tool
	describeToolTool := &mcp.Tool{
		Name:        "describe_tool",
		Description: "指定したツールの詳細情報（説明、引数仕様）を取得",
	}
	mcp.AddTool(g.server, describeToolTool, g.metaToolHandler.HandleDescribeTool)

	// Register call_tool meta-tool
	callToolTool := &mcp.Tool{
		Name:        "call_tool",
		Description: "実際のツール実行を行う",
	}
	mcp.AddTool(g.server, callToolTool, g.metaToolHandler.HandleCallTool)

	log.Println("Registered meta-tools: list_tools, describe_tool, call_tool")
}

// TODO: Implement resource and prompt handlers in the future
// registerResourceHandlers and registerPromptHandlers will be added
// when dynamic resource and prompt aggregation is fully implemented

// handleResourcesList aggregates resources from all backends (currently unused)
// func (g *Gateway) handleResourcesList(ctx context.Context, request *mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
//	var allResources []mcp.Resource
//
//	backends := g.backendManager.GetHealthyBackends()
//	for _, backend := range backends {
//		backendInfo := backend.GetInfo()
//
//		// Check if backend supports resources
//		response, err := backend.SendRequest(ctx, "resources/list", struct{}{})
//		if err != nil {
//			log.Printf("Failed to get resources from backend %s: %v", backendInfo.Name, err)
//			continue
//		}
//
//		var resourcesResponse struct {
//			Resources []mcp.Resource `json:"resources"`
//		}
//
//		if err := json.Unmarshal(*response, &resourcesResponse); err != nil {
//			log.Printf("Failed to unmarshal resources from backend %s: %v", backendInfo.Name, err)
//			continue
//		}
//
//		allResources = append(allResources, resourcesResponse.Resources...)
//	}
//
//	return &mcp.ListResourcesResult{
//		Resources: allResources,
//	}, nil
//}

// TODO: Implement handleResourceRead and handlePromptGet when resource/prompt support is added

// GetServer returns the underlying MCP server
func (g *Gateway) GetServer() *mcp.Server {
	return g.server
}

// GetCapabilities returns the gateway capabilities
func (g *Gateway) GetCapabilities() GatewayCapabilities {
	return g.capabilities
}

// Close closes the gateway and all backends
func (g *Gateway) Close() error {
	log.Println("Closing MCP Gateway...")
	return g.backendManager.Close()
}

// GetBackendManager returns the backend manager (for testing)
func (g *Gateway) GetBackendManager() *BackendManager {
	return g.backendManager
}

// GetRoutingTable returns the routing table (for testing)
func (g *Gateway) GetRoutingTable() *RoutingTable {
	return g.routingTable
}
