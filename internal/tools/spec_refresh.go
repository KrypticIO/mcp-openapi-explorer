package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RegisterRefreshAPISpec registers the refresh_api_spec tool
func RegisterRefreshAPISpec(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface) {
	// Add a tool to refresh API specs
	refreshSpecTool := mcp.NewTool(
		"refresh_api_spec",
		mcp.WithDescription("Refresh one or more OpenAPI specifications by re-downloading them from their source"),
		mcp.WithString("spec_id",
			mcp.Description("ID of the API spec to refresh (leave empty to refresh all specs)"),
		),
	)

	// Register the refresh_api_spec tool
	mcpServer.AddTool(refreshSpecTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Check if spec_id is provided
		specID, hasSpecID := req.Params.Arguments["spec_id"].(string)

		// If no spec_id is provided, refresh all specs
		if !hasSpecID || specID == "" {
			logger.Infow("Refreshing all API specs")

			specs := handler.GetSpecs()
			// If no specs are loaded, return an error
			if len(specs) == 0 {
				return mcp.NewToolResultText("No API specs are currently loaded."), nil
			}

			// Keep track of refreshed and failed specs
			refreshed := 0
			failed := 0
			var failedSpecs []string

			// Copy the map to avoid concurrent modification during iteration
			specsToRefresh := make(map[string]*server.APISpec)
			for id, spec := range specs {
				specsToRefresh[id] = spec
			}

			// Refresh each spec
			for id, spec := range specsToRefresh {
				logger.Infow("Refreshing spec", "specID", id, "url", spec.URL)

				// Re-download the spec
				newSpec, err := handler.LoadOpenAPISpec(ctx, spec.URL)
				if err != nil {
					logger.Warnw("Failed to refresh spec", "error", err, "specID", id)
					failed++
					failedSpecs = append(failedSpecs, id)
					continue
				}

				// Update the spec
				handler.AddSpec(id, &server.APISpec{
					URL:  spec.URL,
					Spec: newSpec,
				})

				// Save the spec to a file for persistence
				if err := handler.SaveSpec(id, handler.GetSpecs()[id]); err != nil {
					logger.Warnw("Failed to save refreshed spec", "error", err, "specID", id)
				}

				refreshed++
			}

			// Prepare result message
			result := fmt.Sprintf("Refreshed %d API spec(s)", refreshed)
			if failed > 0 {
				result += fmt.Sprintf(", %d failed", failed)
				result += "\nFailed specs: " + strings.Join(failedSpecs, ", ")
			}

			return mcp.NewToolResultText(result), nil
		}

		// Handle refreshing a single spec
		specs := handler.GetSpecs()
		spec, exists := specs[specID]
		if !exists {
			logger.Warnw("Spec not found for refresh", "specID", specID)
			return mcp.NewToolResultError(fmt.Sprintf("Spec not found: %s", specID)), nil
		}

		logger.Infow("Refreshing spec", "specID", specID, "url", spec.URL)

		// Re-download the spec
		newSpec, err := handler.LoadOpenAPISpec(ctx, spec.URL)
		if err != nil {
			logger.Errorw("Failed to refresh spec", "error", err, "specID", specID)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to refresh spec %s: %v", specID, err)), nil
		}

		// Update the spec
		handler.AddSpec(specID, &server.APISpec{
			URL:  spec.URL,
			Spec: newSpec,
		})

		// Save the spec to a file for persistence
		if err := handler.SaveSpec(specID, handler.GetSpecs()[specID]); err != nil {
			logger.Warnw("Failed to save refreshed spec", "error", err, "specID", specID)
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully refreshed API spec: %s (version %s)", newSpec.Info.Title, newSpec.Info.Version)), nil
	})
}
