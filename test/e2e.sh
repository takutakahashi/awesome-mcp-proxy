#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
SERVER_PORT=8888
SERVER_URL="http://localhost:${SERVER_PORT}/mcp"
SERVER_PID=""
TEST_FAILED=0

# Note: Streamable HTTP transport is stateless, so each request is independent.
# For full session testing, consider using SSE endpoint or implementing session persistence.

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

# Cleanup function
cleanup() {
    if [ -n "$SERVER_PID" ]; then
        log_info "Stopping server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi

    if [ $TEST_FAILED -eq 1 ]; then
        log_error "Tests failed!"
        exit 1
    else
        log_info "All tests passed!"
        exit 0
    fi
}

trap cleanup EXIT INT TERM

# Build the server
log_info "Building server..."
go build -o awesome-mcp-proxy . || {
    log_error "Build failed"
    TEST_FAILED=1
    exit 1
}

# Start the server
log_info "Starting server on port ${SERVER_PORT}..."
./awesome-mcp-proxy -addr ":${SERVER_PORT}" > /tmp/mcp-server.log 2>&1 &
SERVER_PID=$!

# Wait for server to start
log_info "Waiting for server to start..."
sleep 2

# Check if process is still running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    log_error "Server process died"
    cat /tmp/mcp-server.log
    TEST_FAILED=1
    exit 1
fi

log_info "Server started successfully (PID: $SERVER_PID)"

# Helper function to make MCP requests
mcp_request() {
    local method=$1
    local params=$2
    local id=${3:-1}

    local request=$(cat <<EOF
{
    "jsonrpc": "2.0",
    "id": $id,
    "method": "$method",
    "params": $params
}
EOF
)

    curl -s -X POST "$SERVER_URL" \
        -H "Content-Type: application/json" \
        -d "$request"
}

# Helper function to extract result from SSE response
extract_result() {
    grep "^data: " | sed 's/^data: //' | head -1
}

# Test 1: Initialize
log_test "Test 1: Initialize"
response=$(mcp_request "initialize" '{
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
        "name": "test-client",
        "version": "1.0.0"
    }
}' 1 | extract_result)

if echo "$response" | jq -e '.result.serverInfo.name == "awesome-mcp-proxy"' > /dev/null; then
    log_info "✓ Initialize test passed"
else
    log_error "✗ Initialize test failed"
    echo "Response: $response"
    TEST_FAILED=1
fi

# Test 2: Check server capabilities
log_test "Test 2: Check server capabilities"
if echo "$response" | jq -e '.result.capabilities.tools' > /dev/null && \
   echo "$response" | jq -e '.result.capabilities.resources' > /dev/null && \
   echo "$response" | jq -e '.result.capabilities.prompts' > /dev/null; then
    log_info "✓ Server capabilities test passed"
else
    log_error "✗ Server capabilities test failed"
    echo "Response: $response"
    TEST_FAILED=1
fi

# Test 3: Check protocol version
log_test "Test 3: Check protocol version"
if echo "$response" | jq -e '.result.protocolVersion == "2024-11-05"' > /dev/null; then
    log_info "✓ Protocol version test passed"
else
    log_error "✗ Protocol version test failed"
    echo "Response: $response"
    TEST_FAILED=1
fi

log_info "Note: Streamable HTTP is stateless. Full session tests require SSE endpoint."
log_info "Testing basic server responses and error handling..."

# Test 4: Server info verification
log_test "Test 4: Server info verification"
if echo "$response" | jq -e '.result.serverInfo.version == "0.1.0"' > /dev/null; then
    log_info "✓ Server info test passed"
else
    log_error "✗ Server info test failed"
    echo "Response: $response"
    TEST_FAILED=1
fi

# Test 5: Malformed JSON
log_test "Test 5: Malformed JSON (error handling)"
response=$(curl -s -X POST "$SERVER_URL" \
    -H "Content-Type: application/json" \
    -d '{invalid json}' | extract_result)

if echo "$response" | jq -e '.error' > /dev/null 2>&1 || [ -z "$response" ]; then
    log_info "✓ Malformed JSON test passed"
else
    log_error "✗ Malformed JSON test failed"
    echo "Response: $response"
    TEST_FAILED=1
fi

log_info ""
log_info "Note: Gateway functionality has been implemented."
log_info "Configuration: Use config.yaml with gateway definitions"
log_info "Example: test/gateway-config.yaml shows how to configure remote MCP servers"
log_info "Gateway tools will be prefixed according to config (e.g., remote_echo, remote_add)"
log_info ""
log_info "All E2E tests completed"
