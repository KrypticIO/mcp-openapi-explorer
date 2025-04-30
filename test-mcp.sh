#!/bin/bash

# This script sends test commands to the MCP server

# Initialize the MCP server
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-script","version":"1.0.0"}}}' | ./mcp-openapi-explorer serve

# List all tools
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./mcp-openapi-explorer serve

# Load a sample OpenAPI spec
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"load_api_spec","arguments":{"url":"https://petstore3.swagger.io/api/v3/openapi.json"}}}' | ./mcp-openapi-explorer serve

# List loaded specs
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_api_specs"}}' | ./mcp-openapi-explorer serve

# Call the get_api_info tool
echo '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_api_info","arguments":{"query":"How do I get information about a pet by ID?"}}}' | ./mcp-openapi-explorer serve

# Read the system resource
echo '{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"openapi://system"}}' | ./mcp-openapi-explorer serve 