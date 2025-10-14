#!/bin/bash
# Test script for MCP server mode

# Test initialize request
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}' | ./mcp-manager serve --config test-config.yaml

# Wait a moment
sleep 1

# Test tools/list request
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./mcp-manager serve --config test-config.yaml

# Wait a moment
sleep 1

# Test tools/call - list_servers
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_servers","arguments":{}}}' | ./mcp-manager serve --config test-config.yaml
