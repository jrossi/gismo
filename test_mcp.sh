#!/bin/bash

# Test script for gismo-mcp MCP server

# Build the binary first
echo "Building gismo-mcp..."
go build ./cmd/gismo-mcp

# Test 1: Initialize request
echo "Testing MCP initialize..."
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"clientInfo":{"name":"test","version":"1.0"}}}' | ./gismo-mcp 2>/dev/null | head -1

echo ""

# Test 2: List tools
echo "Testing tools/list..."
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./gismo-mcp 2>/dev/null | head -2 | tail -1

echo ""

# Test 3: Call a tool (example: get_symbols_overview)
echo "Testing tools/call with get_symbols_overview..."
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_symbols_overview","arguments":{"file_path":"cmd/gismo-mcp/main.go","max_depth":2}}}' | ./gismo-mcp 2>/dev/null | head -3 | tail -1

echo ""
echo "MCP server test complete!"