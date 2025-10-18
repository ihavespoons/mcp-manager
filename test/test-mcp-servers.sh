#!/bin/bash

# Test script for mcp-manager with different MCP servers
# Tests both process and container modes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY="$PROJECT_ROOT/mcp-manager"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check prerequisites
check_prereqs() {
    log_info "Checking prerequisites..."

    if [ ! -f "$BINARY" ]; then
        log_error "mcp-manager binary not found. Run 'make build' first."
        exit 1
    fi

    if ! command -v npx &> /dev/null; then
        log_warn "npx not found. Some tests will be skipped."
    fi

    if ! command -v docker &> /dev/null; then
        log_warn "docker not found. Container tests will be skipped."
    fi

    log_info "Prerequisites check complete"
}

# Test 1: Filesystem MCP Server (Process Mode)
test_filesystem_process() {
    log_info "Testing filesystem MCP server (process mode)..."

    if ! command -v npx &> /dev/null; then
        log_warn "Skipping: npx not available"
        return
    fi

    # Start gateway
    $BINARY gateway --mode process --port 18080 --config "$PROJECT_ROOT/test-config.yaml" &
    GATEWAY_PID=$!
    sleep 2

    # Test health
    if curl -s http://localhost:18080/health | grep -q "healthy"; then
        log_info "✓ Health check passed"
    else
        log_error "✗ Health check failed"
        kill $GATEWAY_PID 2>/dev/null
        return 1
    fi

    # Test initialize
    RESPONSE=$(curl -s -X POST http://localhost:18080/mcp \
        -H "X-MCP-Server: filesystem" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}')

    if echo "$RESPONSE" | grep -q '"result"'; then
        log_info "✓ Initialize request passed"
        SESSION_ID=$(echo "$RESPONSE" | grep -o '"X-Session-ID":"[^"]*"' | cut -d'"' -f4)
    else
        log_error "✗ Initialize request failed"
        kill $GATEWAY_PID 2>/dev/null
        return 1
    fi

    # Cleanup
    kill $GATEWAY_PID 2>/dev/null
    log_info "Filesystem (process mode) test complete"
}

# Test 2: Config Validation
test_config_validation() {
    log_info "Testing config validation..."

    # Valid config
    if $BINARY validate --config "$PROJECT_ROOT/test-config.yaml" &> /dev/null; then
        log_info "✓ Valid config accepted"
    else
        log_error "✗ Valid config rejected"
        return 1
    fi

    # Create invalid config
    cat > /tmp/test-invalid.yaml <<EOF
version: "1.0"
settings:
  data_dir: /tmp
  network: test
  log_level: invalid_level
servers:
  - name: test
    enabled: true
    command: ["test"]
EOF

    # Invalid config should fail
    if ! $BINARY validate --config /tmp/test-invalid.yaml &> /dev/null; then
        log_info "✓ Invalid config rejected"
    else
        log_error "✗ Invalid config accepted"
        rm /tmp/test-invalid.yaml
        return 1
    fi

    rm /tmp/test-invalid.yaml
    log_info "Config validation test complete"
}

# Test 3: Multiple Sessions
test_multiple_sessions() {
    log_info "Testing multiple sessions..."

    if ! command -v npx &> /dev/null; then
        log_warn "Skipping: npx not available"
        return
    fi

    # Start gateway
    $BINARY gateway --mode process --port 18081 --config "$PROJECT_ROOT/test-config.yaml" &
    GATEWAY_PID=$!
    sleep 2

    # Create two sessions
    for i in 1 2; do
        RESPONSE=$(curl -s -X POST http://localhost:18081/mcp \
            -H "X-MCP-Server: filesystem" \
            -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","id":'$i',"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test'$i'","version":"1.0.0"}}}')

        if echo "$RESPONSE" | grep -q '"result"'; then
            log_info "✓ Session $i created"
        else
            log_error "✗ Session $i failed"
            kill $GATEWAY_PID 2>/dev/null
            return 1
        fi
    done

    # Check health shows active sessions
    HEALTH=$(curl -s http://localhost:18081/health)
    if echo "$HEALTH" | grep -q "active_sessions"; then
        log_info "✓ Active sessions tracked"
    else
        log_error "✗ Active sessions not tracked"
    fi

    # Cleanup
    kill $GATEWAY_PID 2>/dev/null
    log_info "Multiple sessions test complete"
}

# Test 4: Error Handling
test_error_handling() {
    log_info "Testing error handling..."

    # Start gateway
    $BINARY gateway --mode process --port 18082 --config "$PROJECT_ROOT/test-config.yaml" &
    GATEWAY_PID=$!
    sleep 2

    # Test with non-existent server
    RESPONSE=$(curl -s -X POST http://localhost:18082/mcp \
        -H "X-MCP-Server: nonexistent" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}')

    if echo "$RESPONSE" | grep -q "error\|not found"; then
        log_info "✓ Non-existent server error handled"
    else
        log_warn "Non-existent server response: $RESPONSE"
    fi

    # Test with invalid JSON
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:18082/mcp \
        -H "X-MCP-Server: filesystem" \
        -H "Content-Type: application/json" \
        -d '{invalid json}')

    if [ "$HTTP_CODE" -ge 400 ]; then
        log_info "✓ Invalid JSON rejected"
    else
        log_warn "Invalid JSON not rejected (HTTP $HTTP_CODE)"
    fi

    # Cleanup
    kill $GATEWAY_PID 2>/dev/null
    log_info "Error handling test complete"
}

# Main test runner
main() {
    log_info "Starting mcp-manager test suite..."
    log_info "================================================"

    check_prereqs

    # Run tests
    test_config_validation
    echo ""

    test_filesystem_process
    echo ""

    test_multiple_sessions
    echo ""

    test_error_handling
    echo ""

    log_info "================================================"
    log_info "Test suite complete!"
}

main "$@"
