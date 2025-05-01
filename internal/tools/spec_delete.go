package tools

import (
	"context"
	"fmt"

	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RegisterDeleteAPISpec registers the delete_api_spec tool
func RegisterDeleteAPISpec(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface, config server.ConfigInterface) {
	// Add a tool to delete an API spec
	deleteSpecTool := mcp.NewTool(
		"delete_api_spec",
		mcp.WithDescription("Delete a loaded OpenAPI specification"),
		mcp.WithString("spec_id",
			mcp.Required(),
			mcp.Description("ID of the API spec to delete (use list_api_specs to see available specs)"),
		),
	)

	// Register the delete_api_spec tool
	mcpServer.AddTool(deleteSpecTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		specID, ok := req.Params.Arguments["spec_id"].(string)
		if !ok {
			logger.Errorw("Invalid spec_id parameter", "params", req.Params.Arguments)
			return mcp.NewToolResultError("spec_id parameter must be a string"), nil
		}

		// Check if the spec exists before trying to delete it
		specs := handler.GetSpecs()
		spec, exists := specs[specID]
		if !exists {
			logger.Warnw("Spec not found", "specID", specID)
			return mcp.NewToolResultError(fmt.Sprintf("Spec not found: %s", specID)), nil
		}

		// Use a defer/recover to catch any potential panics
		defer func() {
			if r := recover(); r != nil {
				logger.Errorw("Panic in delete_api_spec", "error", r)
			}
		}()

		// Delete the spec
		err := handler.DeleteSpec(specID)
		if err != nil {
			logger.Errorw("Failed to delete spec", "error", err, "specID", specID)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete spec: %v", err)), nil
		}

		// Update configuration by removing the spec if config supports it
		if config.ShouldUpdateConfig() {
			specURL := spec.URL

			// Remove spec from configuration
			if config.HasSpec(specURL) {
				config.RemoveSpec(specURL)

				// Save the updated configuration
				if err := config.Save(); err != nil {
					logger.Warnw("Failed to update config file after removing spec", "error", err)
				} else {
					logger.Infow("Updated config file after removing spec", "url", specURL)
				}
			}
		}

		logger.Infow("Successfully deleted spec", "specID", specID)
		return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted API spec: %s", specID)), nil
	})
}
