# MCP OpenAPI Explorer

A Model Context Protocol (MCP) server that analyzes OpenAPI specifications and provides context about interacting with APIs.

## Features

- Load OpenAPI specifications from various sources (GitHub, local files, HTTP URLs)
- Parse and provide comprehensive context about API endpoints
- Support for the Model Context Protocol (MCP) via stdin/stdout
- Provide intelligent context about API interactions to LLMs
- Built with Cobra CLI for easy command-line usage

## Getting Started

### Prerequisites

- Go 1.24.2 or later
- Docker (optional)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/krypticlabs/mcp-openapi-explorer.git
cd mcp-openapi-explorer
```

2. Install dependencies:
```bash
go mod download
```

3. Build:
```bash
go build
```

## MCP Usage

The MCP server operates via stdin/stdout, which is the preferred approach for integrating with LLMs. This avoids networking complexities and works well with various LLM integrations.

### Start the server

```bash
./mcp-openapi-explorer serve
```

Options:
- `-v, --verbose` - Enable verbose output
- `-d, --specs-dir string` - Directory to store downloaded API specs (default "./specs")

### Interacting with the MCP server

You can interact with the MCP server by sending JSON-RPC messages to its stdin:

#### Initialize the MCP connection

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-script","version":"1.0.0"}}}' | ./mcp-openapi-explorer serve
```

#### List available tools

```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./mcp-openapi-explorer serve
```

#### Load an OpenAPI specification

```bash
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"load_api_spec","arguments":{"url":"https://petstore3.swagger.io/api/v3/openapi.json"}}}' | ./mcp-openapi-explorer serve
```

#### List loaded API specifications

```bash
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_api_specs"}}' | ./mcp-openapi-explorer serve
```

#### Get information about API endpoints

```bash
echo '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_api_info","arguments":{"query":"How do I find pets by status?"}}}' | ./mcp-openapi-explorer serve
```

## Available MCP Tools

The MCP server exposes the following tools:

### `get_api_info`

Get comprehensive information about API endpoints from loaded OpenAPI specifications.

**Parameters:**
- `query` (string, required): Query about API endpoints (e.g. 'How do I create a new user?', 'What endpoints are available for pet management?')

### `load_api_spec`

Load an OpenAPI specification from a URL or file path.

**Parameters:**
- `url` (string, required): URL or file path to the OpenAPI spec (e.g. 'https://petstore3.swagger.io/api/v3/openapi.json' or 'file:///path/to/spec.json')

### `list_api_specs`

List all loaded OpenAPI specifications.

## MCP Resources

### `openapi://system`

Provides information about the OpenAPI Explorer system, including loaded specifications.

## Integration with LLMs

This MCP server is designed to be integrated with LLMs to provide context about API interactions. The LLM uses the context provided by the server to understand API endpoints and guide users on how to interact with APIs.

Instead of implementing our own search algorithm, we provide the complete API documentation to the LLM, leveraging the LLM's natural language understanding capabilities to match user queries with relevant API endpoints.

## How It Works

1. Users load OpenAPI specifications into the server
2. The server parses and stores these specifications
3. When a user asks about an API endpoint, the server provides comprehensive context about all available endpoints
4. The LLM uses this context to answer user queries accurately

## Example Usage Script

You can use the included `test-mcp.sh` script to interact with the MCP server:

```bash
chmod +x test-mcp.sh
./test-mcp.sh
```

## Docker

```bash
docker build -t mcp-openapi-explorer .
docker run -i mcp-openapi-explorer serve < your-jsonrpc-request.json
```

## License

MIT 