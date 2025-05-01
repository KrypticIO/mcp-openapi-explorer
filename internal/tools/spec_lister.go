package tools

import (
	"context"
	"fmt"

	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RegisterListAPISpecs registers the list_api_specs tool
func RegisterListAPISpecs(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface) {
	// Add a tool to list loaded API specs
	listSpecsTool := mcp.NewTool(
		"list_api_specs",
		mcp.WithDescription("List all loaded OpenAPI specifications"),
		mcp.WithString("random_string",
			mcp.Description("Dummy parameter for no-parameter tools"),
		),
	)

	// Register the list_api_specs tool
	mcpServer.AddTool(listSpecsTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		specs := handler.GetSpecs()
		if len(specs) == 0 {
			return mcp.NewToolResultText("No API specs have been loaded yet. Use the load_api_spec tool to load an OpenAPI spec."), nil
		}

		var result string
		result = "Loaded API Specifications:\n\n"
		for id, spec := range specs {
			result += fmt.Sprintf("- %s\n", id)
			result += fmt.Sprintf("  Title: %s\n", spec.Spec.Info.Title)
			result += fmt.Sprintf("  Version: %s\n", spec.Spec.Info.Version)
			result += fmt.Sprintf("  URL: %s\n\n", spec.URL)
		}

		return mcp.NewToolResultText(result), nil
	})
}
