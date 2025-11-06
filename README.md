# awesome-mcp-proxy

Remote MCP Server proxy by a single endpoint with authn/authz

## Overview

This is a boilerplate implementation of an MCP (Model Context Protocol) server using Go and the official MCP SDK with HTTP/SSE transport support.

## Features

- **HTTP/SSE Transport**: Server-Sent Events based transport for real-time communication
- **Example Tools**: Pre-configured example tools (echo, add)
- **Example Resources**: Static resources with different content types
- **Example Prompts**: Template prompts with arguments
- **Health Check**: Built-in health check endpoint
- **CORS Support**: Cross-Origin Resource Sharing enabled
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
# Run with default port (8080)
./awesome-mcp-proxy

# Run with custom port
./awesome-mcp-proxy -port 3000
```

### Development Mode

```bash
# Run without building
go run main.go
```

## API Endpoints

### MCP Protocol Endpoints

- **POST /sse** - Server-Sent Events endpoint for MCP protocol
- **POST /message** - Message endpoint for sending requests to the server

### Utility Endpoints

- **GET /health** - Health check endpoint

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

## Configuration

See `examples/config.example.yaml` for configuration examples.

## Built With

- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - Official Go implementation of the Model Context Protocol

## License

MIT
