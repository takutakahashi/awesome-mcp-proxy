#!/bin/bash

# Manual Gateway Tool Test Script
# This script provides step-by-step instructions for manually testing gateway tools

echo "=== MCP Gateway Tools Manual Test ==="
echo ""
echo "This script helps you manually test the gateway functionality."
echo "You'll need to run commands in different terminal sessions."
echo ""

echo "Step 1: Start a remote MCP server"
echo "  Terminal 1:"
echo "  $ ./awesome-mcp-proxy -addr :9000"
echo ""

echo "Step 2: Start the main MCP proxy server"
echo "  Terminal 2:"
echo "  $ ./awesome-mcp-proxy -addr :8080"
echo ""

echo "Step 3: Use an MCP client to test gateway tools"
echo "  You can use the Claude Desktop app or any MCP client"
echo ""

echo "Example gateway tool calls:"
echo ""

echo "1. List tools from remote server:"
echo '  Tool: gateway-list-tools'
echo '  Arguments: {"remote_url": "http://localhost:9000/mcp"}'
echo ""

echo "2. Describe a specific tool:"
echo '  Tool: gateway-describe-tool'
echo '  Arguments: {"remote_url": "http://localhost:9000/mcp", "tool_name": "echo"}'
echo ""

echo "3. Execute a tool on remote server:"
echo '  Tool: gateway-execute-tool'
echo '  Arguments: {"remote_url": "http://localhost:9000/mcp", "tool_name": "echo", "arguments": {"message": "Hello!"}}'
echo ""

echo "Note: Streamable HTTP transport is stateless. For automated testing,"
echo "consider using an MCP client library or SSE endpoint with session management."
