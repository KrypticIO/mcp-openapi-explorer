package tools

import (
	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers all tools with the MCP server
func RegisterAll(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface, config server.ConfigInterface) {
	RegisterGetAPIInfo(mcpServer, handler, logger)
	RegisterLoadAPISpec(mcpServer, handler, logger, config)
	RegisterListAPISpecs(mcpServer, handler, logger)
	RegisterDeleteAPISpec(mcpServer, handler, logger, config)
	RegisterRefreshAPISpec(mcpServer, handler, logger)
}

// RegisterEverything registers all tools and resources
func RegisterEverything(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface, config server.ConfigInterface) {
	RegisterAll(mcpServer, handler, logger, config)
	RegisterAllResources(mcpServer, handler, logger)
}
