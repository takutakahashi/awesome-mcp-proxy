# awesome-mcp-proxy

Remote MCP Server proxy by a single endpoint with authn/authz

## Overview

This is a boilerplate implementation of an MCP (Model Context Protocol) server using Go and the **official Model Context Protocol Go SDK** (`github.com/modelcontextprotocol/go-sdk`) with Streamable HTTP transport support.

## Features

- **Streamable HTTP Transport**: Latest MCP protocol version (2025-03-26) with HTTP-based transport
- **Example Tools**: Pre-configured example tools (echo, add)
- **Example Resources**: Static resources with different content types
- **Example Prompts**: Template prompts with arguments
- **Graceful Shutdown**: Proper signal handling for clean server shutdown

## Quick Start

### Prerequisites

- Go 1.21 or later

### Installation

```bash
# Clone the repository
git clone https://github.com/takutakahashi/awesome-mcp-proxy.git
cd awesome-mcp-proxy

# Install dependencies
go mod download

# Build
go build -o awesome-mcp-proxy .
```

### Running the Server

```bash
# Run with default address (:8080)
./awesome-mcp-proxy

# Run with custom address
./awesome-mcp-proxy -addr :3000
```

The server will be accessible at `http://localhost:8080/mcp` (or your specified address).

### Development Mode

```bash
# Run without building
go run main.go
```

## API Endpoints

### MCP Protocol Endpoints

- **POST /mcp** - Main MCP endpoint using Streamable HTTP transport
- **POST /sse** - Server-Sent Events endpoint for session-based communication

The server implements the MCP protocol version 2025-03-26 with Streamable HTTP transport, which provides:
- Bi-directional communication over HTTP
- Support for chunked transfer encoding
- Progressive message delivery
- Better scalability than SSE-based transport

Note: The SSE endpoint is also available for compatibility and testing purposes.

## Example Tools

### Echo Tool

Echoes back the input message.

**Parameters:**
- `message` (string, required): The message to echo back

### Add Tool

Adds two numbers together.

**Parameters:**
- `a` (number, required): First number
- `b` (number, required): Second number

## Example Resources

### Server Info

- **URI**: `info://server`
- **Type**: text/plain
- **Description**: General information about this MCP server

### Health Status

- **URI**: `status://health`
- **Type**: application/json
- **Description**: Current health status of the server

## Example Prompts

### Greeting Prompt

A friendly greeting prompt.

**Arguments:**
- `name` (string, required): The name of the person to greet

## Project Structure

```
.
├── main.go              # Main entry point
├── server/
│   └── server.go        # MCP server implementation
├── examples/
│   └── config.example.yaml  # Example configuration
├── go.mod               # Go module definition
└── README.md            # This file
```

## Testing

### Running E2E Tests

The project includes end-to-end tests that verify the MCP server functionality:

```bash
# Run all E2E tests
./test/e2e.sh
```

The E2E test suite validates:
- Server initialization and capabilities
- Protocol version compatibility
- Error handling and malformed requests

Note: Streamable HTTP transport is stateless. For full session testing, use the SSE endpoint.

## Configuration

### Gateway Mode

awesome-mcp-proxy can act as a gateway to remote MCP servers, allowing you to:
- Connect to multiple remote MCP servers through a single endpoint
- Add authentication and authorization layers
- Implement load balancing, caching, and rate limiting
- Route requests based on tools, resources, or custom rules

#### Quick Start with Gateway Mode

1. **Simple Configuration**

   See `examples/config.gateway-simple.yaml` for a minimal configuration:

   ```yaml
   auth:
     provider: local
     local:
       filePath: path/to/localconfig.yaml

   servers:
     - name: my-remote-mcp
       type: remote
       url: https://your-mcp-server.example.com/mcp
       auth:
         type: bearer
         token: "your-token-here"

   proxy:
     routing:
       - match:
           default: true
         target: my-remote-mcp
   ```

2. **Advanced Configuration**

   See `examples/config.gateway.yaml` for comprehensive examples including:
   - Multiple authentication methods (Bearer, API Key, Basic Auth)
   - Load balancing across multiple servers
   - Circuit breaker and retry policies
   - Health checks and monitoring
   - Request routing based on tool names or resource URIs

#### Configuration Examples

- `examples/config.example.yaml` - Original example with stdio servers
- `examples/config.gateway-simple.yaml` - Simple remote MCP server gateway
- `examples/config.gateway.yaml` - Advanced gateway with all features

#### Supported Remote Server Features

- **Authentication Types:**
  - Bearer Token
  - API Key
  - Basic Authentication
  - Token from environment variables
  - Token from AWS Secrets Manager
  - Token from Kubernetes Secrets

- **Transport Protocols:**
  - HTTP/HTTPS (Streamable HTTP)
  - Server-Sent Events (SSE)

- **Advanced Features:**
  - Load balancing (round-robin, weighted, random)
  - Health checks and automatic failover
  - Request/response caching
  - Rate limiting
  - Circuit breaker pattern
  - Request routing based on tool/resource patterns
  - Monitoring and metrics (Prometheus)
  - Distributed tracing (Jaeger)

## Built With

- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) - Official Go SDK for Model Context Protocol (maintained in collaboration with Google)

## License

MIT
