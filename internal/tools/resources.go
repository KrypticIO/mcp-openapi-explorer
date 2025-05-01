package tools

import (
	"context"
	"fmt"

	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RegisterAllResources registers all resources with the MCP server
func RegisterAllResources(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface) {
	RegisterSystemResource(mcpServer, handler, logger)
}

// RegisterSystemResource registers the system information resource
func RegisterSystemResource(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface) {
	// Add a resource that provides system information
	systemResource := mcp.NewResource(
		"openapi://system",
		"System Information",
		mcp.WithResourceDescription("Information about the OpenAPI Explorer system"),
		mcp.WithMIMEType("text/plain"),
	)

	// Register the resource
	mcpServer.AddResource(systemResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		specs := handler.GetSpecs()

		info := "OpenAPI Explorer MCP Server\n\n"
		info += fmt.Sprintf("Specs directory: %s\n", server.SpecsDir)
		info += fmt.Sprintf("Number of loaded specs: %d\n", len(specs))

		if len(specs) > 0 {
			info += "\nLoaded Specifications:\n"
			for id, spec := range specs {
				info += fmt.Sprintf("- %s (Title: %s, Version: %s)\n", id, spec.Spec.Info.Title, spec.Spec.Info.Version)
			}
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "openapi://system",
				MIMEType: "text/plain",
				Text:     info,
			},
		}, nil
	})
}
