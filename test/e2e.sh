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

# Helper function for gateway requests
mcp_request_gateway() {
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

    curl -s -X POST "$GATEWAY_URL" \
        -H "Content-Type: application/json" \
        -d "$request" | extract_result
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

log_info "All E2E tests completed"

# Test 6: Config loading functionality
log_test "Test 6: Config loading functionality"
log_info "Building config test tool..."
go build -o config-test ./cmd/config-test/ || {
    log_error "Config test tool build failed"
    TEST_FAILED=1
}

if [ $TEST_FAILED -eq 0 ]; then
    # Test config loading with test config file
    config_output=$(./config-test -config test/test-config.yaml 2>/dev/null)
    
    if echo "$config_output" | jq -e '.Gateway.Port == 8889' > /dev/null && \
       echo "$config_output" | jq -e '.Gateway.Host == "127.0.0.1"' > /dev/null && \
       echo "$config_output" | jq -e '.Groups[0].Name == "test-group"' > /dev/null; then
        log_info "✓ Config loading test passed"
    else
        log_error "✗ Config loading test failed"
        echo "Config output: $config_output"
        TEST_FAILED=1
    fi
fi

# Test 7: Config validation
log_test "Test 7: Config validation"
if [ $TEST_FAILED -eq 0 ]; then
    # Create invalid config for testing validation
    cat > /tmp/invalid-config.yaml << INNER_EOF
gateway:
  port: 99999  # Invalid port
groups:
  - name: ""     # Empty name should fail
    backends:
      test:
        name: "test"
        transport: "stdio"
        # Missing command for stdio transport
INNER_EOF

    # This should fail
    if ./config-test -config /tmp/invalid-config.yaml 2>/dev/null; then
        log_error "✗ Config validation test failed (should have rejected invalid config)"
        TEST_FAILED=1
    else
        log_info "✓ Config validation test passed (correctly rejected invalid config)"
    fi
    
    rm -f /tmp/invalid-config.yaml
fi

# Test 8: Environment variable expansion
log_test "Test 8: Environment variable expansion"
if [ $TEST_FAILED -eq 0 ]; then
    export TEST_TOKEN="secret-123"
    export TEST_ENDPOINT="http://test-server:3000/mcp"
    
    # Create config with env vars
    cat > /tmp/env-config.yaml << INNER_EOF
gateway:
  port: 8890
groups:
  - name: "env-test"
    backends:
      env-backend:
        name: "env-backend"
        transport: "http"
        endpoint: "\${TEST_ENDPOINT}"
        headers:
          authorization: "Bearer \${TEST_TOKEN}"
INNER_EOF

    config_output=$(./config-test -config /tmp/env-config.yaml 2>/dev/null)
    
    if echo "$config_output" | jq -e '.Groups[0].Backends["env-backend"].Endpoint == "http://test-server:3000/mcp"' > /dev/null && \
       echo "$config_output" | jq -e '.Groups[0].Backends["env-backend"].Headers.authorization == "Bearer secret-123"' > /dev/null; then
        log_info "✓ Environment variable expansion test passed"
    else
        log_error "✗ Environment variable expansion test failed"
        echo "Config output: $config_output"
        TEST_FAILED=1
    fi
    
    rm -f /tmp/env-config.yaml
    unset TEST_TOKEN TEST_ENDPOINT
fi

# Test 9: Gateway mode with meta-tools
log_test "Test 9: Gateway mode with meta-tools"
if [ $TEST_FAILED -eq 0 ]; then
    # Stop the main test server first
    if [ -n "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
        SERVER_PID=""
    fi
    
    # Start standalone server on port 8887 (for gateway to connect to)
    log_info "Starting standalone server for gateway test..."
    ./awesome-mcp-proxy -addr ":8887" > /tmp/mcp-standalone.log 2>&1 &
    STANDALONE_PID=$!
    
    # Wait for standalone server to start
    sleep 3
    
    # Check if standalone server is running
    if ! kill -0 $STANDALONE_PID 2>/dev/null; then
        log_error "Standalone server failed to start"
        cat /tmp/mcp-standalone.log
        TEST_FAILED=1
    else
        # Start gateway server
        log_info "Starting gateway server..."
        ./awesome-mcp-proxy -config test/gateway-config.yaml > /tmp/mcp-gateway.log 2>&1 &
        GATEWAY_PID=$!
        
        # Wait for gateway to start
        sleep 3
        
        # Check if gateway is running
        if ! kill -0 $GATEWAY_PID 2>/dev/null; then
            log_error "Gateway server failed to start"
            cat /tmp/mcp-gateway.log
            TEST_FAILED=1
        else
            GATEWAY_URL="http://localhost:8889/mcp"
            
            # Test gateway initialize
            log_test "Test 9a: Gateway initialize"
            gateway_response=$(mcp_request_gateway "initialize" '{
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {
                    "name": "test-client",
                    "version": "1.0.0"
                }
            }' 1)
            
            if echo "$gateway_response" | jq -e '.result.serverInfo.name == "mcp-gateway"' > /dev/null; then
                log_info "✓ Gateway initialize test passed"
            else
                log_error "✗ Gateway initialize test failed"
                echo "Response: $gateway_response"
                TEST_FAILED=1
            fi
            
            # Test meta-tools capability
            log_test "Test 9b: Gateway capabilities check"
            if echo "$gateway_response" | jq -e '.result.capabilities.tools' > /dev/null; then
                log_info "✓ Gateway tools capability test passed"
            else
                log_error "✗ Gateway tools capability test failed"
                echo "Response: $gateway_response"
                TEST_FAILED=1
            fi
            
            # Test list_tools meta-tool
            log_test "Test 9c: list_tools meta-tool"
            list_tools_response=$(mcp_request_gateway "tools/call" '{
                "name": "list_tools",
                "arguments": {}
            }' 2)
            
            if echo "$list_tools_response" | jq -e '.result' > /dev/null; then
                log_info "✓ list_tools meta-tool test passed"
            else
                log_error "✗ list_tools meta-tool test failed"
                echo "Response: $list_tools_response"
                TEST_FAILED=1
            fi
            
            # Test describe_tool meta-tool
            log_test "Test 9d: describe_tool meta-tool"
            describe_response=$(mcp_request_gateway "tools/call" '{
                "name": "describe_tool",
                "arguments": {"tool_name": "echo"}
            }' 3)
            
            if echo "$describe_response" | jq -e '.result' > /dev/null; then
                log_info "✓ describe_tool meta-tool test passed"
            else
                log_error "✗ describe_tool meta-tool test failed"  
                echo "Response: $describe_response"
                TEST_FAILED=1
            fi
            
            # Test call_tool meta-tool
            log_test "Test 9e: call_tool meta-tool"
            call_tool_response=$(mcp_request_gateway "tools/call" '{
                "name": "call_tool",
                "arguments": {
                    "tool_name": "echo",
                    "arguments": {"message": "Hello from gateway!"}
                }
            }' 4)
            
            if echo "$call_tool_response" | jq -e '.result' > /dev/null; then
                log_info "✓ call_tool meta-tool test passed"
            else
                log_error "✗ call_tool meta-tool test failed"
                echo "Response: $call_tool_response"
                TEST_FAILED=1
            fi
            
            # Test direct tool call prohibition
            log_test "Test 9f: Direct tool call prohibition"
            direct_call_response=$(mcp_request_gateway "tools/call" '{
                "name": "echo",
                "arguments": {"message": "This should fail"}
            }' 5)
            
            if echo "$direct_call_response" | jq -e '.error' > /dev/null; then
                log_info "✓ Direct tool call prohibition test passed"
            else
                log_error "✗ Direct tool call prohibition test failed"
                echo "Response: $direct_call_response"
                TEST_FAILED=1
            fi
        fi
        
        # Cleanup gateway
        if [ -n "$GATEWAY_PID" ]; then
            kill $GATEWAY_PID 2>/dev/null || true
            wait $GATEWAY_PID 2>/dev/null || true
        fi
    fi
    
    # Cleanup standalone server
    if [ -n "$STANDALONE_PID" ]; then
        kill $STANDALONE_PID 2>/dev/null || true
        wait $STANDALONE_PID 2>/dev/null || true
    fi
fi

# Cleanup test binaries
rm -f config-test
